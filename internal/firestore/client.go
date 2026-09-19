package firestore

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	cloudfs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

// Client wraps the official Google Cloud Firestore client.
type Client struct {
	client *cloudfs.Client
	cfg    *config.Config
}

// NewClient initializes a connection to Google Cloud Firestore with ADC.
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("configuration cannot be nil")
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, errors.New("project_id is required to initialize firestore client")
	}

	databaseID := cmp.Or(strings.TrimSpace(cfg.DatabaseID), "(default)")

	c, err := cloudfs.NewClientWithDatabase(ctx, cfg.ProjectID, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &Client{
		client: c,
		cfg:    cfg,
	}, nil
}

// Close closes the underlying gRPC connection to Firestore.
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// UpsertConversation persists top-level conversation metadata.
func (c *Client) UpsertConversation(ctx context.Context, conv *models.Conversation) error {
	if conv == nil || conv.ID == "" {
		return errors.New("invalid conversation record")
	}

	docRef := c.client.Collection("conversations").Doc(conv.ID)
	_, err := docRef.Set(ctx, conv)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation %s: %w", conv.ID, err)
	}
	return nil
}

// GetConversation retrieves conversation metadata by ID.
func (c *Client) GetConversation(ctx context.Context, id string) (*models.Conversation, error) {
	docRef := c.client.Collection("conversations").Doc(id)
	snap, err := docRef.Get(ctx)
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return nil, nil
		}
		// If document does not exist
		if strings.Contains(err.Error(), "NotFound") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation %s: %w", id, err)
	}

	if !snap.Exists() {
		return nil, nil
	}

	var conv models.Conversation
	if err := snap.DataTo(&conv); err != nil {
		return nil, fmt.Errorf("failed to parse conversation data: %w", err)
	}
	return &conv, nil
}

// ListConversations retrieves all stored conversations from Firestore.
func (c *Client) ListConversations(ctx context.Context) ([]*models.Conversation, error) {
	iter := c.client.Collection("conversations").Documents(ctx)
	defer iter.Stop()

	var conversations []*models.Conversation
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed iterating conversations: %w", err)
		}
		var conv models.Conversation
		if err := doc.DataTo(&conv); err != nil {
			continue
		}
		conversations = append(conversations, &conv)
	}
	return conversations, nil
}

// DeleteConversation removes a conversation and its subcollections.
func (c *Client) DeleteConversation(ctx context.Context, convID string) error {
	if strings.TrimSpace(convID) == "" {
		return errors.New("conversation_id cannot be empty")
	}
	if c.client == nil {
		return errors.New("firestore client is not initialized")
	}

	convRef := c.client.Collection("conversations").Doc(convID)
	bw := c.client.BulkWriter(ctx)
	defer bw.End()

	// Subcollections known to agy-sync: steps, artifacts, db_chunks
	subcollections := []string{"steps", "artifacts", "db_chunks"}
	for _, subColName := range subcollections {
		docRefs := convRef.Collection(subColName).DocumentRefs(ctx)
		for {
			docRef, err := docRefs.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return fmt.Errorf("failed listing %s documents: %w", subColName, err)
			}
			if _, err := bw.Delete(docRef); err != nil {
				return fmt.Errorf("failed queuing deletion for %s: %w", docRef.Path, err)
			}
		}
	}

	// Queue deletion of parent conversation document
	if _, err := bw.Delete(convRef); err != nil {
		return fmt.Errorf("failed queuing conversation deletion: %w", err)
	}

	bw.Flush()
	return nil
}

// ClearAll removes all conversations and subcollections.
func (c *Client) ClearAll(ctx context.Context) error {
	if c.client == nil {
		return errors.New("firestore client is not initialized")
	}

	docRefs := c.client.Collection("conversations").DocumentRefs(ctx)
	var convIDs []string
	for {
		docRef, err := docRefs.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed listing conversations: %w", err)
		}
		convIDs = append(convIDs, docRef.ID)
	}

	for _, id := range convIDs {
		if err := c.DeleteConversation(ctx, id); err != nil {
			return fmt.Errorf("failed deleting conversation %s: %w", id, err)
		}
	}

	return nil
}
