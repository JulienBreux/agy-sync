package test_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/discovery"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/internal/transaction"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestE2E_MultiMachineRoundTripSync(t *testing.T) {
	ctx := t.Context()

	// 1. Shared Firestore Repository (simulates cloud firestore instance)
	sharedRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = sharedRepo.Close()
	})

	// 2. Setup Machine Alpha (Machine A)
	machineAlphaBrain := t.TempDir()
	alphaConvsDir := filepath.Join(t.TempDir(), "alpha_conversations")
	alphaSummariesDB := filepath.Join(t.TempDir(), "alpha_summaries.db")
	cfgAlpha := &config.Config{
		ProjectID:        "e2e-project",
		BrainDir:         machineAlphaBrain,
		MachineID:        "machine-alpha",
		ConversationsDir: alphaConvsDir,
		SummariesDB:      alphaSummariesDB,
	}

	convID := "624296c6-d623-4c39-92d4-3906f8c07140"
	alphaConvDir := filepath.Join(machineAlphaBrain, convID)
	alphaLogsDir := filepath.Join(alphaConvDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(alphaLogsDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(alphaConvDir, "scratch"), 0o755))

	// Initial transcript on Alpha
	alphaTranscriptPath := filepath.Join(alphaLogsDir, "transcript.jsonl")
	step0 := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T10:00:00Z","content":"Please design a sync tool"}` + "\n"
	step1 := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T10:01:00Z","thinking":"Designing architecture","tool_calls":[{"id":"call_1","name":"write_file","arguments":{"file":"design.md"}}]}` + "\n"
	step2 := `{"step_index":2,"source":"SYSTEM","type":"TOOL_RESULT","status":"DONE","created_at":"2026-09-15T10:02:00Z","content":"File created successfully"}` + "\n"
	require.NoError(t, os.WriteFile(alphaTranscriptPath, []byte(step0+step1+step2), 0o644))

	// Artifacts on Alpha
	designContent := []byte("# System Design Architecture Document\n\nHybrid Bidirectional Sync.")
	scratchContent := []byte("temporary scratch notes for development")
	require.NoError(t, os.WriteFile(filepath.Join(alphaConvDir, "design.md"), designContent, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(alphaConvDir, "scratch", "test.txt"), scratchContent, 0o644))

	// Machine Alpha Push to Firestore
	engineAlpha := syncer.NewEngine(cfgAlpha, sharedRepo)
	pushResultAlpha, err := engineAlpha.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushResultAlpha.ConversationsSynced)
	assert.Equal(t, 3, pushResultAlpha.StepsSynced)
	assert.Equal(t, 2, pushResultAlpha.ArtifactsSynced)

	// Verify remote state in Firestore
	remoteConv, err := sharedRepo.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, remoteConv)
	assert.Equal(t, 2, remoteConv.LastSyncedStep)
	assert.Equal(t, "machine-alpha", remoteConv.SourceMachine)

	remoteSteps, err := sharedRepo.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	assert.Len(t, remoteSteps, 3)

	remoteArts, err := sharedRepo.ListArtifacts(ctx, convID)
	require.NoError(t, err)
	assert.Len(t, remoteArts, 2)

	// 3. Setup Machine Beta (Machine B) - initially empty brain dir
	machineBetaBrain := t.TempDir()
	betaConvsDir := filepath.Join(t.TempDir(), "beta_conversations")
	betaSummariesDB := filepath.Join(t.TempDir(), "beta_summaries.db")
	cfgBeta := &config.Config{
		ProjectID:        "e2e-project",
		BrainDir:         machineBetaBrain,
		MachineID:        "machine-beta",
		ConversationsDir: betaConvsDir,
		SummariesDB:      betaSummariesDB,
	}

	engineBeta := syncer.NewEngine(cfgBeta, sharedRepo)

	// Machine Beta Pulls the conversation
	pullResultBeta, err := engineBeta.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 3, pullResultBeta.StepsPulled)
	assert.Equal(t, 2, pullResultBeta.ArtifactsPulled)

	// 4. Verify exact line-by-line fidelity on Machine Beta
	betaTranscriptPath := filepath.Join(machineBetaBrain, convID, ".system_generated", "logs", "transcript.jsonl")
	assert.FileExists(t, betaTranscriptPath)

	// Parse both transcripts and verify exact structural equivalence
	p := parser.NewTranscriptParser()
	alphaParse, err := p.ParseFile(alphaTranscriptPath)
	require.NoError(t, err)
	betaParse, err := p.ParseFile(betaTranscriptPath)
	require.NoError(t, err)

	assert.Equal(t, alphaParse.LastStepIndex, betaParse.LastStepIndex)
	require.Len(t, betaParse.Steps, len(alphaParse.Steps))
	assert.True(t, slices.EqualFunc(alphaParse.Steps, betaParse.Steps, func(a, b models.Step) bool {
		return a.StepIndex == b.StepIndex &&
			a.Source == b.Source &&
			a.Type == b.Type &&
			a.Content == b.Content
	}))

	// Verify exact byte fidelity and SHA256 checksums of artifacts on Machine Beta
	betaDesignPath := filepath.Join(machineBetaBrain, convID, "design.md")
	betaScratchPath := filepath.Join(machineBetaBrain, convID, "scratch", "test.txt")
	assert.FileExists(t, betaDesignPath)
	assert.FileExists(t, betaScratchPath)

	betaDesignBytes, err := os.ReadFile(betaDesignPath)
	require.NoError(t, err)
	assert.Equal(t, designContent, betaDesignBytes)
	assert.Equal(t, sha256Bytes(designContent), sha256Bytes(betaDesignBytes))

	betaScratchBytes, err := os.ReadFile(betaScratchPath)
	require.NoError(t, err)
	assert.Equal(t, scratchContent, betaScratchBytes)
	assert.Equal(t, sha256Bytes(scratchContent), sha256Bytes(betaScratchBytes))

	// Verify SQLite database reconstruction on Machine Beta
	betaDBPath := filepath.Join(betaConvsDir, convID+".db")
	assert.FileExists(t, betaDBPath)
	dbBeta, err := sql.Open("sqlite", betaDBPath)
	require.NoError(t, err)
	defer func() {
		_ = dbBeta.Close()
	}()

	var betaStepCount int
	err = dbBeta.QueryRowContext(ctx, "SELECT count(*) FROM steps").Scan(&betaStepCount)
	require.NoError(t, err)
	assert.Equal(t, 3, betaStepCount)

	// Verify conversation_summaries.db on Machine Beta
	assert.FileExists(t, betaSummariesDB)
	dbBetaSum, err := sql.Open("sqlite", betaSummariesDB)
	require.NoError(t, err)
	defer func() {
		_ = dbBetaSum.Close()
	}()

	var betaSummaryPreview string
	var betaSummaryCount int
	err = dbBetaSum.QueryRowContext(ctx, "SELECT preview, step_count FROM conversation_summaries WHERE conversation_id = ?", convID).
		Scan(&betaSummaryPreview, &betaSummaryCount)
	require.NoError(t, err)
	assert.Equal(t, "Please design a sync tool", betaSummaryPreview)
	assert.Equal(t, 3, betaSummaryCount)

	// 5. Machine Beta appends Step 3 and writes a new artifact
	step3 := `{"step_index":3,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T10:05:00Z","content":"Beta machine follow up"}` + "\n"
	fBeta, err := os.OpenFile(betaTranscriptPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = fBeta.WriteString(step3)
	require.NoError(t, err)
	_ = fBeta.Close()

	betaNewNote := []byte("Created by Machine Beta")
	require.NoError(t, os.WriteFile(filepath.Join(machineBetaBrain, convID, "beta_note.txt"), betaNewNote, 0o644))

	// Machine Beta Pushes
	pushResultBeta, err := engineBeta.Push(ctx, syncer.PushOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 1, pushResultBeta.StepsSynced)
	assert.Equal(t, 1, pushResultBeta.ArtifactsSynced)

	// 6. Machine Alpha Syncs Remote Changes
	pulledByAlpha, err := engineAlpha.SyncRemoteChanges(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, pulledByAlpha)

	// Check Machine Alpha's local filesystem received Step 3 and beta_note.txt
	alphaParseUpdated, err := p.ParseFile(alphaTranscriptPath)
	require.NoError(t, err)
	assert.Equal(t, 3, alphaParseUpdated.LastStepIndex)
	assert.Equal(t, "Beta machine follow up", alphaParseUpdated.Steps[3].Content)

	alphaBetaNotePath := filepath.Join(alphaConvDir, "beta_note.txt")
	assert.FileExists(t, alphaBetaNotePath)
	alphaBetaNoteBytes, err := os.ReadFile(alphaBetaNotePath)
	require.NoError(t, err)
	assert.Equal(t, betaNewNote, alphaBetaNoteBytes)

	// Verify SQLite database reconstruction on Machine Alpha after remote sync
	alphaDBPath := filepath.Join(alphaConvsDir, convID+".db")
	assert.FileExists(t, alphaDBPath)
	dbAlpha, err := sql.Open("sqlite", alphaDBPath)
	require.NoError(t, err)
	defer func() {
		_ = dbAlpha.Close()
	}()

	var alphaStepCount int
	err = dbAlpha.QueryRowContext(ctx, "SELECT count(*) FROM steps").Scan(&alphaStepCount)
	require.NoError(t, err)
	assert.Equal(t, 4, alphaStepCount)

	// Verify conversation_summaries.db on Machine Alpha
	assert.FileExists(t, alphaSummariesDB)
	dbAlphaSum, err := sql.Open("sqlite", alphaSummariesDB)
	require.NoError(t, err)
	defer func() {
		_ = dbAlphaSum.Close()
	}()

	var alphaSummaryCount int
	err = dbAlphaSum.QueryRowContext(ctx, "SELECT step_count FROM conversation_summaries WHERE conversation_id = ?", convID).
		Scan(&alphaSummaryCount)
	require.NoError(t, err)
	assert.Equal(t, 4, alphaSummaryCount)

	// 7. Verify loop prevention: Alpha pushing again does not push redundant steps
	pushResultAlpha2, err := engineAlpha.Push(ctx, syncer.PushOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 0, pushResultAlpha2.StepsSynced, "No new steps should be pushed by Alpha")

	// Verify discovery on Alpha
	discovered, err := discovery.DiscoverConversations(machineAlphaBrain)
	require.NoError(t, err)
	assert.Len(t, discovered, 1)
	assert.Len(t, discovered[0].Artifacts, 3) // design.md, scratch/test.txt, beta_note.txt

	fmt.Println("E2E Round-trip multi-machine sync completed with 100% fidelity!")
}

func TestE2E_MultiMachineRoundTripSync_Emulator(t *testing.T) {
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("Skipping live emulator test: FIRESTORE_EMULATOR_HOST not set")
	}

	ctx := t.Context()
	projectID := "e2e-emulator-test"

	clientRepo, err := firestore.NewClient(ctx, &config.Config{
		ProjectID: projectID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = clientRepo.Close()
	})

	machineAlphaBrain := t.TempDir()
	cfgAlpha := &config.Config{
		ProjectID: projectID,
		BrainDir:  machineAlphaBrain,
		MachineID: "machine-alpha-emu",
	}

	convID := "conv-emu-e2e"
	alphaConvDir := filepath.Join(machineAlphaBrain, convID)
	alphaLogsDir := filepath.Join(alphaConvDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(alphaLogsDir, 0o755))

	alphaTranscriptPath := filepath.Join(alphaLogsDir, "transcript.jsonl")
	step0 := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T10:00:00Z","content":"Initial emulator prompt"}` + "\n"
	require.NoError(t, os.WriteFile(alphaTranscriptPath, []byte(step0), 0o644))

	engineAlpha := syncer.NewEngine(cfgAlpha, clientRepo)
	pushRes, err := engineAlpha.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushRes.StepsSynced)

	machineBetaBrain := t.TempDir()
	cfgBeta := &config.Config{
		ProjectID: projectID,
		BrainDir:  machineBetaBrain,
		MachineID: "machine-beta-emu",
	}

	engineBeta := syncer.NewEngine(cfgBeta, clientRepo)
	pullRes, err := engineBeta.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 1, pullRes.StepsPulled)

	betaTranscript := filepath.Join(machineBetaBrain, convID, ".system_generated", "logs", "transcript.jsonl")
	assert.FileExists(t, betaTranscript)
}

func TestE2E_ChunkedDBSyncAndSummaryPreservation(t *testing.T) {
	ctx := t.Context()
	sharedRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = sharedRepo.Close()
	})

	convID := "e2e-chunk-sync-conversation-uuid"

	// 1. Setup Machine Alpha with a real SQLite DB containing protobuf binary mock
	alphaTemp := t.TempDir()
	alphaBrain := filepath.Join(alphaTemp, "brain")
	alphaConvs := filepath.Join(alphaTemp, "conversations")
	alphaSummaries := filepath.Join(alphaTemp, "conversation_summaries.db")
	require.NoError(t, os.MkdirAll(alphaConvs, 0o755))

	// Setup brain transcript
	logsDir := filepath.Join(alphaBrain, convID, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T12:00:00Z","content":"Build an autonomous compiler"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T12:01:00Z","content":"Starting compiler implementation"}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(transcriptContent), 0o644))

	// Initialize local SQLite conversation database with custom binary payload
	alphaDBPath := filepath.Join(alphaConvs, convID+".db")
	dbAlpha, err := sql.Open("sqlite", alphaDBPath)
	require.NoError(t, err)
	_, err = dbAlpha.Exec(`
		CREATE TABLE steps (
			idx INTEGER PRIMARY KEY,
			step_type INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0,
			has_subtrajectory NUMERIC NOT NULL DEFAULT 0,
			metadata BLOB,
			error_details BLOB,
			permissions BLOB,
			task_details BLOB,
			render_info BLOB,
			step_payload BLOB,
			step_format INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO steps (idx, step_type, status, step_payload) VALUES (0, 14, 3, X'0800120650726f6d7074');
		INSERT INTO steps (idx, step_type, status, step_payload) VALUES (1, 15, 3, X'08011206416e73776572');
	`)
	require.NoError(t, err)
	require.NoError(t, dbAlpha.Close())

	// Initialize local summary in conversation_summaries.db
	recAlpha := reconstructor.New(alphaConvs, alphaSummaries)
	err = recAlpha.UpsertSummary(ctx, reconstructor.SummaryParams{
		ConversationID: convID,
		Title:          "Autonomous Compiler Project",
		Preview:        "Build an autonomous compiler",
		StepCount:      2,
		RawSummary:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
		ProjectID:      "compiler-proj",
	})
	require.NoError(t, err)

	// Machine Alpha Pushes to Shared Repository
	cfgAlpha := &config.Config{
		ProjectID:        "compiler-proj",
		BrainDir:         alphaBrain,
		ConversationsDir: alphaConvs,
		SummariesDB:      alphaSummaries,
		MachineID:        "machine-alpha",
	}
	engineAlpha := syncer.NewEngine(cfgAlpha, sharedRepo)
	pushRes, err := engineAlpha.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushRes.ConversationsSynced)

	// Check remote metadata in Firestore
	remoteConv, err := sharedRepo.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, remoteConv)
	assert.Equal(t, "Autonomous Compiler Project", remoteConv.Title)
	assert.Equal(t, "Build an autonomous compiler", remoteConv.Preview)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, remoteConv.RawSummary)
	assert.NotEmpty(t, remoteConv.DBSHA256)
	assert.Positive(t, remoteConv.DBChunksCount)
	assert.Positive(t, remoteConv.DBSizeBytes)

	// 2. Setup Machine Beta (clean environment, simulating Cloud Shell)
	betaTemp := t.TempDir()
	betaBrain := filepath.Join(betaTemp, "brain")
	betaConvs := filepath.Join(betaTemp, "conversations")
	betaSummaries := filepath.Join(betaTemp, "conversation_summaries.db")

	cfgBeta := &config.Config{
		ProjectID:        "compiler-proj",
		BrainDir:         betaBrain,
		ConversationsDir: betaConvs,
		SummariesDB:      betaSummaries,
		MachineID:        "cloud-shell-beta",
	}
	engineBeta := syncer.NewEngine(cfgBeta, sharedRepo)

	// Machine Beta Pulls
	pullRes, err := engineBeta.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, convID, pullRes.ConversationID)

	// Verify local SQLite DB on Machine Beta
	betaDBPath := filepath.Join(betaConvs, convID+".db")
	require.FileExists(t, betaDBPath)

	// Verify byte-level SQLite database contents and readability on Machine Beta
	dbBeta, err := sql.Open("sqlite", betaDBPath)
	require.NoError(t, err)
	defer func() {
		_ = dbBeta.Close()
	}()

	var count int
	var payload []byte
	err = dbBeta.QueryRowContext(ctx, "SELECT count(*), step_payload FROM steps WHERE idx = 0").Scan(&count, &payload)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []byte{0x08, 0x00, 0x12, 0x06, 0x50, 0x72, 0x6f, 0x6d, 0x70, 0x74}, payload)

	// Verify conversation_summaries.db on Machine Beta has human-readable title and preview
	recBeta := reconstructor.New(betaConvs, betaSummaries)
	betaSummary, err := recBeta.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, betaSummary)
	assert.Equal(t, "Autonomous Compiler Project", betaSummary.Title)
	assert.Equal(t, "Build an autonomous compiler", betaSummary.Preview)
	assert.Equal(t, 2, betaSummary.StepCount)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, betaSummary.RawSummary)
}

func TestE2E_Full21ColumnsAndTrajectoryTableRoundTripSync(t *testing.T) {
	ctx := t.Context()

	// 1. Shared Firestore Repository (simulates cloud firestore instance)
	sharedRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = sharedRepo.Close()
	})

	// 2. Setup Machine Alpha (Machine A - source)
	alphaTemp := t.TempDir()
	alphaBrain := filepath.Join(alphaTemp, "brain")
	alphaConvs := filepath.Join(alphaTemp, "conversations")
	alphaSummaries := filepath.Join(alphaTemp, "conversation_summaries.db")

	convID := "99999999-1111-2222-3333-444444444444"
	alphaConvDir := filepath.Join(alphaBrain, convID)
	alphaLogsDir := filepath.Join(alphaConvDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(alphaLogsDir, 0o755))

	// Write transcript
	step0 := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-18T10:00:00Z","content":"Please implement full sync fidelity"}` + "\n"
	step1 := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-18T10:01:00Z","thinking":"Planning models","tool_calls":[{"id":"c1","name":"edit","arguments":{}}]}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(alphaLogsDir, "transcript.jsonl"), []byte(step0+step1), 0o644))

	// Build SQLite conversation DB with trajectory tables
	recAlpha := reconstructor.New(alphaConvs, alphaSummaries)
	parsedSteps := []models.Step{
		{StepIndex: 0, Source: "USER_EXPLICIT", Type: "USER_INPUT", Status: "DONE", Content: "Please implement full sync fidelity"},
		{StepIndex: 1, Source: "MODEL", Type: "PLANNER_RESPONSE", Status: "DONE"},
	}
	require.NoError(t, recAlpha.ReconstructConversationDB(ctx, convID, parsedSteps))

	// Populate summary with all 21 columns on Alpha
	t1 := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 18, 10, 5, 0, 0, time.UTC)
	err := recAlpha.UpsertSummary(ctx, reconstructor.SummaryParams{
		ConversationID:         convID,
		Title:                  "Full Fidelity Sync Feature",
		Preview:                "Please implement full sync fidelity",
		StepCount:              2,
		LastModifiedTime:       t2,
		WorkspaceURIs:          []string{"file:///Users/alice/Projects/testapp"},
		Status:                 "in_progress",
		Source:                 "agy-cli",
		ProjectID:              "alpha-project",
		AgentName:              "conductor",
		ParentConversationID:   "parent-conv-root",
		NestingDepth:           3,
		BattleID:               "battle-mode-77",
		WinningConversationID:  "win-conv-88",
		NotFullyIdle:           true,
		Killed:                 false,
		LastUserInputTime:      t1,
		LastUserInputStepIndex: 0,
		AppDataDir:             "/Users/alice/.gemini/antigravity-cli",
		RawSummary:             []byte{0xDE, 0xAD, 0xBE, 0xEF},
		GroupID:                "group-full-fidelity",
	})
	require.NoError(t, err)

	cfgAlpha := &config.Config{
		ProjectID:        "alpha-project",
		BrainDir:         alphaBrain,
		ConversationsDir: alphaConvs,
		SummariesDB:      alphaSummaries,
		MachineID:        "machine-alpha",
	}
	engineAlpha := syncer.NewEngine(cfgAlpha, sharedRepo)

	// Push from Alpha
	pushRes, err := engineAlpha.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushRes.ConversationsSynced)

	// Verify remote Firestore conversation record has all 21 attributes
	remoteConv, err := sharedRepo.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, remoteConv)
	assert.Equal(t, "Full Fidelity Sync Feature", remoteConv.Title)
	assert.Equal(t, "Please implement full sync fidelity", remoteConv.Preview)
	assert.Equal(t, []string{"file:///Users/alice/Projects/testapp"}, remoteConv.WorkspaceURIs)
	assert.Equal(t, "in_progress", remoteConv.Status)
	assert.Equal(t, "agy-cli", remoteConv.Source)
	assert.Equal(t, "alpha-project", remoteConv.ProjectID)
	assert.Equal(t, "conductor", remoteConv.AgentName)
	assert.Equal(t, "parent-conv-root", remoteConv.ParentConversationID)
	assert.Equal(t, 3, remoteConv.NestingDepth)
	assert.Equal(t, "battle-mode-77", remoteConv.BattleID)
	assert.Equal(t, "win-conv-88", remoteConv.WinningConversationID)
	assert.True(t, remoteConv.NotFullyIdle)
	assert.False(t, remoteConv.Killed)
	assert.Equal(t, 0, remoteConv.LastUserInputStepIndex)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, remoteConv.RawSummary)
	assert.Equal(t, "group-full-fidelity", remoteConv.GroupID)
	assert.Positive(t, remoteConv.DBChunksCount)
	assert.NotEmpty(t, remoteConv.DBSHA256)

	// 3. Setup Machine Beta (Machine B - destination)
	betaTemp := t.TempDir()
	betaBrain := filepath.Join(betaTemp, "brain")
	betaConvs := filepath.Join(betaTemp, "conversations")
	betaSummaries := filepath.Join(betaTemp, "conversation_summaries.db")

	cfgBeta := &config.Config{
		ProjectID:        "beta-project",
		BrainDir:         betaBrain,
		ConversationsDir: betaConvs,
		SummariesDB:      betaSummaries,
		MachineID:        "machine-beta",
	}
	engineBeta := syncer.NewEngine(cfgBeta, sharedRepo)

	// Pull on Beta
	pullRes, err := engineBeta.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, convID, pullRes.ConversationID)

	// Verify local SQLite DB on Machine Beta
	betaDBPath := filepath.Join(betaConvs, convID+".db")
	require.FileExists(t, betaDBPath)

	// Verify trajectory tables in <convID>.db
	dbBeta, err := sql.Open("sqlite", betaDBPath)
	require.NoError(t, err)
	defer func() {
		_ = dbBeta.Close()
	}()

	var stepCount int
	err = dbBeta.QueryRowContext(ctx, "SELECT count(*) FROM steps").Scan(&stepCount)
	require.NoError(t, err)
	assert.Equal(t, 2, stepCount)

	var trajectoryMetaCount int
	err = dbBeta.QueryRowContext(ctx, "SELECT count(*) FROM trajectory_meta").Scan(&trajectoryMetaCount)
	require.NoError(t, err)
	assert.Equal(t, 1, trajectoryMetaCount)

	// Verify conversation_summaries.db on Machine Beta has all 21 columns and adapted workspace URI
	recBeta := reconstructor.New(betaConvs, betaSummaries)
	betaSummary, err := recBeta.ReadLocalSummary(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, betaSummary)

	assert.Equal(t, "Full Fidelity Sync Feature", betaSummary.Title)
	assert.Equal(t, "Please implement full sync fidelity", betaSummary.Preview)
	assert.Equal(t, 2, betaSummary.StepCount)

	// Workspace URI should be adapted to the destination machine's user home
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expectedAdaptedURI := "file://" + home + "/Projects/testapp"
	assert.Equal(t, []string{expectedAdaptedURI}, betaSummary.WorkspaceURIs)

	assert.Equal(t, "in_progress", betaSummary.Status)
	assert.Equal(t, "agy-cli", betaSummary.Source)
	assert.Equal(t, "alpha-project", betaSummary.ProjectID)
	assert.Equal(t, "conductor", betaSummary.AgentName)
	assert.Equal(t, "parent-conv-root", betaSummary.ParentConversationID)
	assert.Equal(t, 3, betaSummary.NestingDepth)
	assert.Equal(t, "battle-mode-77", betaSummary.BattleID)
	assert.Equal(t, "win-conv-88", betaSummary.WinningConversationID)
	assert.True(t, betaSummary.NotFullyIdle)
	assert.False(t, betaSummary.Killed)
	assert.Equal(t, 0, betaSummary.LastUserInputStepIndex)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, betaSummary.RawSummary)
	assert.Equal(t, "group-full-fidelity", betaSummary.GroupID)
}

func TestE2E_ClearDatabase(t *testing.T) {
	ctx := t.Context()

	// 1. Shared Firestore Repository
	sharedRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = sharedRepo.Close()
	})

	// 2. Setup Local Machine State
	localBrain := t.TempDir()
	convsDir := filepath.Join(t.TempDir(), "conversations")
	summariesDB := filepath.Join(t.TempDir(), "conversation_summaries.db")
	cfg := &config.Config{
		ProjectID:        "e2e-clear-project",
		BrainDir:         localBrain,
		MachineID:        "machine-alpha",
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
	}

	convID := "e2e-clear-test-conv"
	convDir := filepath.Join(localBrain, convID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	step0 := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-18T10:00:00Z","content":"Clear test"}` + "\n"
	require.NoError(t, os.WriteFile(transcriptPath, []byte(step0), 0o644))

	artPath := filepath.Join(convDir, "doc.md")
	artContent := []byte("# Local Document to Preserve")
	require.NoError(t, os.WriteFile(artPath, artContent, 0o644))

	// Push local data to remote
	engine := syncer.NewEngine(cfg, sharedRepo)
	pushResult, err := engine.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushResult.ConversationsSynced)

	// Verify remote has conversation and artifacts
	remoteConvs, err := sharedRepo.ListConversations(ctx)
	require.NoError(t, err)
	assert.Len(t, remoteConvs, 1)

	// Perform Clear
	clearResult, err := engine.Clear(ctx, syncer.ClearOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, clearResult.ConversationsDeleted)
	assert.Equal(t, "success", clearResult.Status)

	// 1. Remote Firestore should now be empty
	remoteConvsAfter, err := sharedRepo.ListConversations(ctx)
	require.NoError(t, err)
	assert.Empty(t, remoteConvsAfter)

	remoteStepsAfter, err := sharedRepo.GetStepsSince(ctx, convID, -1)
	require.NoError(t, err)
	assert.Empty(t, remoteStepsAfter)

	// 2. Local files must remain COMPLETELY intact!
	readTranscript, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)
	assert.Equal(t, step0, string(readTranscript))

	readArt, err := os.ReadFile(artPath)
	require.NoError(t, err)
	assert.Equal(t, artContent, readArt)
}

func TestE2E_TransactionsAuditLogging(t *testing.T) {
	ctx := t.Context()

	sharedRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = sharedRepo.Close()
	})

	// Machine Alpha (source)
	alphaBrain := t.TempDir()
	alphaTxDB := filepath.Join(t.TempDir(), "alpha_tx.db")
	cfgAlpha := &config.Config{
		ProjectID:      "e2e-project",
		BrainDir:       alphaBrain,
		MachineID:      "machine-alpha",
		TransactionsDB: alphaTxDB,
	}

	convID := "tx-e2e-conversation"
	alphaConvDir := filepath.Join(alphaBrain, convID)
	alphaLogsDir := filepath.Join(alphaConvDir, ".system_generated", "logs")
	require.NoError(t, os.MkdirAll(alphaLogsDir, 0o755))

	alphaTranscriptPath := filepath.Join(alphaLogsDir, "transcript.jsonl")
	step0 := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-19T10:00:00Z","content":"Audit logging test"}` + "\n"
	require.NoError(t, os.WriteFile(alphaTranscriptPath, []byte(step0), 0o644))

	artPath := filepath.Join(alphaConvDir, "report.pdf")
	require.NoError(t, os.WriteFile(artPath, []byte("%PDF-1.4 test content"), 0o644))

	// Push from Alpha
	engineAlpha := syncer.NewEngine(cfgAlpha, sharedRepo)
	t.Cleanup(func() { _ = engineAlpha.Close() })

	pushRes, err := engineAlpha.Push(ctx, syncer.PushOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, pushRes.ConversationsSynced)
	assert.Equal(t, 1, pushRes.StepsSynced)
	assert.Equal(t, 1, pushRes.ArtifactsSynced)

	// Machine Beta (destination)
	betaBrain := t.TempDir()
	betaTxDB := filepath.Join(t.TempDir(), "beta_tx.db")
	cfgBeta := &config.Config{
		ProjectID:      "e2e-project",
		BrainDir:       betaBrain,
		MachineID:      "machine-beta",
		TransactionsDB: betaTxDB,
	}

	engineBeta := syncer.NewEngine(cfgBeta, sharedRepo)
	t.Cleanup(func() { _ = engineBeta.Close() })

	pullRes, err := engineBeta.Pull(ctx, syncer.PullOptions{ConversationID: convID})
	require.NoError(t, err)
	assert.Equal(t, 1, pullRes.StepsPulled)
	assert.Equal(t, 1, pullRes.ArtifactsPulled)

	// Verify Alpha transactions via store
	alphaStore := engineAlpha.TransactionStore()
	require.NotNil(t, alphaStore)
	alphaOutTxs, err := alphaStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionOut})
	require.NoError(t, err)
	assert.NotEmpty(t, alphaOutTxs)

	// Verify Beta transactions via store
	betaStore := engineBeta.TransactionStore()
	require.NotNil(t, betaStore)
	betaInTxs, err := betaStore.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn})
	require.NoError(t, err)
	assert.NotEmpty(t, betaInTxs)

	// Verify CLI transactions command on Alpha (Outbound / EXPORT)
	rootAlpha := cmd.NewRootCommand()
	bufAlpha := new(bytes.Buffer)
	rootAlpha.SetOut(bufAlpha)
	rootAlpha.SetErr(bufAlpha)
	rootAlpha.SetArgs([]string{"transactions", "--db", alphaTxDB, "--out"})
	require.NoError(t, rootAlpha.Execute())
	assert.Contains(t, bufAlpha.String(), "EXPORT")
	assert.Contains(t, bufAlpha.String(), convID)
	assert.Contains(t, bufAlpha.String(), "report.pdf")

	// Verify CLI transactions command on Beta (Inbound / IMPORT)
	rootBeta := cmd.NewRootCommand()
	bufBeta := new(bytes.Buffer)
	rootBeta.SetOut(bufBeta)
	rootBeta.SetErr(bufBeta)
	rootBeta.SetArgs([]string{"transactions", "--db", betaTxDB, "--in"})
	require.NoError(t, rootBeta.Execute())
	assert.Contains(t, bufBeta.String(), "IMPORT")
	assert.Contains(t, bufBeta.String(), convID)
	assert.Contains(t, bufBeta.String(), "report.pdf")
}
