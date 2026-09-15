package discovery_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/discovery"
)

func createMockBrain(t *testing.T) string {
	t.Helper()
	brainDir := t.TempDir()

	// Conversation 1: complete with transcript and artifacts
	conv1 := filepath.Join(brainDir, "conv-1")
	logs1 := filepath.Join(conv1, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logs1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logs1, "transcript.jsonl"), []byte(`{"step_index":0}`+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(conv1, "artifact1.md"), []byte("# Artifact 1"), 0o644))

	scratch1 := filepath.Join(conv1, "scratch")
	require.NoError(t, os.MkdirAll(scratch1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scratch1, "scratch.txt"), []byte("temp note"), 0o644))

	// Conversation 2: with transcript only
	conv2 := filepath.Join(brainDir, "conv-2")
	logs2 := filepath.Join(conv2, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logs2, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logs2, "transcript.jsonl"), []byte(`{"step_index":0}`+"\n"), 0o644))

	// Non-conversation file in brain dir (should be ignored)
	require.NoError(t, os.WriteFile(filepath.Join(brainDir, "random.txt"), []byte("ignore"), 0o644))

	return brainDir
}

func TestDiscoverConversations(t *testing.T) {
	brainDir := createMockBrain(t)

	convs, err := discovery.DiscoverConversations(brainDir)
	require.NoError(t, err)

	require.Len(t, convs, 2)

	// Check conv-1
	var c1, c2 *discovery.DiscoveredConversation
	for i := range convs {
		switch convs[i].ID {
		case "conv-1":
			c1 = &convs[i]
		case "conv-2":
			c2 = &convs[i]
		}
	}

	require.NotNil(t, c1)
	require.NotNil(t, c2)

	assert.True(t, c1.HasTranscript)
	assert.FileExists(t, c1.TranscriptPath)
	assert.Len(t, c1.Artifacts, 2) // artifact1.md and scratch/scratch.txt

	assert.True(t, c2.HasTranscript)
	assert.Empty(t, c2.Artifacts)
}

func TestDiscoverSpecificConversation(t *testing.T) {
	brainDir := createMockBrain(t)

	c, err := discovery.DiscoverConversation(brainDir, "conv-1")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "conv-1", c.ID)
	assert.True(t, c.HasTranscript)
	assert.Len(t, c.Artifacts, 2)

	// Non-existent conversation
	_, err = discovery.DiscoverConversation(brainDir, "does-not-exist")
	assert.Error(t, err)
}

func TestScanArtifactsExcludesSystemGenerated(t *testing.T) {
	tempDir := t.TempDir()
	convDir := filepath.Join(tempDir, "conv-test")

	// System generated logs and internal files
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte("log"), 0o644))

	// User artifact
	require.NoError(t, os.WriteFile(filepath.Join(convDir, "spec.md"), []byte("# Spec"), 0o644))

	artifacts, err := discovery.ScanArtifacts(convDir)
	require.NoError(t, err)

	require.Len(t, artifacts, 1)
	assert.Equal(t, "spec.md", artifacts[0].RelativePath)
	assert.NotEmpty(t, artifacts[0].SHA256)
	assert.Equal(t, int64(len("# Spec")), artifacts[0].SizeBytes)
}
