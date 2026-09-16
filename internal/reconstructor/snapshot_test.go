package reconstructor_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/julienbreux/agy-sync/internal/reconstructor"
)

func TestSnapshotConversationDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_source.db")

	// Create and populate an SQLite DB with WAL mode
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`
		PRAGMA journal_mode = WAL;
		CREATE TABLE steps (idx INTEGER PRIMARY KEY, content TEXT);
		INSERT INTO steps (idx, content) VALUES (1, 'step 1');
		INSERT INTO steps (idx, content) VALUES (2, 'step 2');
	`)
	require.NoError(t, err)
	_ = db.Close()

	// Snapshot the DB
	data, hash, size, err := reconstructor.SnapshotConversationDB(dbPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.NotEmpty(t, hash)
	assert.Equal(t, int64(len(data)), size)

	// Verify the snapshot can be opened and queried
	snapPath := filepath.Join(tempDir, "verified_snapshot.db")
	err = os.WriteFile(snapPath, data, 0o600)
	require.NoError(t, err)

	snapDB, err := sql.Open("sqlite", snapPath)
	require.NoError(t, err)
	defer func() { _ = snapDB.Close() }()

	var count int
	err = snapDB.QueryRow("SELECT COUNT(*) FROM steps;").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSnapshotConversationDB_NonExistent(t *testing.T) {
	_, _, _, err := reconstructor.SnapshotConversationDB("/path/does/not/exist.db")
	require.Error(t, err)
}
