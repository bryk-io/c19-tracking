package utils

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"go.bryk.io/x/amqp"
	"go.bryk.io/x/ccg/did"
	"golang.org/x/crypto/sha3"
)

// ResolveDID fetch a published DID instance
func ResolveDID(id string, providers []*did.Provider) (*did.Identifier, error) {
	content, err := did.Resolve(id, providers)
	if err != nil {
		return nil, err
	}
	doc := &did.Document{}
	if err := json.Unmarshal(content, doc); err != nil {
		return nil, err
	}
	return did.FromDocument(doc)
}

// VerifySignature ensures the provided signature LD document was generated
// by the provided DID instance for 'data'
func VerifySignature(id *did.Identifier, data []byte, ldSignature []byte) error {
	// Decode signature document
	signature := &did.SignatureLD{}
	if err := json.Unmarshal(ldSignature, signature); err != nil {
		return errors.New("invalid signature document")
	}

	// Retrieve key
	key := id.Key(signature.Creator)
	if key == nil {
		return errors.New("invalid key identifier")
	}

	// Hash original signed data
	input := sha3.Sum256(data)

	// Verify signature
	if !key.VerifySignatureLD(input[:], signature) {
		return errors.New("invalid signature")
	}

	// All good!
	return nil
}

// ReadInput prompt the user to interactively enter information.
func ReadInput(prompt string, val interface{}) {
	fmt.Printf("%s: ", prompt)
	_, _ = fmt.Scanln(val)
}

// AccessPolicy returns the default RBAC style platform's access policy
func AccessPolicy() string {
	return `
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
}

// BrokerTopology returns the default AMQP topology for the broker server.
func BrokerTopology() amqp.Topology {
	return amqp.Topology{
		Exchanges: []amqp.Exchange{
			{
				Name:    "tasks",
				Kind:    "direct",
				Durable: true,
			},
			{
				Name:    "notifications",
				Kind:    "fanout",
				Durable: true,
			},
		},
		Queues: []amqp.Queue{
			{
				Name:    "tasks",
				Durable: true,
			},
			{
				Name:    "notifications",
				Durable: true,
			},
		},
		Bindings: []amqp.Binding{
			{
				Exchange: "tasks",
				Queue:    "tasks",
			},
			{
				Exchange: "notifications",
				Queue:    "notifications",
			},
		},
	}
}
