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
	clusterName := flagString(cmd, "cluster")
	if clusterName == "" {
		clusterName = os.Getenv("TENTACULAR_CLUSTER")
	}
	cfg := LoadConfig()
	if clusterName == "" {
		clusterName = cfg.DefaultCluster
	}
	if clusterName == "" {
		clusterName = "default"
	}

	if err := RemoveOIDCToken(clusterName); err != nil {
		return fmt.Errorf("removing tokens: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Logged out of environment %q\n", clusterName)
	return nil
}
