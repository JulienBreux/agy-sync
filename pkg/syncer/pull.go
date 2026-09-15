package syncer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/parser"
)

// PullOptions specifies configuration for a pull synchronization operation.
type PullOptions struct {
	ConversationID string
}

// PullResult summarizes the changes pulled from Firestore to the local filesystem.
type PullResult struct {
	ConversationID  string `json:"conversation_id"`
	StepsPulled     int    `json:"steps_pulled"`
	ArtifactsPulled int    `json:"artifacts_pulled"`
	TargetDirectory string `json:"target_directory"`
}

// Pull downloads conversation steps and artifacts from Firestore and reconstructs local brain structures.
func (e *Engine) Pull(ctx context.Context, opts PullOptions) (*PullResult, error) {
	if opts.ConversationID == "" {
		return nil, errors.New("conversation_id is required for pull operation")
	}

	remoteConv, err := e.repo.GetConversation(ctx, opts.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed fetching remote conversation %s: %w", opts.ConversationID, err)
	}
	if remoteConv == nil {
		return nil, fmt.Errorf("remote conversation %s not found in firestore", opts.ConversationID)
	}

	convDir := filepath.Join(e.cfg.BrainDir, opts.ConversationID)
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed creating conversation logs dir %s: %w", logsDir, err)
	}

	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	localLastStep := -1

	if _, err := os.Stat(transcriptPath); err == nil {
		parseRes, err := e.parser.ParseFile(transcriptPath)
		if err == nil && parseRes.LastStepIndex > localLastStep {
			localLastStep = parseRes.LastStepIndex
		}
	}

	// 1. Fetch remote steps since localLastStep
	newSteps, err := e.repo.GetStepsSince(ctx, opts.ConversationID, localLastStep)
	if err != nil {
		return nil, fmt.Errorf("failed fetching remote steps: %w", err)
	}

	if len(newSteps) > 0 {
		f, err := os.OpenFile(transcriptPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed opening transcript %s: %w", transcriptPath, err)
		}
		defer func() {
			_ = f.Close()
		}()

		for _, step := range newSteps {
			line, err := parser.SerializeStepToJSONL(&step)
			if err != nil {
				return nil, fmt.Errorf("failed serializing step %d: %w", step.StepIndex, err)
			}
			if _, err := f.Write(line); err != nil {
				return nil, fmt.Errorf("failed writing step to transcript: %w", err)
			}
		}
	}

	// 2. Fetch and restore remote artifacts
	artifacts, err := e.repo.ListArtifacts(ctx, opts.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed listing remote artifacts: %w", err)
	}

	artifactsPulled := 0
	for _, art := range artifacts {
		artPath := filepath.Join(convDir, filepath.FromSlash(art.RelativePath))
		if err := os.MkdirAll(filepath.Dir(artPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed creating artifact dir: %w", err)
		}

		if err := os.WriteFile(artPath, art.Content, 0o644); err != nil {
			return nil, fmt.Errorf("failed writing artifact file %s: %w", artPath, err)
		}
		artifactsPulled++
	}

	return &PullResult{
		ConversationID:  opts.ConversationID,
		StepsPulled:     len(newSteps),
		ArtifactsPulled: artifactsPulled,
		TargetDirectory: convDir,
	}, nil
}
