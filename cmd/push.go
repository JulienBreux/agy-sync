package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/syncer"
)

var newFirestoreClient = func(ctx context.Context, cfg *config.Config) (firestore.Repository, error) {
	return firestore.NewClient(ctx, cfg)
}

func newPushCommand() *cobra.Command {
	var conversationID string

	pushCmd := &cobra.Command{
		Use:     "push",
		GroupID: "sync",
		Short:   "Push local Antigravity conversations and artifacts to Firestore",
		Long: `Scans the configured brain directory for conversation transcripts and artifacts,
and pushes them to Firestore incrementally.`,
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

			engine := syncer.NewEngine(cfg, client)
			opts := syncer.PushOptions{
				ConversationID: conversationID,
			}

			if !globalOpts.JSON {
				cmd.Println("Starting sync push to Firestore...")
			}

			result, err := engine.Push(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("sync push failed: %w", err)
			}

			if globalOpts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			cmd.Printf("Sync push complete: %d conversations, %d steps, %d artifacts pushed.\n",
				result.ConversationsSynced, result.StepsSynced, result.ArtifactsSynced)

			if len(result.Errors) > 0 {
				cmd.PrintErrf("Encountered %d errors during sync:\n", len(result.Errors))
				for _, syncErr := range result.Errors {
					cmd.PrintErrf("  - %s\n", syncErr.Error())
				}
			}

			return nil
		},
	}

	pushCmd.Flags().StringVarP(&conversationID, "conversation", "c", "", "Specific conversation ID to push")

	return pushCmd
}
