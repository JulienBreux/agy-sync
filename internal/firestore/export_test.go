package firestore

import (
	cloudfs "cloud.google.com/go/firestore"
	"github.com/julienbreux/agy-sync/pkg/config"
)

// NewTestClient creates a Client with injected internal cloudfs client for testing.
func NewTestClient(c *cloudfs.Client, cfg *config.Config) *Client {
	return &Client{
		client: c,
		cfg:    cfg,
	}
}
