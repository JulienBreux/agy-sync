package cmd_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/daemon"
)

func TestStopCommand_NotRunning(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{
		"stop",
		"--pid-file", pidFile,
		"--log-file", logFile,
	})

	err := root.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "not running")
}

func TestStopCommand_Success(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	// Start a long-running process to stop
	child := exec.Command("sleep", "30")
	require.NoError(t, child.Start())

	mgr := daemon.NewManager(pidFile, logFile)
	require.NoError(t, mgr.WritePID(child.Process.Pid))

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{
		"stop",
		"--pid-file", pidFile,
		"--log-file", logFile,
	})

	err := root.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "stopped successfully")

	running, _, err := mgr.IsRunning()
	assert.NoError(t, err)
	assert.False(t, running)
}
