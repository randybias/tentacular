package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewLogoutCmd creates the "logout" cobra command.
func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored OIDC tokens for the current environment",
		RunE:  runLogout,
	}
	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
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

	if err := RemoveOIDCToken(envName); err != nil {
		return fmt.Errorf("removing tokens: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Logged out of environment %q\n", envName)
	return nil
}
