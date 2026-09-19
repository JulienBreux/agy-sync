package transaction

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	direction TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'success'
);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_direction ON transactions(direction);
CREATE INDEX IF NOT EXISTS idx_transactions_entity_type ON transactions(entity_type);
CREATE INDEX IF NOT EXISTS idx_transactions_conversation_id ON transactions(conversation_id);
`

// SQLiteStore implements the Store interface using an embedded SQLite database.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore initializes a SQLiteStore at the specified path and runs auto-migration.
func NewStore(dbPath string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed creating transaction db directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening transaction database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed applying transaction schema: %w", err)
	}

	return &SQLiteStore{
		db: db,
	}, nil
}

// Record inserts a new transaction into the database.
func (s *SQLiteStore) Record(ctx context.Context, tx Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := tx.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	status := cmp.Or(tx.Status, StatusSuccess)

	query := `
		INSERT INTO transactions (timestamp, direction, entity_type, conversation_id, entity_id, details, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		ts.Format(time.RFC3339Nano),
		string(tx.Direction),
		string(tx.EntityType),
		tx.ConversationID,
		tx.EntityID,
		tx.Details,
		string(status),
	)
	if err != nil {
		return fmt.Errorf("failed inserting transaction: %w", err)
	}

	return nil
}

// Query retrieves transactions matching the given filter in reverse chronological order.
func (s *SQLiteStore) Query(ctx context.Context, filter Filter) ([]Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []any

	if filter.Direction != "" {
		conditions = append(conditions, "direction = ?")
		args = append(args, string(filter.Direction))
	}

	if filter.EntityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, string(filter.EntityType))
	}

	if filter.ConversationID != "" {
		conditions = append(conditions, "conversation_id = ?")
		args = append(args, filter.ConversationID)
	}

	query := "SELECT id, timestamp, direction, entity_type, conversation_id, entity_id, details, status FROM transactions"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY timestamp DESC, id DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed querying transactions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []Transaction
	for rows.Next() {
		var tx Transaction
		var tsStr, dirStr, typeStr, statusStr string

		err := rows.Scan(
			&tx.ID,
			&tsStr,
			&dirStr,
			&typeStr,
			&tx.ConversationID,
			&tx.EntityID,
			&tx.Details,
			&statusStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning transaction row: %w", err)
		}

		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if parsedTime, err := time.Parse(layout, tsStr); err == nil {
				tx.Timestamp = parsedTime
				break
			}
		}

		tx.Direction = Direction(dirStr)
		tx.EntityType = EntityType(typeStr)
		tx.Status = Status(statusStr)

		results = append(results, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during transactions row iteration: %w", err)
	}

	return results, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
