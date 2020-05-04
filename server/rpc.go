package server

import (
	"context"

	"github.com/gogo/protobuf/types"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
)

type remoteInterface struct {
	sh *Handler
}

// Ping provides a sample reachability test method. This method does not require authentication.
func (ri *remoteInterface) Ping(_ context.Context, _ *types.Empty) (*protov1.PingResponse, error) {
	return &protov1.PingResponse{Ok: true}, nil
}

// UserActivationCode generates an return a new device activation code. This method does not
// require authentication for "user" activation codes.
func (ri *remoteInterface) ActivationCode(ctx context.Context,
	req *protov1.ActivationCodeRequest) (*protov1.ActivationCodeResponse, error) {
	// For security, admin codes can't be generated via the API
	if !isRoleValid(req.Role) || req.Role == "admin" {
		return nil, errInvalidRequest
	}

	// Activation codes for "agent" role require authentication and authorization
	if req.Role == "agent" {
		// Authentication (ignoring expiration date)
		token, err := ri.sh.authenticate(ctx, true)
		if err != nil {
			return nil, err
		}

		// Authorization
		if !ri.sh.authorize(token, "/activation_code/agent", "create") {
			return nil, errUnauthorized
		}
	}

	// Process request
	code, err := ri.sh.ActivationCode(req)
	if err != nil {
		return nil, err
	}
	return &protov1.ActivationCodeResponse{ActivationCode: code}, nil
}

// Credentials requests for platform access. This method does not require authentication.
func (ri *remoteInterface) Credentials(_ context.Context,
	req *protov1.CredentialsRequest) (*protov1.CredentialsResponse, error) {
	// For security, admin credentials can't be generated via the API
	if !isRoleValid(req.Role) || req.Role == "admin" {
		return nil, errInvalidRequest
	}
	return ri.sh.AccessToken(req, true)
}

// RenewCredentials allows to refresh a valid but expired access token for a new one.
// This method requires authentication.
func (ri *remoteInterface) RenewCredentials(ctx context.Context,
	req *protov1.RenewCredentialsRequest) (*protov1.CredentialsResponse, error) {
	// Authentication (ignoring expiration date)
	token, err := ri.sh.authenticate(ctx, false)
	if err != nil {
		return nil, err
	}

	// Authorization
	if !ri.sh.authorize(token, "/credentials", "renew") {
		return nil, errUnauthorized
	}

	return ri.sh.RenewToken(token, req.RefreshCode)
}

// Record location events.
func (ri *remoteInterface) Record(ctx context.Context,
	req *protov1.RecordRequest) (*protov1.RecordResponse, error) {
	// Authentication
	token, err := ri.sh.authenticate(ctx, true)
	if err != nil {
		return nil, err
	}

	// Authorization
	if !ri.sh.authorize(token, "/record", "create") {
		return nil, errUnauthorized
	}

	return ri.sh.LocationRecord(token, req)
}
