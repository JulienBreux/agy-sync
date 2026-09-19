package setup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/setup"
)

func TestCheckReport_AllPassedAggregation(t *testing.T) {
	reportAllPass := &setup.SetupReport{
		Items: []setup.CheckItem{
			{Name: "Auth", Status: setup.StatusPass},
			{Name: "Project", Status: setup.StatusPass},
			{Name: "APIs", Status: setup.StatusPass},
		},
	}
	// An aggregate function or Check method calculates AllPassed
	checker := setup.NewGCPChecker()
	assert.True(t, checker.CalculateAllPassed(reportAllPass.Items))

	reportWithFail := &setup.SetupReport{
		Items: []setup.CheckItem{
			{Name: "Auth", Status: setup.StatusPass},
			{Name: "Project", Status: setup.StatusFail},
			{Name: "APIs", Status: setup.StatusPass},
		},
	}
	assert.False(t, checker.CalculateAllPassed(reportWithFail.Items))

	// Warnings do not cause AllPassed to be false
	reportWithWarn := &setup.SetupReport{
		Items: []setup.CheckItem{
			{Name: "Auth", Status: setup.StatusPass},
			{Name: "Storage", Status: setup.StatusWarn},
		},
	}
	assert.True(t, checker.CalculateAllPassed(reportWithWarn.Items))
}

func TestCheckLocal_ValidPaths(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	brainDir := filepath.Join(tmpDir, "brain")
	convsDir := filepath.Join(tmpDir, "conversations")
	summariesDB := filepath.Join(tmpDir, "summaries.db")
	txDB := filepath.Join(tmpDir, "transactions.db")

	require.NoError(t, os.WriteFile(configPath, []byte("project_id: test\n"), 0o644))
	require.NoError(t, os.MkdirAll(brainDir, 0o755))
	require.NoError(t, os.MkdirAll(convsDir, 0o755))

	checker := setup.NewGCPChecker()
	item := checker.CheckLocal(context.Background(), setup.CheckOptions{
		ConfigPath:       configPath,
		BrainDir:         brainDir,
		ConversationsDir: convsDir,
		SummariesDB:      summariesDB,
		TransactionsDB:   txDB,
	})

	assert.Equal(t, setup.StatusPass, item.Status)
	assert.Contains(t, item.Message, "accessible")
}

func TestCheckLocal_MissingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	checker := setup.NewGCPChecker()
	item := checker.CheckLocal(context.Background(), setup.CheckOptions{
		ConfigPath: configPath,
		BrainDir:   filepath.Join(tmpDir, "brain"),
	})

	assert.Equal(t, setup.StatusWarn, item.Status)
	assert.NotEmpty(t, item.Remediation)
	assert.Contains(t, item.Remediation, "agy-sync init")
}

func TestCheckAuth_CustomValidator(t *testing.T) {
	t.Run("Valid ADC", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetAuthValidator(func(ctx context.Context) (string, error) {
			return "developer@example.com", nil
		})

		item := checker.CheckAuth(context.Background())
		assert.Equal(t, setup.StatusPass, item.Status)
		assert.Contains(t, item.Message, "developer@example.com")
	})

	t.Run("Missing ADC", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetAuthValidator(func(ctx context.Context) (string, error) {
			return "", errors.New("could not find default credentials")
		})

		item := checker.CheckAuth(context.Background())
		assert.Equal(t, setup.StatusFail, item.Status)
		assert.NotEmpty(t, item.Remediation)
		assert.Contains(t, item.Remediation, "gcloud auth application-default login")
	})
}

func TestCheckProject_CustomChecker(t *testing.T) {
	t.Run("Project Accessible", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetProjectChecker(func(ctx context.Context, projectID string) (string, error) {
			return "My Test Project", nil
		})

		item := checker.CheckProject(context.Background(), "my-test-proj")
		assert.Equal(t, setup.StatusPass, item.Status)
		assert.Contains(t, item.Message, "my-test-proj")
	})

	t.Run("Project Not Found or Forbidden", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetProjectChecker(func(ctx context.Context, projectID string) (string, error) {
			return "", errors.New("permission denied on project")
		})

		item := checker.CheckProject(context.Background(), "forbidden-proj")
		assert.Equal(t, setup.StatusFail, item.Status)
		assert.Contains(t, item.Message, "permission denied")
		assert.NotEmpty(t, item.Remediation)
	})
}

func TestCheckAPIs_CustomChecker(t *testing.T) {
	t.Run("All Required APIs Enabled", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetAPIChecker(func(ctx context.Context, projectID string, services []string) (map[string]bool, error) {
			res := make(map[string]bool)
			for _, s := range services {
				res[s] = true
			}
			return res, nil
		})

		item := checker.CheckAPIs(context.Background(), "my-proj")
		assert.Equal(t, setup.StatusPass, item.Status)
		assert.Len(t, item.Details, 3)
	})

	t.Run("Some APIs Disabled", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetAPIChecker(func(ctx context.Context, projectID string, services []string) (map[string]bool, error) {
			return map[string]bool{
				"firestore.googleapis.com":           true,
				"cloudresourcemanager.googleapis.com": false,
				"storage.googleapis.com":              true,
			}, nil
		})

		item := checker.CheckAPIs(context.Background(), "my-proj")
		assert.Equal(t, setup.StatusFail, item.Status)
		assert.Contains(t, item.Message, "cloudresourcemanager.googleapis.com")
		assert.Contains(t, item.Remediation, "gcloud services enable")
	})
}

func TestCheckDatabase_CustomChecker(t *testing.T) {
	t.Run("Database Exists and Ready", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetDatabaseChecker(func(ctx context.Context, projectID, databaseID string) (bool, string, error) {
			return true, "READY", nil
		})

		item := checker.CheckDatabase(context.Background(), "my-proj", "(default)", false)
		assert.Equal(t, setup.StatusPass, item.Status)
		assert.Contains(t, item.Message, "READY")
	})

	t.Run("Database Does Not Exist - Dry Run", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetDatabaseChecker(func(ctx context.Context, projectID, databaseID string) (bool, string, error) {
			return false, "", nil
		})

		item := checker.CheckDatabase(context.Background(), "my-proj", "(default)", true)
		assert.Equal(t, setup.StatusWarn, item.Status)
		assert.Contains(t, item.Message, "dry-run")
	})

	t.Run("Database Does Not Exist - Live Run", func(t *testing.T) {
		checker := setup.NewGCPChecker()
		checker.SetDatabaseChecker(func(ctx context.Context, projectID, databaseID string) (bool, string, error) {
			return false, "", nil
		})

		item := checker.CheckDatabase(context.Background(), "my-proj", "(default)", false)
		assert.Equal(t, setup.StatusWarn, item.Status)
		assert.Contains(t, item.Remediation, "agy-sync setup")
	})
}

func TestCheckFullFlow(t *testing.T) {
	checker := setup.NewGCPChecker()
	checker.SetAuthValidator(func(ctx context.Context) (string, error) { return "user@example.com", nil })
	checker.SetProjectChecker(func(ctx context.Context, projectID string) (string, error) { return "Test Proj", nil })
	checker.SetAPIChecker(func(ctx context.Context, projectID string, services []string) (map[string]bool, error) {
		res := make(map[string]bool)
		for _, s := range services {
			res[s] = true
		}
		return res, nil
	})
	checker.SetDatabaseChecker(func(ctx context.Context, projectID, databaseID string) (bool, string, error) {
		return true, "READY", nil
	})
	checker.SetStorageChecker(func(ctx context.Context, projectID string) (bool, error) {
		return true, nil
	})

	report, err := checker.Check(context.Background(), setup.CheckOptions{
		ProjectID:  "test-proj",
		DatabaseID: "(default)",
		DryRun:     true,
	})
	require.NoError(t, err)
	assert.True(t, report.AllPassed)
	assert.Len(t, report.Items, 6)
}
