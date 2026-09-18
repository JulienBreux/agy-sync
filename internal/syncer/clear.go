package syncer

import (
	"context"
	"fmt"

	"github.com/julienbreux/agy-sync/internal/logger"
)

// ClearOptions specifies configuration parameters for a remote database clear operation.
type ClearOptions struct {
	ConversationID string
}

// ClearResult summarizes the outcome of a clear operation.
type ClearResult struct {
	ConversationID       string `json:"conversation_id,omitempty"`
	ConversationsDeleted int    `json:"conversations_deleted"`
	Status               string `json:"status"`
}

// Clear purges Firestore data for all conversations or a targeted single conversation.
func (e *Engine) Clear(ctx context.Context, opts ClearOptions) (*ClearResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	log := logger.FromContext(ctx)

	if opts.ConversationID != "" {
		log.InfoContext(ctx, "Clearing single conversation from Firestore", "conversation_id", opts.ConversationID)
		if err := e.repo.DeleteConversation(ctx, opts.ConversationID); err != nil {
			return nil, fmt.Errorf("failed deleting conversation %s: %w", opts.ConversationID, err)
		}
		return &ClearResult{
			ConversationID:       opts.ConversationID,
			ConversationsDeleted: 1,
			Status:               "success",
		}, nil
	}

	log.InfoContext(ctx, "Clearing all conversations from Firestore")

	// Get count of conversations before clearing
	convs, err := e.repo.ListConversations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed listing conversations before clear: %w", err)
	}

	if err := e.repo.ClearAll(ctx); err != nil {
		return nil, fmt.Errorf("failed clearing all data from firestore: %w", err)
	}

	return &ClearResult{
		ConversationsDeleted: len(convs),
		Status:               "success",
	}, nil
}
