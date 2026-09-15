package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/watcher"
)

func TestWatcher_DetectTranscriptChange(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "test-watch-conv"
	convDir := filepath.Join(tempBrain, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	transcriptFile := filepath.Join(logsDir, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptFile, []byte(""), 0o644))

	w, err := watcher.NewWatcher(tempBrain, 50*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		_ = w.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := w.Start(ctx)
	require.NoError(t, err)

	// Append to transcript
	time.Sleep(50 * time.Millisecond) // Let watcher attach
	f, err := os.OpenFile(transcriptFile, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(`{"step_index":0,"content":"hello world"}` + "\n")
	require.NoError(t, err)
	_ = f.Close()

	select {
	case event := <-events:
		assert.Equal(t, convID, event.ConversationID)
		assert.True(t, event.IsTranscript)
	case err := <-errs:
		t.Fatalf("unexpected error from watcher: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transcript watch event")
	}
}

func TestWatcher_DetectNewConversation(t *testing.T) {
	tempBrain := t.TempDir()

	w, err := watcher.NewWatcher(tempBrain, 50*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		_ = w.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := w.Start(ctx)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a new conversation and its transcript
	newConvID := "dynamic-conv-123"
	logsDir := filepath.Join(tempBrain, newConvID, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Re-scan / watch new dir
	require.NoError(t, w.WatchDirectory(logsDir))

	transcriptFile := filepath.Join(logsDir, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptFile, []byte(`{"step_index":0}`+"\n"), 0o644))

	select {
	case event := <-events:
		assert.Equal(t, newConvID, event.ConversationID)
		assert.True(t, event.IsTranscript)
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for new conversation event")
	}
}

func TestWatcher_ExtractConversationID(t *testing.T) {
	brain := "/Users/test/.gemini/antigravity-cli/brain"
	target := "/Users/test/.gemini/antigravity-cli/brain/conv-456/.system_generated/logs/transcript.jsonl"

	convID, isTranscript := watcher.InspectPath(brain, target)
	assert.Equal(t, "conv-456", convID)
	assert.True(t, isTranscript)

	// Artifact
	artPath := "/Users/test/.gemini/antigravity-cli/brain/conv-456/notes.md"
	artConvID, isTranscript2 := watcher.InspectPath(brain, artPath)
	assert.Equal(t, "conv-456", artConvID)
	assert.False(t, isTranscript2)

	// Outside path
	outsideConvID, _ := watcher.InspectPath(brain, "/tmp/other/notes.md")
	assert.Empty(t, outsideConvID)
}
