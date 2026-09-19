package setup

import (
	"context"
	"fmt"

	"google.golang.org/api/firestore/v1"
	"google.golang.org/api/serviceusage/v1"
)

// GCPProvisioner implements the Provisioner interface for GCP resources.
type GCPProvisioner struct {
	dbCreator  func(ctx context.Context, projectID, databaseID, location string) error
	apiEnabler func(ctx context.Context, projectID string, apis []string) error
}

// NewGCPProvisioner creates a new GCPProvisioner with default live GCP logic.
func NewGCPProvisioner() *GCPProvisioner {
	p := &GCPProvisioner{}
	p.dbCreator = p.defaultDBCreator
	p.apiEnabler = p.defaultAPIEnabler
	return p
}

// SetDBCreator sets a custom database creator (useful for testing).
func (p *GCPProvisioner) SetDBCreator(fn func(ctx context.Context, projectID, databaseID, location string) error) {
	p.dbCreator = fn
}

// SetAPIEnabler sets a custom service API enabler (useful for testing).
func (p *GCPProvisioner) SetAPIEnabler(fn func(ctx context.Context, projectID string, apis []string) error) {
	p.apiEnabler = fn
}

// CreateDatabase provisions a Google Cloud Firestore database in Native mode.
func (p *GCPProvisioner) CreateDatabase(ctx context.Context, opts ProvisionOptions) (*ProvisionResult, error) {
	dbID := opts.DatabaseID
	if dbID == "" {
		dbID = "(default)"
	}
	loc := opts.Location
	if loc == "" {
		loc = "nam5"
	}

	if opts.DryRun {
		return &ProvisionResult{
			Created:    false,
			DatabaseID: dbID,
			Location:   loc,
			Type:       "FIRESTORE_NATIVE",
			Message:    fmt.Sprintf("Database %q would be created in project %q (location: %s, dry-run)", dbID, opts.ProjectID, loc),
		}, nil
	}

	if !opts.AutoApprove && opts.ConfirmPrompt != nil {
		promptMsg := fmt.Sprintf("Create Firestore database %q in project %q (location: %s)?", dbID, opts.ProjectID, loc)
		confirmed, err := opts.ConfirmPrompt(promptMsg)
		if err != nil {
			return nil, err
		}
		if !confirmed {
			return &ProvisionResult{
				Created:    false,
				DatabaseID: dbID,
				Location:   loc,
				Type:       "FIRESTORE_NATIVE",
				Message:    "Database creation was declined by user",
			}, nil
		}
	}

	if err := p.dbCreator(ctx, opts.ProjectID, dbID, loc); err != nil {
		return nil, fmt.Errorf("failed to create Firestore database %q in project %q: %w", dbID, opts.ProjectID, err)
	}

	return &ProvisionResult{
		Created:    true,
		DatabaseID: dbID,
		Location:   loc,
		Type:       "FIRESTORE_NATIVE",
		Message:    fmt.Sprintf("Firestore database %q created successfully in region %s", dbID, loc),
	}, nil
}

// EnableAPIs enables the specified Google Cloud service APIs.
func (p *GCPProvisioner) EnableAPIs(ctx context.Context, projectID string, apis []string, dryRun bool) (*EnableAPIsResult, error) {
	if len(apis) == 0 {
		return &EnableAPIsResult{
			Message: "No APIs to enable",
		}, nil
	}

	if dryRun {
		return &EnableAPIsResult{
			Skipped: apis,
			Message: fmt.Sprintf("%d API(s) would be enabled (dry-run)", len(apis)),
		}, nil
	}

	if err := p.apiEnabler(ctx, projectID, apis); err != nil {
		return nil, fmt.Errorf("failed to enable APIs on project %q: %w", projectID, err)
	}

	return &EnableAPIsResult{
		Enabled: apis,
		Message: fmt.Sprintf("%d API(s) enabled successfully", len(apis)),
	}, nil
}

// --- Default Live GCP Implementations ---

func (p *GCPProvisioner) defaultDBCreator(ctx context.Context, projectID, databaseID, location string) error {
	fsService, err := firestore.NewService(ctx)
	if err != nil {
		return err
	}

	parent := "projects/" + projectID
	db := &firestore.GoogleFirestoreAdminV1Database{
		Type:       "FIRESTORE_NATIVE",
		LocationId: location,
	}

	_, err = fsService.Projects.Databases.Create(parent, db).DatabaseId(databaseID).Context(ctx).Do()
	return err
}

func (p *GCPProvisioner) defaultAPIEnabler(ctx context.Context, projectID string, apis []string) error {
	suService, err := serviceusage.NewService(ctx)
	if err != nil {
		return err
	}

	batchReq := &serviceusage.BatchEnableServicesRequest{
		ServiceIds: apis,
	}
	parent := "projects/" + projectID
	_, err = suService.Services.BatchEnable(parent, batchReq).Context(ctx).Do()
	return err
}
