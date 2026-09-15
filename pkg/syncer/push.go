package syncer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/discovery"
	"github.com/julienbreux/agy-sync/pkg/firestore"
	"github.com/julienbreux/agy-sync/pkg/models"
	"github.com/julienbreux/agy-sync/pkg/parser"
)

// Engine orchestrates bidirectional synchronization between local Antigravity brain and Firestore.
type Engine struct {
	cfg    *config.Config
	repo   firestore.Repository
	parser *parser.TranscriptParser
}

// NewEngine creates an initialized sync Engine.
func NewEngine(cfg *config.Config, repo firestore.Repository) *Engine {
	return &Engine{
		cfg:    cfg,
		repo:   repo,
		parser: parser.NewTranscriptParser(),
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
	result := &PushResult{
		Errors: make([]error, 0),
	}

	var discovered []discovery.DiscoveredConversation
	if opts.ConversationID != "" {
		conv, err := discovery.DiscoverConversation(e.cfg.BrainDir, opts.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("failed discovering conversation %s: %w", opts.ConversationID, err)
		}
		discovered = append(discovered, *conv)
	} else {
		var err error
		discovered, err = discovery.DiscoverConversations(e.cfg.BrainDir)
		if err != nil {
			return nil, fmt.Errorf("failed discovering conversations in %s: %w", e.cfg.BrainDir, err)
		}
	}

	for _, dConv := range discovered {
		if err := e.pushConversation(ctx, &dConv, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("conversation %s sync error: %w", dConv.ID, err))
		}
	}

	return result, nil
}

func (e *Engine) pushConversation(ctx context.Context, dConv *discovery.DiscoveredConversation, res *PushResult) error {
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
		}
	}

	// 2. Push artifacts
	for _, art := range dConv.Artifacts {
		existingArt, err := e.repo.GetArtifact(ctx, dConv.ID, art.RelativePath)
		if err == nil && existingArt != nil && existingArt.SHA256 == art.SHA256 {
			continue
		}

		content, err := os.ReadFile(art.AbsolutePath)
		if err != nil {
			continue
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

		if err := e.repo.SaveArtifact(ctx, artifactModel); err != nil {
			return fmt.Errorf("failed saving artifact %s: %w", art.RelativePath, err)
		}
		res.ArtifactsSynced++
	}

	res.ConversationsSynced++
	return nil
}
