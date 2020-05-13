package api

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/covid-tracking/utils"
	"go.bryk.io/x/auth"
	"go.bryk.io/x/ccg/did"
	"go.bryk.io/x/jwx"
	"go.bryk.io/x/net/rpc"
	"go.bryk.io/x/pki"
	"golang.org/x/crypto/sha3"
	"google.golang.org/grpc/metadata"
)

var defaultPKIConf = `{
  "signing": {
    "default": {
      "expiry": "720h",
      "usage": [
        "key encipherment",
        "digital signature",
        "client auth"
      ]
    },
    "profiles": {
      "namespace": {
        "ca_constraint": {
          "is_ca": true,
          "max_path_len": 1
        },
        "expiry": "8760h",
        "usages": [
          "cert sign",
          "crl sign"
        ]
      },
      "agent": {
        "ca_constraint": {
          "is_ca": false
        },
        "expiry": "8760h",
        "usages": [
          "key encipherment",
          "digital signature",
          "client auth"
        ]
      }
    }
  }
}
`

var defaultRootCSR = `{
  "cn": "ct19-api-server",
  "key": {
    "algo": "ecdsa",
    "size": 384
  },
  "names": [{}]
}
`

// Ensure the root CA files are in place or create it if required.
func verifyRootCA(home string) error {
	certFile := filepath.Clean(filepath.Join(home, "root-ca.crt"))
	keyFile := filepath.Clean(filepath.Join(home, "root-ca.pem"))

	// Valid root CA exists already
	if pki.IsKeyPairFile(certFile, keyFile) {
		return nil
	}

	// Create new root CA
	var csr []byte
	csr, err := ioutil.ReadFile(filepath.Clean(filepath.Join(home, "root-ca.json")))
	if err != nil {
		csr = []byte(defaultRootCSR)
	}
	cert, key, err := pki.RootCA(csr)
	if err != nil {
		return errors.Wrap(err, "failed to create root CA")
	}
	if err := ioutil.WriteFile(certFile, cert, 0400); err != nil {
		return errors.Wrap(err, "failed to save certificate")
	}
	if err := ioutil.WriteFile(keyFile, key, 0400); err != nil {
		return errors.Wrap(err, "failed to save private key")
	}
	return nil
}

// Ensure the TLS certificate is in place and valid.
func verifyTLSCertificate(home string) (*rpc.ServerTLSConfig, error) {
	certFile := filepath.Join(home, "tls", "tls.crt")
	keyFile := filepath.Join(home, "tls", "tls.key")
	if !pki.IsKeyPairFile(certFile, keyFile) {
		return nil, errors.New("TLS certificate is required")
	}
	cert, err := ioutil.ReadFile(filepath.Clean(certFile))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read TLS certificate")
	}
	key, err := ioutil.ReadFile(filepath.Clean(keyFile))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read TLS key")
	}
	tlsConf := rpc.ServerTLSConfig{
		Cert:             cert,
		PrivateKey:       key,
		IncludeSystemCAs: true,
	}
	return &tlsConf, nil
}

// Prepare the internal PKI.
func setupPKI(home string) (*pki.CA, error) {
	certFile := filepath.Clean(filepath.Join(home, "root-ca.crt"))
	keyFile := filepath.Clean(filepath.Join(home, "root-ca.pem"))
	if !pki.IsKeyPairFile(certFile, keyFile) {
		return nil, errors.New("invalid root CA credentials")
	}
	var conf []byte
	conf, err := ioutil.ReadFile(filepath.Clean(filepath.Join(home, "pki.json")))
	if err != nil {
		conf = []byte(defaultPKIConf)
	}
	caConf, err := pki.DecodeConfig(conf)
	if err != nil {
		return nil, err
	}
	return pki.NewCA(certFile, keyFile, nil, caConf)
}

// Prepare the HTTP gateway interface.
func setupHTTPGateway(port int) (*rpc.HTTPGateway, error) {
	gwOpts := []rpc.HTTPGatewayOption{
		rpc.WithGatewayPort(port),
		rpc.WithClientOptions([]rpc.ClientOption{
			rpc.WithInsecureSkipVerify(),
			rpc.WithClientTLS(rpc.ClientTLSConfig{IncludeSystemCAs: true}),
		}),
	}
	return rpc.NewHTTPGateway(gwOpts...)
}

// Prepare authorization enforcer.
func setupAuthEnforcer() (*auth.Enforcer, error) {
	enf, err := auth.NewEnforcer()
	if err != nil {
		return nil, err
	}
	for _, r := range strings.Split(utils.AccessPolicy(), "\n") {
		if strings.HasPrefix(r, "#") || strings.TrimSpace(r) == "" {
			continue // Ignore comments and empty lines
		}
		ar := &auth.Rule{}
		if err := ar.FromString(r); err != nil {
			return nil, err
		}
		if err := enf.GetAdapter().AddRule(ar); err != nil {
			return nil, err
		}
	}
	return enf, nil
}

// Prepares a new token generator instance.
func setupTokenGenerator(serverName string, serverHome string) (*jwx.Generator, error) {
	keyPEM, err := ioutil.ReadFile(filepath.Clean(filepath.Join(serverHome, "root-ca.pem")))
	if err != nil {
		return nil, err
	}
	key, err := jwx.NewGeneratorKey("master", jwx.KeyTypeEC, keyPEM)
	if err != nil {
		return nil, err
	}
	return jwx.NewGenerator(serverName, *key)
}

// Return the key used for authenticated hash operations.
func hashKey(home string) ([]byte, error) {
	src, err := ioutil.ReadFile(filepath.Clean(filepath.Join(home, "root-ca.pem")))
	if err != nil {
		return nil, err
	}
	h := sha3.Sum256(src)
	return h[:], nil
}

// Retrieve a bearer credential from the incoming request context.
func getTokenFromContext(ctx context.Context) (*jwx.Token, error) {
	// Get token
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errUnauthenticated
	}
	t := md.Get("authorization")
	if len(t) != 1 {
		return nil, errUnauthenticated
	}
	if !strings.HasPrefix(t[0], "Bearer") {
		return nil, errUnauthenticated
	}
	return jwx.Parse(strings.Split(t[0], " ")[1])
}

// Verify the provided role literal is supported.
func isRoleValid(role string) bool {
	for _, r := range supportedRoles {
		if role == r {
			return true
		}
	}
	return false
}

// Ensure a location record is valid and can be safely indexed and stored.
func validateRecord(id *did.Identifier, r *protov1.LocationRecord) bool {
	// Verify DID is correct on the record entry
	if r.Did != id.DID() {
		return false
	}

	// Lat and Lng are required
	if r.Lat == 0 || r.Lng == 0 {
		return false
	}

	// Invalid timestamp value
	now := time.Now()
	if r.Timestamp == 0 || r.Timestamp > now.Unix() {
		return false
	}

	// Invalid hash value
	if r.GenerateHash() != r.Hash {
		return false
	}

	// Validate record's signature
	if err := utils.VerifySignature(id, []byte(r.GetHash()), r.Proof); err != nil {
		return false
	}

	// All good!
	return true
}
