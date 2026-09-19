package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/setup"
)

type mockChecker struct {
	report *setup.SetupReport
	err    error
}

func (m *mockChecker) Check(ctx context.Context, opts setup.CheckOptions) (*setup.SetupReport, error) {
	if m.err != nil {
		return nil, m.err
	}
	r := *m.report
	r.ProjectID = opts.ProjectID
	r.DatabaseID = opts.DatabaseID
	r.DryRun = opts.DryRun
	return &r, nil
}

func (m *mockChecker) CheckAuth(ctx context.Context) setup.CheckItem                       { return setup.CheckItem{} }
func (m *mockChecker) CheckProject(ctx context.Context, projectID string) setup.CheckItem { return setup.CheckItem{} }
func (m *mockChecker) CheckAPIs(ctx context.Context, projectID string) setup.CheckItem    { return setup.CheckItem{} }
func (m *mockChecker) CheckDatabase(ctx context.Context, projectID, databaseID string, dryRun bool) setup.CheckItem {
	return setup.CheckItem{}
}
func (m *mockChecker) CheckStorage(ctx context.Context, projectID string) setup.CheckItem { return setup.CheckItem{} }
func (m *mockChecker) CheckLocal(ctx context.Context, opts setup.CheckOptions) setup.CheckItem {
	return setup.CheckItem{}
}

type mockProvisioner struct {
	created    bool
	dbCalled   bool
	apisCalled bool
}

func (m *mockProvisioner) CreateDatabase(ctx context.Context, opts setup.ProvisionOptions) (*setup.ProvisionResult, error) {
	m.dbCalled = true
	if opts.DryRun {
		return &setup.ProvisionResult{Created: false, Message: "dry-run"}, nil
	}
	if !opts.AutoApprove && opts.ConfirmPrompt != nil {
		ok, err := opts.ConfirmPrompt("Create database?")
		if err != nil {
			return nil, err
		}
		if !ok {
			return &setup.ProvisionResult{Created: false, Message: "declined"}, nil
		}
	}
	m.created = true
	return &setup.ProvisionResult{Created: true, DatabaseID: opts.DatabaseID, Location: opts.Location, Message: "created"}, nil
}

func (m *mockProvisioner) EnableAPIs(ctx context.Context, projectID string, apis []string, dryRun bool) (*setup.EnableAPIsResult, error) {
	m.apisCalled = true
	return &setup.EnableAPIsResult{Enabled: apis, Message: "enabled"}, nil
}

func TestSetupCommand_AllPass(t *testing.T) {
	mockChk := &mockChecker{
		report: &setup.SetupReport{
			AllPassed: true,
			Items: []setup.CheckItem{
				{Name: "Google Cloud Authentication (ADC)", Status: setup.StatusPass, Message: "Active credentials"},
				{Name: "Google Cloud Project Access", Status: setup.StatusPass, Message: "Project accessible"},
				{Name: "Required Cloud APIs", Status: setup.StatusPass, Message: "All APIs enabled"},
				{Name: "Firestore Database Access", Status: setup.StatusPass, Message: "Database ready"},
				{Name: "Google Cloud Storage Access", Status: setup.StatusPass, Message: "Storage verified"},
				{Name: "Local Environment & Configuration", Status: setup.StatusPass, Message: "Local paths OK"},
			},
		},
	}
	mockProv := &mockProvisioner{}

	cmd := newSetupCommandWith(mockChk, mockProv)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--project-id", "test-proj"})

	err := cmd.Execute()
	require.NoError(t, err)
	output := outBuf.String()
	assert.Contains(t, output, "AGY-SYNC ENVIRONMENT & CLOUD SETUP")
	assert.Contains(t, output, "[✓]")
	assert.Contains(t, output, "ALL CHECKS PASSED")
}

func TestSetupCommand_JSON(t *testing.T) {
	mockChk := &mockChecker{
		report: &setup.SetupReport{
			AllPassed: true,
			Items: []setup.CheckItem{
				{Name: "Auth", Status: setup.StatusPass, Message: "Active credentials"},
			},
		},
	}
	mockProv := &mockProvisioner{}

	cmd := newSetupCommandWith(mockChk, mockProv)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--project-id", "test-proj", "--json"})

	err := cmd.Execute()
	require.NoError(t, err)

	var report setup.SetupReport
	err = json.Unmarshal(outBuf.Bytes(), &report)
	require.NoError(t, err)
	assert.True(t, report.AllPassed)
	assert.Equal(t, "test-proj", report.ProjectID)
}

func TestSetupCommand_DryRun(t *testing.T) {
	mockChk := &mockChecker{
		report: &setup.SetupReport{
			AllPassed: true,
			Items: []setup.CheckItem{
				{Name: "Auth", Status: setup.StatusPass, Message: "Active credentials"},
			},
		},
	}
	mockProv := &mockProvisioner{}

	cmd := newSetupCommandWith(mockChk, mockProv)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--project-id", "test-proj", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "Dry Run")
	assert.False(t, mockProv.dbCalled)
}

func TestSetupCommand_FailureReportsRemediation(t *testing.T) {
	mockChk := &mockChecker{
		report: &setup.SetupReport{
			AllPassed: false,
			Items: []setup.CheckItem{
				{
					Name:        "Google Cloud Authentication (ADC)",
					Status:      setup.StatusFail,
					Message:     "Credentials not found",
					Remediation: "Run 'gcloud auth application-default login'",
				},
			},
		},
	}
	mockProv := &mockProvisioner{}

	cmd := newSetupCommandWith(mockChk, mockProv)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--project-id", "test-proj"})

	err := cmd.Execute()
	require.Error(t, err)
	output := outBuf.String()
	assert.Contains(t, output, "[✗]")
	assert.Contains(t, output, "ISSUES DETECTED")
	assert.Contains(t, output, "gcloud auth application-default login")
}

func TestSetupCommand_DatabaseCreationFlow(t *testing.T) {
	t.Run("Auto-approve with --yes creates database", func(t *testing.T) {
		mockChk := &mockChecker{
			report: &setup.SetupReport{
				AllPassed: true,
				Items: []setup.CheckItem{
					{Name: "Firestore Database Access", Status: setup.StatusWarn, Message: "Database does not exist"},
				},
			},
		}
		mockProv := &mockProvisioner{}

		cmd := newSetupCommandWith(mockChk, mockProv)
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--project-id", "test-proj", "--yes"})

		err := cmd.Execute()
		require.NoError(t, err)
		assert.True(t, mockProv.created)
	})

	t.Run("Interactive confirm yes creates database", func(t *testing.T) {
		mockChk := &mockChecker{
			report: &setup.SetupReport{
				AllPassed: true,
				Items: []setup.CheckItem{
					{Name: "Firestore Database Access", Status: setup.StatusWarn, Message: "Database does not exist"},
				},
			},
		}
		mockProv := &mockProvisioner{}

		cmd := newSetupCommandWith(mockChk, mockProv)
		var inBuf bytes.Buffer
		inBuf.WriteString("y\n")
		cmd.SetIn(&inBuf)
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--project-id", "test-proj"})

		err := cmd.Execute()
		require.NoError(t, err)
		assert.True(t, mockProv.created)
	})

	t.Run("Interactive confirm no skips database creation", func(t *testing.T) {
		mockChk := &mockChecker{
			report: &setup.SetupReport{
				AllPassed: true,
				Items: []setup.CheckItem{
					{Name: "Firestore Database Access", Status: setup.StatusWarn, Message: "Database does not exist"},
				},
			},
		}
		mockProv := &mockProvisioner{}

		cmd := newSetupCommandWith(mockChk, mockProv)
		var inBuf bytes.Buffer
		inBuf.WriteString("n\n")
		cmd.SetIn(&inBuf)
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--project-id", "test-proj"})

		err := cmd.Execute()
		require.NoError(t, err)
		assert.False(t, mockProv.created)
	})
}

func TestSetupCommand_LoadConfigDefault(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("project_id: loaded-proj\ndatabase_id: loaded-db\n"), 0o644))

	mockChk := &mockChecker{
		report: &setup.SetupReport{
			AllPassed: true,
			Items:     []setup.CheckItem{},
		},
	}
	mockProv := &mockProvisioner{}

	cmd := newSetupCommandWith(mockChk, mockProv)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetArgs([]string{"--config", cfgPath, "--json"})

	err := cmd.Execute()
	require.NoError(t, err)

	var report setup.SetupReport
	err = json.Unmarshal(outBuf.Bytes(), &report)
	require.NoError(t, err)
	assert.Equal(t, "loaded-proj", report.ProjectID)
	assert.Equal(t, "loaded-db", report.DatabaseID)
}
