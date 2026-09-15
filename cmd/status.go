package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/discovery"
	"github.com/julienbreux/agy-sync/pkg/parser"
)

// ConversationStatus summarizes sync state for a single conversation.
type ConversationStatus struct {
	ID             string `json:"id"`
	LocalSteps     int    `json:"local_steps"`
	RemoteSteps    int    `json:"remote_steps"`
	ArtifactsCount int    `json:"artifacts_count"`
	Synced         bool   `json:"synced"`
}

// StatusReport represents the overall system synchronization status.
type StatusReport struct {
	ProjectID          string               `json:"project_id"`
	MachineID          string               `json:"machine_id"`
	BrainDir           string               `json:"brain_dir"`
	ConversationsCount int                  `json:"conversations_count"`
	Conversations      []ConversationStatus `json:"conversations"`
}

func newStatusCommand() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Display sync status between local brain and remote Firestore",
		Long: `Inspects local Antigravity conversation sessions and compares their step and
artifact counts against remote Firestore metadata.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(globalOpts.ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w (remediation: run 'agy-sync init' or specify --config)", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w (remediation: check ~/.config/agy-sync/config.yaml)", err)
			}

			client, err := newFirestoreClient(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed connecting to firestore: %w (remediation: check GCP project ID and ADC credentials)", err)
			}
			defer func() {
				_ = client.Close()
			}()

			discovered, err := discovery.DiscoverConversations(cfg.BrainDir)
			if err != nil {
				return fmt.Errorf("failed scanning brain directory: %w", err)
			}

			p := parser.NewTranscriptParser()
			report := &StatusReport{
				ProjectID:          cfg.ProjectID,
				MachineID:          cfg.MachineID,
				BrainDir:           cfg.BrainDir,
				ConversationsCount: len(discovered),
				Conversations:      make([]ConversationStatus, 0, len(discovered)),
			}

			for _, d := range discovered {
				localSteps := 0
				if d.HasTranscript {
					res, err := p.ParseFile(d.TranscriptPath)
					if err == nil {
						localSteps = len(res.Steps)
					}
				}

				remoteSteps := 0
				synced := false

				remoteConv, err := client.GetConversation(cmd.Context(), d.ID)
				if err == nil && remoteConv != nil {
					remoteSteps = remoteConv.LastSyncedStep + 1
					if remoteSteps >= localSteps && localSteps > 0 {
						synced = true
					}
				}

				report.Conversations = append(report.Conversations, ConversationStatus{
					ID:             d.ID,
					LocalSteps:     localSteps,
					RemoteSteps:    remoteSteps,
					ArtifactsCount: len(d.Artifacts),
					Synced:         synced,
				})
			}

			if globalOpts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			cmd.Println("==================================================")
			cmd.Println("          Antigravity Sync Status                ")
			cmd.Println("==================================================")
			cmd.Printf("GCP Project ID:  %s\n", report.ProjectID)
			cmd.Printf("Machine ID:      %s\n", report.MachineID)
			cmd.Printf("Brain Directory: %s\n", report.BrainDir)
			cmd.Printf("Sessions Found:  %d\n\n", report.ConversationsCount)

			if len(report.Conversations) == 0 {
				cmd.Println("No conversations found in brain directory.")
				return nil
			}

			cmd.Printf("%-38s %-12s %-12s %-10s %-8s\n", "CONVERSATION ID", "LOCAL STEPS", "REMOTE STEPS", "ARTIFACTS", "SYNCED")
			cmd.Println(strings.Repeat("-", 84))

			for _, c := range report.Conversations {
				syncedStr := "No"
				if c.Synced {
					syncedStr = "Yes"
				}
				cmd.Printf("%-38s %-12d %-12d %-10d %-8s\n",
					c.ID, c.LocalSteps, c.RemoteSteps, c.ArtifactsCount, syncedStr)
			}

			return nil
		},
	}

	return statusCmd
}
