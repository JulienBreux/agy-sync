package firestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestMemoryRepository_DeleteConversation(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() { _ = repo.Close() })

	conv1 := &models.Conversation{ID: "conv-1", Title: "Conversation 1", CreatedAt: time.Now()}
	conv2 := &models.Conversation{ID: "conv-2", Title: "Conversation 2", CreatedAt: time.Now()}

	require.NoError(t, repo.UpsertConversation(ctx, conv1))
	require.NoError(t, repo.UpsertConversation(ctx, conv2))

	// Add steps, artifacts, db chunks for both conversations
	require.NoError(t, repo.AppendSteps(ctx, "conv-1", []models.Step{
		{StepIndex: 0, Content: "Hello"},
	}))
	require.NoError(t, repo.AppendSteps(ctx, "conv-2", []models.Step{
		{StepIndex: 0, Content: "World"},
	}))

	require.NoError(t, repo.SaveArtifact(ctx, &models.Artifact{
		ID:             "art-1",
		ConversationID: "conv-1",
		RelativePath:   "file.txt",
	}))
	require.NoError(t, repo.SaveArtifact(ctx, &models.Artifact{
		ID:             "art-2",
		ConversationID: "conv-2",
		RelativePath:   "file2.txt",
	}))

	require.NoError(t, repo.SaveDBChunks(ctx, "conv-1", []models.DBChunk{
		{ChunkIndex: 0, TotalChunks: 1},
	}))
	require.NoError(t, repo.SaveDBChunks(ctx, "conv-2", []models.DBChunk{
		{ChunkIndex: 0, TotalChunks: 1},
	}))

	// Delete conv-1
	err := repo.DeleteConversation(ctx, "conv-1")
	require.NoError(t, err)

	// Verify conv-1 data is gone
	gotConv, err := repo.GetConversation(ctx, "conv-1")
	require.NoError(t, err)
	assert.Nil(t, gotConv)

	steps, err := repo.GetStepsSince(ctx, "conv-1", -1)
	require.NoError(t, err)
	assert.Empty(t, steps)

	arts, err := repo.ListArtifacts(ctx, "conv-1")
	require.NoError(t, err)
	assert.Empty(t, arts)

	art, err := repo.GetArtifact(ctx, "conv-1", "art-1")
	require.NoError(t, err)
	assert.Nil(t, art)

	chunks, err := repo.GetDBChunks(ctx, "conv-1")
	require.NoError(t, err)
	assert.Empty(t, chunks)

	// Verify conv-2 data is untouched
	gotConv2, err := repo.GetConversation(ctx, "conv-2")
	require.NoError(t, err)
	assert.NotNil(t, gotConv2)

	steps2, err := repo.GetStepsSince(ctx, "conv-2", -1)
	require.NoError(t, err)
	assert.Len(t, steps2, 1)

	arts2, err := repo.ListArtifacts(ctx, "conv-2")
	require.NoError(t, err)
	assert.Len(t, arts2, 1)

	chunks2, err := repo.GetDBChunks(ctx, "conv-2")
	require.NoError(t, err)
	assert.Len(t, chunks2, 1)

	// Deleting empty ID returns error
	assert.Error(t, repo.DeleteConversation(ctx, ""))
}

func TestMemoryRepository_ClearAll(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() { _ = repo.Close() })

	conv1 := &models.Conversation{ID: "conv-1", Title: "Conversation 1"}
	conv2 := &models.Conversation{ID: "conv-2", Title: "Conversation 2"}

	require.NoError(t, repo.UpsertConversation(ctx, conv1))
	require.NoError(t, repo.UpsertConversation(ctx, conv2))

	require.NoError(t, repo.AppendSteps(ctx, "conv-1", []models.Step{
		{StepIndex: 0, Content: "Hello"},
	}))
	require.NoError(t, repo.SaveArtifact(ctx, &models.Artifact{
		ID:             "art-1",
		ConversationID: "conv-1",
		RelativePath:   "file.txt",
	}))
	require.NoError(t, repo.SaveDBChunks(ctx, "conv-1", []models.DBChunk{
		{ChunkIndex: 0, TotalChunks: 1},
	}))

	// Clear all
	err := repo.ClearAll(ctx)
	require.NoError(t, err)

	// Verify all conversations and subcollections are empty
	list, err := repo.ListConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	steps, err := repo.GetStepsSince(ctx, "conv-1", -1)
	require.NoError(t, err)
	assert.Empty(t, steps)

	arts, err := repo.ListArtifacts(ctx, "conv-1")
	require.NoError(t, err)
	assert.Empty(t, arts)

	chunks, err := repo.GetDBChunks(ctx, "conv-1")
	require.NoError(t, err)
	assert.Empty(t, chunks)
}
