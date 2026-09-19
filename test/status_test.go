package test_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/daemon"
)

func TestE2E_StatusCommand_DefaultVsFull(t *testing.T) {
	tempBrain := t.TempDir()
	configPath := filepath.Join(tempBrain, "config.yaml")
	cfgContent := `brain_dir: ` + tempBrain + `
project_id: "e2e-status-project"
machine_id: "e2e-status-machine"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	statePath := filepath.Join(tempBrain, "daemon.state.json")
	mgr := daemon.NewManagerWithState("", "", statePath)
	fixedTime := time.Date(2026, 9, 19, 8, 30, 0, 0, time.UTC)
	require.NoError(t, mgr.RecordPoll(fixedTime))

	// Create test conversation
	convID := "e2e-status-conv-12345"
	convDir := filepath.Join(tempBrain, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte(`{"step_index":0,"content":"hello world"}`+"\n"), 0o644))

	// 1. Default Compact Mode (no --full)
	rootDefault := cmd.NewRootCommand()
	bufDefault := new(bytes.Buffer)
	rootDefault.SetOut(bufDefault)
	rootDefault.SetErr(bufDefault)
	rootDefault.SetArgs([]string{"status", "--config", configPath, "--state-file", statePath})

	err := rootDefault.Execute()
	require.NoError(t, err)
	defaultOut := bufDefault.String()

	assert.Contains(t, defaultOut, "Antigravity Sync Status")
	assert.Contains(t, defaultOut, "Sessions Found:  1")
	assert.Contains(t, defaultOut, "Run 'agy-sync status --full' to inspect conversations.")
	assert.NotContains(t, defaultOut, "CONVERSATION ID")
	assert.NotContains(t, defaultOut, convID)

	// 2. Full Mode (--full)
	rootFull := cmd.NewRootCommand()
	bufFull := new(bytes.Buffer)
	rootFull.SetOut(bufFull)
	rootFull.SetErr(bufFull)
	rootFull.SetArgs([]string{"status", "--full", "--config", configPath, "--state-file", statePath})

	err = rootFull.Execute()
	require.NoError(t, err)
	fullOut := bufFull.String()

	assert.Contains(t, fullOut, "Antigravity Sync Status")
	assert.Contains(t, fullOut, "CONVERSATION ID")
	assert.Contains(t, fullOut, convID)

	// 3. JSON Default Mode (omits conversations)
	rootJSON := cmd.NewRootCommand()
	bufJSON := new(bytes.Buffer)
	rootJSON.SetOut(bufJSON)
	rootJSON.SetErr(bufJSON)
	rootJSON.SetArgs([]string{"status", "--json", "--config", configPath, "--state-file", statePath})

	err = rootJSON.Execute()
	require.NoError(t, err)

	var reportDefault cmd.StatusReport
	require.NoError(t, json.Unmarshal(bufJSON.Bytes(), &reportDefault))
	assert.Equal(t, 1, reportDefault.ConversationsCount)
	assert.Nil(t, reportDefault.Conversations)
	assert.NotContains(t, bufJSON.String(), `"conversations"`)

	// 4. JSON Full Mode (includes conversations)
	rootJSONFull := cmd.NewRootCommand()
	bufJSONFull := new(bytes.Buffer)
	rootJSONFull.SetOut(bufJSONFull)
	rootJSONFull.SetErr(bufJSONFull)
	rootJSONFull.SetArgs([]string{"status", "--json", "--full", "--config", configPath, "--state-file", statePath})

	err = rootJSONFull.Execute()
	require.NoError(t, err)

	var reportFull cmd.StatusReport
	require.NoError(t, json.Unmarshal(bufJSONFull.Bytes(), &reportFull))
	assert.Equal(t, 1, reportFull.ConversationsCount)
	require.Len(t, reportFull.Conversations, 1)
	assert.Equal(t, convID, reportFull.Conversations[0].ID)
	assert.Contains(t, bufJSONFull.String(), `"conversations"`)
}
