package reconstructor

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Reconstructor coordinates the reconstruction of local Antigravity SQLite databases.
type Reconstructor struct {
	conversationsDir string
	summariesDBPath  string
}

// New creates a new Reconstructor instance.
func New(conversationsDir, summariesDBPath string) *Reconstructor {
	return &Reconstructor{
		conversationsDir: conversationsDir,
		summariesDBPath:  summariesDBPath,
	}
}

// OpenDB opens a SQLite database connection with safe concurrency pragmas.
func (r *Reconstructor) OpenDB(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed creating directory %s: %w", dir, err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening sqlite db at %s: %w", path, err)
	}

	// SQLite single-writer recommendation for WAL mode
	db.SetMaxOpenConns(1)

	return db, nil
}
