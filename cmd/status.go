package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/julienbreux/agy-sync/internal/daemon"
	"github.com/julienbreux/agy-sync/internal/discovery"
	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

// DaemonStatus represents the runtime status of the background synchronization daemon.
type DaemonStatus struct {
	State        string     `json:"state"`
	PID          int        `json:"pid,omitempty"`
	LogFile      string     `json:"log_file"`
	PIDFile      string     `json:"pid_file"`
	LastPolledAt *time.Time `json:"last_polled_at,omitempty"`
}

// ConversationStatus summarizes sync state for a single conversation.
type ConversationStatus struct {
	ID             string `json:"id"`
	LocalSteps     int    `json:"local_steps"`
	RemoteSteps    int    `json:"remote_steps"`
	ArtifactsCount int    `json:"artifacts_count"`
	Synced         bool   `json:"synced"`
	HasLocalDB     bool   `json:"has_local_db"`
}

// StatusReport represents the overall system synchronization status.
type StatusReport struct {
	Daemon             DaemonStatus         `json:"daemon"`
	ProjectID          string               `json:"project_id"`
	MachineID          string               `json:"machine_id"`
	BrainDir           string               `json:"brain_dir"`
	ConversationsDir   string               `json:"conversations_dir"`
	SummariesDB        string               `json:"summaries_db"`
	DBSyncEnabled      bool                 `json:"db_sync_enabled"`
	ConversationsCount int                  `json:"conversations_count"`
	Conversations      []ConversationStatus `json:"conversations"`
}

type statusOptions struct {
	conversationID   string
	pidFile          string
	logFile          string
	stateFile        string
	noDBSync         bool
	conversationsDir string
	summariesDB      string
}

func newStatusCommand() *cobra.Command {
	opts := statusOptions{}

	statusCmd := &cobra.Command{
		Use:     "status [conversation-id]",
		GroupID: "daemon",
		Short:   "Display sync status between local brain and remote Firestore",
		Long: `Inspects local Antigravity conversation sessions, compares their step and
artifact counts against remote Firestore metadata, and reports daemon process health.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.conversationID = args[0]
			}

			cfg, err := config.LoadConfig(globalOpts.ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w (remediation: run 'agy-sync init' or specify --config)", err)
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
				return fmt.Errorf("invalid configuration: %w (remediation: check ~/.config/agy-sync/config.yaml)", err)
			}

			mgr := daemon.NewManagerWithState(opts.pidFile, opts.logFile, opts.stateFile)
			daemonState, daemonPID, logFile, err := mgr.Status()
			if err != nil {
				daemonState = daemon.StateStopped
			}
			rtState, _ := mgr.GetRuntimeState()

			client, err := newFirestoreClient(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed connecting to firestore: %w (remediation: check GCP project ID and ADC credentials)", err)
			}
			defer func() {
				_ = client.Close()
			}()

			var discovered []discovery.DiscoveredConversation
			if opts.conversationID != "" {
				single, err := discovery.DiscoverConversation(cfg.BrainDir, opts.conversationID)
				if err != nil {
					return fmt.Errorf("failed locating conversation %s: %w", opts.conversationID, err)
				}
				discovered = []discovery.DiscoveredConversation{*single}
			} else {
				discovered, err = discovery.DiscoverConversations(cfg.BrainDir)
				if err != nil {
					return fmt.Errorf("failed scanning brain directory: %w", err)
				}
			}

			// Batch fetch remote conversations to avoid N sequential round-trips
			remoteMap := make(map[string]*models.Conversation)
			queryCtx, queryCancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer queryCancel()

			if opts.conversationID != "" {
				rc, errGet := client.GetConversation(queryCtx, opts.conversationID)
				if errGet == nil && rc != nil {
					remoteMap[rc.ID] = rc
				}
			} else {
				remoteConvs, errList := client.ListConversations(queryCtx)
				if errList == nil {
					for _, rc := range remoteConvs {
						remoteMap[rc.ID] = rc
					}
				}
			}

			p := parser.NewTranscriptParser()
			report := &StatusReport{
				Daemon: DaemonStatus{
					State:        string(daemonState),
					PID:          daemonPID,
					LogFile:      logFile,
					PIDFile:      mgr.PIDFile,
					LastPolledAt: rtState.LastPolledAt,
				},
				ProjectID:          cfg.ProjectID,
				MachineID:          cfg.MachineID,
				BrainDir:           cfg.BrainDir,
				ConversationsDir:   cfg.ConversationsDir,
				SummariesDB:        cfg.SummariesDB,
				DBSyncEnabled:      !cfg.NoDBSync,
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

				if remoteConv, found := remoteMap[d.ID]; found && remoteConv != nil {
					remoteSteps = remoteConv.LastSyncedStep + 1
					if remoteSteps >= localSteps && localSteps > 0 {
						synced = true
					}
				}

				hasLocalDB := false
				if cfg.ConversationsDir != "" {
					dbPath := filepath.Join(cfg.ConversationsDir, d.ID+".db")
					if _, err := os.Stat(dbPath); err == nil {
						hasLocalDB = true
					}
				}

				report.Conversations = append(report.Conversations, ConversationStatus{
					ID:             d.ID,
					LocalSteps:     localSteps,
					RemoteSteps:    remoteSteps,
					ArtifactsCount: len(d.Artifacts),
					Synced:         synced,
					HasLocalDB:     hasLocalDB,
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
			if report.Daemon.State == string(daemon.StateRunning) {
				cmd.Printf("Daemon Status:   RUNNING (PID: %d)\n", report.Daemon.PID)
			} else {
				cmd.Println("Daemon Status:   STOPPED")
			}
			if report.Daemon.LastPolledAt != nil {
				cmd.Printf("Last Polling:    %s (%s ago)\n",
					report.Daemon.LastPolledAt.Format("2006-01-02 15:04:05 UTC"),
					time.Since(*report.Daemon.LastPolledAt).Truncate(time.Second),
				)
			} else {
				cmd.Println("Last Polling:    Never / Inactive")
			}
			cmd.Printf("Daemon Log:      %s\n", report.Daemon.LogFile)
			cmd.Printf("GCP Project ID:  %s\n", report.ProjectID)
			cmd.Printf("Machine ID:      %s\n", report.MachineID)
			cmd.Printf("Brain Directory: %s\n", report.BrainDir)
			cmd.Printf("Conversations:   %s\n", report.ConversationsDir)
			cmd.Printf("Summaries DB:    %s\n", report.SummariesDB)
			dbSyncStr := "Enabled"
			if !report.DBSyncEnabled {
				dbSyncStr = "Disabled"
			}
			cmd.Printf("SQLite DB Sync:  %s\n", dbSyncStr)
			cmd.Printf("Sessions Found:  %d\n\n", report.ConversationsCount)

			if len(report.Conversations) == 0 {
				cmd.Println("No conversations found in brain directory.")
				return nil
			}

			cmd.Printf("%-38s %-12s %-12s %-10s %-10s %-8s\n", "CONVERSATION ID", "LOCAL STEPS", "REMOTE STEPS", "ARTIFACTS", "LOCAL DB", "SYNCED")
			cmd.Println(strings.Repeat("-", 94))

			for _, c := range report.Conversations {
				syncedStr := "No"
				if c.Synced {
					syncedStr = "Yes"
				}
				dbStr := "No"
				if c.HasLocalDB {
					dbStr = "Yes"
				}
				cmd.Printf("%-38s %-12d %-12d %-10d %-10s %-8s\n",
					c.ID, c.LocalSteps, c.RemoteSteps, c.ArtifactsCount, dbStr, syncedStr)
			}

			return nil
		},
	}

	statusCmd.Flags().StringVarP(&opts.conversationID, "conversation", "c", "", "Optional conversation ID to filter status")
	statusCmd.Flags().StringVar(&opts.pidFile, "pid-file", daemon.DefaultPIDPath(), "Path to PID file")
	statusCmd.Flags().StringVar(&opts.logFile, "log-file", daemon.DefaultLogPath(), "Path to daemon log file")
	statusCmd.Flags().StringVar(&opts.stateFile, "state-file", daemon.DefaultStatePath(), "Path to daemon runtime state file")
	statusCmd.Flags().BoolVar(&opts.noDBSync, "no-db-sync", false, "Disable SQLite database sync display")
	statusCmd.Flags().StringVar(&opts.conversationsDir, "conversations-dir", "", "Path to local conversations directory")
	statusCmd.Flags().StringVar(&opts.summariesDB, "summaries-db", "", "Path to conversation summaries SQLite database")

	return statusCmd
}
