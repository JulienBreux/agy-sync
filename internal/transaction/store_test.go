package transaction_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/transaction"
)

func TestStore_InitializeAndRecord(t *testing.T) {
	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "transactions.db")

	store, err := transaction.NewStore(dbPath)
	require.NoError(t, err)
	defer func() {
		_ = store.Close()
	}()

	now := time.Now().UTC().Truncate(time.Second)
	tx1 := transaction.Transaction{
		Timestamp:      now,
		Direction:      transaction.DirectionOut,
		EntityType:     transaction.EntityTypeConv,
		ConversationID: "conv-1",
		EntityID:       "conv-1",
		Details:        "title: Initial Plan",
		Status:         transaction.StatusSuccess,
	}

	err = store.Record(ctx, tx1)
	require.NoError(t, err)

	results, err := store.Query(ctx, transaction.Filter{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.NotZero(t, results[0].ID)
	assert.Equal(t, transaction.DirectionOut, results[0].Direction)
	assert.Equal(t, transaction.EntityTypeConv, results[0].EntityType)
	assert.Equal(t, "conv-1", results[0].ConversationID)
	assert.Equal(t, "conv-1", results[0].EntityID)
	assert.Equal(t, "title: Initial Plan", results[0].Details)
	assert.Equal(t, transaction.StatusSuccess, results[0].Status)
}

func TestStore_Filters(t *testing.T) {
	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "transactions.db")

	store, err := transaction.NewStore(dbPath)
	require.NoError(t, err)
	defer func() {
		_ = store.Close()
	}()

	baseTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)

	// Seed 4 transactions
	items := []transaction.Transaction{
		{
			Timestamp:      baseTime.Add(1 * time.Second),
			Direction:      transaction.DirectionIn,
			EntityType:     transaction.EntityTypeConv,
			ConversationID: "conv-alpha",
			EntityID:       "conv-alpha",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(2 * time.Second),
			Direction:      transaction.DirectionIn,
			EntityType:     transaction.EntityTypeArtifact,
			ConversationID: "conv-alpha",
			EntityID:       "spec.md",
			Details:        "size: 1024",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(3 * time.Second),
			Direction:      transaction.DirectionOut,
			EntityType:     transaction.EntityTypeBrain,
			ConversationID: "conv-beta",
			EntityID:       "transcript.jsonl",
			Details:        "+5 steps",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(4 * time.Second),
			Direction:      transaction.DirectionOut,
			EntityType:     transaction.EntityTypeArtifact,
			ConversationID: "conv-beta",
			EntityID:       "plan.md",
			Details:        "size: 2048",
			Status:         transaction.StatusSuccess,
		},
	}

	for _, it := range items {
		require.NoError(t, store.Record(ctx, it))
	}

	t.Run("Filter by Direction In", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{Direction: transaction.DirectionIn})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		for _, r := range res {
			assert.Equal(t, transaction.DirectionIn, r.Direction)
		}
	})

	t.Run("Filter by Direction Out", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{Direction: transaction.DirectionOut})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		for _, r := range res {
			assert.Equal(t, transaction.DirectionOut, r.Direction)
		}
	})

	t.Run("Filter by EntityType artifact", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{EntityType: transaction.EntityTypeArtifact})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		for _, r := range res {
			assert.Equal(t, transaction.EntityTypeArtifact, r.EntityType)
		}
	})

	t.Run("Filter by EntityType brain", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{EntityType: transaction.EntityTypeBrain})
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "conv-beta", res[0].ConversationID)
		assert.Equal(t, "transcript.jsonl", res[0].EntityID)
	})

	t.Run("Filter by Conversation ID", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{ConversationID: "conv-alpha"})
		require.NoError(t, err)
		assert.Len(t, res, 2)
	})

	t.Run("Pagination Limit", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{Limit: 2})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		// Should be reverse chronological order (newest first)
		assert.Equal(t, "conv-beta", res[0].ConversationID)
		assert.Equal(t, "plan.md", res[0].EntityID)
	})

	t.Run("Pagination Offset", func(t *testing.T) {
		res, err := store.Query(ctx, transaction.Filter{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, "conv-alpha", res[0].ConversationID)
	})
}

func TestStore_ThreadSafety(t *testing.T) {
	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "transactions.db")

	store, err := transaction.NewStore(dbPath)
	require.NoError(t, err)
	defer func() {
		_ = store.Close()
	}()

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for range iterations {
				tx := transaction.Transaction{
					Timestamp:      time.Now().UTC(),
					Direction:      transaction.DirectionOut,
					EntityType:     transaction.EntityTypeArtifact,
					ConversationID: "conv-stress",
					EntityID:       "file.txt",
					Status:         transaction.StatusSuccess,
				}
				_ = store.Record(ctx, tx)
				_, _ = store.Query(ctx, transaction.Filter{Limit: 5})
			}
		}(i)
	}

	wg.Wait()

	all, err := store.Query(ctx, transaction.Filter{Limit: 1000})
	require.NoError(t, err)
	assert.Len(t, all, workers*iterations)
}
