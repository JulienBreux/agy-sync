package syncer

import (
	"context"
	"fmt"

	"github.com/julienbreux/agy-sync/internal/logger"
	"github.com/julienbreux/agy-sync/pkg/models"
)

// ShouldSyncRemoteConversation evaluates whether a remote conversation update should be pulled locally.
// Loop Prevention: If the change was produced by the current machine, it is skipped.
func (e *Engine) ShouldSyncRemoteConversation(conv *models.Conversation) bool {
	if conv == nil {
		return false
	}
	// Do not pull changes that originated from our own machine
	if conv.SourceMachine == e.cfg.MachineID {
		return false
	}
	return true
}

// SyncRemoteChanges checks remote Firestore conversations and pulls any new updates from other machines.
func (e *Engine) SyncRemoteChanges(ctx context.Context) (int, error) {
	log := logger.FromContext(ctx)
	log.DebugContext(ctx, "Checking remote Firestore conversations for sync", "machine_id", e.cfg.MachineID)

	conversations, err := e.repo.ListConversations(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed listing remote conversations: %w", err)
	}

	pulled := 0
	for _, conv := range conversations {
		if !e.ShouldSyncRemoteConversation(conv) {
			continue
		}

		log.InfoContext(ctx, "Pulling conversation updated on remote machine",
			"conversation_id", conv.ID,
			"source_machine", conv.SourceMachine,
		)

		_, err := e.Pull(ctx, PullOptions{ConversationID: conv.ID})
		if err != nil {
			log.WarnContext(ctx, "Failed pulling remote conversation", "conversation_id", conv.ID, "error", err)
			continue
		}
		pulled++
	}

	return pulled, nil
}
