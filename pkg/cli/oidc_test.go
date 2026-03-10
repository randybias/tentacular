package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOIDCTokenStore_SaveLoadRemove(t *testing.T) {
	// Use temp dir as HOME
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	envName := "test-env"
	store := &OIDCTokenStore{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    time.Now().Add(1 * time.Hour).Truncate(time.Second),
		Email:        "user@example.com",
	}

	// Save
	if err := SaveOIDCToken(envName, store); err != nil {
		t.Fatalf("SaveOIDCToken: %v", err)
	}

	// Verify file permissions
	path, _ := tokenPath(envName)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %o", fi.Mode().Perm())
	}

	// Load
	loaded, err := LoadOIDCToken(envName)
	if err != nil {
		t.Fatalf("LoadOIDCToken: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadOIDCToken returned nil")
	}
	if loaded.AccessToken != store.AccessToken {
		t.Errorf("access token mismatch: got %q, want %q", loaded.AccessToken, store.AccessToken)
	}
	if loaded.RefreshToken != store.RefreshToken {
		t.Errorf("refresh token mismatch: got %q, want %q", loaded.RefreshToken, store.RefreshToken)
	}
	if loaded.Email != store.Email {
		t.Errorf("email mismatch: got %q, want %q", loaded.Email, store.Email)
	}

	// Remove
	if err := RemoveOIDCToken(envName); err != nil {
		t.Fatalf("RemoveOIDCToken: %v", err)
	}

	// Verify gone
	loaded, err = LoadOIDCToken(envName)
	if err != nil {
		t.Fatalf("LoadOIDCToken after remove: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil after remove")
	}
}

func TestOIDCTokenStore_LoadNonexistent(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	store, err := LoadOIDCToken("nonexistent-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store != nil {
		t.Error("expected nil for nonexistent token")
	}
}

func TestOIDCTokenStore_IsExpired(t *testing.T) {
	active := &OIDCTokenStore{ExpiresAt: time.Now().Add(5 * time.Minute)}
	if active.IsExpired() {
		t.Error("token with 5min remaining should not be expired")
	}

	expired := &OIDCTokenStore{ExpiresAt: time.Now().Add(-1 * time.Minute)}
	if !expired.IsExpired() {
		t.Error("token expired 1min ago should be expired")
	}

	// Within 30s buffer
	almostExpired := &OIDCTokenStore{ExpiresAt: time.Now().Add(20 * time.Second)}
	if !almostExpired.IsExpired() {
		t.Error("token expiring in 20s should be treated as expired (30s buffer)")
	}
}

func TestOIDCTokenStore_DefaultEnvName(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	store := &OIDCTokenStore{
		AccessToken: "test",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Email:       "test@test.com",
	}

	// Empty env name should use "default"
	if err := SaveOIDCToken("", store); err != nil {
		t.Fatalf("SaveOIDCToken with empty env: %v", err)
	}

	path := filepath.Join(tmpHome, ".tentacular", "tokens", "default.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected token file at %s: %v", path, err)
	}
}

func TestDecodeJWTClaims(t *testing.T) {
	claims := map[string]interface{}{
		"sub":                "user-123",
		"email":              "alice@example.com",
		"name":               "Alice Smith",
		"preferred_username": "alice",
		"exp":                float64(time.Now().Add(1 * time.Hour).Unix()),
		"iat":                float64(time.Now().Unix()),
		"identity_provider":  "google",
	}

	payload, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	token := header + "." + body + "." + sig

	decoded, err := DecodeJWTClaims(token)
	if err != nil {
		t.Fatalf("DecodeJWTClaims: %v", err)
	}
	if decoded.Email != "alice@example.com" {
		t.Errorf("email: got %q, want %q", decoded.Email, "alice@example.com")
	}
	if decoded.Name != "Alice Smith" {
		t.Errorf("name: got %q, want %q", decoded.Name, "Alice Smith")
	}
	if decoded.Sub != "user-123" {
		t.Errorf("sub: got %q, want %q", decoded.Sub, "user-123")
	}
	if decoded.IdentityProvider != "google" {
		t.Errorf("provider: got %q, want %q", decoded.IdentityProvider, "google")
	}
}

func TestDecodeJWTClaims_InvalidToken(t *testing.T) {
	_, err := DecodeJWTClaims("not-a-jwt")
	if err == nil {
		t.Error("expected error for invalid JWT")
	}
}

// buildTestJWT constructs a minimal JWT for testing (no real signature).
func buildTestJWT(claims map[string]interface{}) string {
	payload, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	return header + "." + body + "." + sig
}

func TestDeviceAuthFlow_MockServer(t *testing.T) {
	// Create a mock OIDC server
	pollCount := 0
	testJWT := buildTestJWT(map[string]interface{}{
		"sub":   "user-abc",
		"email": "test@example.com",
		"name":  "Test User",
		"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			// Discovery endpoint
			base := "http://" + r.Host
			json.NewEncoder(w).Encode(map[string]string{
				"device_authorization_endpoint": base + "/auth/device",
				"token_endpoint":                base + "/token",
			})

		case strings.HasSuffix(r.URL.Path, "/auth/device"):
			// Device auth endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":               "test-device-code",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "http://example.com/verify",
				"verification_uri_complete": "http://example.com/verify?code=ABCD-EFGH",
				"expires_in":               300,
				"interval":                 1,
			})

		case strings.HasSuffix(r.URL.Path, "/token"):
			pollCount++
			if pollCount < 2 {
				// First poll: pending
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "authorization_pending",
				})
				return
			}
			// Second poll: success
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  testJWT,
				"refresh_token": "test-refresh-token",
				"expires_in":    3600,
				"token_type":    "Bearer",
			})
		}
	}))
	defer server.Close()

	// Test discovery
	deviceEndpoint, tokenEndpoint, err := discoverOIDCEndpoints(server.URL)
	if err != nil {
		t.Fatalf("discoverOIDCEndpoints: %v", err)
	}
	if !strings.HasSuffix(deviceEndpoint, "/auth/device") {
		t.Errorf("unexpected device endpoint: %s", deviceEndpoint)
	}
	if !strings.HasSuffix(tokenEndpoint, "/token") {
		t.Errorf("unexpected token endpoint: %s", tokenEndpoint)
	}

	// Test device auth request
	authResp, err := requestDeviceAuth(deviceEndpoint, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("requestDeviceAuth: %v", err)
	}
	if authResp.DeviceCode != "test-device-code" {
		t.Errorf("device_code: got %q, want %q", authResp.DeviceCode, "test-device-code")
	}
	if authResp.UserCode != "ABCD-EFGH" {
		t.Errorf("user_code: got %q, want %q", authResp.UserCode, "ABCD-EFGH")
	}

	// Test token polling (will get "pending" once, then succeed)
	tokenResp, err := pollForToken(
		tokenEndpoint,
		"test-device-code",
		"test-client",
		"test-secret",
		1*time.Second,
		time.Now().Add(30*time.Second),
	)
	if err != nil {
		t.Fatalf("pollForToken: %v", err)
	}
	if tokenResp.AccessToken != testJWT {
		t.Error("unexpected access token")
	}
	if tokenResp.RefreshToken != "test-refresh-token" {
		t.Errorf("refresh token: got %q, want %q", tokenResp.RefreshToken, "test-refresh-token")
	}
}

func TestDeviceAuthFlow_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return pending
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "authorization_pending",
		})
	}))
	defer server.Close()

	_, err := pollForToken(
		server.URL+"/token",
		"test-device-code",
		"test-client",
		"",
		100*time.Millisecond,
		time.Now().Add(-1*time.Second), // Already expired
	)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestDeviceAuthFlow_AccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "access_denied",
		})
	}))
	defer server.Close()

	_, err := pollForToken(
		server.URL+"/token",
		"test-device-code",
		"test-client",
		"",
		100*time.Millisecond,
		time.Now().Add(30*time.Second),
	)
	if err == nil {
		t.Error("expected access denied error")
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Errorf("expected denied error, got: %v", err)
	}
}

func TestTokenRefresh_MockServer(t *testing.T) {
	refreshedJWT := buildTestJWT(map[string]interface{}{
		"sub":   "user-abc",
		"email": "refreshed@example.com",
		"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			base := "http://" + r.Host
			json.NewEncoder(w).Encode(map[string]string{
				"device_authorization_endpoint": base + "/auth/device",
				"token_endpoint":                base + "/token",
			})
		case strings.HasSuffix(r.URL.Path, "/token"):
			_ = r.ParseForm()
			if r.FormValue("grant_type") != "refresh_token" {
				t.Errorf("expected grant_type=refresh_token, got %q", r.FormValue("grant_type"))
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  refreshedJWT,
				"refresh_token": "new-refresh-token",
				"expires_in":    3600,
			})
		}
	}))
	defer server.Close()

	// Set up temp HOME and config
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Write config with OIDC settings pointing to mock server
	configDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	configContent := fmt.Sprintf(`default_env: test
environments:
  test:
    oidc_issuer: %s
    oidc_client_id: test-client
    oidc_client_secret: test-secret
`, server.URL)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644)

	// Save an expired token
	store := &OIDCTokenStore{
		AccessToken:  "old-expired-token",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		Email:        "old@example.com",
	}
	if err := SaveOIDCToken("test", store); err != nil {
		t.Fatalf("saving expired token: %v", err)
	}

	// Refresh
	refreshed, err := RefreshOIDCToken("test", store)
	if err != nil {
		t.Fatalf("RefreshOIDCToken: %v", err)
	}
	if refreshed.AccessToken != refreshedJWT {
		t.Error("unexpected access token after refresh")
	}
	if refreshed.RefreshToken != "new-refresh-token" {
		t.Errorf("unexpected refresh token: %s", refreshed.RefreshToken)
	}
	if refreshed.Email != "refreshed@example.com" {
		t.Errorf("unexpected email: %s", refreshed.Email)
	}

	// Verify it was persisted
	loaded, err := LoadOIDCToken("test")
	if err != nil {
		t.Fatalf("loading refreshed token: %v", err)
	}
	if loaded.AccessToken != refreshedJWT {
		t.Error("persisted token doesn't match refreshed token")
	}
}

func TestWhoami_NotAuthenticated(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := NewWhoamiCmd()
	// Add env flag like root command does
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	_ = cmd.Flags().Set("env", "nonexistent")
	cmd.SetOut(&strings.Builder{})

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Error("expected error for unauthenticated env")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnvironmentConfig_OIDCFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Write config with OIDC settings
	configDir := filepath.Join(tmpDir, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	configYAML := `environments:
  staging:
    namespace: staging
    mcp_endpoint: http://mcp.example.com/mcp
    oidc_issuer: https://auth.example.com/realms/test
    oidc_client_id: my-client
    oidc_client_secret: my-secret
`
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configYAML), 0o644)

	cfg := LoadConfig()
	env, ok := cfg.Environments["staging"]
	if !ok {
		t.Fatal("staging environment not found")
	}
	if env.OIDCIssuer != "https://auth.example.com/realms/test" {
		t.Errorf("oidc_issuer: got %q", env.OIDCIssuer)
	}
	if env.OIDCClientID != "my-client" {
		t.Errorf("oidc_client_id: got %q", env.OIDCClientID)
	}
	if env.OIDCClientSecret != "my-secret" {
		t.Errorf("oidc_client_secret: got %q", env.OIDCClientSecret)
	}
}

func TestResolveOIDCToken_NoToken(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	token, err := resolveOIDCToken("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

func TestResolveOIDCToken_ValidToken(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	store := &OIDCTokenStore{
		AccessToken: "valid-access-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Email:       "user@test.com",
	}
	if err := SaveOIDCToken("myenv", store); err != nil {
		t.Fatalf("saving token: %v", err)
	}

	token, err := resolveOIDCToken("myenv")
	if err != nil {
		t.Fatalf("resolveOIDCToken: %v", err)
	}
	if token != "valid-access-token" {
		t.Errorf("expected valid-access-token, got %q", token)
	}
}
