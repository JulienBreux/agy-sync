package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/config"
	"github.com/julienbreux/ayg-conv-to-fs/pkg/syncer"
	"github.com/julienbreux/ayg-conv-to-fs/pkg/watcher"
)

func newWatchCommand() *cobra.Command {
	var (
		pollInterval     time.Duration
		debounceDuration time.Duration
	)

	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch Antigravity brain directory and auto-sync with Firestore in real-time",
		Long: `Runs as a persistent daemon monitoring local conversation modifications and
remote Firestore changes, synchronizing updates continuously with loop prevention.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(globalOpts.ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			client, err := newFirestoreClient(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed connecting to firestore: %w", err)
			}
			defer func() {
				_ = client.Close()
			}()

			w, err := watcher.NewWatcher(cfg.BrainDir, debounceDuration)
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
			cmd.Printf("  Polling interval:    %s\n", pollInterval)

			// Initial push of any pending local turns
			_, _ = engine.Push(cmd.Context(), syncer.PushOptions{})

			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()

			ctx := cmd.Context()
			for {
				select {
				case <-ctx.Done():
					cmd.Println("Shutting down synchronization daemon...")
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
					// Push updated conversation
					_, _ = engine.Push(ctx, syncer.PushOptions{
						ConversationID: event.ConversationID,
					})

				case <-ticker.C:
					// Check remote changes
					_, _ = engine.SyncRemoteChanges(ctx)
				}
			}
		},
	}

	watchCmd.Flags().DurationVar(&pollInterval, "interval", 3*time.Second, "Remote Firestore check interval")
	watchCmd.Flags().DurationVar(&debounceDuration, "debounce", 200*time.Millisecond, "Filesystem event debounce interval")

	return watchCmd
}
