package server

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/pkg/errors"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/covid-tracking/storage"
	"go.bryk.io/covid-tracking/utils"
	"go.bryk.io/x/auth"
	"go.bryk.io/x/ccg/did"
	"go.bryk.io/x/jwx"
	"go.bryk.io/x/net/rpc"
	"go.bryk.io/x/pki"
	"golang.org/x/crypto/blake2b"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RBAC access policy for the API server.
var accessPolicy = `
# Users can:
# - Renew credentials
# - Register location records
r, user, /credentials, renew
r, user, /record, create

# Agents can:
# - Renew credentials
# - Register location records
# - Create notifications
r, agent, /credentials, renew
r, agent, /record, create
r, agent, /notification, create

# Admins are treated as super users
r, admin, .*, .*
`

// Support user roles on the platform.
var supportedRoles = []string{
	"user",
	"agent",
	"admin",
}

// Custom claims included in access credentials.
type credentialsData struct {
	DID  string `json:"did"`
	Role string `json:"role"`
}

// Common error codes
var (
	errUnauthorized    = status.Error(codes.PermissionDenied, "unauthorized request")
	errUnauthenticated = status.Error(codes.Unauthenticated, "invalid credentials")
	errInvalidRequest  = status.Error(codes.InvalidArgument, "invalid request argument")
)

// Handler instances provide all the functionality related to the
// Tracking server platform component.
type Handler struct {
	name      string
	enf       *auth.Enforcer
	tls       *rpc.ServerTLSConfig
	gw        *rpc.HTTPGateway
	ca        *pki.CA
	tg        *jwx.Generator
	hk        []byte
	store     *storage.Handler
	providers []*did.Provider
}

// NewHandler returns a new service handler instance.
func NewHandler(name string, home string, store string, port int, providers []*did.Provider) (*Handler, error) {
	var err error
	h := &Handler{
		name:      name,
		providers: providers,
	}

	// Authorization enforcer
	h.enf, err = setupAuthEnforcer()
	if err != nil {
		return nil, err
	}

	// Verify credentials
	if err = verifyRootCA(home); err != nil {
		return nil, err
	}

	// Load TLS settings
	h.tls, err = verifyTLSCertificate(home)
	if err != nil {
		return nil, err
	}

	// Setup HTTP gateway
	h.gw, err = setupHTTPGateway(port)
	if err != nil {
		return nil, err
	}

	// Setup PKI
	h.ca, err = setupPKI(home)
	if err != nil {
		return nil, err
	}

	// Get hash key
	h.hk, err = hashKey(home)
	if err != nil {
		return nil, err
	}

	// Setup token generator
	h.tg, err = setupTokenGenerator(name, home)
	if err != nil {
		return nil, err
	}

	// Get storage handler
	h.store, err = storage.NewHandler(store)
	if err != nil {
		return nil, err
	}

	// All good!
	return h, nil
}

// Close properly finish handler components and execution.
func (sh *Handler) Close() {
	sh.store.Close()
}

// GetServiceDefinition allows to expose the handler instance through an RPC server.
func (sh *Handler) GetServiceDefinition() *rpc.Service {
	return &rpc.Service{
		GatewaySetup: protov1.RegisterTrackingServerAPIHandlerFromEndpoint,
		ServerSetup: func(server *grpc.Server) {
			protov1.RegisterTrackingServerAPIServer(server, &remoteInterface{sh: sh})
		},
	}
}

// TLSConfig return the TLS settings to setup secure communications with the handler
// instance when exposed as an RPC server.
func (sh *Handler) TLSConfig() rpc.ServerTLSConfig {
	return *sh.tls
}

// HTTPGateway allow HTTPS access to the handler instance.
func (sh *Handler) HTTPGateway() *rpc.HTTPGateway {
	return sh.gw
}

// ActivationCode returns a new activation code for the provided request.
func (sh *Handler) ActivationCode(req *protov1.ActivationCodeRequest) (string, error) {
	if _, err := did.Parse(req.Did); err != nil {
		return "", errInvalidRequest
	}
	return sh.store.ActivationCode(req)
}

// AccessToken process an incoming credentials request.
func (sh *Handler) AccessToken(req *protov1.CredentialsRequest,
	validateCode bool) (*protov1.CredentialsResponse, error) {
	// Retrieve DID instance
	identifier, err := utils.ResolveDID(req.Did, sh.providers)
	if err != nil {
		return nil, errors.Wrap(err, "resolve DID")
	}

	// Verify registration proof
	if err := utils.VerifySignature(identifier, []byte(req.ActivationCode), req.Proof); err != nil {
		return nil, errors.Wrap(err, "invalid signature")
	}

	// Validate activation code
	if validateCode {
		if !sh.store.VerifyActivationCode(req) {
			return nil, errInvalidRequest
		}
	}

	// Request is valid, return credentials result.
	return sh.getToken(req.Did, req.Role)
}

// RenewToken will refresh a valid but expired access token.
func (sh *Handler) RenewToken(token *jwx.Token, refreshCode string) (*protov1.CredentialsResponse, error) {
	// Validate refresh code
	cc := sh.getRefreshCode(token.String())
	if cc == "" || cc != refreshCode {
		return nil, errInvalidRequest
	}

	// Create new token using claims present in the expired version.
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return nil, errUnauthenticated
	}
	return sh.getToken(data.DID, data.Role)
}

// LocationRecord receive and process incoming location update events.
// nolint: interfacer
func (sh *Handler) LocationRecord(token *jwx.Token, req *protov1.RecordRequest) (*protov1.RecordResponse, error) {
	// Maximum of 100 records per-request
	if len(req.Records) > 100 {
		return nil, errInvalidRequest
	}

	// Resolve DID document for the credential's subject
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return nil, errUnauthenticated
	}
	id, err := utils.ResolveDID(data.DID, sh.providers)
	if err != nil {
		return nil, errInvalidRequest
	}

	// Validate records
	var records []*protov1.LocationRecord
	for _, r := range req.Records {
		if validateRecord(id, r) {
			records = append(records, r)
		}
	}

	// Store valid records and return final result
	if err := sh.store.LocationRecords(records); err != nil {
		return &protov1.RecordResponse{Ok: false}, err
	}
	return &protov1.RecordResponse{Ok: true}, nil
}

// Generate bearer token and refresh code.
func (sh *Handler) getToken(id, role string) (*protov1.CredentialsResponse, error) {
	// Get access token
	params := &jwx.TokenParameters{
		Audience:   []string{sh.name},
		Subject:    id,
		Method:     jwx.ES384,
		NotBefore:  "0ms",
		Expiration: "168h", // 1 week by default
		CustomPayloadClaims: &credentialsData{
			DID:  id,
			Role: role,
		},
	}
	token, err := sh.tg.NewToken("master", params)
	if err != nil {
		return nil, err
	}

	// Return result
	return &protov1.CredentialsResponse{
		AccessToken: token.String(),
		RefreshCode: sh.getRefreshCode(token.String()),
	}, nil
}

// Refresh codes are base64-encoded authenticated hashes for generated credentials.
func (sh *Handler) getRefreshCode(seed string) string {
	h, err := blake2b.New256(sh.hk)
	if err != nil {
		return ""
	}
	defer h.Reset()
	_, err = h.Write([]byte(seed))
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Handle authentication for requests that require it. Authentication is based on
// "bearer" JWT credentials.
func (sh *Handler) authenticate(ctx context.Context, checkExpiration bool) (*jwx.Token, error) {
	// Retrieve credentials
	token, err := getTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate token
	now := time.Now()
	checks := []jwx.ValidatorFunc{
		jwx.IssuerValidator(sh.name),
		jwx.AudienceValidator([]string{sh.name}),
		jwx.NotBeforeValidator(now),
		jwx.IssuedAtValidator(now),
	}
	if checkExpiration {
		checks = append(checks, jwx.ExpirationTimeValidator(now, true))
	}
	if err := token.Validate(checks...); err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return token, nil
}

// Handle authorization requests based on the platform's access policy.
// nolint: interfacer
func (sh *Handler) authorize(token *jwx.Token, resource string, action string) bool {
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return false
	}
	return sh.enf.Evaluate(auth.Request{
		Subject:  data.Role,
		Resource: resource,
		Action:   action,
	})
}
