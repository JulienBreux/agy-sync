package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/julienbreux/agy-sync/internal/discovery"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/internal/reconstructor"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

// Engine orchestrates bidirectional synchronization between local Antigravity brain and Firestore.
type Engine struct {
	cfg           *config.Config
	repo          firestore.Repository
	parser        *parser.TranscriptParser
	reconstructor *reconstructor.Reconstructor
}

// NewEngine creates an initialized sync Engine.
func NewEngine(cfg *config.Config, repo firestore.Repository) *Engine {
	var rec *reconstructor.Reconstructor
	if cfg != nil && !cfg.NoDBSync && cfg.ConversationsDir != "" && cfg.SummariesDB != "" {
		rec = reconstructor.New(cfg.ConversationsDir, cfg.SummariesDB)
	}

	return &Engine{
		cfg:           cfg,
		repo:          repo,
		parser:        parser.NewTranscriptParser(),
		reconstructor: rec,
	}
}

// PushOptions configures execution parameters for a push operation.
type PushOptions struct {
	ConversationID string
}

// PushResult summarizes items synchronized during a push operation.
type PushResult struct {
	ConversationsSynced int     `json:"conversations_synced"`
	StepsSynced         int     `json:"steps_synced"`
	ArtifactsSynced     int     `json:"artifacts_synced"`
	Errors              []error `json:"errors,omitempty"`
}

// Push scans the local Antigravity directory and pushes un-synchronized steps and artifacts to Firestore.
func (e *Engine) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	log := logger.FromContext(ctx)
	result := &PushResult{
		Errors: make([]error, 0),
	}

	var discovered []discovery.DiscoveredConversation
	if opts.ConversationID != "" {
		log.DebugContext(ctx, "Pushing single conversation", "conversation_id", opts.ConversationID)
		conv, err := discovery.DiscoverConversation(e.cfg.BrainDir, opts.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("failed discovering conversation %s: %w", opts.ConversationID, err)
		}
		discovered = append(discovered, *conv)
	} else {
		log.DebugContext(ctx, "Scanning brain directory for all conversations", "brain_dir", e.cfg.BrainDir)
		var err error
		discovered, err = discovery.DiscoverConversations(e.cfg.BrainDir)
		if err != nil {
			return nil, fmt.Errorf("failed discovering conversations in %s: %w", e.cfg.BrainDir, err)
		}
	}

	for _, dConv := range discovered {
		if err := e.pushConversation(ctx, &dConv, result); err != nil {
			log.ErrorContext(ctx, "Failed pushing conversation", "conversation_id", dConv.ID, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("conversation %s sync error: %w", dConv.ID, err))
		}
	}

	log.InfoContext(ctx, "Push completed",
		"conversations_synced", result.ConversationsSynced,
		"steps_synced", result.StepsSynced,
		"artifacts_synced", result.ArtifactsSynced,
		"errors_count", len(result.Errors),
	)

	return result, nil
}

func (e *Engine) pushConversation(ctx context.Context, dConv *discovery.DiscoveredConversation, res *PushResult) error {
	log := logger.FromContext(ctx)
	remoteConv, err := e.repo.GetConversation(ctx, dConv.ID)
	if err != nil {
		return fmt.Errorf("failed checking remote conversation: %w", err)
	}

	lastSyncedStep := -1
	if remoteConv != nil {
		lastSyncedStep = remoteConv.LastSyncedStep
	} else {
		remoteConv = &models.Conversation{
			ID:             dConv.ID,
			CreatedAt:      dConv.LastModified,
			UpdatedAt:      time.Now().UTC(),
			LastSyncedStep: -1,
			SourceMachine:  e.cfg.MachineID,
		}
		if err := e.repo.UpsertConversation(ctx, remoteConv); err != nil {
			return fmt.Errorf("failed upserting remote conversation: %w", err)
		}
	}

	// 1. Push incremental transcript steps
	if dConv.HasTranscript {
		parseRes, err := e.parser.ParseFile(dConv.TranscriptPath)
		if err != nil {
			return fmt.Errorf("failed parsing transcript %s: %w", dConv.TranscriptPath, err)
		}

		var newSteps []models.Step
		for _, step := range parseRes.Steps {
			if step.StepIndex > lastSyncedStep {
				step.MachineID = e.cfg.MachineID
				newSteps = append(newSteps, step)
			}
		}

		if len(newSteps) > 0 {
			if err := e.repo.AppendSteps(ctx, dConv.ID, newSteps); err != nil {
				return fmt.Errorf("failed appending steps: %w", err)
			}
			res.StepsSynced += len(newSteps)

			remoteConv.LastSyncedStep = newSteps[len(newSteps)-1].StepIndex
			remoteConv.SourceMachine = e.cfg.MachineID
			remoteConv.UpdatedAt = time.Now().UTC()
			if err := e.repo.UpsertConversation(ctx, remoteConv); err != nil {
				return fmt.Errorf("failed updating conversation metadata: %w", err)
			}
			log.DebugContext(ctx, "Appended new conversation steps", "conversation_id", dConv.ID, "count", len(newSteps))
		}
	}

	// 2. Push artifacts concurrently with bounded parallelism
	if len(dConv.Artifacts) > 0 {
		var mu sync.Mutex
		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(5)

		for _, art := range dConv.Artifacts {
			g.Go(func() error {
				existingArt, err := e.repo.GetArtifact(gCtx, dConv.ID, art.RelativePath)
				if err == nil && existingArt != nil && existingArt.SHA256 == art.SHA256 {
					return nil
				}

				content, readErr := os.ReadFile(art.AbsolutePath)
				if readErr != nil {
					return nil //nolint:nilerr // Best-effort push skips unreadable artifacts
				}

				artifactModel := &models.Artifact{
					ID:             art.RelativePath,
					ConversationID: dConv.ID,
					RelativePath:   art.RelativePath,
					SizeBytes:      art.SizeBytes,
					SHA256:         art.SHA256,
					UpdatedAt:      art.LastModified,
					Content:        content,
				}

				if err := e.repo.SaveArtifact(gCtx, artifactModel); err != nil {
					return fmt.Errorf("failed saving artifact %s: %w", art.RelativePath, err)
				}

				mu.Lock()
				res.ArtifactsSynced++
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
	}

	// 3. Extract summary metadata (title, preview, timestamps, step count, raw_summary)
	var summaryExtracted bool
	if e.reconstructor != nil {
		if summary, err := e.reconstructor.ReadLocalSummary(ctx, dConv.ID); err == nil && summary != nil {
			if summary.Title != "" {
				remoteConv.Title = summary.Title
			}
			if summary.Preview != "" {
				remoteConv.Preview = summary.Preview
			}
			if summary.StepCount > 0 {
				remoteConv.StepCount = summary.StepCount
			}
			if !summary.LastUserInputTime.IsZero() {
				remoteConv.LastUserInputTime = summary.LastUserInputTime
			}
			if summary.LastUserInputStepIndex >= 0 {
				remoteConv.LastUserInputStepIndex = summary.LastUserInputStepIndex
			}
			if len(summary.RawSummary) > 0 {
				remoteConv.RawSummary = summary.RawSummary
			}
			summaryExtracted = true
		}
	}

	// Ensure StepCount and summary metadata are up-to-date with transcript
	if dConv.HasTranscript {
		if parseRes, err := e.parser.ParseFile(dConv.TranscriptPath); err == nil && len(parseRes.Steps) > 0 {
			if len(parseRes.Steps) > remoteConv.StepCount {
				remoteConv.StepCount = len(parseRes.Steps)
			}
			if !summaryExtracted {
				summary := reconstructor.BuildSummaryFromSteps(dConv.ID, remoteConv.Title, parseRes.Steps)
				if remoteConv.Title == "" {
					remoteConv.Title = summary.Title
				}
				if remoteConv.Preview == "" {
					remoteConv.Preview = summary.Preview
				}
				remoteConv.LastUserInputTime = summary.LastUserInputTime
				remoteConv.LastUserInputStepIndex = summary.LastUserInputStepIndex
			}
		}
	}

	// 4. Push SQLite database snapshot (chunked)
	if e.cfg != nil && !e.cfg.NoDBSync && e.cfg.ConversationsDir != "" {
		if e.reconstructor != nil && dConv.HasTranscript {
			if parseRes, err := e.parser.ParseFile(dConv.TranscriptPath); err == nil && len(parseRes.Steps) > 0 {
				if err := e.reconstructor.ReconstructConversationDB(ctx, dConv.ID, parseRes.Steps); err != nil {
					log.WarnContext(ctx, "Failed updating local DB before snapshot", "conversation_id", dConv.ID, "error", err)
				}
			}
		}

		localDBPath := filepath.Join(e.cfg.ConversationsDir, dConv.ID+".db")
		if _, statErr := os.Stat(localDBPath); statErr == nil {
			data, sha256Hex, sizeBytes, err := reconstructor.SnapshotConversationDB(localDBPath)
			if err != nil {
				log.WarnContext(ctx, "Failed creating SQLite snapshot for push", "conversation_id", dConv.ID, "error", err)
			} else if sha256Hex != remoteConv.DBSHA256 {
				const chunkSize = 512 * 1024
				totalChunks := (len(data) + chunkSize - 1) / chunkSize
				if totalChunks == 0 {
					totalChunks = 1
				}
				chunks := make([]models.DBChunk, 0, totalChunks)
				for i := 0; i < len(data); i += chunkSize {
					end := min(i+chunkSize, len(data))
					chunkData := data[i:end]
					chunkHash := sha256.Sum256(chunkData)
					chunks = append(chunks, models.DBChunk{
						ChunkIndex:  len(chunks),
						TotalChunks: totalChunks,
						SizeBytes:   len(chunkData),
						SHA256:      hex.EncodeToString(chunkHash[:]),
						Data:        chunkData,
					})
				}
				if err := e.repo.SaveDBChunks(ctx, dConv.ID, chunks); err != nil {
					return fmt.Errorf("failed saving db chunks for %s: %w", dConv.ID, err)
				}
				remoteConv.DBSHA256 = sha256Hex
				remoteConv.DBSizeBytes = sizeBytes
				remoteConv.DBChunksCount = len(chunks)
				log.DebugContext(ctx, "Uploaded DB snapshot chunks", "conversation_id", dConv.ID, "chunks", len(chunks), "size_bytes", sizeBytes)
			}
		}
	}

	// Update conversation metadata
	remoteConv.SourceMachine = e.cfg.MachineID
	remoteConv.UpdatedAt = time.Now().UTC()
	if err := e.repo.UpsertConversation(ctx, remoteConv); err != nil {
		return fmt.Errorf("failed updating conversation metadata: %w", err)
	}

	res.ConversationsSynced++
	return nil
}
