package reconstructor

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/pkg/models"
)

const summarySchema = `
CREATE TABLE IF NOT EXISTS conversation_summaries (
	conversation_id text PRIMARY KEY,
	title text NOT NULL DEFAULT "",
	preview text NOT NULL DEFAULT "",
	step_count integer NOT NULL DEFAULT 0,
	last_modified_time datetime NOT NULL,
	workspace_uris text NOT NULL DEFAULT "",
	status text NOT NULL DEFAULT "",
	source text NOT NULL DEFAULT "",
	project_id text NOT NULL DEFAULT "",
	agent_name text NOT NULL DEFAULT "",
	parent_conversation_id text NOT NULL DEFAULT "",
	nesting_depth integer NOT NULL DEFAULT 0,
	battle_id text NOT NULL DEFAULT "",
	winning_conversation_id text NOT NULL DEFAULT "",
	not_fully_idle numeric NOT NULL DEFAULT 0,
	killed numeric NOT NULL DEFAULT 0,
	last_user_input_time datetime NOT NULL,
	last_user_input_step_index integer NOT NULL DEFAULT -1,
	app_data_dir text NOT NULL DEFAULT "",
	raw_summary BLOB,
	group_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_conversation_summaries_last_user_input_time ON conversation_summaries(last_user_input_time);
CREATE INDEX IF NOT EXISTS idx_conversation_summaries_last_modified_time ON conversation_summaries(last_modified_time);
`

// SummaryParams holds parameters for upserting into conversation_summaries.db.
type SummaryParams struct {
	ConversationID         string
	Title                  string
	Preview                string
	StepCount              int
	LastModifiedTime       time.Time
	WorkspaceURIs          []string
	Status                 string
	Source                 string
	ProjectID              string
	AgentName              string
	ParentConversationID   string
	NestingDepth           int
	BattleID               string
	WinningConversationID  string
	NotFullyIdle           bool
	Killed                 bool
	LastUserInputTime      time.Time
	LastUserInputStepIndex int
	AppDataDir             string
	RawSummary             []byte
	GroupID                string
}

// BuildSummaryFromSteps creates a SummaryParams structure from a list of steps.
func BuildSummaryFromSteps(conversationID, title string, steps []models.Step) SummaryParams {
	params := SummaryParams{
		ConversationID:         conversationID,
		Title:                  title,
		Preview:                title,
		StepCount:              len(steps),
		LastUserInputStepIndex: -1,
		LastUserInputTime:      time.Time{},
		LastModifiedTime:       time.Time{},
	}

	var firstUserInputContent string

	for _, step := range steps {
		if params.LastModifiedTime.IsZero() || step.CreatedAt.After(params.LastModifiedTime) {
			params.LastModifiedTime = step.CreatedAt
		}

		if step.Type == "USER_INPUT" {
			if firstUserInputContent == "" && step.Content != "" {
				firstUserInputContent = step.Content
			}
			params.LastUserInputTime = step.CreatedAt
			params.LastUserInputStepIndex = step.StepIndex
		}
	}

	if params.LastModifiedTime.IsZero() {
		params.LastModifiedTime = time.Now()
	}

	if params.Preview == "" && firstUserInputContent != "" {
		cleaned := strings.TrimSpace(firstUserInputContent)
		cleaned, _, _ = strings.Cut(cleaned, "\n")
		params.Preview = cleaned[:min(len(cleaned), 80)]
	}

	params.Title = cmp.Or(params.Title, params.Preview)

	return params
}

// UpsertSummary inserts or updates the record for a conversation in conversation_summaries.db.
func (r *Reconstructor) UpsertSummary(ctx context.Context, params SummaryParams) error {
	if params.ConversationID == "" {
		return errors.New("conversation_id cannot be empty")
	}

	log := logger.FromContext(ctx)
	log.DebugContext(ctx, "Upserting conversation summary in sqlite",
		"conversation_id", params.ConversationID,
		"path", r.summariesDBPath)

	db, err := r.OpenDB(r.summariesDBPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed starting sqlite transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, summarySchema); err != nil {
		return fmt.Errorf("failed creating conversation_summaries schema: %w", err)
	}

	workspaceURIsJSON := ""
	if len(params.WorkspaceURIs) > 0 {
		b, err := json.Marshal(params.WorkspaceURIs)
		if err == nil {
			workspaceURIsJSON = string(b)
		}
	}

	const upsertQuery = `
	INSERT INTO conversation_summaries (
		conversation_id, title, preview, step_count, last_modified_time,
		workspace_uris, status, source, project_id, agent_name,
		parent_conversation_id, nesting_depth, battle_id, winning_conversation_id,
		not_fully_idle, killed, last_user_input_time, last_user_input_step_index,
		app_data_dir, raw_summary, group_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(conversation_id) DO UPDATE SET
		title = CASE WHEN excluded.title != '' THEN excluded.title ELSE conversation_summaries.title END,
		preview = CASE WHEN excluded.preview != '' THEN excluded.preview ELSE conversation_summaries.preview END,
		step_count = MAX(excluded.step_count, conversation_summaries.step_count),
		last_modified_time = excluded.last_modified_time,
		workspace_uris = CASE WHEN excluded.workspace_uris != '' AND excluded.workspace_uris != '[]' THEN excluded.workspace_uris ELSE conversation_summaries.workspace_uris END,
		status = CASE WHEN excluded.status != '' THEN excluded.status ELSE conversation_summaries.status END,
		source = CASE WHEN excluded.source != '' THEN excluded.source ELSE conversation_summaries.source END,
		project_id = CASE WHEN excluded.project_id != '' THEN excluded.project_id ELSE conversation_summaries.project_id END,
		agent_name = CASE WHEN excluded.agent_name != '' THEN excluded.agent_name ELSE conversation_summaries.agent_name END,
		parent_conversation_id = CASE WHEN excluded.parent_conversation_id != '' THEN excluded.parent_conversation_id ELSE conversation_summaries.parent_conversation_id END,
		nesting_depth = CASE WHEN excluded.nesting_depth > 0 THEN excluded.nesting_depth ELSE conversation_summaries.nesting_depth END,
		battle_id = CASE WHEN excluded.battle_id != '' THEN excluded.battle_id ELSE conversation_summaries.battle_id END,
		winning_conversation_id = CASE WHEN excluded.winning_conversation_id != '' THEN excluded.winning_conversation_id ELSE conversation_summaries.winning_conversation_id END,
		not_fully_idle = excluded.not_fully_idle,
		killed = excluded.killed,
		last_user_input_time = CASE WHEN excluded.last_user_input_step_index >= 0 THEN excluded.last_user_input_time ELSE conversation_summaries.last_user_input_time END,
		last_user_input_step_index = MAX(excluded.last_user_input_step_index, conversation_summaries.last_user_input_step_index),
		app_data_dir = CASE WHEN excluded.app_data_dir != '' THEN excluded.app_data_dir ELSE conversation_summaries.app_data_dir END,
		raw_summary = CASE WHEN excluded.raw_summary IS NOT NULL THEN excluded.raw_summary ELSE conversation_summaries.raw_summary END,
		group_id = CASE WHEN excluded.group_id != '' THEN excluded.group_id ELSE conversation_summaries.group_id END;
	`

	const timeFormat = "2006-01-02 15:04:05.999999-07:00"
	lastModStr := params.LastModifiedTime.Format(timeFormat)
	lastUserInputStr := params.LastUserInputTime.Format(timeFormat)

	_, err = tx.ExecContext(ctx, upsertQuery,
		params.ConversationID,
		params.Title,
		params.Preview,
		params.StepCount,
		lastModStr,
		workspaceURIsJSON,
		params.Status,
		params.Source,
		params.ProjectID,
		params.AgentName,
		params.ParentConversationID,
		params.NestingDepth,
		params.BattleID,
		params.WinningConversationID,
		params.NotFullyIdle,
		params.Killed,
		lastUserInputStr,
		params.LastUserInputStepIndex,
		params.AppDataDir,
		params.RawSummary,
		params.GroupID,
	)
	if err != nil {
		return fmt.Errorf("failed executing upsert on conversation_summaries: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing sqlite transaction: %w", err)
	}

	log.InfoContext(ctx, "Successfully upserted conversation summary in sqlite",
		"conversation_id", params.ConversationID)

	return nil
}

// ReadLocalSummary reads an existing summary record from conversation_summaries.db.
// If the file or conversation does not exist, it returns (nil, nil).
func (r *Reconstructor) ReadLocalSummary(ctx context.Context, conversationID string) (*SummaryParams, error) {
	if conversationID == "" {
		return nil, errors.New("conversation_id cannot be empty")
	}

	if _, err := os.Stat(r.summariesDBPath); err != nil {
		return nil, nil
	}

	db, err := r.OpenDB(r.summariesDBPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = db.Close()
	}()

	const query = `
	SELECT
		title, preview, step_count, last_modified_time, workspace_uris,
		status, source, project_id, agent_name, parent_conversation_id,
		nesting_depth, battle_id, winning_conversation_id, not_fully_idle,
		killed, last_user_input_time, last_user_input_step_index, app_data_dir,
		raw_summary, group_id
	FROM conversation_summaries
	WHERE conversation_id = ?;
	`

	var (
		title, preview, workspaceURIsJSON string
		stepCount, nestingDepth           int
		status, source, projectID         string
		agentName, parentConvID           string
		battleID, winningConvID           string
		notFullyIdle, killed              bool
		lastModStr, lastInputStr          string
		lastInputStepIndex                int
		appDataDir, groupID               string
		rawSummary                        []byte
	)

	err = db.QueryRowContext(ctx, query, conversationID).Scan(
		&title, &preview, &stepCount, &lastModStr, &workspaceURIsJSON,
		&status, &source, &projectID, &agentName, &parentConvID,
		&nestingDepth, &battleID, &winningConvID, &notFullyIdle,
		&killed, &lastInputStr, &lastInputStepIndex, &appDataDir,
		&rawSummary, &groupID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed reading conversation summary %s: %w", conversationID, err)
	}

	var uris []string
	if workspaceURIsJSON != "" && workspaceURIsJSON != "[]" {
		_ = json.Unmarshal([]byte(workspaceURIsJSON), &uris)
	}

	return &SummaryParams{
		ConversationID:         conversationID,
		Title:                  title,
		Preview:                preview,
		StepCount:              stepCount,
		LastModifiedTime:       parseSQLiteTime(lastModStr),
		WorkspaceURIs:          uris,
		Status:                 status,
		Source:                 source,
		ProjectID:              projectID,
		AgentName:              agentName,
		ParentConversationID:   parentConvID,
		NestingDepth:           nestingDepth,
		BattleID:               battleID,
		WinningConversationID:  winningConvID,
		NotFullyIdle:           notFullyIdle,
		Killed:                 killed,
		LastUserInputTime:      parseSQLiteTime(lastInputStr),
		LastUserInputStepIndex: lastInputStepIndex,
		AppDataDir:             appDataDir,
		RawSummary:             rawSummary,
		GroupID:                groupID,
	}, nil
}

func parseSQLiteTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999+00:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05+00:00",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	return time.Time{}
}
