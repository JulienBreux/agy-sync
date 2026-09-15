package firestore_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestMemoryRepository_ConversationOperations(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	defer func() {
		_ = repo.Close()
	}()

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
	assert.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestMemoryRepository_StepOperations(t *testing.T) {
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	defer func() {
		_ = repo.Close()
	}()

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
	ctx := context.Background()
	repo := firestore.NewMemoryRepository()
	defer func() {
		_ = repo.Close()
	}()

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
	ctx := context.Background()

	t.Run("nil config", func(t *testing.T) {
		_, err := firestore.NewClient(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("missing project id", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID: "",
		}
		_, err := firestore.NewClient(ctx, cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project_id is required")
	})
}

func TestClient_EmulatorIntegration(t *testing.T) {
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("Skipping live emulator test: FIRESTORE_EMULATOR_HOST not set")
	}

	ctx := context.Background()
	cfg := &config.Config{
		ProjectID:  "emulator-test-project",
		DatabaseID: "(default)",
		MachineID:  "emulator-box",
	}

	client, err := firestore.NewClient(ctx, cfg)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

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
}
