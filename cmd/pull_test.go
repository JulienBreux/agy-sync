package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestPullCommand_MissingConversationID(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-proj"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"pull", "--config", configPath})
	err := root.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation ID is required")
}

func TestPullCommand_Success(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-proj"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	convID := "conv-pull-cli"
	memRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = memRepo.Close()
	})
	ctx := t.Context()
	require.NoError(t, memRepo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "Pull CLI Test",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
	}))
	require.NoError(t, memRepo.AppendSteps(ctx, convID, []models.Step{
		{StepIndex: 0, Content: "Hello from remote"},
	}))

	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	t.Cleanup(cmd.ResetFirestoreClientFactory)

	// 1. Text output
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"pull", convID, "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Sync pull complete")

	// 2. JSON output
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)

	rootJSON.SetArgs([]string{"pull", "--conversation", convID, "--config", configPath, "--json"})
	err = rootJSON.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"conversation_id": "conv-pull-cli"`)
}

func TestPullCommand_Flags_SQLiteOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-proj"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	convID := "conv-pull-flags"
	memRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = memRepo.Close()
	})
	ctx := t.Context()
	require.NoError(t, memRepo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "Pull Flags Test",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
	}))
	require.NoError(t, memRepo.AppendSteps(ctx, convID, []models.Step{
		{StepIndex: 0, Content: "Hello from remote with flags"},
	}))

	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	t.Cleanup(cmd.ResetFirestoreClientFactory)

	customConvs := filepath.Join(tempDir, "custom-conversations")
	customSummaries := filepath.Join(tempDir, "custom-summaries.db")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{
		"pull", convID,
		"--config", configPath,
		"--conversations-dir", customConvs,
		"--summaries-db", customSummaries,
	})
	err := root.Execute()
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(customConvs, convID+".db"))
	assert.FileExists(t, customSummaries)

	// Test --no-db-sync flag
	convID2 := "conv-pull-no-db"
	require.NoError(t, memRepo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID2,
		Title:          "Pull No DB Test",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
	}))
	require.NoError(t, memRepo.AppendSteps(ctx, convID2, []models.Step{
		{StepIndex: 0, Content: "No DB sync"},
	}))

	rootNoDB := cmd.NewRootCommand()
	bufNoDB := new(bytes.Buffer)
	rootNoDB.SetOut(bufNoDB)
	rootNoDB.SetErr(bufNoDB)

	rootNoDB.SetArgs([]string{
		"pull", convID2,
		"--config", configPath,
		"--conversations-dir", customConvs,
		"--summaries-db", customSummaries,
		"--no-db-sync",
	})
	err = rootNoDB.Execute()
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(customConvs, convID2+".db"))
}
