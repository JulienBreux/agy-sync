package setup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/setup"
)

func TestCreateDatabase_DryRun(t *testing.T) {
	called := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		called = true
		return nil
	})

	res, err := provisioner.CreateDatabase(context.Background(), setup.ProvisionOptions{
		ProjectID:   "test-proj",
		DatabaseID:  "(default)",
		Location:    "nam5",
		DryRun:      true,
		AutoApprove: true,
	})

	require.NoError(t, err)
	assert.False(t, called, "dbCreator should not be invoked in dry-run mode")
	assert.False(t, res.Created)
	assert.Contains(t, res.Message, "dry-run")
}

func TestCreateDatabase_InteractiveDeclined(t *testing.T) {
	called := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		called = true
		return nil
	})

	res, err := provisioner.CreateDatabase(context.Background(), setup.ProvisionOptions{
		ProjectID:   "test-proj",
		DatabaseID:  "(default)",
		Location:    "nam5",
		DryRun:      false,
		AutoApprove: false,
		ConfirmPrompt: func(prompt string) (bool, error) {
			return false, nil
		},
	})

	require.NoError(t, err)
	assert.False(t, called, "dbCreator should not be invoked if user declines confirmation")
	assert.False(t, res.Created)
	assert.Contains(t, res.Message, "declined")
}

func TestCreateDatabase_InteractiveAccepted(t *testing.T) {
	called := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		called = true
		assert.Equal(t, "test-proj", projectID)
		assert.Equal(t, "(default)", databaseID)
		assert.Equal(t, "eur3", location)
		return nil
	})

	res, err := provisioner.CreateDatabase(context.Background(), setup.ProvisionOptions{
		ProjectID:   "test-proj",
		DatabaseID:  "(default)",
		Location:    "eur3",
		DryRun:      false,
		AutoApprove: false,
		ConfirmPrompt: func(prompt string) (bool, error) {
			return true, nil
		},
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.True(t, res.Created)
	assert.Contains(t, res.Message, "created successfully")
}

func TestCreateDatabase_AutoApprove(t *testing.T) {
	called := false
	promptCalled := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		called = true
		return nil
	})

	res, err := provisioner.CreateDatabase(context.Background(), setup.ProvisionOptions{
		ProjectID:   "test-proj",
		DatabaseID:  "custom-db",
		Location:    "nam5",
		DryRun:      false,
		AutoApprove: true,
		ConfirmPrompt: func(prompt string) (bool, error) {
			promptCalled = true
			return true, nil
		},
	})

	require.NoError(t, err)
	assert.False(t, promptCalled, "Prompt should not be displayed when AutoApprove is true")
	assert.True(t, called)
	assert.True(t, res.Created)
	assert.Equal(t, "custom-db", res.DatabaseID)
}

func TestCreateDatabase_ErrorHandling(t *testing.T) {
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetDBCreator(func(ctx context.Context, projectID, databaseID, location string) error {
		return errors.New("permission denied")
	})

	_, err := provisioner.CreateDatabase(context.Background(), setup.ProvisionOptions{
		ProjectID:   "test-proj",
		DatabaseID:  "(default)",
		Location:    "nam5",
		DryRun:      false,
		AutoApprove: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestEnableAPIs_DryRun(t *testing.T) {
	called := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetAPIEnabler(func(ctx context.Context, projectID string, apis []string) error {
		called = true
		return nil
	})

	res, err := provisioner.EnableAPIs(context.Background(), "test-proj", []string{"firestore.googleapis.com"}, true)
	require.NoError(t, err)
	assert.False(t, called)
	assert.NotEmpty(t, res.Skipped)
	assert.Contains(t, res.Message, "dry-run")
}

func TestEnableAPIs_Success(t *testing.T) {
	called := false
	provisioner := setup.NewGCPProvisioner()
	provisioner.SetAPIEnabler(func(ctx context.Context, projectID string, apis []string) error {
		called = true
		assert.Equal(t, "test-proj", projectID)
		assert.Equal(t, []string{"firestore.googleapis.com"}, apis)
		return nil
	})

	res, err := provisioner.EnableAPIs(context.Background(), "test-proj", []string{"firestore.googleapis.com"}, false)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, []string{"firestore.googleapis.com"}, res.Enabled)
	assert.Contains(t, res.Message, "enabled successfully")
}
