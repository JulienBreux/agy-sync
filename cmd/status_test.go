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
	"github.com/julienbreux/agy-sync/internal/daemon"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/config"
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
	t.Cleanup(func() {
		_ = memRepo.Close()
	})
	ctx := t.Context()
	require.NoError(t, memRepo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		LastSyncedStep: 0,
		UpdatedAt:      time.Now().UTC(),
	}))

	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	t.Cleanup(cmd.ResetFirestoreClientFactory)

	stateFile := filepath.Join(tempBrain, "test-none.state.json")
	pidFile := filepath.Join(tempBrain, "test-none.pid")

	// Text mode
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"status", "--config", configPath, "--state-file", stateFile, "--pid-file", pidFile})
	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Antigravity Sync Status")
	assert.Contains(t, buf.String(), "Daemon Status:   STOPPED")
	assert.Contains(t, buf.String(), "Last Polling:    Never / Inactive")
	assert.Contains(t, buf.String(), "SQLite DB Sync:  Enabled")
	assert.Contains(t, buf.String(), "test-status-proj")
	assert.Contains(t, buf.String(), "Sessions Found:  1")
	assert.Contains(t, buf.String(), "Run 'agy-sync status --full' to inspect conversations.")
	assert.NotContains(t, buf.String(), "LOCAL DB")
	assert.NotContains(t, buf.String(), convID)

	// Text mode with --full
	rootFull := cmd.NewRootCommand()
	bufFull := new(bytes.Buffer)
	rootFull.SetOut(bufFull)
	rootFull.SetErr(bufFull)
	rootFull.SetArgs([]string{"status", "--full", "--config", configPath, "--state-file", stateFile, "--pid-file", pidFile})
	err = rootFull.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufFull.String(), "CONVERSATION ID")
	assert.Contains(t, bufFull.String(), "LOCAL DB")
	assert.Contains(t, bufFull.String(), convID)

	// JSON mode default (omit conversations)
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)

	rootJSON.SetArgs([]string{"status", "--config", configPath, "--state-file", stateFile, "--pid-file", pidFile, "--json"})
	err = rootJSON.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"project_id": "test-status-proj"`)
	assert.Contains(t, bufJSON.String(), `"conversations_count": 1`)
	assert.Contains(t, bufJSON.String(), `"db_sync_enabled": true`)
	assert.Contains(t, bufJSON.String(), `"daemon": {`)
	assert.Contains(t, bufJSON.String(), `"state": "STOPPED"`)
	assert.NotContains(t, bufJSON.String(), `"conversations":`)

	// JSON mode with --full (include conversations)
	rootJSONFull := cmd.NewRootCommand()
	bufJSONFull := new(bytes.Buffer)
	rootJSONFull.SetOut(bufJSONFull)
	rootJSONFull.SetErr(bufJSONFull)

	rootJSONFull.SetArgs([]string{"status", "--full", "--config", configPath, "--state-file", stateFile, "--pid-file", pidFile, "--json"})
	err = rootJSONFull.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSONFull.String(), `"conversations": [`)
	assert.Contains(t, bufJSONFull.String(), `"id": "status-conv-1"`)

	// Status with --no-db-sync flag
	rootDisabled := cmd.NewRootCommand()
	bufDisabled := new(bytes.Buffer)
	rootDisabled.SetOut(bufDisabled)
	rootDisabled.SetErr(bufDisabled)

	rootDisabled.SetArgs([]string{"status", "--config", configPath, "--state-file", stateFile, "--no-db-sync"})
	err = rootDisabled.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufDisabled.String(), "SQLite DB Sync:  Disabled")

	// Single conversation filter with --full
	rootFilter := cmd.NewRootCommand()
	bufFilter := new(bytes.Buffer)
	rootFilter.SetOut(bufFilter)
	rootFilter.SetErr(bufFilter)

	rootFilter.SetArgs([]string{"status", convID, "--full", "--config", configPath, "--state-file", stateFile})
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remediation:")
}

func TestStatusCommand_WithLastPollingDate(t *testing.T) {
	tempBrain := t.TempDir()
	configPath := filepath.Join(tempBrain, "config.yaml")
	cfgContent := `brain_dir: ` + tempBrain + `
project_id: "test-status-proj"
machine_id: "status-machine-1"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	statePath := filepath.Join(tempBrain, "daemon.state.json")
	mgr := daemon.NewManagerWithState("", "", statePath)
	fixedTime := time.Date(2026, 9, 15, 16, 40, 0, 0, time.UTC)
	require.NoError(t, mgr.RecordPoll(fixedTime))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "--config", configPath, "--state-file", statePath})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Last Polling:    2026-09-15 16:40:00 UTC")

	// JSON mode with last polled at
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)
	rootJSON.SetArgs([]string{"status", "--config", configPath, "--state-file", statePath, "--json"})

	err = rootJSON.Execute()
	require.NoError(t, err)
	assert.Contains(t, bufJSON.String(), `"last_polled_at": "2026-09-15T16:40:00Z"`)
}

func TestStatusCommand_Full_Empty(t *testing.T) {
	tempBrain := t.TempDir()
	configPath := filepath.Join(tempBrain, "config.yaml")
	cfgContent := `brain_dir: ` + tempBrain + `
project_id: "test-status-empty"
machine_id: "status-machine-empty"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "--full", "--config", configPath})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No conversations found in brain directory.")
}

func TestStatusCommand_Full_InteractivePager(t *testing.T) {
	tempBrain := t.TempDir()
	configPath := filepath.Join(tempBrain, "config.yaml")
	cfgContent := `brain_dir: ` + tempBrain + `
project_id: "test-status-pager"
machine_id: "status-machine-pager"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	convID := "status-conv-interactive"
	convDir := filepath.Join(tempBrain, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte(`{"step_index":0,"content":"hello"}`+"\n"), 0o644))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	cmd.SetIsTerminalFunc(func() bool { return true })
	defer cmd.ResetIsTerminalFunc()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetIn(bytes.NewReader([]byte{'q'}))
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "--full", "--config", configPath})

	err := root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Page 1 of 1")
	assert.Contains(t, buf.String(), "[↑/↓] Row")
	assert.Contains(t, buf.String(), "[q] Quit")
	assert.Contains(t, buf.String(), convID)
}
