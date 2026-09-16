package syncer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestPull_ReconstructNewConversation(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "remote-conv-1"

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  tempBrain,
		MachineID: "machine-puller",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := t.Context()

	// Seed remote repository with conversation, steps, and artifacts
	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "Remote Seed Conversation",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 1,
		SourceMachine:  "remote-machine",
	}))

	steps := []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Content:   "Build an app",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Thinking:  "Planning app architecture...",
		},
	}
	require.NoError(t, repo.AppendSteps(ctx, convID, steps))

	require.NoError(t, repo.SaveArtifact(ctx, &models.Artifact{
		ID:             "design.md",
		ConversationID: convID,
		RelativePath:   "design.md",
		SizeBytes:      int64(len("# System Design")),
		SHA256:         "mocksha",
		UpdatedAt:      time.Now().UTC(),
		Content:        []byte("# System Design"),
	}))

	engine := syncer.NewEngine(cfg, repo)
	result, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	assert.Equal(t, convID, result.ConversationID)
	assert.Equal(t, 2, result.StepsPulled)
	assert.Equal(t, 1, result.ArtifactsPulled)

	// Verify local filesystem reconstruction
	localConvDir := filepath.Join(tempBrain, convID)
	assert.DirExists(t, localConvDir)

	transcriptPath := filepath.Join(localConvDir, ".system_generated", "logs", "transcript.jsonl")
	assert.FileExists(t, transcriptPath)

	content, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"Build an app"`)
	assert.Contains(t, string(content), `"Planning app architecture..."`)

	artifactPath := filepath.Join(localConvDir, "design.md")
	assert.FileExists(t, artifactPath)
	artBytes, err := os.ReadFile(artifactPath)
	require.NoError(t, err)
	assert.Equal(t, "# System Design", string(artBytes))
}

func TestPull_IncrementalUpdate(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "existing-conv"

	// Pre-create local conversation with step 0
	localLogs := filepath.Join(tempBrain, convID, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(localLogs, 0o755))
	transcriptPath := filepath.Join(localLogs, "transcript.jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte(`{"step_index":0,"content":"initial"}`+"\n"), 0o644))

	cfg := &config.Config{
		ProjectID: "test-proj",
		BrainDir:  tempBrain,
		MachineID: "machine-puller",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := t.Context()

	// Remote has step 0 and step 1
	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		LastSyncedStep: 1,
	}))
	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{StepIndex: 0, Content: "initial"},
		{StepIndex: 1, Content: "remote follow-up"},
	}))

	engine := syncer.NewEngine(cfg, repo)
	res, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	assert.Equal(t, 1, res.StepsPulled) // only step 1 should be pulled and appended

	// Verify local transcript now has 2 lines
	content, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"initial"`)
	assert.Contains(t, string(content), `"remote follow-up"`)
}

func TestPull_MissingConversationID(t *testing.T) {
	engine := syncer.NewEngine(&config.Config{}, nil)
	_, err := engine.Pull(t.Context(), syncer.PullOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation_id is required")
}

func TestPull_ReconstructSQLiteDB(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "pull-sqlite-test"

	cfg := &config.Config{
		ProjectID:        "test-proj",
		BrainDir:         tempBrain,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		NoDBSync:         false,
		MachineID:        "machine-puller",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := t.Context()

	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "SQLite Pull Test Title",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
		SourceMachine:  "remote-machine",
	}))

	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Content:   "Testing SQLite reconstruction on pull",
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	_, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	// Verify conversation DB was created
	convDBPath := filepath.Join(convsDir, convID+".db")
	assert.FileExists(t, convDBPath)

	// Verify summaries DB was created
	assert.FileExists(t, summariesDB)
}

func TestPull_NoDBSync(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "no-db-sync-test"

	cfg := &config.Config{
		ProjectID:        "test-proj",
		BrainDir:         tempBrain,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		NoDBSync:         true,
		MachineID:        "machine-puller",
	}

	repo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = repo.Close()
	})

	ctx := t.Context()

	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "No DB Sync Title",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		LastSyncedStep: 0,
		SourceMachine:  "remote-machine",
	}))

	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC(),
			Content:   "Testing No DB Sync",
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	_, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	convDBPath := filepath.Join(convsDir, convID+".db")
	assert.NoFileExists(t, convDBPath)
	assert.NoFileExists(t, summariesDB)
}
