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
	defer func() {
		_ = db.Close()
	}()

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

func TestBuildSummaryFromSteps_DefaultTitleToPreview(t *testing.T) {
	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now(),
			Content:   "Explain SQLite WAL Mode in Antigravity",
		},
	}

	// When title is empty, Title should default to Preview so agy displays the title instead of UUID
	params := reconstructor.BuildSummaryFromSteps("conv-auto-title", "", steps)
	assert.Equal(t, "Explain SQLite WAL Mode in Antigravity", params.Preview)
	assert.Equal(t, "Explain SQLite WAL Mode in Antigravity", params.Title)
}

func TestReadLocalSummary(t *testing.T) {
	tempDir := t.TempDir()
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convsDir := filepath.Join(tempDir, "conversations")
	rec := reconstructor.New(convsDir, summariesDB)
	ctx := context.Background()

	convID := "conv-read-summary-1"
	t1 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	// Upsert a summary first
	params := reconstructor.SummaryParams{
		ConversationID:         convID,
		Title:                  "Local Read Title",
		Preview:                "Local Read Preview",
		StepCount:              15,
		LastModifiedTime:       t1,
		LastUserInputTime:      t1,
		LastUserInputStepIndex: 4,
		RawSummary:             []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	err := rec.UpsertSummary(ctx, params)
	require.NoError(t, err)

	// Read local summary using reconstructor
	readParams, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, readParams)
	assert.Equal(t, convID, readParams.ConversationID)
	assert.Equal(t, "Local Read Title", readParams.Title)
	assert.Equal(t, "Local Read Preview", readParams.Preview)
	assert.Equal(t, 15, readParams.StepCount)
	assert.Equal(t, 4, readParams.LastUserInputStepIndex)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, readParams.RawSummary)

	// Non-existent conversation should return nil, nil
	notFound, err := rec.ReadLocalSummary(ctx, "nonexistent-conv")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestUpsertAndReadLocalSummary_All21Columns(t *testing.T) {
	tempDir := t.TempDir()
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convsDir := filepath.Join(tempDir, "conversations")
	rec := reconstructor.New(convsDir, summariesDB)
	ctx := context.Background()

	convID := "conv-all-21-columns"
	t1 := time.Date(2026, 9, 18, 14, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 18, 14, 35, 0, 0, time.UTC)

	params := reconstructor.SummaryParams{
		ConversationID:         convID,
		Title:                  "Full Fidelity Title",
		Preview:                "Full Fidelity Preview",
		StepCount:              42,
		LastModifiedTime:       t2,
		WorkspaceURIs:          []string{"file:///Users/julienbreux/workspace1", "file:///Users/julienbreux/workspace2"},
		Status:                 "active",
		Source:                 "cli",
		ProjectID:              "project-xyz",
		AgentName:              "conductor",
		ParentConversationID:   "parent-conv-999",
		NestingDepth:           2,
		BattleID:               "battle-mode-1",
		WinningConversationID:  convID,
		NotFullyIdle:           true,
		Killed:                 false,
		LastUserInputTime:      t1,
		LastUserInputStepIndex: 7,
		AppDataDir:             "/Users/julienbreux/.gemini/antigravity-cli",
		RawSummary:             []byte{0x01, 0x02, 0x03, 0x04},
		GroupID:                "group-meta-1",
	}

	err := rec.UpsertSummary(ctx, params)
	require.NoError(t, err)

	read, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, read)

	assert.Equal(t, params.ConversationID, read.ConversationID)
	assert.Equal(t, params.Title, read.Title)
	assert.Equal(t, params.Preview, read.Preview)
	assert.Equal(t, params.StepCount, read.StepCount)
	assert.Equal(t, params.WorkspaceURIs, read.WorkspaceURIs)
	assert.Equal(t, params.Status, read.Status)
	assert.Equal(t, params.Source, read.Source)
	assert.Equal(t, params.ProjectID, read.ProjectID)
	assert.Equal(t, params.AgentName, read.AgentName)
	assert.Equal(t, params.ParentConversationID, read.ParentConversationID)
	assert.Equal(t, params.NestingDepth, read.NestingDepth)
	assert.Equal(t, params.BattleID, read.BattleID)
	assert.Equal(t, params.WinningConversationID, read.WinningConversationID)
	assert.True(t, read.NotFullyIdle)
	assert.False(t, read.Killed)
	assert.Equal(t, params.LastUserInputStepIndex, read.LastUserInputStepIndex)
	assert.Equal(t, params.AppDataDir, read.AppDataDir)
	assert.Equal(t, params.RawSummary, read.RawSummary)
	assert.Equal(t, params.GroupID, read.GroupID)

	// Test ON CONFLICT DO UPDATE SET for secondary fields
	updatedParams := params
	updatedParams.StepCount = 50
	updatedParams.Source = "ide"
	updatedParams.AgentName = "gemini-developer"
	updatedParams.ProjectID = "project-updated"
	updatedParams.ParentConversationID = "parent-conv-updated"
	updatedParams.NestingDepth = 3
	updatedParams.BattleID = "battle-updated"
	updatedParams.WinningConversationID = "winner-updated"
	updatedParams.NotFullyIdle = false
	updatedParams.Killed = true
	updatedParams.AppDataDir = "/custom/app_data_dir"

	err = rec.UpsertSummary(ctx, updatedParams)
	require.NoError(t, err)

	readUpdated, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, readUpdated)

	assert.Equal(t, 50, readUpdated.StepCount)
	assert.Equal(t, "ide", readUpdated.Source)
	assert.Equal(t, "gemini-developer", readUpdated.AgentName)
	assert.Equal(t, "project-updated", readUpdated.ProjectID)
	assert.Equal(t, "parent-conv-updated", readUpdated.ParentConversationID)
	assert.Equal(t, 3, readUpdated.NestingDepth)
	assert.Equal(t, "battle-updated", readUpdated.BattleID)
	assert.Equal(t, "winner-updated", readUpdated.WinningConversationID)
	assert.False(t, readUpdated.NotFullyIdle)
	assert.True(t, readUpdated.Killed)
	assert.Equal(t, "/custom/app_data_dir", readUpdated.AppDataDir)
}

func TestReadLocalSummary_StrictMirroringEmptyTitle(t *testing.T) {
	tempDir := t.TempDir()
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convsDir := filepath.Join(tempDir, "conversations")
	rec := reconstructor.New(convsDir, summariesDB)
	ctx := context.Background()

	convID := "conv-strict-empty-title"
	// Upsert with empty title and non-empty preview
	params := reconstructor.SummaryParams{
		ConversationID: convID,
		Title:          "",
		Preview:        "Preview message only",
		StepCount:      1,
	}
	err := rec.UpsertSummary(ctx, params)
	require.NoError(t, err)

	read, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, read)

	// Strict mirroring: Title should be empty as stored, not overwritten by Preview
	assert.Empty(t, read.Title)
	assert.Equal(t, "Preview message only", read.Preview)
}

