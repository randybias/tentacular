package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// NewWhoamiCmd creates the "whoami" cobra command.
func NewWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the currently authenticated user",
		RunE:  runWhoami,
	}
	return cmd
}

// whoamiResult is the structured output for JSON mode.
type whoamiResult struct {
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Environment string `json:"environment"`
	Expired     bool   `json:"expired"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

func runWhoami(cmd *cobra.Command, args []string) error {
	envName := flagString(cmd, "env")
	if envName == "" {
		envName = os.Getenv("TENTACULAR_ENV")
	}
	cfg := LoadConfig()
	if envName == "" {
		envName = cfg.DefaultEnv
	}
	if envName == "" {
		envName = "default"
	}

	outputFormat := flagString(cmd, "output")

	store, err := LoadOIDCToken(envName)
	if err != nil {
		return fmt.Errorf("reading tokens: %w", err)
	}
	if store == nil {
		return fmt.Errorf("not authenticated for environment %q; run 'tntc login -e %s'", envName, envName)
	}

	// Decode claims from access token for fresh info
	claims, err := DecodeJWTClaims(store.AccessToken)
	if err != nil {
		// Fall back to stored email
		if outputFormat == "json" {
			result := whoamiResult{
				Email:       store.Email,
				Environment: envName,
				Expired:     store.IsExpired(),
			}
			if !store.IsExpired() {
				result.ExpiresAt = store.ExpiresAt.Format(time.RFC3339)
			}
			return emitWhoamiJSON(cmd, result)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Email:       %s\n", store.Email)
		fmt.Fprintf(cmd.OutOrStdout(), "Environment: %s\n", envName)
		if store.IsExpired() {
			fmt.Fprintln(cmd.OutOrStdout(), "Token:       EXPIRED")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Expires:     %s\n", store.ExpiresAt.Format(time.RFC3339))
		}
		return nil
	}

	email := claims.Email
	if email == "" {
		email = claims.PreferredUsername
	}
	if email == "" {
		email = store.Email
	}

	name := claims.Name
	provider := claims.IdentityProvider
	issuer := claims.Iss

	expired := store.IsExpired()
	var expiresAt string
	if !expired {
		expiresAt = time.Unix(claims.Exp, 0).Format(time.RFC3339)
	}

	if outputFormat == "json" {
		result := whoamiResult{
			Email:       email,
			Name:        name,
			Subject:     claims.Sub,
			Issuer:      issuer,
			Provider:    provider,
			Environment: envName,
			Expired:     expired,
			ExpiresAt:   expiresAt,
		}
		return emitWhoamiJSON(cmd, result)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Email:       %s\n", email)
	if name != "" {
		fmt.Fprintf(out, "Name:        %s\n", name)
	}
	fmt.Fprintf(out, "Subject:     %s\n", claims.Sub)
	if issuer != "" {
		fmt.Fprintf(out, "Issuer:      %s\n", issuer)
	}
	if provider != "" {
		fmt.Fprintf(out, "Provider:    %s\n", provider)
	}
	fmt.Fprintf(out, "Environment: %s\n", envName)

	if expired {
		fmt.Fprintln(out, "Token:       EXPIRED (run 'tntc login' to re-authenticate)")
	} else {
		fmt.Fprintf(out, "Expires:     %s\n", expiresAt)
	}

	return nil
}

// emitWhoamiJSON marshals and writes the whoami result as JSON.
func emitWhoamiJSON(cmd *cobra.Command, result whoamiResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling whoami result: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
