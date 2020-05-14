package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/x/ccg/did"
)

const signature = `{
  "@context": [
    "https://w3id.org/security/v1"
  ],
  "type": "Ed25519Signature2018",
  "creator": "did:bryk:7889c965-4644-44ff-b760-f396f1d11444#master",
  "created": "2020-05-04T19:08:59Z",
  "domain": "did.bryk.io",
  "nonce": "135fdd076c7ea45b00c352119c1c46b7",
  "signatureValue": "UZPNyuEzLycM69vhrMInj/J3KEFSufClWJqJaReleTkQwEfIpKvw09dxxYZsEZ6yRYYH1e/ryUriVnMG8VcLBA=="
}`

func TestHandler_LocationRecord(t *testing.T) {
	// Sample record
	r := &protov1.LocationRecord{
		Did:       "did:bryk:7889c965-4644-44ff-b760-f396f1d11444",
		Lng:       38.862848,
		Lat:       -77.08672,
		Alt:       0.0,
		Timestamp: 1588619270,
		Proof:     []byte(signature),
	}
	r.Hash = r.GenerateHash()
	req := &protov1.RecordRequest{
		Records: []*protov1.LocationRecord{r},
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		OrigName:     true,
	}
	output, err := m.MarshalToString(req)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s", output)
}

func TestPublishTicket(t *testing.T) {
	var err error

	// New DID instance
	id, _ := did.NewIdentifierWithMode("iadb", "", did.ModeUUID)
	if err = id.AddNewKey("master", did.KeyTypeEd, did.EncodingBase58); err != nil {
		t.Fatal(err)
	}
	if err = id.AddAuthenticationKey("master"); err != nil {
		t.Fatal(err)
	}
	if err = id.AddProof("master", "sample-ct19.iadb.org"); err != nil {
		t.Fatal(err)
	}

	// Get publish ticket
	sd, _ := json.Marshal(id.SafeDocument())
	ticket := &publishTicket{
		Timestamp:  time.Now().Unix(),
		Content:    sd,
		KeyID:      "master",
		NonceValue: 0,
	}
	key := id.Key("master")
	ticket.Signature, err = key.Sign(ticket.Solve(18))
	if err != nil {
		t.Fatal(err)
	}
	// ticket.Submit()
}
