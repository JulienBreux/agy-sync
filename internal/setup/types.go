package setup

import (
	"context"
	"time"
)

// CheckStatus represents the status of a single diagnostic check.
type CheckStatus string

const (
	StatusPass    CheckStatus = "PASS"
	StatusFail    CheckStatus = "FAIL"
	StatusWarn    CheckStatus = "WARN"
	StatusSkipped CheckStatus = "SKIPPED"
)

// CheckItem holds the result and diagnostics for an individual verification step.
type CheckItem struct {
	Name        string        `json:"name"`
	Status      CheckStatus   `json:"status"`
	Message     string        `json:"message"`
	Details     []string      `json:"details,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
	Duration    time.Duration `json:"duration_ms"`
}

// SetupReport encapsulates the complete diagnostic check results.
type SetupReport struct {
	Timestamp  time.Time   `json:"timestamp"`
	ProjectID  string      `json:"project_id"`
	DatabaseID string      `json:"database_id"`
	DryRun     bool        `json:"dry_run"`
	AllPassed  bool        `json:"all_passed"`
	Items      []CheckItem `json:"items"`
}

// CheckOptions defines configuration options for running setup checks.
type CheckOptions struct {
	ProjectID        string
	DatabaseID       string
	DryRun           bool
	ConfigPath       string
	BrainDir         string
	ConversationsDir string
	SummariesDB      string
	TransactionsDB   string
}

// Checker defines the interface for running setup diagnostics against GCP and local environment.
type Checker interface {
	Check(ctx context.Context, opts CheckOptions) (*SetupReport, error)
	CheckAuth(ctx context.Context) CheckItem
	CheckProject(ctx context.Context, projectID string) CheckItem
	CheckAPIs(ctx context.Context, projectID string) CheckItem
	CheckDatabase(ctx context.Context, projectID, databaseID string, dryRun bool) CheckItem
	CheckStorage(ctx context.Context, projectID string) CheckItem
	CheckLocal(ctx context.Context, opts CheckOptions) CheckItem
}
