package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/covid-tracking/storage"
	"go.bryk.io/covid-tracking/utils"
	"go.bryk.io/x/amqp"
	"go.bryk.io/x/auth"
	"go.bryk.io/x/ccg/did"
	"go.bryk.io/x/jwx"
	xlog "go.bryk.io/x/log"
	"go.bryk.io/x/net/rpc"
	"go.bryk.io/x/pki"
	"golang.org/x/crypto/blake2b"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common error codes
var (
	errUnauthorized    = status.Error(codes.PermissionDenied, "unauthorized request")
	errUnauthenticated = status.Error(codes.Unauthenticated, "invalid credentials")
	errInvalidRequest  = status.Error(codes.InvalidArgument, "invalid request argument")
	errInternalError   = status.Error(codes.Internal, "internal error")
	errFailedToPublish = status.Error(codes.Unavailable, "failed to publish message")
)

// ServerOptions provide the configuration settings available/required
// when creating a new API server instance.
type ServerOptions struct {
	// API server identifier. Will be used as the "issuer" value on
	// access credentials.
	Name string

	// Work directory for the instance. Will be used to store and locate
	// required assets like TLS certificate, etc.
	Home string

	// Storage mechanism connection string. If not supported by the API server,
	// an error will be returned.
	Store string

	// Message broker connection string. Used by the API server to publish
	// tasks and notifications.
	Broker string

	// Supported DID methods.
	Providers []*did.Provider

	// To handle output.
	Logger xlog.Logger
}

// Server instances provide all the functionality for API server on the
// contact tracing platform.
type Server struct {
	name      string
	ctx       context.Context
	halt      context.CancelFunc
	pub       *amqp.Publisher
	enf       *auth.Enforcer
	tls       *rpc.ServerTLSConfig
	log       xlog.Logger
	gw        *rpc.HTTPGateway
	ca        *pki.CA
	tg        *jwx.Generator
	hk        []byte
	store     *storage.Handler
	providers []*did.Provider
}

// NewServer returns a new service handler instance.
func NewServer(opts *ServerOptions) (*Server, error) {
	var err error
	srv := &Server{
		name:      opts.Name,
		providers: opts.Providers,
		log:       opts.Logger,
	}

	// Authorization enforcer
	srv.enf, err = setupAuthEnforcer()
	if err != nil {
		return nil, err
	}

	// Verify credentials
	if err = verifyRootCA(opts.Home); err != nil {
		return nil, err
	}

	// Load TLS settings
	srv.tls, err = verifyTLSCertificate(opts.Home)
	if err != nil {
		return nil, err
	}

	// Setup PKI
	srv.ca, err = setupPKI(opts.Home)
	if err != nil {
		return nil, err
	}

	// Get hash key
	srv.hk, err = hashKey(opts.Home)
	if err != nil {
		return nil, err
	}

	// Setup token generator
	srv.tg, err = setupTokenGenerator(opts.Name, opts.Home)
	if err != nil {
		return nil, err
	}

	// Get storage handler
	srv.store, err = storage.NewHandler(opts.Store)
	if err != nil {
		return nil, err
	}

	// Setup message publisher
	srv.pub, err = amqp.NewPublisher(opts.Broker, []amqp.Option{
		amqp.WithTopology(utils.BrokerTopology()),
		amqp.WithLogger(srv.log.Sub(xlog.Fields{
			"component": "amqp",
		})),
	}...)
	if err != nil {
		return nil, err
	}

	// All good!
	srv.ctx, srv.halt = context.WithCancel(context.Background())
	go srv.eventLoop()
	return srv, nil
}

// Close properly finish handler components and execution.
func (srv *Server) Close() {
	srv.halt()
	<-srv.ctx.Done()
	_ = srv.pub.Close()
	srv.store.Close()
}

// GetServiceDefinition allows to expose the handler instance through an RPC server.
func (srv *Server) GetServiceDefinition() *rpc.Service {
	return &rpc.Service{
		GatewaySetup: protov1.RegisterTrackingServerAPIHandlerFromEndpoint,
		ServerSetup: func(server *grpc.Server) {
			protov1.RegisterTrackingServerAPIServer(server, &remoteInterface{srv: srv})
		},
	}
}

// TLSConfig return the TLS settings to setup secure communications with the handler
// instance when exposed as an RPC server.
func (srv *Server) TLSConfig() rpc.ServerTLSConfig {
	return *srv.tls
}

// HTTPGateway allow HTTPS access to the handler instance.
func (srv *Server) HTTPGateway(port int) (*rpc.HTTPGateway, error) {
	if srv.gw == nil {
		var err error
		srv.gw, err = setupHTTPGateway(port)
		if err != nil {
			return nil, err
		}
	}
	return srv.gw, nil
}

// ActivationCode returns a new activation code for the provided request.
func (srv *Server) ActivationCode(req *protov1.ActivationCodeRequest) (string, error) {
	if _, err := did.Parse(req.Did); err != nil {
		return "", errInvalidRequest
	}
	return srv.store.ActivationCode(req)
}

// AccessToken process an incoming credentials request.
func (srv *Server) AccessToken(req *protov1.CredentialsRequest,
	validateCode bool) (*protov1.CredentialsResponse, error) {
	// Retrieve DID instance
	identifier, err := utils.ResolveDID(req.Did, srv.providers)
	if err != nil {
		return nil, errors.Wrap(err, "resolve DID")
	}

	// Verify registration proof
	if err := utils.VerifySignature(identifier, []byte(req.ActivationCode), req.Proof); err != nil {
		return nil, errors.Wrap(err, "invalid signature")
	}

	// Validate activation code
	if validateCode {
		if !srv.store.VerifyActivationCode(req) {
			return nil, errInvalidRequest
		}
	}

	// Request is valid, return credentials result.
	return srv.getToken(req.Did, req.Role)
}

// RenewToken will refresh a valid but expired access token.
func (srv *Server) RenewToken(token *jwx.Token, refreshCode string) (*protov1.CredentialsResponse, error) {
	// Validate refresh code
	cc := srv.getRefreshCode(token.String())
	if cc == "" || cc != refreshCode {
		return nil, errInvalidRequest
	}

	// Create new token using claims present in the expired version.
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return nil, errUnauthenticated
	}
	return srv.getToken(data.DID, data.Role)
}

// LocationRecord receive and process incoming location update events.
// nolint: interfacer
func (srv *Server) LocationRecord(token *jwx.Token, req *protov1.RecordRequest) (*protov1.RecordResponse, error) {
	// Maximum of 100 records per-request
	if len(req.Records) > 100 {
		return nil, errInvalidRequest
	}

	// Get DID for the credential's subject
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return nil, errUnauthenticated
	}

	// Publish message
	contents, err := req.Marshal()
	if err != nil {
		return nil, errInvalidRequest
	}
	msg := amqp.Message{
		Type:        "ct19.location_record",
		Timestamp:   time.Now().UTC(),
		MessageId:   uuid.New().String(),
		ContentType: "application/protobuf",
		Body:        contents,
		Headers: map[string]interface{}{
			"did": data.DID,
		},
	}
	res, err := srv.pub.Push(msg, amqp.MessageOptions{
		Exchange:   "tasks",
		Persistent: true,
	})
	if err != nil {
		return nil, errFailedToPublish
	}
	return &protov1.RecordResponse{Ok: res}, nil
}

// NewIdentifier provides a helper method to generate a new DID instances for
// clients that can't generate it locally. This is not recommended but supported
// for legacy and development purposes. This method does not require authentication.
func (srv *Server) NewIdentifier(req *protov1.NewIdentifierRequest) (*protov1.NewIdentifierResponse, error) {
	// Validate parameters
	if req.Method == "" {
		return nil, errInvalidRequest
	}

	// New DID instance
	var err error
	id, _ := did.NewIdentifierWithMode(req.Method, "", did.ModeUUID)
	if err = id.AddNewKey("master", did.KeyTypeEd, did.EncodingBase58); err != nil {
		return nil, errInternalError
	}
	if err = id.AddAuthenticationKey("master"); err != nil {
		return nil, errInternalError
	}
	if err = id.AddProof("master", "sample-ct19.iadb.org"); err != nil {
		return nil, errInternalError
	}

	// Full document as contents
	js, _ := json.Marshal(id.Document())
	contents := base64.StdEncoding.EncodeToString(js)

	// Publish
	if req.AutoPublish {
		msg := amqp.Message{
			Type:        "ct19.new_did",
			Timestamp:   time.Now().UTC(),
			MessageId:   uuid.New().String(),
			ContentType: "application/json",
			Body:        js,
		}
		_, err := srv.pub.Push(msg, amqp.MessageOptions{
			Exchange:   "tasks",
			Persistent: true,
		})
		if err != nil {
			srv.log.WithField("did", id.String()).Warning("failed to submit publish request")
		}
	}
	return &protov1.NewIdentifierResponse{Document: contents}, nil
}

// Generate bearer token and refresh code.
func (srv *Server) getToken(id, role string) (*protov1.CredentialsResponse, error) {
	// Get access token
	params := &jwx.TokenParameters{
		Audience:   []string{srv.name},
		Subject:    id,
		Method:     jwx.ES384,
		NotBefore:  "0ms",
		Expiration: "168h", // 1 week by default
		CustomPayloadClaims: &credentialsData{
			DID:  id,
			Role: role,
		},
	}
	token, err := srv.tg.NewToken("master", params)
	if err != nil {
		return nil, err
	}

	// Return result
	return &protov1.CredentialsResponse{
		AccessToken: token.String(),
		RefreshCode: srv.getRefreshCode(token.String()),
	}, nil
}

// Refresh codes are base64-encoded authenticated hashes for generated credentials.
func (srv *Server) getRefreshCode(seed string) string {
	h, err := blake2b.New256(srv.hk)
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
func (srv *Server) authenticate(ctx context.Context, checkExpiration bool) (*jwx.Token, error) {
	// Retrieve credentials
	token, err := getTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate token
	now := time.Now()
	checks := []jwx.ValidatorFunc{
		jwx.IssuerValidator(srv.name),
		jwx.AudienceValidator([]string{srv.name}),
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
func (srv *Server) authorize(token *jwx.Token, resource string, action string) bool {
	data := &credentialsData{}
	if err := token.Decode(&data); err != nil {
		return false
	}
	return srv.enf.Evaluate(auth.Request{
		Subject:  data.Role,
		Resource: resource,
		Action:   action,
	})
}

// Internal event processing.
func (srv *Server) eventLoop() {
	for {
		select {
		case <-srv.ctx.Done():
			return
		case msg, ok := <-srv.pub.MessageReturns():
			if !ok {
				return
			}
			srv.log.WithFields(xlog.Fields{
				"id":    msg.MessageId,
				"stamp": msg.Timestamp,
			}).Warning("message returned by the broker")
		}
	}
}
