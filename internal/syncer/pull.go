package syncer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/internal/reconstructor"
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
	log := logger.FromContext(ctx)
	if opts.ConversationID == "" {
		return nil, errors.New("conversation_id is required for pull operation")
	}

	log.DebugContext(ctx, "Pulling conversation from remote", "conversation_id", opts.ConversationID)
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
		if parseRes, err := e.parser.ParseFile(transcriptPath); err == nil {
			localLastStep = max(localLastStep, parseRes.LastStepIndex)
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

		for _, step := range newSteps {
			line, err := parser.SerializeStepToJSONL(&step)
			if err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("failed serializing step %d: %w", step.StepIndex, err)
			}
			if _, err := f.Write(line); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("failed writing step to transcript: %w", err)
			}
		}
		_ = f.Close()
		log.DebugContext(ctx, "Pulled remote transcript steps", "conversation_id", opts.ConversationID, "count", len(newSteps))
	}

	// 2. Fetch and restore remote artifacts concurrently
	artifacts, err := e.repo.ListArtifacts(ctx, opts.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed listing remote artifacts: %w", err)
	}

	artifactsPulled := 0
	if len(artifacts) > 0 {
		root, err := os.OpenRoot(convDir)
		if err != nil {
			return nil, fmt.Errorf("failed opening conversation root dir %s: %w", convDir, err)
		}
		defer func() {
			_ = root.Close()
		}()

		var mu sync.Mutex
		g, _ := errgroup.WithContext(ctx)
		g.SetLimit(5)

		for _, art := range artifacts {
			g.Go(func() error {
				relPath := filepath.FromSlash(art.RelativePath)
				dir := filepath.Dir(relPath)
				if dir != "." && dir != "" {
					if err := root.MkdirAll(dir, 0o755); err != nil {
						return fmt.Errorf("failed creating artifact dir %s: %w", dir, err)
					}
				}

				if err := root.WriteFile(relPath, art.Content, 0o644); err != nil {
					return fmt.Errorf("failed writing artifact file %s: %w", relPath, err)
				}
				mu.Lock()
				artifactsPulled++
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// 3. Reconstruct local SQLite database and summary if enabled
	if e.reconstructor != nil && !e.cfg.NoDBSync {
		if parseRes, err := e.parser.ParseFile(transcriptPath); err == nil {
			if err := e.reconstructor.ReconstructConversationDB(ctx, opts.ConversationID, parseRes.Steps); err != nil {
				log.WarnContext(ctx, "Failed reconstructing conversation sqlite database", "conversation_id", opts.ConversationID, "error", err)
			} else {
				summaryParams := reconstructor.BuildSummaryFromSteps(opts.ConversationID, remoteConv.Title, parseRes.Steps)
				summaryParams.ProjectID = e.cfg.ProjectID
				if err := e.reconstructor.UpsertSummary(ctx, summaryParams); err != nil {
					log.WarnContext(ctx, "Failed upserting conversation summary in sqlite", "conversation_id", opts.ConversationID, "error", err)
				}
			}
		}
	}

	log.InfoContext(ctx, "Pull completed",
		"conversation_id", opts.ConversationID,
		"steps_pulled", len(newSteps),
		"artifacts_pulled", artifactsPulled,
	)

	return &PullResult{
		ConversationID:  opts.ConversationID,
		StepsPulled:     len(newSteps),
		ArtifactsPulled: artifactsPulled,
		TargetDirectory: convDir,
	}, nil
}
