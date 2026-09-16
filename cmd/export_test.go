package cmd

import (
	"context"

	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/pkg/config"
)

// SetFirestoreClientFactory allows tests in cmd_test to stub the repository client.
func SetFirestoreClientFactory(fn func(ctx context.Context, cfg *config.Config) (firestore.Repository, error)) {
	newFirestoreClient = fn
}

// ResetFirestoreClientFactory restores default constructor.
func ResetFirestoreClientFactory() {
	newFirestoreClient = func(ctx context.Context, cfg *config.Config) (firestore.Repository, error) {
		return firestore.NewClient(ctx, cfg)
	}
}
