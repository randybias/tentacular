package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/randyb/pipedreamer2/pkg/k8s"
	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "View workflow pod logs",
		Args:  cobra.ExactArgs(1),
		RunE:  runLogs,
	}
	cmd.Flags().BoolP("follow", "f", false, "Stream logs in real time")
	cmd.Flags().Int64("tail", 100, "Number of recent log lines to show")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	follow, _ := cmd.Flags().GetBool("follow")
	tailLines, _ := cmd.Flags().GetInt64("tail")

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if follow {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()
	}

	stream, err := client.GetPodLogs(ctx, namespace, name, follow, tailLines)
	if err != nil {
		return fmt.Errorf("getting logs: %w", err)
	}
	defer stream.Close()

	_, err = io.Copy(os.Stdout, stream)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("reading logs: %w", err)
	}

	return nil
}
