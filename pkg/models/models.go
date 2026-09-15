package models

import (
	"time"
)

// ToolCall represents an individual tool invocation recorded in a transcript step.
type ToolCall struct {
	Name string         `json:"name" firestore:"name"`
	ID   string         `json:"id,omitempty" firestore:"id,omitempty"`
	Args map[string]any `json:"args,omitempty" firestore:"args,omitempty"`
}

// Step represents a single event/turn in an Antigravity conversation transcript.
type Step struct {
	StepIndex       int        `json:"step_index" firestore:"step_index"`
	Source          string     `json:"source" firestore:"source"`
	Type            string     `json:"type" firestore:"type"`
	Status          string     `json:"status" firestore:"status"`
	CreatedAt       time.Time  `json:"created_at" firestore:"created_at"`
	Content         string     `json:"content,omitempty" firestore:"content,omitempty"`
	Thinking        string     `json:"thinking,omitempty" firestore:"thinking,omitempty"`
	ToolCalls       []ToolCall `json:"tool_calls,omitempty" firestore:"tool_calls,omitempty"`
	TruncatedFields []string   `json:"truncated_fields,omitempty" firestore:"truncated_fields,omitempty"`
	MachineID       string     `json:"machine_id,omitempty" firestore:"machine_id,omitempty"`
}

// Conversation represents the metadata container for an Antigravity session.
type Conversation struct {
	ID             string    `json:"id" firestore:"id"`
	Title          string    `json:"title,omitempty" firestore:"title,omitempty"`
	CreatedAt      time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" firestore:"updated_at"`
	LastSyncedStep int       `json:"last_synced_step" firestore:"last_synced_step"`
	SourceMachine  string    `json:"source_machine" firestore:"source_machine"`
}

// Artifact represents a generated or referenced file in an Antigravity brain directory.
type Artifact struct {
	ID             string    `json:"id" firestore:"id"`
	ConversationID string    `json:"conversation_id" firestore:"conversation_id"`
	RelativePath   string    `json:"relative_path" firestore:"relative_path"`
	SizeBytes      int64     `json:"size_bytes" firestore:"size_bytes"`
	SHA256         string    `json:"sha256" firestore:"sha256"`
	UpdatedAt      time.Time `json:"updated_at" firestore:"updated_at"`
	Content        []byte    `json:"content,omitempty" firestore:"content,omitempty"`
	StorageURI     string    `json:"storage_uri,omitempty" firestore:"storage_uri,omitempty"`
}
