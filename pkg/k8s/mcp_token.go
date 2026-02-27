package k8s

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateMCPToken generates a cryptographically random 32-byte bearer token,
// returned as a 64-character hex string.
func GenerateMCPToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
