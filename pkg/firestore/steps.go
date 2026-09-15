package firestore

import (
	"context"
	"fmt"
	"time"

	cloudfs "cloud.google.com/go/firestore"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/models"
)

// AppendSteps writes a slice of steps to the conversation's steps subcollection using BulkWriter.
func (c *Client) AppendSteps(ctx context.Context, convID string, steps []models.Step) error {
	if len(steps) == 0 {
		return nil
	}

	convRef := c.client.Collection("conversations").Doc(convID)
	stepsCol := convRef.Collection("steps")

	bw := c.client.BulkWriter(ctx)
	lastIndex := -1

	for _, step := range steps {
		stepDoc := stepsCol.Doc(fmt.Sprintf("%06d", step.StepIndex))
		if _, err := bw.Set(stepDoc, step); err != nil {
			return fmt.Errorf("failed queuing step write to firestore: %w", err)
		}
		if step.StepIndex > lastIndex {
			lastIndex = step.StepIndex
		}
	}

	bw.Flush()

	if lastIndex >= 0 {
		_, _ = convRef.Set(ctx, map[string]interface{}{
			"last_synced_step": lastIndex,
			"updated_at":       time.Now().UTC(),
		}, cloudfs.MergeAll)
	}

	return nil
}
