package cli

import (
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

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Email:       %s\n", email)
	if name != "" {
		fmt.Fprintf(out, "Name:        %s\n", name)
	}
	fmt.Fprintf(out, "Subject:     %s\n", claims.Sub)
	if provider != "" {
		fmt.Fprintf(out, "Provider:    %s\n", provider)
	}
	fmt.Fprintf(out, "Environment: %s\n", envName)

	if store.IsExpired() {
		fmt.Fprintln(out, "Token:       EXPIRED (run 'tntc login' to re-authenticate)")
	} else {
		expiresAt := time.Unix(claims.Exp, 0)
		fmt.Fprintf(out, "Expires:     %s\n", expiresAt.Format(time.RFC3339))
	}

	return nil
}
