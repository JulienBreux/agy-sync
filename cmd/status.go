package cmd

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/julienbreux/agy-sync/internal/daemon"
	"github.com/julienbreux/agy-sync/internal/discovery"
	"github.com/julienbreux/agy-sync/internal/pager"
	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

var isTerminalFunc = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

// SetIsTerminalFunc allows overriding terminal detection in tests.
func SetIsTerminalFunc(fn func() bool) {
	isTerminalFunc = fn
}

// ResetIsTerminalFunc restores standard terminal detection.
func ResetIsTerminalFunc() {
	isTerminalFunc = func() bool {
		return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
	}
}

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
	Conversations      []ConversationStatus `json:"conversations,omitempty"`
}

type statusOptions struct {
	conversationID   string
	full             bool
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
			cfg.ConversationsDir = cmp.Or(opts.conversationsDir, cfg.ConversationsDir)
			cfg.SummariesDB = cmp.Or(opts.summariesDB, cfg.SummariesDB)

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
				if !opts.full {
					report.Conversations = nil
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			if !opts.full {
				var buf bytes.Buffer
				renderStatusSummary(report, &buf)
				if report.ConversationsCount > 0 {
					buf.WriteString("\nRun 'agy-sync status --full' to inspect conversations.\n")
				}
				_, err := cmd.OutOrStdout().Write(buf.Bytes())
				return err
			}

			if len(report.Conversations) == 0 {
				var buf bytes.Buffer
				renderStatusSummary(report, &buf)
				buf.WriteString("\nNo conversations found in brain directory.\n")
				_, err := cmd.OutOrStdout().Write(buf.Bytes())
				return err
			}

			isTTY := isTerminalFunc()
			pg := pager.New(pager.Options{
				IsTTY:           isTTY,
				In:              cmd.InOrStdin(),
				Out:             cmd.OutOrStdout(),
				Overhead:        19,
				AlternateScreen: true,
			})

			totalConvs := len(report.Conversations)
			return pg.Run(totalConvs, func(start, end, selected int, out *bytes.Buffer) {
				renderStatusSummary(report, out)
				out.WriteString("\n\n")

				headerPrefix := ""
				if isTTY {
					headerPrefix = "  "
				}

				fmt.Fprintf(out, "%s%-38s %-12s %-12s %-10s %-10s %-8s\n",
					headerPrefix, "CONVERSATION ID", "LOCAL STEPS", "REMOTE STEPS", "ARTIFACTS", "LOCAL DB", "SYNCED")
				out.WriteString(headerPrefix + strings.Repeat("-", 95) + "\n")

				for i := start; i < end; i++ {
					c := report.Conversations[i]
					syncedStr := "No"
					if c.Synced {
						syncedStr = "Yes"
					}
					dbStr := "No"
					if c.HasLocalDB {
						dbStr = "Yes"
					}
					prefix := ""
					if isTTY {
						if i == selected {
							prefix = "> "
						} else {
							prefix = "  "
						}
					}
					fmt.Fprintf(out, "%s%-38s %-12d %-12d %-10d %-10s %-8s\n",
						prefix, c.ID, c.LocalSteps, c.RemoteSteps, c.ArtifactsCount, dbStr, syncedStr)
				}

				if isTTY {
					out.WriteString(headerPrefix + strings.Repeat("-", 95) + "\n")
					pageSize := max(1, end-start)
					currentPage := (start / pageSize) + 1
					totalPages := totalConvs / pageSize
					if totalConvs%pageSize != 0 {
						totalPages++
					}
					totalPages = max(1, totalPages)
					fmt.Fprintf(out, "Page %d of %d (%d-%d of %d) | [↑/↓] Row  [←/→] Page  [q] Quit\n",
						currentPage, totalPages, start+1, end, totalConvs)
				}
			})
		},
	}

	statusCmd.Flags().BoolVar(&opts.full, "full", false, "Display detailed list of conversations with viewport pagination")
	statusCmd.Flags().StringVarP(&opts.conversationID, "conversation", "c", "", "Optional conversation ID to filter status")
	statusCmd.Flags().StringVar(&opts.pidFile, "pid-file", daemon.DefaultPIDPath(), "Path to PID file")
	statusCmd.Flags().StringVar(&opts.logFile, "log-file", daemon.DefaultLogPath(), "Path to daemon log file")
	statusCmd.Flags().StringVar(&opts.stateFile, "state-file", daemon.DefaultStatePath(), "Path to daemon runtime state file")
	statusCmd.Flags().BoolVar(&opts.noDBSync, "no-db-sync", false, "Disable SQLite database sync display")
	statusCmd.Flags().StringVar(&opts.conversationsDir, "conversations-dir", "", "Path to local conversations directory")
	statusCmd.Flags().StringVar(&opts.summariesDB, "summaries-db", "", "Path to conversation summaries SQLite database")

	return statusCmd
}

func renderStatusSummary(report *StatusReport, out *bytes.Buffer) {
	out.WriteString("==================================================\n")
	out.WriteString("          Antigravity Sync Status                \n")
	out.WriteString("==================================================\n")
	if report.Daemon.State == string(daemon.StateRunning) {
		fmt.Fprintf(out, "Daemon Status:   RUNNING (PID: %d)\n", report.Daemon.PID)
	} else {
		out.WriteString("Daemon Status:   STOPPED\n")
	}
	if report.Daemon.LastPolledAt != nil {
		fmt.Fprintf(out, "Last Polling:    %s (%s ago)\n",
			report.Daemon.LastPolledAt.Format("2006-01-02 15:04:05 UTC"),
			time.Since(*report.Daemon.LastPolledAt).Truncate(time.Second),
		)
	} else {
		out.WriteString("Last Polling:    Never / Inactive\n")
	}
	fmt.Fprintf(out, "Daemon Log:      %s\n", report.Daemon.LogFile)
	fmt.Fprintf(out, "GCP Project ID:  %s\n", report.ProjectID)
	fmt.Fprintf(out, "Machine ID:      %s\n", report.MachineID)
	fmt.Fprintf(out, "Brain Directory: %s\n", report.BrainDir)
	fmt.Fprintf(out, "Conversations:   %s\n", report.ConversationsDir)
	fmt.Fprintf(out, "Summaries DB:    %s\n", report.SummariesDB)
	dbSyncStr := "Enabled"
	if !report.DBSyncEnabled {
		dbSyncStr = "Disabled"
	}
	fmt.Fprintf(out, "SQLite DB Sync:  %s\n", dbSyncStr)
	fmt.Fprintf(out, "Sessions Found:  %d\n", report.ConversationsCount)
}
