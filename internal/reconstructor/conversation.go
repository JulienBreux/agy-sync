package reconstructor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/pkg/models"
)

const conversationSchema = `
CREATE TABLE IF NOT EXISTS trajectory_meta (
    trajectory_id TEXT PRIMARY KEY,
    cascade_id TEXT,
    trajectory_type INTEGER,
    source INTEGER
);

CREATE TABLE IF NOT EXISTS steps (
    idx INTEGER PRIMARY KEY,
    step_type INTEGER NOT NULL DEFAULT 0,
    status INTEGER NOT NULL DEFAULT 0,
    has_subtrajectory NUMERIC NOT NULL DEFAULT 0,
    metadata BLOB,
    error_details BLOB,
    permissions BLOB,
    task_details BLOB,
    render_info BLOB,
    step_payload BLOB,
    step_format INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_steps_status ON steps(status);
CREATE INDEX IF NOT EXISTS idx_steps_step_type ON steps(step_type);
CREATE TABLE IF NOT EXISTS gen_metadata (idx INTEGER PRIMARY KEY, data BLOB, size INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS executor_metadata (idx INTEGER PRIMARY KEY, data BLOB);
CREATE TABLE IF NOT EXISTS parent_references (idx INTEGER PRIMARY KEY, data BLOB);
CREATE TABLE IF NOT EXISTS trajectory_metadata_blob (id TEXT PRIMARY KEY DEFAULT "main", data BLOB);
CREATE TABLE IF NOT EXISTS battle_mode_infos (idx INTEGER PRIMARY KEY, data BLOB);
`

// StepTypeToInt converts high-level string step types to Antigravity internal codes.
func StepTypeToInt(t string) int {
	switch t {
	case "USER_INPUT":
		return 14
	case "PLANNER_RESPONSE":
		return 15
	case "GENERIC":
		return 132
	case "INIT", "SYSTEM":
		return 21
	case "TOOL_CALL":
		return 23
	case "SUBAGENT":
		return 101
	case "THINKING":
		return 17
	default:
		return 0
	}
}

// StepStatusToInt converts high-level string step statuses to Antigravity internal codes.
func StepStatusToInt(s string) int {
	switch s {
	case "DONE", "SUCCESS":
		return 3
	case "RUNNING", "PENDING":
		return 2
	case "ERROR", "FAILED":
		return 7
	case "CANCELED", "CANCELLED":
		return 4
	default:
		return 0
	}
}

// TrajectoryMetaFromConversation creates a TrajectoryMeta struct for a conversation.
func TrajectoryMetaFromConversation(conversationID string) models.TrajectoryMeta {
	return models.TrajectoryMeta{
		TrajectoryID:   conversationID,
		CascadeID:      conversationID,
		TrajectoryType: 4,
		Source:         17,
	}
}

// StepToConversationDBStep converts a high-level models.Step into a models.ConversationDBStep.
func StepToConversationDBStep(step models.Step) (models.ConversationDBStep, error) {
	stepJSON, err := json.Marshal(step)
	if err != nil {
		return models.ConversationDBStep{}, fmt.Errorf("failed marshaling step %d payload: %w", step.StepIndex, err)
	}

	return models.ConversationDBStep{
		Idx:              step.StepIndex,
		StepType:         StepTypeToInt(step.Type),
		Status:           StepStatusToInt(step.Status),
		HasSubtrajectory: false,
		Metadata:         stepJSON,
		StepPayload:      stepJSON,
		StepFormat:       0,
	}, nil
}

// ReconstructConversationDB recreates or updates the SQLite database for an individual conversation.
func (r *Reconstructor) ReconstructConversationDB(ctx context.Context, conversationID string, steps []models.Step) error {
	if conversationID == "" {
		return errors.New("conversation_id cannot be empty")
	}

	log := logger.FromContext(ctx)
	dbPath := filepath.Join(r.conversationsDir, conversationID+".db")
	log.DebugContext(ctx, "Reconstructing conversation sqlite database", "conversation_id", conversationID, "path", dbPath)

	db, err := r.OpenDB(dbPath)
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

	if _, err := tx.ExecContext(ctx, conversationSchema); err != nil {
		return fmt.Errorf("failed creating conversation schema: %w", err)
	}

	// Ensure trajectory_meta entry exists
	trajMeta := TrajectoryMetaFromConversation(conversationID)
	const metaQuery = `
	INSERT INTO trajectory_meta (trajectory_id, cascade_id, trajectory_type, source)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(trajectory_id) DO NOTHING;
	`
	if _, err := tx.ExecContext(ctx, metaQuery, trajMeta.TrajectoryID, trajMeta.CascadeID, trajMeta.TrajectoryType, trajMeta.Source); err != nil {
		return fmt.Errorf("failed writing trajectory_meta: %w", err)
	}

	// Upsert steps
	const stepQuery = `
	INSERT INTO steps (idx, step_type, status, has_subtrajectory, metadata, step_payload, step_format)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(idx) DO UPDATE SET
		step_type = excluded.step_type,
		status = excluded.status,
		has_subtrajectory = excluded.has_subtrajectory,
		metadata = excluded.metadata,
		step_payload = COALESCE(steps.step_payload, excluded.step_payload),
		step_format = excluded.step_format;
	`
	stmt, err := tx.PrepareContext(ctx, stepQuery)
	if err != nil {
		return fmt.Errorf("failed preparing step statement: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, step := range steps {
		dbStep, err := StepToConversationDBStep(step)
		if err != nil {
			return err
		}

		var hasSub int
		if dbStep.HasSubtrajectory {
			hasSub = 1
		}

		if _, err := stmt.ExecContext(ctx, dbStep.Idx, dbStep.StepType, dbStep.Status, hasSub, dbStep.Metadata, dbStep.StepPayload, dbStep.StepFormat); err != nil {
			return fmt.Errorf("failed executing step upsert for index %d: %w", dbStep.Idx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing sqlite transaction: %w", err)
	}

	log.InfoContext(ctx, "Successfully reconstructed conversation sqlite database",
		"conversation_id", conversationID,
		"steps_count", len(steps))

	return nil
}
