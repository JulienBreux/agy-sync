package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/daemon"
	"github.com/julienbreux/agy-sync/pkg/firestore"
)

func TestStartCommand_MissingConfig(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentConfig := filepath.Join(tempDir, "missing.yaml")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"start", "-f", "--config", nonExistentConfig})
	err := root.Execute()

	assert.Error(t, err)
}

func TestStartCommand_ForegroundRun(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-start-proj"
machine_id: "test-start-box"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{
		"start",
		"-f",
		"--config", configPath,
		"--pid-file", pidFile,
		"--log-file", logFile,
		"--interval", "50ms",
		"--debounce", "20ms",
	})

	err := root.ExecuteContext(ctx)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Starting real-time synchronization daemon")

	// PID file should be cleaned up after foreground shutdown
	mgr := daemon.NewManager(pidFile, logFile)
	running, _, err := mgr.IsRunning()
	assert.NoError(t, err)
	assert.False(t, running)
}

func TestStartCommand_AlreadyRunning(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-start-proj"
machine_id: "test-start-box"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	mgr := daemon.NewManager(pidFile, logFile)
	require.NoError(t, mgr.WritePID(os.Getpid()))

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{
		"start",
		"--config", configPath,
		"--pid-file", pidFile,
		"--log-file", logFile,
	})

	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}
