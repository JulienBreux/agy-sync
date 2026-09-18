package firestore_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestMemoryRepository_ConversationOperations(t *testing.T) {
	ctx := t.Context()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	conv := &models.Conversation{
		ID:             "conv-test-1",
		Title:          "First Conversation",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
		SourceMachine:  "machine-a",
	}

	err := repo.UpsertConversation(ctx, conv)
	require.NoError(t, err)

	fetched, err := repo.GetConversation(ctx, "conv-test-1")
	require.NoError(t, err)
	assert.Equal(t, conv.ID, fetched.ID)
	assert.Equal(t, conv.Title, fetched.Title)
	assert.Equal(t, conv.SourceMachine, fetched.SourceMachine)

	list, err := repo.ListConversations(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, conv.ID, list[0].ID)

	// Non-existent
	notFound, err := repo.GetConversation(ctx, "unknown-id")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestMemoryRepository_StepOperations(t *testing.T) {
	ctx := t.Context()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	convID := "conv-test-steps"

	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Content:   "Step zero",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Thinking:  "Step one thinking",
		},
		{
			StepIndex: 2,
			Source:    "SYSTEM",
			Type:      "SYSTEM_RESPONSE",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Content:   "Step two",
		},
	}

	err := repo.AppendSteps(ctx, convID, steps)
	require.NoError(t, err)

	// Fetch all steps
	allSteps, err := repo.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	assert.Len(t, allSteps, 3)
	assert.Equal(t, 0, allSteps[0].StepIndex)
	assert.Equal(t, 1, allSteps[1].StepIndex)
	assert.Equal(t, 2, allSteps[2].StepIndex)

	// Fetch steps since index 1
	recentSteps, err := repo.GetStepsSince(ctx, convID, 1)
	require.NoError(t, err)
	require.Len(t, recentSteps, 1)
	assert.Equal(t, 2, recentSteps[0].StepIndex)
	assert.Equal(t, "Step two", recentSteps[0].Content)
}

func TestMemoryRepository_ArtifactOperations(t *testing.T) {
	ctx := t.Context()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	artifact := &models.Artifact{
		ID:             "spec.md",
		ConversationID: "conv-artifacts",
		RelativePath:   "spec.md",
		SizeBytes:      100,
		SHA256:         "hash123",
		UpdatedAt:      time.Now().UTC(),
		Content:        []byte("# Specification Content"),
	}

	err := repo.SaveArtifact(ctx, artifact)
	require.NoError(t, err)

	fetched, err := repo.GetArtifact(ctx, "conv-artifacts", "spec.md")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, artifact.ID, fetched.ID)
	assert.Equal(t, artifact.Content, fetched.Content)

	list, err := repo.ListArtifacts(ctx, "conv-artifacts")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestClient_Validation(t *testing.T) {
	ctx := t.Context()

	t.Run("nil config", func(t *testing.T) {
		_, err := firestore.NewClient(ctx, nil)
		require.Error(t, err)
	})

	t.Run("missing project id", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID: "",
		}
		_, err := firestore.NewClient(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project_id is required")
	})

	t.Run("empty conversation_id in DeleteConversation", func(t *testing.T) {
		c := firestore.NewTestClient(nil, nil)
		err := c.DeleteConversation(ctx, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conversation_id cannot be empty")
	})

	t.Run("uninitialized client in DeleteConversation", func(t *testing.T) {
		c := firestore.NewTestClient(nil, nil)
		err := c.DeleteConversation(ctx, "conv-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "firestore client is not initialized")
	})

	t.Run("uninitialized client in ClearAll", func(t *testing.T) {
		c := firestore.NewTestClient(nil, nil)
		err := c.ClearAll(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "firestore client is not initialized")
	})
}

func TestClient_EmulatorIntegration(t *testing.T) {
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("Skipping live emulator test: FIRESTORE_EMULATOR_HOST not set")
	}

	ctx := t.Context()
	cfg := &config.Config{
		ProjectID:  "emulator-test-project",
		DatabaseID: "(default)",
		MachineID:  "emulator-box",
	}

	client, err := firestore.NewClient(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
	})

	convID := "emulator-conv-1"

	// 1. Upsert & Get Conversation
	conv := &models.Conversation{
		ID:             convID,
		Title:          "Emulator Conversation",
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt:      time.Now().UTC().Truncate(time.Millisecond),
		LastSyncedStep: 0,
		SourceMachine:  "test-runner",
	}
	err = client.UpsertConversation(ctx, conv)
	require.NoError(t, err)

	fetchedConv, err := client.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, fetchedConv)
	assert.Equal(t, conv.ID, fetchedConv.ID)
	assert.Equal(t, conv.Title, fetchedConv.Title)

	// 2. Append & Get Steps
	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
			Content:   "Hello emulator",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
			Thinking:  "Emulator thinking",
		},
	}
	err = client.AppendSteps(ctx, convID, steps)
	require.NoError(t, err)

	fetchedSteps, err := client.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	require.Len(t, fetchedSteps, 2)
	assert.Equal(t, 0, fetchedSteps[0].StepIndex)
	assert.Equal(t, 1, fetchedSteps[1].StepIndex)

	// 3. Save & Get Artifact
	artifact := &models.Artifact{
		ID:             "scratch/test.txt",
		ConversationID: convID,
		RelativePath:   "scratch/test.txt",
		SizeBytes:      14,
		SHA256:         "mockhash",
		UpdatedAt:      time.Now().UTC().Truncate(time.Millisecond),
		Content:        []byte("emulator notes"),
	}
	err = client.SaveArtifact(ctx, artifact)
	require.NoError(t, err)

	fetchedArt, err := client.GetArtifact(ctx, convID, "scratch/test.txt")
	require.NoError(t, err)
	require.NotNil(t, fetchedArt)
	assert.Equal(t, artifact.ID, fetchedArt.ID)
	assert.Equal(t, artifact.Content, fetchedArt.Content)

	arts, err := client.ListArtifacts(ctx, convID)
	require.NoError(t, err)
	assert.NotEmpty(t, arts)

	// 4. Delete Conversation
	err = client.DeleteConversation(ctx, convID)
	require.NoError(t, err)

	deletedConv, err := client.GetConversation(ctx, convID)
	require.NoError(t, err)
	assert.Nil(t, deletedConv)

	deletedSteps, err := client.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	assert.Empty(t, deletedSteps)

	// 5. Clear All
	conv2 := &models.Conversation{ID: "emulator-conv-2", Title: "Conv 2"}
	require.NoError(t, client.UpsertConversation(ctx, conv2))

	err = client.ClearAll(ctx)
	require.NoError(t, err)

	allConvs, err := client.ListConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, allConvs)
}

func TestMemoryRepository_DBChunkOperations(t *testing.T) {
	ctx := t.Context()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	convID := "conv-test-chunks"
	chunks := []models.DBChunk{
		{
			ChunkIndex:  0,
			TotalChunks: 2,
			SizeBytes:   4,
			SHA256:      "sha-chunk-0",
			Data:        []byte("part"),
		},
		{
			ChunkIndex:  1,
			TotalChunks: 2,
			SizeBytes:   4,
			SHA256:      "sha-chunk-1",
			Data:        []byte("two!"),
		},
	}

	err := repo.SaveDBChunks(ctx, convID, chunks)
	require.NoError(t, err)

	fetchedChunks, err := repo.GetDBChunks(ctx, convID)
	require.NoError(t, err)
	require.Len(t, fetchedChunks, 2)
	assert.Equal(t, 0, fetchedChunks[0].ChunkIndex)
	assert.Equal(t, []byte("part"), fetchedChunks[0].Data)
	assert.Equal(t, 1, fetchedChunks[1].ChunkIndex)
	assert.Equal(t, []byte("two!"), fetchedChunks[1].Data)

	// Test empty / unknown convID
	emptyChunks, err := repo.GetDBChunks(ctx, "nonexistent-conv")
	require.NoError(t, err)
	assert.Empty(t, emptyChunks)
}

func TestMemoryRepository_ExtendedConversationMetadata(t *testing.T) {
	ctx := t.Context()
	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	now := time.Now().UTC()
	conv := &models.Conversation{
		ID:                     "conv-meta",
		Title:                  "Full Title",
		Preview:                "Preview text",
		StepCount:              10,
		CreatedAt:              now,
		UpdatedAt:              now,
		LastSyncedStep:         9,
		SourceMachine:          "mac",
		DBSHA256:               "sha-db",
		DBSizeBytes:            2048,
		DBChunksCount:          2,
		LastUserInputTime:      now,
		LastUserInputStepIndex: 5,
		RawSummary:             []byte("raw-summary-bytes"),
	}

	err := repo.UpsertConversation(ctx, conv)
	require.NoError(t, err)

	fetched, err := repo.GetConversation(ctx, "conv-meta")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Full Title", fetched.Title)
	assert.Equal(t, "Preview text", fetched.Preview)
	assert.Equal(t, 10, fetched.StepCount)
	assert.Equal(t, "sha-db", fetched.DBSHA256)
	assert.Equal(t, int64(2048), fetched.DBSizeBytes)
	assert.Equal(t, 2, fetched.DBChunksCount)
	assert.Equal(t, []byte("raw-summary-bytes"), fetched.RawSummary)
}
