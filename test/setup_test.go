package test_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/setup"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func TestE2E_SetupDiagnosticsAndProvisioning(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.yaml")
	brainDir := filepath.Join(tmpDir, "brain")
	convsDir := filepath.Join(tmpDir, "conversations")
	summariesDB := filepath.Join(tmpDir, "summaries.db")
	txDB := filepath.Join(tmpDir, "tx.db")

	cfg := &config.Config{
		ProjectID:        "e2e-project",
		DatabaseID:       "(default)",
		MachineID:        "e2e-machine",
		BrainDir:         brainDir,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		TransactionsDB:   txDB,
	}
	require.NoError(t, config.SaveConfig(configPath, cfg))
	require.NoError(t, os.MkdirAll(brainDir, 0o755))
	require.NoError(t, os.MkdirAll(convsDir, 0o755))

	checker := setup.NewGCPChecker()
	checker.SetAuthValidator(func(ctx context.Context) (string, error) {
		return "tester@example.com", nil
	})
	checker.SetProjectChecker(func(ctx context.Context, projectID string) (string, error) {
		return "E2E Test Project", nil
	})
	checker.SetAPIChecker(func(ctx context.Context, projectID string, services []string) (map[string]bool, error) {
		res := make(map[string]bool)
		for _, s := range services {
			res[s] = true
		}
		return res, nil
	})

	dbState := false
	checker.SetDatabaseChecker(func(ctx context.Context, projectID, databaseID string) (bool, string, error) {
		if dbState {
			return true, "READY", nil
		}
		return false, "", nil
	})
	checker.SetStorageChecker(func(ctx context.Context, projectID string) (bool, error) {
		return true, nil
	})

	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		dbState = true
		return nil
	})

	// Step 1: Dry run when DB does not exist
	report1, err := checker.Check(ctx, setup.CheckOptions{
		ProjectID:        "e2e-project",
		DatabaseID:       "(default)",
		DryRun:           true,
		ConfigPath:       configPath,
		BrainDir:         brainDir,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		TransactionsDB:   txDB,
	})
	require.NoError(t, err)
	assert.True(t, report1.AllPassed, "Warnings in dry run do not fail AllPassed")

	var dbItem *setup.CheckItem
	for i := range report1.Items {
		if report1.Items[i].Name == "Firestore Database Access" {
			dbItem = &report1.Items[i]
			break
		}
	}
	require.NotNil(t, dbItem)
	assert.Equal(t, setup.StatusWarn, dbItem.Status)
	assert.Contains(t, dbItem.Message, "dry-run")
	assert.False(t, dbState, "Database should not have been created in dry-run")

	// Step 2: Provision database
	provRes, err := provisioner.CreateDatabase(ctx, setup.ProvisionOptions{
		ProjectID:   "e2e-project",
		DatabaseID:  "(default)",
		Location:    "nam5",
		DryRun:      false,
		AutoApprove: true,
	})
	require.NoError(t, err)
	assert.True(t, provRes.Created)
	assert.True(t, dbState)

	// Step 3: Re-check after provisioning
	report2, err := checker.Check(ctx, setup.CheckOptions{
		ProjectID:        "e2e-project",
		DatabaseID:       "(default)",
		DryRun:           false,
		ConfigPath:       configPath,
		BrainDir:         brainDir,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		TransactionsDB:   txDB,
	})
	require.NoError(t, err)
	assert.True(t, report2.AllPassed)

	for _, item := range report2.Items {
		assert.Equal(t, setup.StatusPass, item.Status, "Item %s should be PASS", item.Name)
	}

	// Verify JSON serialization round-trip
	jsonData, err := json.Marshal(report2)
	require.NoError(t, err)
	var decoded setup.SetupReport
	require.NoError(t, json.Unmarshal(jsonData, &decoded))
	assert.True(t, decoded.AllPassed)
	assert.Equal(t, "e2e-project", decoded.ProjectID)
	assert.Equal(t, "(default)", decoded.DatabaseID)
	assert.Len(t, decoded.Items, 6)
}
