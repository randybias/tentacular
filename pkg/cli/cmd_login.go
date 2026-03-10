package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewLoginCmd creates the "login" cobra command for OIDC device authorization.
func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate via OIDC device authorization flow",
		Long: `Authenticate to the tentacular platform using OIDC device authorization.

This opens a browser window where you can sign in via Google SSO (or other
identity providers configured in Keycloak). The CLI polls for completion
and stores the resulting tokens locally.

Requires oidc_issuer, oidc_client_id, and oidc_client_secret to be configured
in the environment config.`,
		RunE: runLogin,
	}
	return cmd
}

// deviceAuthResponse is the JSON response from the device authorization endpoint.
type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// tokenResponse is the JSON response from the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func runLogin(cmd *cobra.Command, args []string) error {
	envName := flagString(cmd, "env")
	env, issuer, clientID, clientSecret, err := resolveOIDCConfig(envName)
	if err != nil {
		return err
	}
	if env == "" {
		env = "default"
	}

	// Step 1: Discover device auth and token endpoints
	deviceEndpoint, tokenEndpoint, err := discoverOIDCEndpoints(issuer)
	if err != nil {
		return fmt.Errorf("OIDC discovery failed: %w", err)
	}

	// Step 2: Request device authorization
	deviceAuth, err := requestDeviceAuth(deviceEndpoint, clientID, clientSecret)
	if err != nil {
		return fmt.Errorf("device authorization request failed: %w", err)
	}

	// Step 3: Display instructions
	verifyURL := deviceAuth.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = deviceAuth.VerificationURI
	}
	fmt.Fprintf(cmd.OutOrStdout(), "To authenticate, visit: %s\n", verifyURL)
	fmt.Fprintf(cmd.OutOrStdout(), "Enter code: %s\n\n", deviceAuth.UserCode)
	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for authentication...")

	// Step 4: Open browser
	openBrowser(verifyURL)

	// Step 5: Poll for token
	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second)

	tokenResp, err := pollForToken(tokenEndpoint, deviceAuth.DeviceCode, clientID, clientSecret, interval, deadline)
	if err != nil {
		return err
	}

	// Step 6: Extract email from token
	claims, err := DecodeJWTClaims(tokenResp.AccessToken)
	if err != nil {
		return fmt.Errorf("decoding access token: %w", err)
	}

	email := claims.Email
	if email == "" {
		email = claims.PreferredUsername
	}

	// Step 7: Store tokens
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	store := &OIDCTokenStore{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		Email:        email,
	}
	if err := SaveOIDCToken(env, store); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nAuthenticated as %s\n", email)
	return nil
}

// resolveOIDCConfig resolves OIDC configuration for the given environment.
// Returns envName, issuer, clientID, clientSecret.
func resolveOIDCConfig(envName string) (string, string, string, string, error) {
	if envName == "" {
		envName = os.Getenv("TENTACULAR_ENV")
	}
	cfg := LoadConfig()
	if envName == "" {
		envName = cfg.DefaultEnv
	}

	if envName == "" {
		return "", "", "", "", fmt.Errorf("no environment specified; use -e <env> or set TENTACULAR_ENV")
	}

	env, ok := cfg.Environments[envName]
	if !ok {
		return "", "", "", "", fmt.Errorf("environment %q not found in config", envName)
	}

	if env.OIDCIssuer == "" {
		return "", "", "", "", fmt.Errorf("OIDC not configured for environment %q; add oidc_issuer, oidc_client_id, oidc_client_secret to environment config", envName)
	}
	if env.OIDCClientID == "" {
		return "", "", "", "", fmt.Errorf("oidc_client_id not configured for environment %q", envName)
	}

	return envName, env.OIDCIssuer, env.OIDCClientID, env.OIDCClientSecret, nil
}

// discoverOIDCEndpoints fetches the OIDC discovery document and returns
// the device_authorization_endpoint and token_endpoint.
func discoverOIDCEndpoints(issuer string) (string, string, error) {
	discoveryURL := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching discovery document: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("discovery endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading discovery response: %w", err)
	}

	var doc struct {
		DeviceAuthEndpoint string `json:"device_authorization_endpoint"`
		TokenEndpoint      string `json:"token_endpoint"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", fmt.Errorf("parsing discovery document: %w", err)
	}

	if doc.DeviceAuthEndpoint == "" {
		return "", "", fmt.Errorf("OIDC provider does not support device authorization flow")
	}
	if doc.TokenEndpoint == "" {
		return "", "", fmt.Errorf("OIDC provider did not return a token endpoint")
	}

	return doc.DeviceAuthEndpoint, doc.TokenEndpoint, nil
}

// requestDeviceAuth initiates the device authorization flow.
func requestDeviceAuth(endpoint, clientID, clientSecret string) (*deviceAuthResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"openid email profile"},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device authorization: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var authResp deviceAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("parsing device auth response: %w", err)
	}

	if authResp.DeviceCode == "" {
		return nil, fmt.Errorf("device auth response missing device_code")
	}

	return &authResp, nil
}

// pollForToken polls the token endpoint until the user completes authorization.
func pollForToken(tokenEndpoint, deviceCode, clientID, clientSecret string, interval time.Duration, deadline time.Time) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("device authorization timed out; please try again")
		}

		time.Sleep(interval)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("polling token endpoint: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("reading token response: %w", err)
		}

		var tokenResp tokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return nil, fmt.Errorf("parsing token response: %w", err)
		}

		switch tokenResp.Error {
		case "":
			// Success
			if tokenResp.AccessToken == "" {
				return nil, fmt.Errorf("token response missing access_token")
			}
			return &tokenResp, nil
		case "authorization_pending":
			// User hasn't completed auth yet, continue polling
			continue
		case "slow_down":
			// Server wants us to slow down
			interval += 5 * time.Second
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired; please run 'tntc login' again")
		case "access_denied":
			return nil, fmt.Errorf("authorization denied by user")
		default:
			desc := tokenResp.ErrorDesc
			if desc == "" {
				desc = tokenResp.Error
			}
			return nil, fmt.Errorf("token endpoint error: %s", desc)
		}
	}
}

// RefreshOIDCToken attempts to refresh an expired access token using the refresh token.
// Returns the updated token store, or an error if refresh fails.
func RefreshOIDCToken(envName string, store *OIDCTokenStore) (*OIDCTokenStore, error) {
	_, issuer, clientID, clientSecret, err := resolveOIDCConfig(envName)
	if err != nil {
		return nil, err
	}

	_, tokenEndpoint, err := discoverOIDCEndpoints(issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed during refresh: %w", err)
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {store.RefreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading refresh response: %w", err)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("refresh failed: %s", tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access_token")
	}

	claims, err := DecodeJWTClaims(tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("decoding refreshed token: %w", err)
	}

	email := claims.Email
	if email == "" {
		email = store.Email
	}

	newStore := &OIDCTokenStore{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Email:        email,
	}
	// Keep old refresh token if server didn't return a new one
	if newStore.RefreshToken == "" {
		newStore.RefreshToken = store.RefreshToken
	}

	if err := SaveOIDCToken(envName, newStore); err != nil {
		return nil, fmt.Errorf("saving refreshed token: %w", err)
	}

	return newStore, nil
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	// Best effort — don't fail login if browser can't open
	_ = cmd.Start()
}
