package syncer_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/internal/transaction"
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

func TestPull_DBChunkReassemblyAndSummary(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "chunk-pull-conv"

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

	fullBinaryData := []byte("SQLite format 3\x00-actual-database-binary-content-reassembled-from-chunks")
	fullHash := sha256.Sum256(fullBinaryData)
	fullHashHex := hex.EncodeToString(fullHash[:])

	now := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:                     convID,
		Title:                  "Reassembled Title",
		Preview:                "Reassembled Preview",
		StepCount:              5,
		CreatedAt:              now,
		UpdatedAt:              now,
		LastSyncedStep:         0,
		SourceMachine:          "remote-laptop",
		DBSHA256:               fullHashHex,
		DBSizeBytes:            int64(len(fullBinaryData)),
		DBChunksCount:          2,
		LastUserInputTime:      now,
		LastUserInputStepIndex: 0,
		RawSummary:             []byte("raw-protobuf-bytes"),
	}))

	chunk0 := fullBinaryData[:20]
	chunk1 := fullBinaryData[20:]
	hash0 := sha256.Sum256(chunk0)
	hash1 := sha256.Sum256(chunk1)

	require.NoError(t, repo.SaveDBChunks(ctx, convID, []models.DBChunk{
		{
			ChunkIndex:  0,
			TotalChunks: 2,
			SizeBytes:   len(chunk0),
			SHA256:      hex.EncodeToString(hash0[:]),
			Data:        chunk0,
		},
		{
			ChunkIndex:  1,
			TotalChunks: 2,
			SizeBytes:   len(chunk1),
			SHA256:      hex.EncodeToString(hash1[:]),
			Data:        chunk1,
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	res, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, convID, res.ConversationID)

	// 1. Verify local SQLite DB was reassembled with exact byte-for-byte content
	convDBPath := filepath.Join(convsDir, convID+".db")
	require.FileExists(t, convDBPath)
	fileBytes, err := os.ReadFile(convDBPath)
	require.NoError(t, err)
	assert.Equal(t, fullBinaryData, fileBytes)

	// 2. Verify summary in conversation_summaries.db
	rec := reconstructor.New(convsDir, summariesDB)
	summary, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, "Reassembled Title", summary.Title)
	assert.Equal(t, "Reassembled Preview", summary.Preview)
	assert.Equal(t, 5, summary.StepCount)
	assert.Equal(t, []byte("raw-protobuf-bytes"), summary.RawSummary)
}

func TestPull_FallbackToTranscriptDefaultTitle(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "fallback-conv"

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

	// Seed conversation without DB chunks, empty title, and a prompt
	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "",
		Preview:        "Write a REST API in Go",
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
			Content:   "Write a REST API in Go",
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	_, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	rec := reconstructor.New(convsDir, summariesDB)
	summary, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, summary)
	// Title must NOT be empty
	assert.Equal(t, "Write a REST API in Go", summary.Title)
	assert.Equal(t, "Write a REST API in Go", summary.Preview)
}

func TestPull_RestoreAll21ColumnsWithAdaptedURIs(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "pull-conv-all-21"

	cfg := &config.Config{
		ProjectID:        "local-proj",
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
	t1 := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 18, 12, 5, 0, 0, time.UTC)

	// Seed dummy SQLite chunk
	dummyChunkData := make([]byte, 16)
	copy(dummyChunkData, []byte("SQLite format 3\x00"))
	hash := sha256.Sum256(dummyChunkData)
	hashHex := hex.EncodeToString(hash[:])

	// Seed conversation with all 21 columns
	remoteConv := &models.Conversation{
		ID:                     convID,
		Title:                  "Remote Title",
		Preview:                "Remote Preview",
		StepCount:              10,
		CreatedAt:              t1,
		UpdatedAt:              t2,
		LastSyncedStep:         2,
		SourceMachine:          "remote-machine",
		WorkspaceURIs:          []string{"file:///Users/remoteuser/Projects/myproject"},
		Status:                 "active",
		Source:                 "cli",
		ProjectID:              "remote-proj",
		AgentName:              "conductor",
		ParentConversationID:   "parent-xyz",
		NestingDepth:           1,
		BattleID:               "battle-1",
		WinningConversationID:  "winner-1",
		NotFullyIdle:           true,
		Killed:                 false,
		LastUserInputTime:      t1,
		LastUserInputStepIndex: 3,
		AppDataDir:             "/Users/remoteuser/.gemini/antigravity-cli",
		RawSummary:             []byte{0xDE, 0xAD},
		GroupID:                "grp-456",
		DBChunksCount:          1,
		DBSizeBytes:            16,
		DBSHA256:               hashHex,
	}
	require.NoError(t, repo.UpsertConversation(ctx, remoteConv))

	require.NoError(t, repo.SaveDBChunks(ctx, convID, []models.DBChunk{
		{
			ChunkIndex:  0,
			TotalChunks: 1,
			SizeBytes:   len(dummyChunkData),
			SHA256:      hashHex,
			Data:        dummyChunkData,
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	_, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	rec := reconstructor.New(convsDir, summariesDB)
	summary, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, "Remote Title", summary.Title)
	assert.Equal(t, "Remote Preview", summary.Preview)
	assert.Equal(t, 10, summary.StepCount)

	// Verify workspace URI adaptation to local machine home
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expectedURI := "file://" + home + "/Projects/myproject"
	assert.Equal(t, []string{expectedURI}, summary.WorkspaceURIs)

	assert.Equal(t, "active", summary.Status)
	assert.Equal(t, "cli", summary.Source)
	assert.Equal(t, "remote-proj", summary.ProjectID)
	assert.Equal(t, "conductor", summary.AgentName)
	assert.Equal(t, "parent-xyz", summary.ParentConversationID)
	assert.Equal(t, 1, summary.NestingDepth)
	assert.Equal(t, "battle-1", summary.BattleID)
	assert.Equal(t, "winner-1", summary.WinningConversationID)
	assert.True(t, summary.NotFullyIdle)
	assert.False(t, summary.Killed)
	assert.Equal(t, 3, summary.LastUserInputStepIndex)
	assert.Equal(t, []byte{0xDE, 0xAD}, summary.RawSummary)
	assert.Equal(t, "grp-456", summary.GroupID)
}

func TestPull_FallbackTranscript_RestoreAll21Columns(t *testing.T) {
	tempDir := t.TempDir()
	tempBrain := filepath.Join(tempDir, "brain")
	convsDir := filepath.Join(tempDir, "conversations")
	summariesDB := filepath.Join(tempDir, "conversation_summaries.db")
	convID := "pull-conv-fallback-21"

	cfg := &config.Config{
		ProjectID:        "local-proj",
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
	t1 := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	// Seed conversation without DB chunks
	remoteConv := &models.Conversation{
		ID:                    convID,
		Title:                 "Fallback Title",
		Preview:               "Fallback Preview",
		CreatedAt:             t1,
		UpdatedAt:             t1,
		WorkspaceURIs:         []string{"file:///home/remoteuser/workspace"},
		Status:                "active",
		Source:                "cli",
		ProjectID:             "remote-proj",
		AgentName:             "conductor",
		ParentConversationID:  "parent-fallback",
		NestingDepth:          2,
		BattleID:              "battle-fb",
		WinningConversationID: "winner-fb",
		NotFullyIdle:          false,
		Killed:                true,
		RawSummary:            []byte{0xBE, 0xEF},
		GroupID:               "grp-fb",
	}
	require.NoError(t, repo.UpsertConversation(ctx, remoteConv))

	// Seed 1 step in repo so transcript is created
	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: t1,
			Content:   "Fallback prompt",
		},
	}))

	engine := syncer.NewEngine(cfg, repo)
	_, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)

	rec := reconstructor.New(convsDir, summariesDB)
	summary, err := rec.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, "Fallback Title", summary.Title)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, []string{"file://" + home + "/workspace"}, summary.WorkspaceURIs)
	assert.Equal(t, "active", summary.Status)
	assert.Equal(t, "cli", summary.Source)
	assert.Equal(t, "remote-proj", summary.ProjectID)
	assert.Equal(t, "conductor", summary.AgentName)
	assert.Equal(t, "parent-fallback", summary.ParentConversationID)
	assert.Equal(t, 2, summary.NestingDepth)
	assert.Equal(t, "battle-fb", summary.BattleID)
	assert.Equal(t, "winner-fb", summary.WinningConversationID)
	assert.False(t, summary.NotFullyIdle)
	assert.True(t, summary.Killed)
	assert.Equal(t, []byte{0xBE, 0xEF}, summary.RawSummary)
	assert.Equal(t, "grp-fb", summary.GroupID)
}

func TestPull_RecordsTransactions(t *testing.T) {
	tempBrain := t.TempDir()
	convID := "pull-tx-conv"
	txDBPath := filepath.Join(t.TempDir(), "tx.db")
	txStore, err := transaction.NewStore(txDBPath)
	require.NoError(t, err)
	defer func() { _ = txStore.Close() }()

	cfg := &config.Config{
		ProjectID:      "test-proj",
		BrainDir:       tempBrain,
		MachineID:      "pull-machine",
		TransactionsDB: txDBPath,
	}

	repo := firestore.NewMemoryRepository()
	defer func() { _ = repo.Close() }()

	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.UpsertConversation(ctx, &models.Conversation{
		ID:             convID,
		Title:          "Pulled Conv",
		CreatedAt:      now,
		UpdatedAt:      now,
		LastSyncedStep: 0,
		StepCount:      1,
		SourceMachine:  "other-machine",
	}))

	require.NoError(t, repo.AppendSteps(ctx, convID, []models.Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: now,
			Content:   "Hello from remote",
			MachineID: "other-machine",
		},
	}))

	require.NoError(t, repo.SaveArtifact(ctx, &models.Artifact{
		ID:             "result.txt",
		ConversationID: convID,
		RelativePath:   "result.txt",
		SizeBytes:      11,
		SHA256:         "dummy",
		UpdatedAt:      now,
		Content:        []byte("hello world"),
	}))

	engine := syncer.NewEngine(cfg, repo)
	engine.SetTransactionStore(txStore)

	result, err := engine.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, convID, result.ConversationID)
	assert.Equal(t, 1, result.StepsPulled)
	assert.Equal(t, 1, result.ArtifactsPulled)

	// Verify transactions recorded
	txs, err := txStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn})
	require.NoError(t, err)
	require.NotEmpty(t, txs)

	// Check conv import
	convTxs, err := txStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn, EntityType: transaction.EntityTypeConv})
	require.NoError(t, err)
	require.Len(t, convTxs, 1)
	assert.Equal(t, convID, convTxs[0].ConversationID)

	// Check artifact import
	artTxs, err := txStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn, EntityType: transaction.EntityTypeArtifact})
	require.NoError(t, err)
	require.Len(t, artTxs, 1)
	assert.Equal(t, "result.txt", artTxs[0].EntityID)

	// Check brain import
	brainTxs, err := txStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn, EntityType: transaction.EntityTypeBrain})
	require.NoError(t, err)
	require.Len(t, brainTxs, 1)
	assert.Equal(t, "transcript.jsonl", brainTxs[0].EntityID)
}


