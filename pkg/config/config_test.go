package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "(default)", cfg.DatabaseID)
	assert.NotEmpty(t, cfg.MachineID)
	assert.NotEmpty(t, cfg.BrainDir)
	assert.NotEmpty(t, cfg.ConversationsDir)
	assert.NotEmpty(t, cfg.SummariesDB)
	assert.NotEmpty(t, cfg.TransactionsDB)
	assert.False(t, cfg.NoDBSync)
	assert.Equal(t, 2, cfg.SyncIntervalSeconds)
	assert.Equal(t, "INFO", cfg.LogLevel)
}

func TestConfigValidation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:           "test-gcp-project",
			DatabaseID:          "(default)",
			MachineID:           "macbook-pro-1",
			BrainDir:            "/tmp/brain",
			ConversationsDir:    "/tmp/conversations",
			SummariesDB:         "/tmp/conversation_summaries.db",
			SyncIntervalSeconds: 2,
			LogLevel:            "INFO",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing project ID", func(t *testing.T) {
		cfg := &config.Config{
			DatabaseID:       "(default)",
			MachineID:        "macbook-pro-1",
			BrainDir:         "/tmp/brain",
			ConversationsDir: "/tmp/conversations",
			SummariesDB:      "/tmp/conversation_summaries.db",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project_id is required")
	})

	t.Run("missing machine ID", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:        "test-gcp-project",
			DatabaseID:       "(default)",
			MachineID:        "",
			BrainDir:         "/tmp/brain",
			ConversationsDir: "/tmp/conversations",
			SummariesDB:      "/tmp/conversation_summaries.db",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "machine_id is required")
	})

	t.Run("missing brain dir", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:        "test-gcp-project",
			DatabaseID:       "(default)",
			MachineID:        "macbook-pro-1",
			BrainDir:         "",
			ConversationsDir: "/tmp/conversations",
			SummariesDB:      "/tmp/conversation_summaries.db",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "brain_dir is required")
	})

	t.Run("missing conversations dir when db sync enabled", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:        "test-gcp-project",
			DatabaseID:       "(default)",
			MachineID:        "macbook-pro-1",
			BrainDir:         "/tmp/brain",
			ConversationsDir: "",
			SummariesDB:      "/tmp/conversation_summaries.db",
			NoDBSync:         false,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conversations_dir is required")
	})

	t.Run("missing summaries db when db sync enabled", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:        "test-gcp-project",
			DatabaseID:       "(default)",
			MachineID:        "macbook-pro-1",
			BrainDir:         "/tmp/brain",
			ConversationsDir: "/tmp/conversations",
			SummariesDB:      "",
			NoDBSync:         false,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "summaries_db is required")
	})

	t.Run("missing conversations dir and summaries db when db sync disabled", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:  "test-gcp-project",
			DatabaseID: "(default)",
			MachineID:  "macbook-pro-1",
			BrainDir:   "/tmp/brain",
			NoDBSync:   true,
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})
}

func TestSaveAndLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	original := &config.Config{
		ProjectID:           "my-gcp-proj",
		DatabaseID:          "custom-db",
		MachineID:           "test-machine-42",
		BrainDir:            "/custom/brain/path",
		ConversationsDir:    "/custom/convs/path",
		SummariesDB:         "/custom/summaries.db",
		NoDBSync:            true,
		SyncIntervalSeconds: 5,
		LogLevel:            "DEBUG",
	}

	err := config.SaveConfig(configPath, original)
	require.NoError(t, err)

	assert.FileExists(t, configPath)

	loaded, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, original.ProjectID, loaded.ProjectID)
	assert.Equal(t, original.DatabaseID, loaded.DatabaseID)
	assert.Equal(t, original.MachineID, loaded.MachineID)
	assert.Equal(t, original.BrainDir, loaded.BrainDir)
	assert.Equal(t, original.ConversationsDir, loaded.ConversationsDir)
	assert.Equal(t, original.SummariesDB, loaded.SummariesDB)
	assert.Equal(t, original.NoDBSync, loaded.NoDBSync)
	assert.Equal(t, original.SyncIntervalSeconds, loaded.SyncIntervalSeconds)
	assert.Equal(t, original.LogLevel, loaded.LogLevel)
}

func TestEnvironmentOverride(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	cfg := &config.Config{
		ProjectID:  "yaml-proj",
		DatabaseID: "(default)",
		MachineID:  "yaml-machine",
		BrainDir:   "/tmp/brain",
	}
	err := config.SaveConfig(configPath, cfg)
	require.NoError(t, err)

	// Test primary AGY_SYNC prefix
	t.Setenv("AGY_SYNC_PROJECT_ID", "agy-env-override-proj")
	t.Setenv("AGY_SYNC_NO_DB_SYNC", "true")
	t.Setenv("AGY_SYNC_CONVERSATIONS_DIR", "/env/convs")
	t.Setenv("AGY_SYNC_SUMMARIES_DB", "/env/summaries.db")

	loaded, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "agy-env-override-proj", loaded.ProjectID)
	assert.True(t, loaded.NoDBSync)
	assert.Equal(t, "/env/convs", loaded.ConversationsDir)
	assert.Equal(t, "/env/summaries.db", loaded.SummariesDB)
}
