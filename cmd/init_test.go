package cmd_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/pkg/config"
)

func TestRootCommand(t *testing.T) {
	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "agy-sync")
}

func TestInitCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"init",
		"--config", configPath,
		"--project-id", "my-test-project",
		"--database-id", "(default)",
		"--machine-id", "test-box",
		"--brain-dir", tempDir,
	})

	err := root.Execute()
	require.NoError(t, err)

	assert.FileExists(t, configPath)
	loaded, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "my-test-project", loaded.ProjectID)
	assert.Equal(t, "(default)", loaded.DatabaseID)
	assert.Equal(t, "test-box", loaded.MachineID)
	assert.Equal(t, tempDir, loaded.BrainDir)
}

func TestInitCommand_MissingProject(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"init",
		"--config", configPath,
		"--machine-id", "test-box",
	})

	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project-id is required")
}
