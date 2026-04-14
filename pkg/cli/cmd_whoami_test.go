package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupWhoamiEnv sets up a temp HOME with optional token for whoami tests.
func setupWhoamiEnv(t *testing.T, token *OIDCTokenStore) func() {
	t.Helper()

	origHome := os.Getenv("HOME")
	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	_ = os.Unsetenv("TENTACULAR_CLUSTER")

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	// Create minimal config
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	if token != nil {
		if err := SaveOIDCToken("default", token); err != nil {
			t.Fatalf("saving test token: %v", err)
		}
	}

	return func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("TENTACULAR_CLUSTER", origEnv)
		_ = os.Chdir(origDir)
	}
}

func TestWhoami_TextOutput_AllFields(t *testing.T) {
	jwt := buildTestJWT(map[string]any{
		"sub":                "user-123",
		"email":              "alice@example.com",
		"name":               "Alice Smith",
		"preferred_username": "alice",
		"iss":                "https://auth.example.com/realms/test",
		"identity_provider":  "google",
		"exp":                float64(time.Now().Add(1 * time.Hour).Unix()),
		"iat":                float64(time.Now().Unix()),
	})

	token := &OIDCTokenStore{
		AccessToken: jwt,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Email:       "alice@example.com",
	}

	cleanup := setupWhoamiEnv(t, token)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runWhoami: %v", err)
	}

	output := out.String()

	checks := map[string]string{
		"Email":       "alice@example.com",
		"Name":        "Alice Smith",
		"Subject":     "user-123",
		"Issuer":      "https://auth.example.com/realms/test",
		"Provider":    "google",
		"Environment": "default",
		"Expires":     "", // just check it's present
	}

	for label, value := range checks {
		if !strings.Contains(output, label+":") {
			t.Errorf("expected %s label in output, got:\n%s", label, output)
		}
		if value != "" && !strings.Contains(output, value) {
			t.Errorf("expected %q in output, got:\n%s", value, output)
		}
	}
}

func TestWhoami_JSONOutput(t *testing.T) {
	jwt := buildTestJWT(map[string]any{
		"sub":                "user-456",
		"email":              "bob@example.com",
		"name":               "Bob Jones",
		"preferred_username": "bob",
		"iss":                "https://auth.example.com/realms/prod",
		"identity_provider":  "github",
		"exp":                float64(time.Now().Add(2 * time.Hour).Unix()),
		"iat":                float64(time.Now().Unix()),
	})

	token := &OIDCTokenStore{
		AccessToken: jwt,
		ExpiresAt:   time.Now().Add(2 * time.Hour),
		Email:       "bob@example.com",
	}

	cleanup := setupWhoamiEnv(t, token)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	_ = cmd.PersistentFlags().Set("output", "json")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runWhoami -o json: %v", err)
	}

	var result whoamiResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\nraw: %s", err, out.String())
	}

	if result.Email != "bob@example.com" {
		t.Errorf("expected email=bob@example.com, got %q", result.Email)
	}
	if result.Name != "Bob Jones" {
		t.Errorf("expected name=Bob Jones, got %q", result.Name)
	}
	if result.Subject != "user-456" {
		t.Errorf("expected subject=user-456, got %q", result.Subject)
	}
	if result.Issuer != "https://auth.example.com/realms/prod" {
		t.Errorf("expected issuer, got %q", result.Issuer)
	}
	if result.Provider != "github" {
		t.Errorf("expected provider=github, got %q", result.Provider)
	}
	if result.Environment != "default" {
		t.Errorf("expected environment=default, got %q", result.Environment)
	}
	if result.Expired {
		t.Error("expected expired=false")
	}
	if result.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

func TestWhoami_NoToken(t *testing.T) {
	cleanup := setupWhoamiEnv(t, nil)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("expected 'not authenticated' error, got: %v", err)
	}
}

func TestWhoami_ExpiredToken(t *testing.T) {
	jwt := buildTestJWT(map[string]any{
		"sub":   "user-789",
		"email": "expired@example.com",
		"exp":   float64(time.Now().Add(-1 * time.Hour).Unix()),
	})

	token := &OIDCTokenStore{
		AccessToken: jwt,
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
		Email:       "expired@example.com",
	}

	cleanup := setupWhoamiEnv(t, token)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runWhoami with expired token: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "EXPIRED") {
		t.Errorf("expected EXPIRED in output for expired token, got:\n%s", output)
	}
}

func TestWhoami_JSONOutput_ExpiredToken(t *testing.T) {
	jwt := buildTestJWT(map[string]any{
		"sub":   "user-expired",
		"email": "old@example.com",
		"exp":   float64(time.Now().Add(-2 * time.Hour).Unix()),
	})

	token := &OIDCTokenStore{
		AccessToken: jwt,
		ExpiresAt:   time.Now().Add(-2 * time.Hour),
		Email:       "old@example.com",
	}

	cleanup := setupWhoamiEnv(t, token)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	_ = cmd.PersistentFlags().Set("output", "json")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runWhoami -o json expired: %v", err)
	}

	var result whoamiResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\nraw: %s", err, out.String())
	}

	if !result.Expired {
		t.Error("expected expired=true")
	}
}

func TestWhoami_FallbackToStoredEmail(t *testing.T) {
	// Use an invalid JWT that can't be decoded -- should fall back to stored email
	token := &OIDCTokenStore{
		AccessToken: "not-a-valid-jwt-at-all",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Email:       "fallback@example.com",
	}

	cleanup := setupWhoamiEnv(t, token)
	defer cleanup()

	cmd := NewWhoamiCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runWhoami with invalid JWT: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "fallback@example.com") {
		t.Errorf("expected fallback email, got:\n%s", output)
	}
}
