package syncer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/firestore"
	"github.com/julienbreux/agy-sync/pkg/syncer"
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
	defer func() {
		_ = repo.Close()
	}()

	engine := syncer.NewEngine(cfg, repo)

	ctx := context.Background()
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
	defer func() {
		_ = repo.Close()
	}()

	engine := syncer.NewEngine(cfg, repo)
	ctx := context.Background()

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
	defer func() {
		_ = repo.Close()
	}()

	engine := syncer.NewEngine(cfg, repo)
	ctx := context.Background()

	// Push only convID
	res, err := engine.Push(ctx, syncer.PushOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 1, res.ConversationsSynced)

	// Second conversation should not be in repository
	remoteSecond, err := repo.GetConversation(ctx, "conv-second")
	require.NoError(t, err)
	assert.Nil(t, remoteSecond)
}
