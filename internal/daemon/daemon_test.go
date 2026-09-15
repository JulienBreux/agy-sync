package daemon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/daemon"
)

func TestDefaultPaths(t *testing.T) {
	pidPath := daemon.DefaultPIDPath()
	assert.NotEmpty(t, pidPath)
	assert.Equal(t, "agy-sync.pid", filepath.Base(pidPath))

	logPath := daemon.DefaultLogPath()
	assert.NotEmpty(t, logPath)
	assert.Equal(t, "daemon.log", filepath.Base(logPath))
}

func TestPIDFile_ReadWriteRemove(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Initially not running
	running, pid, err := mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, pid)

	// Write PID of current running test process
	currentPID := os.Getpid()
	err = mgr.WritePID(currentPID)
	require.NoError(t, err)

	// Now should report running
	running, pid, err = mgr.IsRunning()
	require.NoError(t, err)
	assert.True(t, running)
	assert.Equal(t, currentPID, pid)

	// Remove PID
	err = mgr.RemovePID()
	require.NoError(t, err)

	running, _, err = mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)
}

func TestPIDFile_StalePIDCleanup(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test_stale.pid")
	logFile := filepath.Join(tempDir, "test_stale.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Spawn a short-lived process and get its PID
	cmd := exec.Command("sleep", "0.01")
	require.NoError(t, cmd.Start())
	deadPID := cmd.Process.Pid
	_ = cmd.Wait() // wait until dead

	// Write dead PID to pid file
	require.NoError(t, mgr.WritePID(deadPID))

	// IsRunning should detect that the process is dead and clean up the stale PID file
	running, _, err := mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)

	// Stale PID file should have been removed
	assert.NoFileExists(t, pidFile)
}

func TestDaemon_StopRunningProcess(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test_stop.pid")
	logFile := filepath.Join(tempDir, "test_stop.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Start a sleep process to simulate background daemon
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	childPID := cmd.Process.Pid

	if err := mgr.WritePID(childPID); err != nil {
		_ = cmd.Process.Kill()
		require.NoError(t, err)
	}

	// Stop with 2s timeout
	err := mgr.Stop(2 * time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		require.NoError(t, err)
	}

	// Verify process is stopped
	running, _, err := mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)

	// Verify PID file removed
	assert.NoFileExists(t, pidFile)
}

func TestDaemon_StopWhenNotRunning(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "nonexistent.pid")
	logFile := filepath.Join(tempDir, "nonexistent.log")

	mgr := daemon.NewManager(pidFile, logFile)

	err := mgr.Stop(1 * time.Second)
	assert.ErrorIs(t, err, daemon.ErrNotRunning)
}

func TestDaemon_StartBackgroundAndStatus(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "bg.pid")
	logFile := filepath.Join(tempDir, "bg.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Status initially stopped
	state, pid, logPath, err := mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, daemon.StateStopped, state)
	assert.Equal(t, 0, pid)
	assert.Equal(t, logFile, logPath)

	// Start sleep 30 as background process
	childPID, err := mgr.StartBackground("sleep", []string{"30"})
	require.NoError(t, err)
	assert.Positive(t, childPID)

	// Test starting again returns ErrAlreadyRunning
	_, err = mgr.StartBackground("sleep", []string{"30"})
	require.ErrorIs(t, err, daemon.ErrAlreadyRunning)

	// Status now running
	state, pid, _, err = mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, daemon.StateRunning, state)
	assert.Equal(t, childPID, pid)

	// Stop background process
	require.NoError(t, mgr.Stop(2*time.Second))

	// Status after stop
	state, pid, _, err = mgr.Status()
	require.NoError(t, err)
	assert.Equal(t, daemon.StateStopped, state)
	assert.Equal(t, 0, pid)
}

func TestPIDFile_CorruptedPID(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "corrupted.pid")
	logFile := filepath.Join(tempDir, "corrupted.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Write invalid string into PID file
	require.NoError(t, os.WriteFile(pidFile, []byte("invalid-pid-not-int\n"), 0o644))

	running, pid, err := mgr.IsRunning()
	require.NoError(t, err)
	assert.False(t, running)
	assert.Equal(t, 0, pid)

	// Corrupted PID file should be cleaned up
	assert.NoFileExists(t, pidFile)
}

func TestRuntimeState_RecordAndGetPoll(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")
	stateFile := filepath.Join(tempDir, "test.state.json")

	mgr := daemon.NewManagerWithState(pidFile, logFile, stateFile)

	// Initially empty state
	state, err := mgr.GetRuntimeState()
	require.NoError(t, err)
	assert.Nil(t, state.LastPolledAt)

	pollTime := time.Date(2026, 9, 15, 16, 40, 0, 0, time.UTC)
	require.NoError(t, mgr.RecordPoll(pollTime))

	state, err = mgr.GetRuntimeState()
	require.NoError(t, err)
	require.NotNil(t, state.LastPolledAt)
	assert.True(t, state.LastPolledAt.Equal(pollTime))
}
