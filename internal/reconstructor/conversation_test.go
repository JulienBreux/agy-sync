package reconstructor_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestReconstructConversationDB(t *testing.T) {
	tempDir := t.TempDir()
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")

	rec := reconstructor.New(convsDir, summariesDB)
	require.NotNil(t, rec)

	convID := "test-conv-12345"
	ctx := context.Background()

	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "SYSTEM",
			Type:      "INIT",
			Status:    "DONE",
			CreatedAt: time.Now().Add(-10 * time.Minute),
			Content:   "Initializing conversation context",
		},
		{
			StepIndex: 1,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().Add(-9 * time.Minute),
			Content:   "Build a hello world app",
		},
		{
			StepIndex: 2,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: time.Now().Add(-8 * time.Minute),
			Thinking:  "Thinking about solution...",
			ToolCalls: []models.ToolCall{
				{
					Name: "run_command",
					Args: map[string]any{"cmd": "echo hello"},
				},
			},
		},
	}

	err := rec.ReconstructConversationDB(ctx, convID, steps)
	require.NoError(t, err)

	dbPath := filepath.Join(convsDir, convID+".db")
	require.FileExists(t, dbPath)

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	// Verify trajectory_meta table
	var cascadeID string
	err = db.QueryRowContext(ctx, "SELECT cascade_id FROM trajectory_meta WHERE cascade_id = ?", convID).Scan(&cascadeID)
	require.NoError(t, err)
	assert.Equal(t, convID, cascadeID)

	// Verify steps count and records
	var stepCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM steps").Scan(&stepCount)
	require.NoError(t, err)
	assert.Equal(t, 3, stepCount)

	// Verify step 1 values
	var idx, stepType, status int
	err = db.QueryRowContext(ctx, "SELECT idx, step_type, status FROM steps WHERE idx = 1").Scan(&idx, &stepType, &status)
	require.NoError(t, err)
	assert.Equal(t, 1, idx)
	assert.Equal(t, 14, stepType) // USER_INPUT
	assert.Equal(t, 3, status)    // DONE

	// Idempotency / Upsert test: running with updated step 2 and new step 3
	updatedSteps := slices.Concat(steps, []models.Step{
		{
			StepIndex: 3,
			Source:    "MODEL",
			Type:      "GENERIC",
			Status:    "DONE",
			CreatedAt: time.Now(),
			Content:   "Done!",
		},
	})

	err = rec.ReconstructConversationDB(ctx, convID, updatedSteps)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx, "SELECT count(*) FROM steps").Scan(&stepCount)
	require.NoError(t, err)
	assert.Equal(t, 4, stepCount)
}

func TestReconstructConversationDB_ValidationAndErrors(t *testing.T) {
	tempDir := t.TempDir()
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")

	rec := reconstructor.New(convsDir, summariesDB)
	ctx := context.Background()

	t.Run("empty conversation ID", func(t *testing.T) {
		err := rec.ReconstructConversationDB(ctx, "", []models.Step{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conversation_id cannot be empty")
	})

	t.Run("canceled context", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		err := rec.ReconstructConversationDB(canceledCtx, "canceled-conv", []models.Step{
			{StepIndex: 0, Type: "USER_INPUT", Status: "DONE"},
		})
		require.Error(t, err)
	})
}

func TestTypeAndStatusConversions(t *testing.T) {
	types := map[string]int{
		"USER_INPUT":       14,
		"PLANNER_RESPONSE": 15,
		"GENERIC":          132,
		"INIT":             21,
		"SYSTEM":           21,
		"TOOL_CALL":        23,
		"SUBAGENT":         101,
		"THINKING":         17,
		"UNKNOWN":          0,
	}

	for typeStr, expectedCode := range types {
		assert.Equal(t, expectedCode, reconstructor.StepTypeToInt(typeStr))
	}

	statuses := map[string]int{
		"DONE":      3,
		"SUCCESS":   3,
		"RUNNING":   2,
		"PENDING":   2,
		"ERROR":     7,
		"FAILED":    7,
		"CANCELED":  4,
		"CANCELLED": 4,
		"UNKNOWN":   0,
	}

	for statusStr, expectedCode := range statuses {
		assert.Equal(t, expectedCode, reconstructor.StepStatusToInt(statusStr))
	}
}

func TestTrajectoryTableModelsMapping(t *testing.T) {
	tempDir := t.TempDir()
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")

	rec := reconstructor.New(convsDir, summariesDB)
	convID := "conv-models-test"
	ctx := context.Background()

	step := models.Step{
		StepIndex: 1,
		Type:      "PLANNER_RESPONSE",
		Status:    "DONE",
		Content:   "Planning next step",
	}

	// Verify StepToConversationDBStep helper
	dbStep, err := reconstructor.StepToConversationDBStep(step)
	require.NoError(t, err)
	assert.Equal(t, 1, dbStep.Idx)
	assert.Equal(t, 15, dbStep.StepType)
	assert.Equal(t, 3, dbStep.Status)
	assert.NotEmpty(t, dbStep.StepPayload)

	// Verify TrajectoryMetaFromConversation helper
	meta := reconstructor.TrajectoryMetaFromConversation(convID)
	assert.Equal(t, convID, meta.TrajectoryID)
	assert.Equal(t, convID, meta.CascadeID)
	assert.Equal(t, 4, meta.TrajectoryType)
	assert.Equal(t, 17, meta.Source)

	// Reconstruct DB and verify rows match typed models
	err = rec.ReconstructConversationDB(ctx, convID, []models.Step{step})
	require.NoError(t, err)

	dbPath := filepath.Join(convsDir, convID+".db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	var readMeta models.TrajectoryMeta
	err = db.QueryRowContext(ctx, "SELECT trajectory_id, cascade_id, trajectory_type, source FROM trajectory_meta WHERE trajectory_id = ?", convID).
		Scan(&readMeta.TrajectoryID, &readMeta.CascadeID, &readMeta.TrajectoryType, &readMeta.Source)
	require.NoError(t, err)
	assert.Equal(t, meta, readMeta)

	var readStep models.ConversationDBStep
	err = db.QueryRowContext(ctx, "SELECT idx, step_type, status, has_subtrajectory, step_format FROM steps WHERE idx = 1").
		Scan(&readStep.Idx, &readStep.StepType, &readStep.Status, &readStep.HasSubtrajectory, &readStep.StepFormat)
	require.NoError(t, err)
	assert.Equal(t, dbStep.Idx, readStep.Idx)
	assert.Equal(t, dbStep.StepType, readStep.StepType)
	assert.Equal(t, dbStep.Status, readStep.Status)

	// Verify auxiliary tables can insert and select typed models
	genMeta := models.GenMetadata{Idx: 1, Data: []byte("gen-bytes"), Size: 9}
	_, err = db.ExecContext(ctx, "INSERT INTO gen_metadata (idx, data, size) VALUES (?, ?, ?)", genMeta.Idx, genMeta.Data, genMeta.Size)
	require.NoError(t, err)

	execMeta := models.ExecutorMetadata{Idx: 1, Data: []byte("exec-bytes")}
	_, err = db.ExecContext(ctx, "INSERT INTO executor_metadata (idx, data) VALUES (?, ?)", execMeta.Idx, execMeta.Data)
	require.NoError(t, err)

	parentRef := models.ParentReference{Idx: 1, Data: []byte("parent-bytes")}
	_, err = db.ExecContext(ctx, "INSERT INTO parent_references (idx, data) VALUES (?, ?)", parentRef.Idx, parentRef.Data)
	require.NoError(t, err)

	blob := models.TrajectoryMetadataBlob{ID: "main", Data: []byte("blob-bytes")}
	_, err = db.ExecContext(ctx, "INSERT OR REPLACE INTO trajectory_metadata_blob (id, data) VALUES (?, ?)", blob.ID, blob.Data)
	require.NoError(t, err)

	battle := models.BattleModeInfo{Idx: 1, Data: []byte("battle-bytes")}
	_, err = db.ExecContext(ctx, "INSERT INTO battle_mode_infos (idx, data) VALUES (?, ?)", battle.Idx, battle.Data)
	require.NoError(t, err)
}

