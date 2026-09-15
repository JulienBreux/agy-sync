package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "(default)", cfg.DatabaseID)
	assert.NotEmpty(t, cfg.MachineID)
	assert.NotEmpty(t, cfg.BrainDir)
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
			SyncIntervalSeconds: 2,
			LogLevel:            "INFO",
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing project ID", func(t *testing.T) {
		cfg := &config.Config{
			DatabaseID: "(default)",
			MachineID:  "macbook-pro-1",
			BrainDir:   "/tmp/brain",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project_id is required")
	})

	t.Run("missing machine ID", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:  "test-gcp-project",
			DatabaseID: "(default)",
			MachineID:  "",
			BrainDir:   "/tmp/brain",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "machine_id is required")
	})

	t.Run("missing brain dir", func(t *testing.T) {
		cfg := &config.Config{
			ProjectID:  "test-gcp-project",
			DatabaseID: "(default)",
			MachineID:  "macbook-pro-1",
			BrainDir:   "",
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "brain_dir is required")
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

	t.Setenv("AYG_SYNC_PROJECT_ID", "env-override-proj")

	loaded, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "env-override-proj", loaded.ProjectID)
}
