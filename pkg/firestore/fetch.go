package firestore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cloudfs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/julienbreux/agy-sync/pkg/models"
)

// GetStepsSince retrieves all steps belonging to convID where step_index > afterIndex, sorted ascending.
func (c *Client) GetStepsSince(ctx context.Context, convID string, afterIndex int) ([]models.Step, error) {
	stepsCol := c.client.Collection("conversations").Doc(convID).Collection("steps")
	q := stepsCol.Where("step_index", ">", afterIndex).OrderBy("step_index", cloudfs.Asc)

	iter := q.Documents(ctx)
	defer iter.Stop()

	var steps []models.Step
	for {
		doc, err := iter.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			return nil, fmt.Errorf("failed iterating steps: %w", err)
		}

		var step models.Step
		if err := doc.DataTo(&step); err != nil {
			return nil, fmt.Errorf("failed parsing step data: %w", err)
		}
		steps = append(steps, step)
	}

	return steps, nil
}

// SaveArtifact stores artifact metadata and inline content if under 1MB.
func (c *Client) SaveArtifact(ctx context.Context, artifact *models.Artifact) error {
	if artifact == nil || artifact.ConversationID == "" || artifact.ID == "" {
		return errors.New("invalid artifact record")
	}

	// Sanitize artifact ID for document key (replace slashes with double underscores)
	docID := strings.ReplaceAll(artifact.ID, "/", "__")
	docRef := c.client.Collection("conversations").Doc(artifact.ConversationID).Collection("artifacts").Doc(docID)

	_, err := docRef.Set(ctx, artifact)
	if err != nil {
		return fmt.Errorf("failed saving artifact %s: %w", artifact.ID, err)
	}

	return nil
}

// GetArtifact retrieves a single artifact record by ID.
func (c *Client) GetArtifact(ctx context.Context, convID, artifactID string) (*models.Artifact, error) {
	docID := strings.ReplaceAll(artifactID, "/", "__")
	docRef := c.client.Collection("conversations").Doc(convID).Collection("artifacts").Doc(docID)

	snap, err := docRef.Get(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed getting artifact %s: %w", artifactID, err)
	}

	if !snap.Exists() {
		return nil, nil
	}

	var artifact models.Artifact
	if err := snap.DataTo(&artifact); err != nil {
		return nil, fmt.Errorf("failed decoding artifact %s: %w", artifactID, err)
	}

	return &artifact, nil
}

// ListArtifacts retrieves all artifacts registered for a conversation.
func (c *Client) ListArtifacts(ctx context.Context, convID string) ([]models.Artifact, error) {
	artifactsCol := c.client.Collection("conversations").Doc(convID).Collection("artifacts")
	iter := artifactsCol.Documents(ctx)
	defer iter.Stop()

	var artifacts []models.Artifact
	for {
		doc, err := iter.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			return nil, fmt.Errorf("failed listing artifacts: %w", err)
		}

		var art models.Artifact
		if err := doc.DataTo(&art); err != nil {
			return nil, fmt.Errorf("failed decoding artifact data: %w", err)
		}
		artifacts = append(artifacts, art)
	}

	return artifacts, nil
}
