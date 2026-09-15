package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/internal/firestore"
)

func TestPushCommand_MissingConfig(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentConfig := filepath.Join(tempDir, "missing.yaml")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"push", "--config", nonExistentConfig})
	err := root.Execute()

	assert.Error(t, err)
}

func TestPushCommand_NoProject(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: ""
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"push", "--config", configPath})
	err := root.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestPushCommand_Success(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create conversation folder in brain
	convID := "cmd-push-conv"
	logsDir := filepath.Join(tempDir, convID, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(`{"step_index":0,"content":"hi"}`+"\n"), 0o644))

	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-cmd-proj"
machine_id: "test-box"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	// Stub firestore client with memory repo
	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	// 1. Text output
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"push", "--config", configPath, "--conversation", convID})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Sync push complete")

	// 2. JSON output
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)

	rootJSON.SetArgs([]string{"push", "--config", configPath, "--json"})
	err = rootJSON.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"conversations_synced": 1`)
}
