package firestore

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	cloudfs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/julienbreux/agy-sync/pkg/models"
)

// SaveDBChunks writes a slice of database binary chunks to the conversation's db_chunks subcollection.
func (c *Client) SaveDBChunks(ctx context.Context, convID string, chunks []models.DBChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	if convID == "" {
		return errors.New("conversation ID cannot be empty")
	}

	chunksCol := c.client.Collection("conversations").Doc(convID).Collection("db_chunks")
	bw := c.client.BulkWriter(ctx)

	for _, chunk := range chunks {
		chunkDoc := chunksCol.Doc(fmt.Sprintf("%06d", chunk.ChunkIndex))
		if _, err := bw.Set(chunkDoc, chunk); err != nil {
			return fmt.Errorf("failed queuing db chunk %d write to firestore: %w", chunk.ChunkIndex, err)
		}
	}

	bw.Flush()
	return nil
}

// GetDBChunks retrieves all database chunks for a conversation sorted by chunk_index.
func (c *Client) GetDBChunks(ctx context.Context, convID string) ([]models.DBChunk, error) {
	if convID == "" {
		return nil, errors.New("conversation ID cannot be empty")
	}

	chunksCol := c.client.Collection("conversations").Doc(convID).Collection("db_chunks")
	q := chunksCol.OrderBy("chunk_index", cloudfs.Asc)

	iter := q.Documents(ctx)
	defer iter.Stop()

	var chunks []models.DBChunk
	for {
		doc, err := iter.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			return nil, fmt.Errorf("failed iterating db chunks: %w", err)
		}

		var chunk models.DBChunk
		if err := doc.DataTo(&chunk); err != nil {
			return nil, fmt.Errorf("failed parsing db chunk data: %w", err)
		}
		chunks = append(chunks, chunk)
	}

	slices.SortFunc(chunks, func(a, b models.DBChunk) int {
		return cmp.Compare(a.ChunkIndex, b.ChunkIndex)
	})

	return chunks, nil
}
