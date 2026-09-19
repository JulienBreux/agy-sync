package cmd

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func newClearCommand() *cobra.Command {
	var (
		force          bool
		conversationID string
		projectID      string
		databaseID     string
	)

	clearCmd := &cobra.Command{
		Use:     "clear [conversation-id]",
		GroupID: "sync",
		Short:   "Clear conversations and artifacts from Firestore",
		Long: `Purges remote Firestore data stored by agy-sync, either for all conversations
or for a single conversation. Local files and SQLite databases are never touched.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var argID string
			if len(args) > 0 {
				argID = strings.TrimSpace(args[0])
			}
			targetConvID := cmp.Or(argID, conversationID)

			cfg, err := config.LoadConfig(globalOpts.ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if projectID != "" {
				cfg.ProjectID = projectID
			}
			if databaseID != "" {
				cfg.DatabaseID = databaseID
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			// Interactive confirmation unless --force / -f is passed
			if !force {
				targetDesc := "ALL conversations and subcollections"
				if targetConvID != "" {
					targetDesc = fmt.Sprintf("conversation '%s' and its subcollections", targetConvID)
				}

				dbDesc := cmp.Or(cfg.DatabaseID, "(default)")

				cmd.Printf("WARNING: This will permanently delete %s in Firestore!\n", targetDesc)
				cmd.Printf("Project:  %s\n", cfg.ProjectID)
				cmd.Printf("Database: %s\n\n", dbDesc)
				cmd.Printf("Are you sure you want to clear Firestore data for project '%s'? [y/N]: ", cfg.ProjectID)

				reader := bufio.NewReader(cmd.InOrStdin())
				input, err := reader.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return fmt.Errorf("failed reading confirmation: %w", err)
				}

				answer := strings.TrimSpace(strings.ToLower(input))
				if !slices.Contains([]string{"y", "yes"}, answer) {
					cmd.Println("Operation cancelled.")
					return nil
				}
			}

			client, err := newFirestoreClient(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed connecting to firestore: %w", err)
			}
			defer func() {
				_ = client.Close()
			}()

			engine := syncer.NewEngine(cfg, client)
			opts := syncer.ClearOptions{
				ConversationID: targetConvID,
			}

			result, err := engine.Clear(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("clear operation failed: %w", err)
			}

			if globalOpts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}

			if targetConvID != "" {
				cmd.Printf("Successfully cleared conversation %s from Firestore.\n", targetConvID)
			} else {
				cmd.Printf("Successfully cleared %d conversation(s) from Firestore.\n", result.ConversationsDeleted)
			}

			return nil
		},
	}

	clearCmd.Flags().BoolVarP(&force, "force", "f", false, "bypass confirmation prompt")
	clearCmd.Flags().StringVarP(&conversationID, "conversation", "c", "", "specific conversation ID to clear")
	clearCmd.Flags().StringVar(&projectID, "project-id", "", "Google Cloud project ID (overrides config)")
	clearCmd.Flags().StringVar(&databaseID, "database-id", "", "Firestore database ID (overrides config)")

	return clearCmd
}
