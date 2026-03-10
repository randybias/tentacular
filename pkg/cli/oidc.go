package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OIDCTokenStore holds cached OIDC tokens for an environment.
type OIDCTokenStore struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email"`
}

// IsExpired reports whether the access token has expired (with a 30s buffer).
func (t *OIDCTokenStore) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// tokenDir returns the path to the OIDC token storage directory.
// Tokens are stored in ~/.tentacular/tokens/.
func tokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".tentacular", "tokens"), nil
}

// tokenPath returns the path to the token file for a given environment.
func tokenPath(envName string) (string, error) {
	dir, err := tokenDir()
	if err != nil {
		return "", err
	}
	if envName == "" {
		envName = "default"
	}
	return filepath.Join(dir, envName+".json"), nil
}

// SaveOIDCToken persists OIDC tokens to disk with restricted permissions.
func SaveOIDCToken(envName string, store *OIDCTokenStore) error {
	path, err := tokenPath(envName)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

// LoadOIDCToken reads cached OIDC tokens for an environment.
// Returns nil, nil if no token file exists.
func LoadOIDCToken(envName string) (*OIDCTokenStore, error) {
	path, err := tokenPath(envName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading token file: %w", err)
	}

	var store OIDCTokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing token file: %w", err)
	}
	return &store, nil
}

// RemoveOIDCToken deletes the cached OIDC token for an environment.
func RemoveOIDCToken(envName string) error {
	path, err := tokenPath(envName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing token file: %w", err)
	}
	return nil
}

// jwtClaims holds the subset of JWT claims we care about.
type jwtClaims struct {
	Iss               string `json:"iss"`
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername  string `json:"preferred_username"`
	Exp               int64  `json:"exp"`
	Iat               int64  `json:"iat"`
	IdentityProvider  string `json:"identity_provider"`
}

// DecodeJWTClaims decodes the payload of a JWT without signature verification.
// This is intentional — the server validates signatures; the CLI only needs
// to extract display information.
func DecodeJWTClaims(tokenStr string) (*jwtClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("parsing JWT claims: %w", err)
	}
	return &claims, nil
}
