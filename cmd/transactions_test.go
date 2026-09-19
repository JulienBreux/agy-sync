package cmd_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/transaction"
)

func seedTestTransactions(t *testing.T, dbPath string) {
	t.Helper()
	store, err := transaction.NewStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	ctx := t.Context()
	baseTime := time.Date(2026, 9, 19, 8, 30, 0, 0, time.UTC)

	txs := []transaction.Transaction{
		{
			Timestamp:      baseTime,
			Direction:      transaction.DirectionOut,
			EntityType:     transaction.EntityTypeConv,
			ConversationID: "conv-1",
			EntityID:       "conv-1",
			Details:        "steps: 5",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(10 * time.Second),
			Direction:      transaction.DirectionOut,
			EntityType:     transaction.EntityTypeArtifact,
			ConversationID: "conv-1",
			EntityID:       "plan.md",
			Details:        "size: 1024 bytes",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(20 * time.Second),
			Direction:      transaction.DirectionIn,
			EntityType:     transaction.EntityTypeConv,
			ConversationID: "conv-2",
			EntityID:       "conv-2",
			Details:        "steps: 3",
			Status:         transaction.StatusSuccess,
		},
		{
			Timestamp:      baseTime.Add(30 * time.Second),
			Direction:      transaction.DirectionIn,
			EntityType:     transaction.EntityTypeBrain,
			ConversationID: "conv-2",
			EntityID:       "transcript.jsonl",
			Details:        "+3 steps",
			Status:         transaction.StatusSuccess,
		},
	}

	for _, tx := range txs {
		require.NoError(t, store.Record(ctx, tx))
	}
}

func TestTransactionsCommand_DefaultTabular(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	seedTestTransactions(t, dbPath)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"transactions", "--db", dbPath})

	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "TIMESTAMP")
	assert.Contains(t, out, "ACTION")
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "CONVERSATION ID")
	assert.Contains(t, out, "EXPORT")
	assert.Contains(t, out, "IMPORT")
	assert.Contains(t, out, "conv-1")
	assert.Contains(t, out, "plan.md")
	assert.Contains(t, out, "transcript.jsonl")
}

func TestTransactionsCommand_JSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	seedTestTransactions(t, dbPath)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"transactions", "--db", dbPath, "--json"})

	err := root.Execute()
	require.NoError(t, err)

	var items []transaction.Transaction
	require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
	assert.Len(t, items, 4)
}

func TestTransactionsCommand_FilterDirection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	seedTestTransactions(t, dbPath)

	t.Run("out flag", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--out", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, transaction.DirectionOut, item.Direction)
		}
	})

	t.Run("in flag", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--in", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, transaction.DirectionIn, item.Direction)
		}
	})

	t.Run("direction flag out", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--direction", "out", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 2)
	})

	t.Run("conflicting direction flags", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--in", "--out"})

		err := root.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both")
	})
}

func TestTransactionsCommand_FilterType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	seedTestTransactions(t, dbPath)

	t.Run("conv flag", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--conv", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, transaction.EntityTypeConv, item.EntityType)
		}
	})

	t.Run("artifact flag", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--artifact", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 1)
		assert.Equal(t, "plan.md", items[0].EntityID)
	})

	t.Run("brain flag", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--brain", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 1)
		assert.Equal(t, "transcript.jsonl", items[0].EntityID)
	})

	t.Run("conflicting type flags", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--conv", "--brain"})

		err := root.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify multiple entity types")
	})
}

func TestTransactionsCommand_FilterConversationAndLimit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	seedTestTransactions(t, dbPath)

	t.Run("conversation filter", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "-c", "conv-1", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, "conv-1", item.ConversationID)
		}
	})

	t.Run("limit filter", func(t *testing.T) {
		root := cmd.NewRootCommand()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"transactions", "--db", dbPath, "--limit", "1", "--json"})

		err := root.Execute()
		require.NoError(t, err)

		var items []transaction.Transaction
		require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
		assert.Len(t, items, 1)
	})
}

func TestTransactionsCommand_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	// Empty DB
	store, err := transaction.NewStore(dbPath)
	require.NoError(t, err)
	_ = store.Close()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"transactions", "--db", dbPath})

	err = root.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No transactions found.")
}
