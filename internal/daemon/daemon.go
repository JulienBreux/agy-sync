package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrNotRunning is returned when attempting an operation on a stopped daemon.
	ErrNotRunning = errors.New("daemon is not running")

	// ErrAlreadyRunning is returned when attempting to start a daemon that is already active.
	ErrAlreadyRunning = errors.New("daemon is already running")
)

// State represents the runtime state of the daemon.
type State string

const (
	// StateRunning indicates the daemon process is active and responding.
	StateRunning State = "RUNNING"

	// StateStopped indicates the daemon is not running.
	StateStopped State = "STOPPED"
)

// Manager handles the lifecycle, PID tracking, and log redirection of the agy-sync daemon.
type Manager struct {
	PIDFile string
	LogFile string
}

// DefaultPIDPath returns the standard location for the agy-sync daemon PID file.
func DefaultPIDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "agy-sync.pid"
	}
	return filepath.Join(home, ".config", "agy-sync", "agy-sync.pid")
}

// DefaultLogPath returns the standard location for the background daemon logs.
func DefaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "daemon.log"
	}
	return filepath.Join(home, ".config", "agy-sync", "daemon.log")
}

// NewManager creates a new daemon Manager. If empty paths are provided, standard defaults are used.
func NewManager(pidFile, logFile string) *Manager {
	if pidFile == "" {
		pidFile = DefaultPIDPath()
	}
	if logFile == "" {
		logFile = DefaultLogPath()
	}
	return &Manager{
		PIDFile: pidFile,
		LogFile: logFile,
	}
}

// IsRunning checks whether the daemon process is active.
// If the PID file exists but points to a dead process, it automatically removes the stale PID file.
func (m *Manager) IsRunning() (bool, int, error) {
	data, err := os.ReadFile(m.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read PID file %s: %w", m.PIDFile, err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		_ = os.Remove(m.PIDFile)
		return false, 0, nil
	}

	alive := isProcessAlive(pid)
	if !alive {
		// Clean up stale PID file
		_ = os.Remove(m.PIDFile)
		return false, 0, nil
	}

	return true, pid, nil
}

// WritePID persists the specified process ID to the PID file.
func (m *Manager) WritePID(pid int) error {
	dir := filepath.Dir(m.PIDFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for PID file %s: %w", dir, err)
	}
	data := fmt.Sprintf("%d\n", pid)
	return os.WriteFile(m.PIDFile, []byte(data), 0o644)
}

// RemovePID deletes the PID file if it exists.
func (m *Manager) RemovePID() error {
	if err := os.Remove(m.PIDFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file %s: %w", m.PIDFile, err)
	}
	return nil
}

// StartBackground launches the given executable in detached background mode,
// redirecting standard output and error to the configured log file.
func (m *Manager) StartBackground(binPath string, args []string) (int, error) {
	running, pid, err := m.IsRunning()
	if err != nil {
		return 0, err
	}
	if running {
		return pid, ErrAlreadyRunning
	}

	logDir := filepath.Dir(m.LogFile)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	logF, err := os.OpenFile(m.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file %s: %w", m.LogFile, err)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil

	// Set process group / detachment where possible
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return 0, fmt.Errorf("failed to start background daemon: %w", err)
	}

	childPID := cmd.Process.Pid
	if err := m.WritePID(childPID); err != nil {
		_ = cmd.Process.Kill()
		_ = logF.Close()
		return 0, fmt.Errorf("failed to record PID: %w", err)
	}

	_ = logF.Close()
	return childPID, nil
}

// Stop sends SIGTERM to the running daemon and waits up to timeout for it to exit gracefully.
// If the process remains running after timeout, SIGKILL is issued.
func (m *Manager) Stop(timeout time.Duration) error {
	running, pid, err := m.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		return ErrNotRunning
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = m.RemovePID()
		return nil
	}

	// Send SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			_ = m.RemovePID()
			return nil
		}
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", pid, err)
	}

	// Wait up to timeout for process to terminate
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			_ = m.RemovePID()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// If still alive after timeout, send SIGKILL
	if isProcessAlive(pid) {
		_ = proc.Signal(syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
	}

	_ = m.RemovePID()
	return nil
}

// Status returns the current runtime state, PID, and log path.
func (m *Manager) Status() (State, int, string, error) {
	running, pid, err := m.IsRunning()
	if err != nil {
		return StateStopped, 0, m.LogFile, err
	}
	if running {
		return StateRunning, pid, m.LogFile, nil
	}
	return StateStopped, 0, m.LogFile, nil
}

// isProcessAlive returns true if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
