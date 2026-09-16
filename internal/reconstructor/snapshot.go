package reconstructor

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// SnapshotConversationDB safely creates a standalone snapshot of an active SQLite database using VACUUM INTO,
// ensuring all WAL entries are merged and data is consistent without locking active sessions.
// Returns the database file content bytes, its SHA256 hex string, the size in bytes, or an error.
func SnapshotConversationDB(dbPath string) ([]byte, string, int64, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, "", 0, fmt.Errorf("source database not accessible at %s: %w", dbPath, err)
	}

	// Create a temporary file name for the snapshot
	tmpFile, err := os.CreateTemp("", "agy-sync-snap-*.db")
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed creating temp file for snapshot: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	// VACUUM INTO expects the destination file to NOT exist yet
	_ = os.Remove(tmpPath)
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Open source database
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed opening source database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Execute VACUUM INTO
	escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
	query := fmt.Sprintf("VACUUM INTO '%s';", escapedPath)
	if _, err := db.Exec(query); err != nil {
		// Fallback: if VACUUM INTO fails, read source file directly
		data, readErr := os.ReadFile(dbPath)
		if readErr != nil {
			return nil, "", 0, fmt.Errorf("failed executing VACUUM INTO (%w) and fallback read failed (%w)", err, readErr)
		}
		hash := sha256.Sum256(data)
		return data, hex.EncodeToString(hash[:]), int64(len(data)), nil
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed reading snapshot file: %w", err)
	}

	hash := sha256.Sum256(data)
	return data, hex.EncodeToString(hash[:]), int64(len(data)), nil
}
