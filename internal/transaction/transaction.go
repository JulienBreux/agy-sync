package transaction

import (
	"context"
	"strings"
	"time"
)

// Direction represents the sync flow direction: in (IMPORT) or out (EXPORT).
type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// String returns the raw string value of the Direction.
func (d Direction) String() string {
	return string(d)
}

// Display returns a human-readable uppercase representation (IMPORT or EXPORT).
func (d Direction) Display() string {
	switch strings.ToLower(string(d)) {
	case "in", "import":
		return "IMPORT"
	case "out", "export":
		return "EXPORT"
	default:
		return strings.ToUpper(string(d))
	}
}

// EntityType represents the type of synchronized entity: conv, artifact, or brain.
type EntityType string

const (
	EntityTypeConv     EntityType = "conv"
	EntityTypeArtifact EntityType = "artifact"
	EntityTypeBrain    EntityType = "brain"
)

// String returns the raw string value of the EntityType.
func (e EntityType) String() string {
	return string(e)
}

// Status represents the outcome of a sync transaction.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// Transaction represents a single recorded sync operation event.
type Transaction struct {
	ID             int64      `json:"id"`
	Timestamp      time.Time  `json:"timestamp"`
	Direction      Direction  `json:"direction"`
	EntityType     EntityType `json:"entity_type"`
	ConversationID string     `json:"conversation_id"`
	EntityID       string     `json:"entity_id"`
	Details        string     `json:"details,omitempty"`
	Status         Status     `json:"status"`
}

// Filter specifies criteria for querying historical sync transactions.
type Filter struct {
	Direction      Direction  `json:"direction,omitempty"`
	EntityType     EntityType `json:"entity_type,omitempty"`
	ConversationID string     `json:"conversation_id,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
}

// Store defines operations for storing and querying sync transactions.
type Store interface {
	Record(ctx context.Context, tx Transaction) error
	Query(ctx context.Context, filter Filter) ([]Transaction, error)
	Close() error
}
