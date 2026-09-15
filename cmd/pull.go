package cmd

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func newPullCommand() *cobra.Command {
	var conversationID string

	pullCmd := &cobra.Command{
		Use:     "pull [conversation-id]",
		GroupID: "sync",
		Short:   "Pull Antigravity conversation and artifacts from Firestore",
		Long: `Downloads conversation transcripts and artifacts from Firestore and reconstructs
the local Antigravity brain directory and JSONL log structure.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var argID string
			if len(args) > 0 {
				argID = strings.TrimSpace(args[0])
			}
			targetConvID := cmp.Or(argID, conversationID)

			if targetConvID == "" {
				return errors.New("conversation ID is required: specify as argument or with --conversation")
			}

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
			opts := syncer.PullOptions{
				ConversationID: targetConvID,
			}

			if !globalOpts.JSON {
				cmd.Printf("Pulling conversation %s from Firestore...\n", targetConvID)
			}

			result, err := engine.Pull(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("sync pull failed: %w", err)
			}

			if globalOpts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			cmd.Printf("Sync pull complete for %s:\n", result.ConversationID)
			cmd.Printf("  Steps pulled:     %d\n", result.StepsPulled)
			cmd.Printf("  Artifacts pulled: %d\n", result.ArtifactsPulled)
			cmd.Printf("  Target directory: %s\n", result.TargetDirectory)

			return nil
		},
	}

	pullCmd.Flags().StringVarP(&conversationID, "conversation", "c", "", "Conversation ID to pull")

	return pullCmd
}
