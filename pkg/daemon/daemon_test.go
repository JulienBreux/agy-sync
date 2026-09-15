package daemon_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/julienbreux/agy-sync/pkg/daemon"
)

func TestDefaultPaths(t *testing.T) {
	pidPath := daemon.DefaultPIDPath()
	if pidPath == "" {
		t.Fatal("expected non-empty default PID path")
	}
	if filepath.Base(pidPath) != "agy-sync.pid" {
		t.Errorf("expected pid filename agy-sync.pid, got %s", filepath.Base(pidPath))
	}

	logPath := daemon.DefaultLogPath()
	if logPath == "" {
		t.Fatal("expected non-empty default log path")
	}
	if filepath.Base(logPath) != "daemon.log" {
		t.Errorf("expected log filename daemon.log, got %s", filepath.Base(logPath))
	}
}

func TestPIDFile_ReadWriteRemove(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")
	logFile := filepath.Join(tempDir, "test.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Initially not running
	running, pid, err := mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error checking status: %v", err)
	}
	if running || pid != 0 {
		t.Errorf("expected not running, got running=%v, pid=%d", running, pid)
	}

	// Write PID of current running test process
	currentPID := os.Getpid()
	if err := mgr.WritePID(currentPID); err != nil {
		t.Fatalf("failed to write pid: %v", err)
	}

	// Now should report running
	running, pid, err = mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running || pid != currentPID {
		t.Errorf("expected running=%v pid=%d, got running=%v pid=%d", true, currentPID, running, pid)
	}

	// Remove PID
	if err := mgr.RemovePID(); err != nil {
		t.Fatalf("failed to remove pid: %v", err)
	}

	running, _, err = mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected not running after RemovePID")
	}
}

func TestPIDFile_StalePIDCleanup(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test_stale.pid")
	logFile := filepath.Join(tempDir, "test_stale.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Spawn a short-lived process and get its PID
	cmd := exec.Command("sleep", "0.01")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to spawn helper process: %v", err)
	}
	deadPID := cmd.Process.Pid
	_ = cmd.Wait() // wait until dead

	// Write dead PID to pid file
	if err := mgr.WritePID(deadPID); err != nil {
		t.Fatalf("failed to write pid: %v", err)
	}

	// IsRunning should detect that the process is dead and clean up the stale PID file
	running, pid, err := mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Errorf("expected dead process to report not running, got pid=%d", pid)
	}

	// Stale PID file should have been removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected stale PID file to be automatically removed")
	}
}

func TestDaemon_StopRunningProcess(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test_stop.pid")
	logFile := filepath.Join(tempDir, "test_stop.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Start a sleep process to simulate background daemon
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	childPID := cmd.Process.Pid

	if err := mgr.WritePID(childPID); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to write pid: %v", err)
	}

	// Stop with 2s timeout
	if err := mgr.Stop(2 * time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to stop daemon: %v", err)
	}

	// Verify process is stopped
	running, _, err := mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error checking status: %v", err)
	}
	if running {
		t.Error("expected process to be stopped")
	}

	// Verify PID file removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed after Stop")
	}
}

func TestDaemon_StopWhenNotRunning(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "nonexistent.pid")
	logFile := filepath.Join(tempDir, "nonexistent.log")

	mgr := daemon.NewManager(pidFile, logFile)

	err := mgr.Stop(1 * time.Second)
	if err == nil {
		t.Error("expected error when stopping a non-running daemon")
	}
	if err != daemon.ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}
}

func TestDaemon_StartBackgroundAndStatus(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "bg.pid")
	logFile := filepath.Join(tempDir, "bg.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Status initially stopped
	state, pid, logPath, err := mgr.Status()
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if state != daemon.StateStopped || pid != 0 || logPath != logFile {
		t.Errorf("expected STOPPED with pid 0, got %s, pid %d, log %s", state, pid, logPath)
	}

	// Start sleep 30 as background process
	childPID, err := mgr.StartBackground("sleep", []string{"30"})
	if err != nil {
		t.Fatalf("failed to start background daemon: %v", err)
	}
	if childPID <= 0 {
		t.Errorf("expected positive child PID, got %d", childPID)
	}

	// Test starting again returns ErrAlreadyRunning
	_, err = mgr.StartBackground("sleep", []string{"30"})
	if err != daemon.ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	// Status now running
	state, pid, _, err = mgr.Status()
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if state != daemon.StateRunning || pid != childPID {
		t.Errorf("expected RUNNING with pid %d, got %s, pid %d", childPID, state, pid)
	}

	// Stop background process
	if err := mgr.Stop(2 * time.Second); err != nil {
		t.Fatalf("failed to stop daemon: %v", err)
	}

	// Status after stop
	state, pid, _, err = mgr.Status()
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if state != daemon.StateStopped || pid != 0 {
		t.Errorf("expected STOPPED after stop, got %s, pid %d", state, pid)
	}
}

func TestPIDFile_CorruptedPID(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "corrupted.pid")
	logFile := filepath.Join(tempDir, "corrupted.log")

	mgr := daemon.NewManager(pidFile, logFile)

	// Write invalid string into PID file
	if err := os.WriteFile(pidFile, []byte("invalid-pid-not-int\n"), 0o644); err != nil {
		t.Fatalf("failed to write corrupted pid: %v", err)
	}

	running, pid, err := mgr.IsRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running || pid != 0 {
		t.Errorf("expected not running, got running=%v, pid=%d", running, pid)
	}

	// Corrupted PID file should be cleaned up
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected corrupted PID file to be automatically removed")
	}
}


