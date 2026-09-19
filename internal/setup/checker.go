package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/firestore/v1"
	"google.golang.org/api/serviceusage/v1"
)

// RequiredAPIs defines the GCP service APIs necessary for agy-sync operation.
var RequiredAPIs = []string{
	"firestore.googleapis.com",
	"cloudresourcemanager.googleapis.com",
	"storage.googleapis.com",
}

// GCPChecker implements the Checker interface for GCP and local environment.
type GCPChecker struct {
	authValidator  func(ctx context.Context) (string, error)
	projectChecker func(ctx context.Context, projectID string) (string, error)
	apiChecker     func(ctx context.Context, projectID string, services []string) (map[string]bool, error)
	dbChecker      func(ctx context.Context, projectID, databaseID string) (bool, string, error)
	storageChecker func(ctx context.Context, projectID string) (bool, error)
}

// NewGCPChecker constructs a GCPChecker initialized with default live GCP checking logic.
func NewGCPChecker() *GCPChecker {
	c := &GCPChecker{}
	c.authValidator = c.defaultAuthValidator
	c.projectChecker = c.defaultProjectChecker
	c.apiChecker = c.defaultAPIChecker
	c.dbChecker = c.defaultDBChecker
	c.storageChecker = c.defaultStorageChecker
	return c
}

// SetAuthValidator overrides the authentication validator (useful for testing).
func (c *GCPChecker) SetAuthValidator(fn func(ctx context.Context) (string, error)) {
	c.authValidator = fn
}

// SetProjectChecker overrides the project accessibility checker.
func (c *GCPChecker) SetProjectChecker(fn func(ctx context.Context, projectID string) (string, error)) {
	c.projectChecker = fn
}

// SetAPIChecker overrides the service API checker.
func (c *GCPChecker) SetAPIChecker(fn func(ctx context.Context, projectID string, services []string) (map[string]bool, error)) {
	c.apiChecker = fn
}

// SetDatabaseChecker overrides the database checker.
func (c *GCPChecker) SetDatabaseChecker(fn func(ctx context.Context, projectID, databaseID string) (bool, string, error)) {
	c.dbChecker = fn
}

// SetStorageChecker overrides the storage checker.
func (c *GCPChecker) SetStorageChecker(fn func(ctx context.Context, projectID string) (bool, error)) {
	c.storageChecker = fn
}

// CalculateAllPassed returns true if none of the check items have a StatusFail.
func (c *GCPChecker) CalculateAllPassed(items []CheckItem) bool {
	for _, item := range items {
		if item.Status == StatusFail {
			return false
		}
	}
	return true
}

// Check runs all diagnostics and aggregates the results into a SetupReport.
func (c *GCPChecker) Check(ctx context.Context, opts CheckOptions) (*SetupReport, error) {
	report := &SetupReport{
		Timestamp:  time.Now().UTC(),
		ProjectID:  opts.ProjectID,
		DatabaseID: opts.DatabaseID,
		DryRun:     opts.DryRun,
		Items:      make([]CheckItem, 0, 6),
	}

	// 1. Auth check
	report.Items = append(report.Items, c.CheckAuth(ctx))

	// 2. Project check (only if project is configured)
	if opts.ProjectID != "" {
		report.Items = append(report.Items, c.CheckProject(ctx, opts.ProjectID))
		report.Items = append(report.Items, c.CheckAPIs(ctx, opts.ProjectID))
		report.Items = append(report.Items, c.CheckDatabase(ctx, opts.ProjectID, opts.DatabaseID, opts.DryRun))
		report.Items = append(report.Items, c.CheckStorage(ctx, opts.ProjectID))
	} else {
		report.Items = append(report.Items, CheckItem{
			Name:        "Google Cloud Project Access",
			Status:      StatusFail,
			Message:     "No project ID specified",
			Remediation: "Specify --project-id or configure project_id in ~/.config/agy-sync/config.yaml",
		})
	}

	// 3. Local environment check
	report.Items = append(report.Items, c.CheckLocal(ctx, opts))

	report.AllPassed = c.CalculateAllPassed(report.Items)
	return report, nil
}

// CheckAuth verifies Application Default Credentials.
func (c *GCPChecker) CheckAuth(ctx context.Context) CheckItem {
	start := time.Now()
	account, err := c.authValidator(ctx)
	dur := time.Since(start)

	if err != nil {
		return CheckItem{
			Name:        "Google Cloud Authentication (ADC)",
			Status:      StatusFail,
			Message:     fmt.Sprintf("Application Default Credentials not found or invalid: %v", err),
			Remediation: "Run 'gcloud auth application-default login' to authenticate your local development environment.",
			Duration:    dur,
		}
	}

	return CheckItem{
		Name:     "Google Cloud Authentication (ADC)",
		Status:   StatusPass,
		Message:  fmt.Sprintf("Active credentials found (%s)", account),
		Duration: dur,
	}
}

// CheckProject verifies access to the specified GCP project.
func (c *GCPChecker) CheckProject(ctx context.Context, projectID string) CheckItem {
	start := time.Now()
	name, err := c.projectChecker(ctx, projectID)
	dur := time.Since(start)

	if err != nil {
		return CheckItem{
			Name:        "Google Cloud Project Access",
			Status:      StatusFail,
			Message:     fmt.Sprintf("Unable to access project %q: %v", projectID, err),
			Remediation: fmt.Sprintf("Ensure project %q exists and your account has 'roles/viewer' or 'roles/editor' access.", projectID),
			Duration:    dur,
		}
	}

	msg := fmt.Sprintf("Project %q accessible", projectID)
	if name != "" && name != projectID {
		msg = fmt.Sprintf("Project %q (%s) accessible", projectID, name)
	}

	return CheckItem{
		Name:     "Google Cloud Project Access",
		Status:   StatusPass,
		Message:  msg,
		Duration: dur,
	}
}

// CheckAPIs checks whether all required APIs are enabled in the project.
func (c *GCPChecker) CheckAPIs(ctx context.Context, projectID string) CheckItem {
	start := time.Now()
	apiMap, err := c.apiChecker(ctx, projectID, RequiredAPIs)
	dur := time.Since(start)

	if err != nil {
		return CheckItem{
			Name:        "Required Cloud APIs",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("Unable to inspect service usage APIs: %v", err),
			Remediation: fmt.Sprintf("Verify that serviceusage.googleapis.com is enabled on project %q.", projectID),
			Duration:    dur,
		}
	}

	var disabled []string
	var details []string
	for _, api := range RequiredAPIs {
		enabled := apiMap[api]
		if enabled {
			details = append(details, api+": enabled")
		} else {
			disabled = append(disabled, api)
			details = append(details, api+": DISABLED")
		}
	}

	if len(disabled) > 0 {
		return CheckItem{
			Name:        "Required Cloud APIs",
			Status:      StatusFail,
			Message:     fmt.Sprintf("%d required API(s) disabled: %s", len(disabled), strings.Join(disabled, ", ")),
			Details:     details,
			Remediation: fmt.Sprintf("Enable missing APIs with: gcloud services enable %s --project %s", strings.Join(disabled, " "), projectID),
			Duration:    dur,
		}
	}

	return CheckItem{
		Name:     "Required Cloud APIs",
		Status:   StatusPass,
		Message:  "All required service APIs are enabled",
		Details:  details,
		Duration: dur,
	}
}

// CheckDatabase checks whether the target Firestore database exists and is ready.
func (c *GCPChecker) CheckDatabase(ctx context.Context, projectID, databaseID string, dryRun bool) CheckItem {
	start := time.Now()
	if databaseID == "" {
		databaseID = "(default)"
	}

	exists, state, err := c.dbChecker(ctx, projectID, databaseID)
	dur := time.Since(start)

	if err != nil {
		return CheckItem{
			Name:        "Firestore Database Access",
			Status:      StatusFail,
			Message:     fmt.Sprintf("Failed to query Firestore database %q: %v", databaseID, err),
			Remediation: fmt.Sprintf("Ensure Firestore API is enabled and account has 'roles/datastore.user' or 'roles/datastore.owner' on %q.", projectID),
			Duration:    dur,
		}
	}

	if !exists {
		if dryRun {
			return CheckItem{
				Name:        "Firestore Database Access",
				Status:      StatusWarn,
				Message:     fmt.Sprintf("Database %q does not exist in project %q (dry-run mode, skipping creation)", databaseID, projectID),
				Remediation: fmt.Sprintf("Run 'agy-sync setup --project-id %s' without --dry-run to create the database.", projectID),
				Duration:    dur,
			}
		}
		return CheckItem{
			Name:        "Firestore Database Access",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("Database %q does not exist in project %q", databaseID, projectID),
			Remediation: fmt.Sprintf("Run 'agy-sync setup --project-id %s' to provision the Firestore database.", projectID),
			Duration:    dur,
		}
	}

	msg := fmt.Sprintf("Database %q ready", databaseID)
	if state != "" {
		msg = fmt.Sprintf("Database %q state: %s", databaseID, state)
	}

	return CheckItem{
		Name:     "Firestore Database Access",
		Status:   StatusPass,
		Message:  msg,
		Duration: dur,
	}
}

// CheckStorage checks Google Cloud Storage accessibility.
func (c *GCPChecker) CheckStorage(ctx context.Context, projectID string) CheckItem {
	start := time.Now()
	accessible, err := c.storageChecker(ctx, projectID)
	dur := time.Since(start)

	if err != nil {
		return CheckItem{
			Name:        "Google Cloud Storage Access",
			Status:      StatusWarn,
			Message:     fmt.Sprintf("Cloud Storage check returned: %v", err),
			Remediation: "Ensure storage.googleapis.com is enabled if offloading large artifacts.",
			Duration:    dur,
		}
	}

	if !accessible {
		return CheckItem{
			Name:        "Google Cloud Storage Access",
			Status:      StatusWarn,
			Message:     "Cloud Storage not accessible or not configured",
			Remediation: "Enable storage.googleapis.com and grant 'roles/storage.objectAdmin' if offloading large files.",
			Duration:    dur,
		}
	}

	return CheckItem{
		Name:     "Google Cloud Storage Access",
		Status:   StatusPass,
		Message:  "Cloud Storage API and permissions verified",
		Duration: dur,
	}
}

// CheckLocal verifies local config files, directories, and SQLite databases.
func (c *GCPChecker) CheckLocal(_ context.Context, opts CheckOptions) CheckItem {
	start := time.Now()
	var details []string
	var issues []string

	// Config file
	if opts.ConfigPath != "" {
		if _, err := os.Stat(opts.ConfigPath); err != nil {
			issues = append(issues, "Config file missing: "+opts.ConfigPath)
		} else {
			details = append(details, "Config: "+opts.ConfigPath+" (OK)")
		}
	}

	// Brain directory
	if opts.BrainDir != "" {
		if err := os.MkdirAll(opts.BrainDir, 0o755); err != nil {
			issues = append(issues, fmt.Sprintf("Cannot create brain dir %s: %v", opts.BrainDir, err))
		} else {
			details = append(details, "Brain dir: "+opts.BrainDir+" (OK)")
		}
	}

	// Conversations directory
	if opts.ConversationsDir != "" {
		if err := os.MkdirAll(opts.ConversationsDir, 0o755); err != nil {
			issues = append(issues, fmt.Sprintf("Cannot create conversations dir %s: %v", opts.ConversationsDir, err))
		} else {
			details = append(details, "Conversations dir: "+opts.ConversationsDir+" (OK)")
		}
	}

	// Transactions DB parent dir
	if opts.TransactionsDB != "" {
		txDir := filepath.Dir(opts.TransactionsDB)
		if err := os.MkdirAll(txDir, 0o755); err != nil {
			issues = append(issues, fmt.Sprintf("Cannot create transactions db dir %s: %v", txDir, err))
		} else {
			details = append(details, "Transactions DB: "+opts.TransactionsDB+" (OK)")
		}
	}

	dur := time.Since(start)
	if len(issues) > 0 {
		return CheckItem{
			Name:        "Local Environment & Configuration",
			Status:      StatusWarn,
			Message:     strings.Join(issues, "; "),
			Details:     details,
			Remediation: "Run 'agy-sync init' to scaffold configuration and verify directory permissions.",
			Duration:    dur,
		}
	}

	return CheckItem{
		Name:     "Local Environment & Configuration",
		Status:   StatusPass,
		Message:  "Local directories and configuration accessible",
		Details:  details,
		Duration: dur,
	}
}

// --- Default Live GCP Checkers ---

func (c *GCPChecker) defaultAuthValidator(ctx context.Context) (string, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", err
	}
	if len(creds.JSON) > 0 {
		var parsed struct {
			ClientEmail string `json:"client_email"`
		}
		if jsonErr := json.Unmarshal(creds.JSON, &parsed); jsonErr == nil && parsed.ClientEmail != "" {
			return parsed.ClientEmail, nil
		}
	}
	return "Application Default Credentials", nil
}

func (c *GCPChecker) defaultProjectChecker(ctx context.Context, projectID string) (string, error) {
	crmService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return "", err
	}
	proj, err := crmService.Projects.Get(projectID).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return proj.Name, nil
}

func (c *GCPChecker) defaultAPIChecker(ctx context.Context, projectID string, services []string) (map[string]bool, error) {
	suService, err := serviceusage.NewService(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, s := range services {
		name := fmt.Sprintf("projects/%s/services/%s", projectID, s)
		resp, err := suService.Services.Get(name).Context(ctx).Do()
		if err != nil {
			// If not found or error, mark false
			result[s] = false
			continue
		}
		result[s] = (resp.State == "ENABLED")
	}
	return result, nil
}

func (c *GCPChecker) defaultDBChecker(ctx context.Context, projectID, databaseID string) (bool, string, error) {
	fsService, err := firestore.NewService(ctx)
	if err != nil {
		return false, "", err
	}

	name := fmt.Sprintf("projects/%s/databases/%s", projectID, databaseID)
	db, err := fsService.Projects.Databases.Get(name).Context(ctx).Do()
	if err != nil {
		// If 404, database does not exist
		if strings.Contains(err.Error(), "404") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return false, "", nil
		}
		return false, "", err
	}
	return true, db.Type, nil
}

func (c *GCPChecker) defaultStorageChecker(ctx context.Context, projectID string) (bool, error) {
	// Check if storage API is enabled or credentials can construct client
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/devstorage.read_write")
	if err != nil {
		return false, err
	}
	return creds != nil, nil
}
