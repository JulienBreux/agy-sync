package syncer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestListener_IgnoreSelfUpdates(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "conv-self-loop"

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  tempBrain,
		MachineID: "local-machine-1",
	}

	repo := firestore.NewMemoryRepository()
	defer func() {
		_ = repo.Close()
	}()

	engine := syncer.NewEngine(cfg, repo)

	// Conversation from current machine
	conv := &models.Conversation{
		ID:            convID,
		SourceMachine: "local-machine-1",
	}

	shouldPull := engine.ShouldSyncRemoteConversation(conv)
	assert.False(t, shouldPull, "Must not pull updates originating from the same machine")
}

func TestListener_AcceptRemoteUpdates(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "conv-remote-update"

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  tempBrain,
		MachineID: "local-machine-1",
	}

	repo := firestore.NewMemoryRepository()
	defer func() {
		_ = repo.Close()
	}()

	engine := syncer.NewEngine(cfg, repo)

	// Conversation from remote machine
	conv := &models.Conversation{
		ID:            convID,
		SourceMachine: "remote-machine-2",
	}

	shouldPull := engine.ShouldSyncRemoteConversation(conv)
	assert.True(t, shouldPull, "Must pull updates originating from a different machine")
}

func TestListener_PollAndSyncRemoteChanges(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "conv-remote-poll"

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  tempBrain,
		MachineID: "local-machine-1",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// Seed remote conversation
	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		SourceMachine:  "remote-machine-2",
		LastSyncedStep: 0,
		UpdatedAt:      time.Now().UTC(),
	}))
	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{StepIndex: 0, Content: "Remote greeting"},
	}))

	engine := syncer.NewEngine(cfg, repo)
	pulledCount, err := engine.SyncRemoteChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, pulledCount)
}
