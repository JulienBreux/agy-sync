package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/internal/daemon"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/internal/watcher"
)

type startOptions struct {
	foreground       bool
	pollInterval     time.Duration
	debounceDuration time.Duration
	pidFile          string
	logFile          string
	stateFile        string
	noDBSync         bool
	conversationsDir string
	summariesDB      string
}

func newStartCommand() *cobra.Command {
	opts := startOptions{}

	startCmd := &cobra.Command{
		Use:     "start",
		GroupID: "daemon",
		Short:   "Start the background synchronization daemon",
		Long: `Launches the Antigravity background synchronization engine. By default, it spawns
a detached daemon process monitoring the brain directory and synchronizing with Cloud Firestore.
Use -f / --foreground to run directly in the current terminal session.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := daemon.NewManagerWithState(opts.pidFile, opts.logFile, opts.stateFile)

			if !opts.foreground {
				// Background daemon mode
				running, pid, err := mgr.IsRunning()
				if err != nil {
					return fmt.Errorf("failed to check daemon status: %w", err)
				}
				if running {
					return fmt.Errorf("daemon is already running (PID: %d)", pid)
				}

				binPath, err := os.Executable()
				if err != nil {
					return fmt.Errorf("failed to determine executable path: %w", err)
				}

				bgArgs := []string{
					"start",
					"-f",
					"--config", globalOpts.ConfigFile,
					"--pid-file", mgr.PIDFile,
					"--log-file", mgr.LogFile,
					"--interval", opts.pollInterval.String(),
					"--debounce", opts.debounceDuration.String(),
				}
				if cmd.Flags().Changed("no-db-sync") && opts.noDBSync {
					bgArgs = append(bgArgs, "--no-db-sync")
				}
				if opts.conversationsDir != "" {
					bgArgs = append(bgArgs, "--conversations-dir", opts.conversationsDir)
				}
				if opts.summariesDB != "" {
					bgArgs = append(bgArgs, "--summaries-db", opts.summariesDB)
				}
				if globalOpts.Verbose {
					bgArgs = append(bgArgs, "-v")
				}

				childPID, err := mgr.StartBackground(binPath, bgArgs)
				if err != nil {
					return fmt.Errorf("failed to start daemon: %w", err)
				}

				if globalOpts.JSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{
						"status":   "RUNNING",
						"pid":      childPID,
						"pid_file": mgr.PIDFile,
						"log_file": mgr.LogFile,
					})
				}

				cmd.Printf("agy-sync daemon started successfully (PID: %d)\n", childPID)
				cmd.Printf("  Logs:     %s\n", mgr.LogFile)
				cmd.Printf("  PID file: %s\n", mgr.PIDFile)
				return nil
			}

			// Foreground execution mode
			cfg, err := config.LoadConfig(globalOpts.ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if cmd.Flags().Changed("no-db-sync") {
				cfg.NoDBSync = opts.noDBSync
			}
			if opts.conversationsDir != "" {
				cfg.ConversationsDir = opts.conversationsDir
			}
			if opts.summariesDB != "" {
				cfg.SummariesDB = opts.summariesDB
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			if err := mgr.WritePID(os.Getpid()); err != nil {
				return fmt.Errorf("failed to write PID file: %w", err)
			}
			defer func() {
				_ = mgr.RemovePID()
			}()

			client, err := newFirestoreClient(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed connecting to firestore: %w", err)
			}
			defer func() {
				_ = client.Close()
			}()

			w, err := watcher.NewWatcher(cfg.BrainDir, opts.debounceDuration)
			if err != nil {
				return fmt.Errorf("failed creating filesystem watcher: %w", err)
			}
			defer func() {
				_ = w.Close()
			}()

			events, errs, err := w.Start(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed starting filesystem watcher: %w", err)
			}

			engine := syncer.NewEngine(cfg, client)

			cmd.Println("Starting real-time synchronization daemon...")
			cmd.Printf("  Monitoring directory: %s\n", cfg.BrainDir)
			cmd.Printf("  Machine ID:          %s\n", cfg.MachineID)
			cmd.Printf("  Polling interval:    %s\n", opts.pollInterval)

			// Initial push of any pending local turns
			_ = mgr.RecordPoll(time.Now().UTC())
			_, _ = engine.Push(cmd.Context(), syncer.PushOptions{})

			ticker := time.NewTicker(opts.pollInterval)
			defer ticker.Stop()

			defer cmd.Println("Shutting down synchronization daemon...")

			ctx := cmd.Context()
			for {
				select {
				case <-ctx.Done():
					return nil

				case watchErr, ok := <-errs:
					if !ok {
						return nil
					}
					cmd.PrintErrf("Watcher error: %v\n", watchErr)

				case event, ok := <-events:
					if !ok {
						return nil
					}
					if globalOpts.Verbose {
						cmd.Printf("Detected local modification in %s\n", event.ConversationID)
					}
					_, _ = engine.Push(ctx, syncer.PushOptions{
						ConversationID: event.ConversationID,
					})

				case <-ticker.C:
					_ = mgr.RecordPoll(time.Now().UTC())
					_, _ = engine.SyncRemoteChanges(ctx)
				}
			}
		},
	}

	startCmd.Flags().BoolVarP(&opts.foreground, "foreground", "f", false, "Run daemon in the foreground")
	startCmd.Flags().DurationVar(&opts.pollInterval, "interval", 3*time.Second, "Remote Firestore check interval")
	startCmd.Flags().DurationVar(&opts.debounceDuration, "debounce", 200*time.Millisecond, "Filesystem event debounce interval")
	startCmd.Flags().StringVar(&opts.pidFile, "pid-file", daemon.DefaultPIDPath(), "Path to PID file")
	startCmd.Flags().StringVar(&opts.logFile, "log-file", daemon.DefaultLogPath(), "Path to daemon log file")
	startCmd.Flags().StringVar(&opts.stateFile, "state-file", daemon.DefaultStatePath(), "Path to daemon runtime state file")
	startCmd.Flags().BoolVar(&opts.noDBSync, "no-db-sync", false, "Disable SQLite database reconstruction")
	startCmd.Flags().StringVar(&opts.conversationsDir, "conversations-dir", "", "Path to local conversations directory")
	startCmd.Flags().StringVar(&opts.summariesDB, "summaries-db", "", "Path to conversation summaries SQLite database")

	return startCmd
}
