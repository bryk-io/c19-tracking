package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.bryk.io/x/crypto/pow"
	"golang.org/x/crypto/sha3"
)

const defaultPublishEndpoint = "https://did.bryk.io/v1/process"

type publishTicket struct {
	Timestamp  int64  `json:"timestamp"`
	NonceValue int64  `json:"nonce"`
	KeyID      string `json:"key_id"`
	Content    []byte `json:"content"`
	Signature  []byte `json:"signature"`
}

// ResetNonce returns the internal nonce value back to 0
func (t *publishTicket) ResetNonce() {
	t.NonceValue = 0
}

// IncrementNonce will adjust the internal nonce value by 1
func (t *publishTicket) IncrementNonce() {
	t.NonceValue++
}

// Nonce returns the current value set on the nonce attribute
func (t *publishTicket) Nonce() int64 {
	return t.NonceValue
}

// Encode returns a deterministic binary encoding for the ticket instance using a
// byte concatenation of the form 'timestamp | nonce | key_id | content'; where both
// timestamp and nonce are individually encoded using little endian byte order
func (t *publishTicket) Encode() ([]byte, error) {
	var tc []byte
	nb := bytes.NewBuffer(nil)
	tb := bytes.NewBuffer(nil)
	kb := make([]byte, hex.EncodedLen(len([]byte(t.KeyID))))
	if err := binary.Write(nb, binary.LittleEndian, t.Nonce()); err != nil {
		return nil, fmt.Errorf("failed to encode nonce value: %s", err)
	}
	if err := binary.Write(tb, binary.LittleEndian, t.Timestamp); err != nil {
		return nil, fmt.Errorf("failed to encode nonce value: %s", err)
	}
	hex.Encode(kb, []byte(t.KeyID))
	tc = append(tc, tb.Bytes()...)
	tc = append(tc, nb.Bytes()...)
	tc = append(tc, kb...)
	return append(tc, t.Content...), nil
}

// Solve the ticket challenge using the proof-of-work mechanism
func (t *publishTicket) Solve(difficulty uint) []byte {
	if difficulty == 0 {
		difficulty = 8
	}
	challenge := <-pow.Solve(context.Background(), t, sha3.New256(), difficulty)
	res, _ := hex.DecodeString(challenge)
	return res
}

// Submit a ticket to the network.
func (t *publishTicket) Submit() bool {
	res, err := http.Post(defaultPublishEndpoint, "application/json", t.getRequestData())
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return true
}

// GetRequestData return the ticket properly encoded to submit.
func (t *publishTicket) getRequestData() io.Reader {
	req := map[string]interface{}{
		"task":   0,
		"ticket": t,
	}
	data, _ := json.Marshal(req)
	return bytes.NewReader(data)
}
