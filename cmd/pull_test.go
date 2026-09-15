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

	"github.com/julienbreux/ayg-conv-to-fs/cmd"
	"github.com/julienbreux/ayg-conv-to-fs/pkg/config"
	"github.com/julienbreux/ayg-conv-to-fs/pkg/firestore"
	"github.com/julienbreux/ayg-conv-to-fs/pkg/models"
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

	assert.Error(t, err)
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
	ctx := context.Background()
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
	defer cmd.ResetFirestoreClientFactory()

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
