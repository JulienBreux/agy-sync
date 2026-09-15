package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/daemon"
)

type stopOptions struct {
	pidFile string
	logFile string
	timeout time.Duration
}

func newStopCommand() *cobra.Command {
	opts := stopOptions{}

	stopCmd := &cobra.Command{
		Use:     "stop",
		GroupID: "daemon",
		Short:   "Stop the running background synchronization daemon",
		Long: `Locates the running agy-sync daemon via its PID file, sends a graceful SIGTERM signal,
waits for clean shutdown, and removes the PID file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := daemon.NewManager(opts.pidFile, opts.logFile)

			running, pid, err := mgr.IsRunning()
			if err != nil {
				return fmt.Errorf("failed to check daemon status: %w", err)
			}
			if !running {
				if globalOpts.JSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{
						"status":  "STOPPED",
						"message": "daemon is not running",
					})
				}
				cmd.Println("agy-sync daemon is not running.")
				return nil
			}

			if err := mgr.Stop(opts.timeout); err != nil {
				return fmt.Errorf("failed to stop daemon: %w", err)
			}

			if globalOpts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"status":  "STOPPED",
					"pid":     pid,
					"message": "daemon stopped successfully",
				})
			}

			cmd.Printf("agy-sync daemon (PID: %d) stopped successfully.\n", pid)
			return nil
		},
	}

	stopCmd.Flags().StringVar(&opts.pidFile, "pid-file", daemon.DefaultPIDPath(), "Path to PID file")
	stopCmd.Flags().StringVar(&opts.logFile, "log-file", daemon.DefaultLogPath(), "Path to daemon log file")
	stopCmd.Flags().DurationVar(&opts.timeout, "timeout", 5*time.Second, "Timeout waiting for graceful shutdown")

	return stopCmd
}
