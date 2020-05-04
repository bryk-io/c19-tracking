package protov1

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// GenerateHash returns the corresponding hash value, in hex format, for the
// record instance calculated as: SHA256(did|lat|lng|alt|timestamp)
func (lr *LocationRecord) GenerateHash() string {
	segments := []string{
		lr.Did,
		fmt.Sprintf("%f", lr.Lat),
		fmt.Sprintf("%f", lr.Lng),
		fmt.Sprintf("%f", lr.Alt),
		fmt.Sprintf("%d", lr.Timestamp),
	}
	h := sha256.Sum256([]byte(strings.Join(segments, "|")))
	return fmt.Sprintf("%x", h)
}
