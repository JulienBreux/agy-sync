# Specification: Split watch command into start, stop, and enhanced status daemon commands

## 1. Overview
Refactor the background synchronization lifecycle in `agy-sync`. Remove the foreground-only `watch` command and replace it with proper daemon lifecycle management:
- `agy-sync start`: Launches the sync engine in the background as a detached daemon process (or in the foreground with `--foreground` / `-f`).
- `agy-sync stop`: Gracefully stops the running background daemon via PID file lookup and POSIX signal (SIGTERM).
- `agy-sync status`: Enhanced to report both daemon process health (running status, PID, uptime, log path) and Firestore conversation synchronization statistics.

## 2. Functional Requirements

### 2.1 Daemon Management Package (`pkg/daemon`)
- **PID File Management:** Maintain a PID file at `~/.config/agy-sync/agy-sync.pid` (or configurable). Ensure stale PID cleanup if process crashed or is no longer alive.
- **Log Management:** Redirect background daemon stdout and stderr to `~/.config/agy-sync/daemon.log`.
- **Process Liveness Detection:** Safely probe whether a PID is active using signal 0 check (`syscall.Kill(pid, 0)`).
- **Graceful Termination:** Send SIGTERM to the recorded PID, wait for graceful exit with timeout (e.g. 5 seconds), and fallback to SIGKILL if unresponsive. Clean up the PID file upon termination.

### 2.2 CLI Commands
1. **`agy-sync start`:**
   - Default behavior: Forks/executes a background child process detached from the current shell, writes the child PID to the PID file, redirects output to `daemon.log`, and prints confirmation with PID and log location.
   - If daemon is already running: prints warning and exits with non-zero or informative error.
   - Flag: `-f, --foreground`: Runs the watcher engine directly in the current terminal session without detaching.
   - Flags: `--interval <duration>` (default 3s), `--debounce <duration>` (default 200ms).
2. **`agy-sync stop`:**
   - Reads PID from the PID file.
   - Verifies process liveness.
   - Sends SIGTERM, waits for shutdown, removes PID file, and prints confirmation.
   - If daemon is not running: reports that daemon is not active.
3. **`agy-sync status`:**
   - Enriched report:
     - **Daemon Status:** State (`RUNNING` / `STOPPED`), PID, and Daemon Log path.
     - **Sync Status:** Existing local vs remote step and artifact counts.
   - Full `--json` support including daemon metadata block.
4. **Removal of `watch`:**
   - Remove `cmd/watch.go` and `cmd/watch_test.go` from Cobra root.
   - Replace documentation references from `watch` to `start` / `stop`.

## 3. Acceptance Criteria
- [ ] `agy-sync start` spawns a background sync process, writes PID to `agy-sync.pid`, and logs to `daemon.log`.
- [ ] `agy-sync start -f` runs in foreground directly.
- [ ] `agy-sync stop` terminates running daemon cleanly and cleans up PID file.
- [ ] `agy-sync status` displays daemon process state + sync state in both terminal table and `--json` format.
- [ ] `agy-sync watch` is completely removed.
- [ ] Unit and integration tests cover daemon process management, CLI flags, signal handling, and status reporting with >80% coverage.
