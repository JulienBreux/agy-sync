package firestore

import (
	"context"

	"github.com/julienbreux/agy-sync/pkg/models"
)

// Repository defines the Firestore storage interface for Antigravity conversations.
type Repository interface {
	Close() error
	UpsertConversation(ctx context.Context, conv *models.Conversation) error
	GetConversation(ctx context.Context, id string) (*models.Conversation, error)
	ListConversations(ctx context.Context) ([]*models.Conversation, error)
	AppendSteps(ctx context.Context, convID string, steps []models.Step) error
	GetStepsSince(ctx context.Context, convID string, afterIndex int) ([]models.Step, error)
	SaveArtifact(ctx context.Context, artifact *models.Artifact) error
	GetArtifact(ctx context.Context, convID, artifactID string) (*models.Artifact, error)
	ListArtifacts(ctx context.Context, convID string) ([]models.Artifact, error)
}
