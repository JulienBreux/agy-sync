package test_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/discovery"
	"github.com/julienbreux/agy-sync/pkg/firestore"
	"github.com/julienbreux/agy-sync/pkg/parser"
	"github.com/julienbreux/agy-sync/pkg/syncer"
)

func sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestE2E_MultiMachineRoundTripSync(t *testing.T) {
	ctx := context.Background()

	// 1. Shared Firestore Repository (simulates cloud firestore instance)
	sharedRepo := firestore.NewMemoryRepository()
	defer func() {
		_ = sharedRepo.Close()
	}()

	// 2. Setup Machine Alpha (Machine A)
	machineAlphaBrain := t.TempDir()
	cfgAlpha := &config.Config{
		ProjectID: "ayg-e2e-project",
		BrainDir:  machineAlphaBrain,
		MachineID: "machine-alpha",
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
	cfgBeta := &config.Config{
		ProjectID: "ayg-e2e-project",
		BrainDir:  machineBetaBrain,
		MachineID: "machine-beta",
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
	assert.Equal(t, len(alphaParse.Steps), len(betaParse.Steps))
	for i := range alphaParse.Steps {
		assert.Equal(t, alphaParse.Steps[i].StepIndex, betaParse.Steps[i].StepIndex)
		assert.Equal(t, alphaParse.Steps[i].Source, betaParse.Steps[i].Source)
		assert.Equal(t, alphaParse.Steps[i].Type, betaParse.Steps[i].Type)
		assert.Equal(t, alphaParse.Steps[i].Content, betaParse.Steps[i].Content)
	}

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

	ctx := context.Background()
	projectID := "e2e-emulator-test"

	clientRepo, err := firestore.NewClient(ctx, &config.Config{
		ProjectID: projectID,
	})
	require.NoError(t, err)
	defer func() {
		_ = clientRepo.Close()
	}()

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

