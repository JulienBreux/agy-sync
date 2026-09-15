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
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/firestore"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestStatusCommand_Success(t *testing.T) {
	tempBrain := t.TempDir()
	configPath := filepath.Join(tempBrain, "config.yaml")
	cfgContent := `brain_dir: ` + tempBrain + `
project_id: "test-status-proj"
machine_id: "status-machine-1"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	convID := "status-conv-1"
	convDir := filepath.Join(tempBrain, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte(`{"step_index":0,"content":"hello"}`+"\n"), 0o644))

	memRepo := firestore.NewMemoryRepository()
	ctx := context.Background()
	require.NoError(t, memRepo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		LastSyncedStep: 0,
		UpdatedAt:      time.Now().UTC(),
	}))

	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	// Text mode
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"status", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Antigravity Sync Status")
	assert.Contains(t, buf.String(), "Daemon Status:   STOPPED")
	assert.Contains(t, buf.String(), "test-status-proj")
	assert.Contains(t, buf.String(), convID)

	// JSON mode
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)

	rootJSON.SetArgs([]string{"status", "--config", configPath, "--json"})
	err = rootJSON.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"project_id": "test-status-proj"`)
	assert.Contains(t, bufJSON.String(), `"conversations_count": 1`)
	assert.Contains(t, bufJSON.String(), `"daemon": {`)
	assert.Contains(t, bufJSON.String(), `"state": "STOPPED"`)

	// Single conversation filter
	rootFilter := cmd.NewRootCommand()
	bufFilter := new(bytes.Buffer)
	rootFilter.SetOut(bufFilter)
	rootFilter.SetErr(bufFilter)

	rootFilter.SetArgs([]string{"status", convID, "--config", configPath})
	err = rootFilter.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufFilter.String(), convID)
	assert.Contains(t, bufFilter.String(), "Sessions Found:  1")
}

func TestStatusCommand_MissingConfig(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"status", "--config", "/nonexistent/path/config.yaml"})
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remediation:")
}
