package reconstructor_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestUpsertSummary(t *testing.T) {
	tempDir := t.TempDir()
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")

	rec := reconstructor.New(convsDir, summariesDB)
	ctx := context.Background()

	convID := "summary-conv-test-1"
	t1 := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 16, 10, 5, 0, 0, time.UTC)

	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: t1,
			Content:   "Create a reactive SQLite reconstructor for Antigravity",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: t2,
			Content:   "Understood, starting implementation.",
		},
	}

	params := reconstructor.BuildSummaryFromSteps(convID, "", steps)
	assert.Equal(t, convID, params.ConversationID)
	assert.Equal(t, "Create a reactive SQLite reconstructor for Antigravity", params.Preview)
	assert.Equal(t, 2, params.StepCount)
	assert.Equal(t, 0, params.LastUserInputStepIndex)
	assert.Equal(t, t1, params.LastUserInputTime)
	assert.Equal(t, t2, params.LastModifiedTime)

	// Perform Upsert
	err := rec.UpsertSummary(ctx, params)
	require.NoError(t, err)

	require.FileExists(t, summariesDB)

	db, err := sql.Open("sqlite", summariesDB)
	require.NoError(t, err)
	defer db.Close()

	var preview, status string
	var stepCount, lastInputIdx int
	err = db.QueryRowContext(ctx, "SELECT preview, step_count, last_user_input_step_index, status FROM conversation_summaries WHERE conversation_id = ?", convID).
		Scan(&preview, &stepCount, &lastInputIdx, &status)
	require.NoError(t, err)
	assert.Equal(t, "Create a reactive SQLite reconstructor for Antigravity", preview)
	assert.Equal(t, 2, stepCount)
	assert.Equal(t, 0, lastInputIdx)

	// Test preserving existing workspace URIs on subsequent upsert
	_, err = db.ExecContext(ctx, "UPDATE conversation_summaries SET workspace_uris = ? WHERE conversation_id = ?", `["file:///path/to/project"]`, convID)
	require.NoError(t, err)

	// Second upsert with empty workspace URIs should NOT erase existing workspace URIs
	updatedParams := reconstructor.BuildSummaryFromSteps(convID, "Custom Title", append(steps, models.Step{
		StepIndex: 2,
		Source:    "MODEL",
		Type:      "GENERIC",
		Status:    "DONE",
		CreatedAt: t2.Add(time.Minute),
	}))
	err = rec.UpsertSummary(ctx, updatedParams)
	require.NoError(t, err)

	var title, workspaceURIs string
	err = db.QueryRowContext(ctx, "SELECT title, step_count, workspace_uris FROM conversation_summaries WHERE conversation_id = ?", convID).
		Scan(&title, &stepCount, &workspaceURIs)
	require.NoError(t, err)
	assert.Equal(t, "Custom Title", title)
	assert.Equal(t, 3, stepCount)
	assert.Equal(t, `["file:///path/to/project"]`, workspaceURIs)
}
