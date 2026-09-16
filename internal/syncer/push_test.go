package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func createTestBrain(t *testing.T) (string, string) {
	t.Helper()
	brainDir := t.TempDir()

	convID := "test-conv-push"
	convDir := filepath.Join(brainDir, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T09:40:46Z","content":"Initial prompt"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T09:40:47Z","content":"Agent response"}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(transcriptContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "plan.md"), []byte("# Execution Plan"), 0o644))

	return brainDir, convID
}

func TestPush_InitialSync(t *testing.T) {
	brainDir, convID := createTestBrain(t)

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  brainDir,
		MachineID: "laptop-1",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	engine := syncer.NewEngine(cfg, repo)

	ctx := t.Context()
	result, err := engine.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result.ConversationsSynced)
	assert.Equal(t, 2, result.StepsSynced)
	assert.Equal(t, 1, result.ArtifactsSynced)
	assert.Empty(t, result.Errors)

	// Verify remote state
	remoteConv, err := repo.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, remoteConv)
	assert.Equal(t, 1, remoteConv.LastSyncedStep)

	steps, err := repo.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, "laptop-1", steps[0].MachineID)

	art, err := repo.GetArtifact(ctx, convID, "plan.md")
	require.NoError(t, err)
	require.NotNil(t, art)
	assert.Equal(t, []byte("# Execution Plan"), art.Content)
}

func TestPush_IncrementalAppend(t *testing.T) {
	brainDir, convID := createTestBrain(t)

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  brainDir,
		MachineID: "laptop-1",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	engine := syncer.NewEngine(cfg, repo)
	ctx := t.Context()

	// Initial push
	_, err := engine.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)

	// Append a 3rd step to transcript
	logsDir := filepath.Join(brainDir, convID, ".system_generated", "logs")
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	newStep := `{"step_index":2,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T09:41:00Z","content":"Followup request"}` + "\n"
	_, err = f.WriteString(newStep)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Push again (incremental)
	result2, err := engine.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, result2.ConversationsSynced)
	assert.Equal(t, 1, result2.StepsSynced) // only the 1 new step

	allSteps, err := repo.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	assert.Len(t, allSteps, 3)
}

func TestPush_FilterSpecificConversation(t *testing.T) {
	brainDir, convID := createTestBrain(t)

	// Add a second conversation
	conv2 := filepath.Join(brainDir, "conv-second")
	require.NoError(t, os.MkdirAll(filepath.Join(conv2, ".system_generated", "logs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(conv2, ".system_generated", "logs", "transcript.jsonl"), []byte(`{"step_index":0,"content":"hello"}`+"\n"), 0o644))

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  brainDir,
		MachineID: "laptop-1",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	engine := syncer.NewEngine(cfg, repo)
	ctx := t.Context()

	// Push only convID
	res, err := engine.Push(ctx, syncer.PushOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 1, res.ConversationsSynced)

	// Second conversation should not be in repository
	remoteSecond, err := repo.GetConversation(ctx, "conv-second")
	require.NoError(t, err)
	assert.Nil(t, remoteSecond)
}

func TestPush_DBChunkAndSummarySync(t *testing.T) {
	brainDir, convID := createTestBrain(t)
	tempDir := t.TempDir()
	convsDir := filepath.Join(tempDir, "conversations")
	require.NoError(t, os.MkdirAll(convsDir, 0o755))
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")

	// Create a real SQLite DB for the conversation
	localDBPath := filepath.Join(convsDir, convID+".db")
	require.NoError(t, os.WriteFile(localDBPath, []byte("SQLite format 3\x00-fake-binary-content-for-testing-chunks"), 0o600))

	// Upsert summary into conversation_summaries.db
	rec := reconstructor.New(convsDir, summariesDB)
	ctx := t.Context()
	err := rec.UpsertSummary(ctx, reconstructor.SummaryParams{
		ConversationID: convID,
		Title:          "Snapshot Sync Feature",
		Preview:        "Initial prompt",
		StepCount:      2,
		RawSummary:     []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	cfg := &config.Config{
		ProjectID:        "test-proj",
		BrainDir:         brainDir,
		MachineID:        "laptop-1",
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		NoDBSync:         false,
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	engine := syncer.NewEngine(cfg, repo)
	res, err := engine.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.ConversationsSynced)

	// Verify conversation metadata in remote repository
	remoteConv, err := repo.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, remoteConv)
	assert.Equal(t, "Snapshot Sync Feature", remoteConv.Title)
	assert.Equal(t, "Initial prompt", remoteConv.Preview)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, remoteConv.RawSummary)
	assert.NotEmpty(t, remoteConv.DBSHA256)
	assert.Positive(t, remoteConv.DBChunksCount)
	assert.Positive(t, remoteConv.DBSizeBytes)

	// Verify DB chunks in repository
	chunks, err := repo.GetDBChunks(ctx, convID)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	assert.Equal(t, 0, chunks[0].ChunkIndex)
	assert.Contains(t, string(chunks[0].Data), "SQLite format 3")
}
