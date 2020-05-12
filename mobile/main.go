// +build js,wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
	"time"

	"go.bryk.io/x/ccg/did"
)

// Restore DID instance from its document
func loadDID(contents string) (*did.Identifier, error) {
	// Get DID from document
	doc := &did.Document{}
	if err := json.Unmarshal([]byte(contents), doc); err != nil {
		return nil, errors.New("invalid DID document")
	}
	id, err := did.FromDocument(doc)
	if err != nil {
		return nil, errors.New("invalid DID document")
	}
	return id, nil
}

// Return a properly encoded error message
func encodeError(err error) interface{} {
	msg := map[string]string{
		"error": err.Error(),
	}
	output, _ := json.Marshal(msg)
	return js.ValueOf(fmt.Sprintf("%s", output)).String()
}

// Create a new DID instance and return its complete
// JSON-encoded document.
// Parameters:
// - method (string)
func CreateDID(this js.Value, args []js.Value) interface{} {
	// Get parameters
	if len(args) == 0 {
		return encodeError(errors.New("missing required parameters"))
	}
	method := args[0].String()

	// Generate DID of requested method, add an Ed25519 master
	// key authentication key and prepare a document proof
	var err error
	id, _ := did.NewIdentifierWithMode(method, "", did.ModeUUID)
	if err = id.AddNewKey("master", did.KeyTypeEd, did.EncodingBase58); err != nil {
		return encodeError(err)
	}
	if err = id.AddAuthenticationKey("master"); err != nil {
		return encodeError(err)
	}
	if err = id.AddProof("master", "sample-ct19.iadb.org"); err != nil {
		return encodeError(err)
	}

	// Return JSON-encoded document
	output, _ := json.MarshalIndent(id.Document(), "", "  ")
	return js.ValueOf(fmt.Sprintf("%s", output)).String()
}

// Return a publish request ticket.
// Parameters:
// - did document (string)
// - difficulty (int)
func PublishRequest(this js.Value, args []js.Value) interface{} {
	// Get parameters
	if len(args) != 2 {
		return encodeError(errors.New("missing required parameters"))
	}
	doc := args[0].String()
	diff := args[1].Int()

	// Get DID from document
	id, err := loadDID(doc)
	if err != nil {
		return encodeError(err)
	}

	// Get request ticket
	sd, _ := json.Marshal(id.SafeDocument())
	ticket := &publishTicket{
		Timestamp:  time.Now().Unix(),
		Content:    sd,
		KeyId:      "master",
		NonceValue: 0,
	}

	// Solve ticket and add signature
	key := id.Key("master")
	ticket.Signature, err = key.Sign(ticket.Solve(uint(diff)))
	if err != nil {
		return encodeError(err)
	}

	// Return JSON-encoded publish request
	output, _ := json.MarshalIndent(ticket, "", "  ")
	return js.ValueOf(fmt.Sprintf("%s", output)).String()
}

// Generate a signature LD document.
// Parameters:
// - did document (string)
// - contents to sign (string)
// - domain value (string)
func GetSignatureLD(this js.Value, args []js.Value) interface{} {
	// Get parameters
	if len(args) != 3 {
		return encodeError(errors.New("missing required parameters"))
	}
	doc := args[0].String()
	contents := args[1].String()
	domain := args[2].String()

	// Get DID from document
	id, err := loadDID(doc)
	if err != nil {
		return encodeError(err)
	}

	key := id.Key("master")
	signature, err := key.ProduceSignatureLD([]byte(contents), domain)
	if err != nil {
		return encodeError(err)
	}

	// Return JSON-encoded signature document
	output, _ := json.MarshalIndent(signature, "", "  ")
	return js.ValueOf(fmt.Sprintf("%s", output)).String()
}

func main() {
	// Register "exported" methods
	js.Global().Set("createDID", js.FuncOf(CreateDID))
	js.Global().Set("publishRequest", js.FuncOf(PublishRequest))
	js.Global().Set("signatureLD", js.FuncOf(GetSignatureLD))

	// Block and prevent program to exit
	select {}
}
