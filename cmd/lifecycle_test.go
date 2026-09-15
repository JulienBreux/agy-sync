package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/internal/daemon"
	"github.com/julienbreux/agy-sync/internal/firestore"
)

func TestE2E_DaemonLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	pidFile := filepath.Join(tempDir, "daemon.pid")
	logFile := filepath.Join(tempDir, "daemon.log")

	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-lifecycle-proj"
machine_id: "test-lifecycle-machine"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	mgr := daemon.NewManager(pidFile, logFile)

	// Step 1: Initial status should report STOPPED
	rootStatusInit := cmd.NewRootCommand()
	bufStatusInit := new(bytes.Buffer)
	rootStatusInit.SetOut(bufStatusInit)
	rootStatusInit.SetErr(bufStatusInit)
	rootStatusInit.SetArgs([]string{
		"status",
		"--config", configPath,
		"--pid-file", pidFile,
		"--log-file", logFile,
		"--json",
	})
	require.NoError(t, rootStatusInit.Execute())

	var statusReportInit cmd.StatusReport
	require.NoError(t, json.Unmarshal(bufStatusInit.Bytes(), &statusReportInit))
	assert.Equal(t, "STOPPED", statusReportInit.Daemon.State)
	assert.Equal(t, 0, statusReportInit.Daemon.PID)

	// Step 2: Start foreground daemon with context timeout
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	rootStart := cmd.NewRootCommand()
	bufStart := new(bytes.Buffer)
	rootStart.SetOut(bufStart)
	rootStart.SetErr(bufStart)
	rootStart.SetArgs([]string{
		"start",
		"-f",
		"--config", configPath,
		"--pid-file", pidFile,
		"--log-file", logFile,
		"--interval", "50ms",
		"--debounce", "20ms",
	})

	err := rootStart.ExecuteContext(ctx)
	require.NoError(t, err)
	assert.Contains(t, bufStart.String(), "Starting real-time synchronization daemon")
	assert.Contains(t, bufStart.String(), "Shutting down synchronization daemon")

	// Step 3: Verify clean shutdown cleanup
	running, pid, err := mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, pid)

	// Step 4: Run stop on stopped daemon (idempotent, safe)
	rootStop := cmd.NewRootCommand()
	bufStop := new(bytes.Buffer)
	rootStop.SetOut(bufStop)
	rootStop.SetErr(bufStop)
	rootStop.SetArgs([]string{
		"stop",
		"--pid-file", pidFile,
		"--log-file", logFile,
	})
	require.NoError(t, rootStop.Execute())
	assert.Contains(t, bufStop.String(), "not running")
}
