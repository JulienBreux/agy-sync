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

func TestEngine_Clear_All(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() { _ = repo.Close() })

	cfg := &config.Config{
		ProjectID: "test-proj",
	}
	engine := syncer.NewEngine(cfg, repo)

	// Seed repo with 2 conversations and subcollections
	conv1 := &models.Conversation{ID: "conv-1", Title: "Conv 1", CreatedAt: time.Now()}
	conv2 := &models.Conversation{ID: "conv-2", Title: "Conv 2", CreatedAt: time.Now()}
	require.NoError(t, repo.UpsertConversation(ctx, conv1))
	require.NoError(t, repo.UpsertConversation(ctx, conv2))

	require.NoError(t, repo.AppendSteps(ctx, "conv-1", []models.Step{{StepIndex: 0}}))
	require.NoError(t, repo.AppendSteps(ctx, "conv-2", []models.Step{{StepIndex: 0}}))

	res, err := engine.Clear(ctx, syncer.ClearOptions{})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 2, res.ConversationsDeleted)
	assert.Equal(t, "success", res.Status)
	assert.Empty(t, res.ConversationID)

	convs, err := repo.ListConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, convs)
}

func TestEngine_Clear_SingleConversation(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() { _ = repo.Close() })

	cfg := &config.Config{
		ProjectID: "test-proj",
	}
	engine := syncer.NewEngine(cfg, repo)

	conv1 := &models.Conversation{ID: "conv-1", Title: "Conv 1"}
	conv2 := &models.Conversation{ID: "conv-2", Title: "Conv 2"}
	require.NoError(t, repo.UpsertConversation(ctx, conv1))
	require.NoError(t, repo.UpsertConversation(ctx, conv2))

	res, err := engine.Clear(ctx, syncer.ClearOptions{ConversationID: "conv-1"})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 1, res.ConversationsDeleted)
	assert.Equal(t, "conv-1", res.ConversationID)
	assert.Equal(t, "success", res.Status)

	c1, err := repo.GetConversation(ctx, "conv-1")
	require.NoError(t, err)
	assert.Nil(t, c1)

	c2, err := repo.GetConversation(ctx, "conv-2")
	require.NoError(t, err)
	assert.NotNil(t, c2)
}

func TestEngine_Clear_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() { _ = repo.Close() })

	cfg := &config.Config{
		ProjectID: "test-proj",
	}
	engine := syncer.NewEngine(cfg, repo)

	res, err := engine.Clear(ctx, syncer.ClearOptions{})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, context.Canceled)
}
