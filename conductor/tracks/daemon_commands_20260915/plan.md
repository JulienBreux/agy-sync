# Implementation Plan - Split watch command into start, stop, and enhanced status daemon commands

## Phase 1: Daemon Controller Package (`pkg/daemon`)
- [ ] Task: TDD - Daemon Controller Logic
  - [ ] Write unit tests for PID file read/write, stale PID detection, and process liveness checking in `pkg/daemon/daemon_test.go`
  - [ ] Implement `Daemon` manager in `pkg/daemon/daemon.go` with PID file tracking, log redirection, and SIGTERM process termination
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Daemon Controller Package' (Protocol in workflow.md)

## Phase 2: CLI Commands (`start`, `stop`, enhanced `status`) and Removal of `watch`
- [ ] Task: TDD - `start` and `stop` CLI Commands
  - [ ] Write unit tests in `cmd/start_test.go` and `cmd/stop_test.go`
  - [ ] Implement `cmd/start.go` with default background fork and `-f, --foreground` flag
  - [ ] Implement `cmd/stop.go` with graceful termination and PID cleanup
- [ ] Task: TDD - Enhanced `status` Command & `watch` Removal
  - [ ] Update `cmd/status.go` and `cmd/status_test.go` to report daemon state (status, PID, log path) and sync metrics in table/JSON
  - [ ] Remove `cmd/watch.go` and `cmd/watch_test.go`, unbinding from `cmd/root.go`
- [ ] Task: Conductor - User Manual Verification 'Phase 2: CLI Commands and Watch Removal' (Protocol in workflow.md)

## Phase 3: End-to-End Integration, Documentation & Verification
- [ ] Task: E2E Integration Test for Daemon Lifecycle
  - [ ] Implement lifecycle integration test in `test/daemon_lifecycle_test.go` (`start` -> check running -> `status` -> `stop` -> check dead)
- [ ] Task: Update Project Documentation
  - [ ] Update `README.md` to document `start`, `stop`, enhanced `status`, and remove references to `watch`
  - [ ] Verify `golangci-lint run ./...` and `CI=true go test -v -cover ./...`
- [ ] Task: Conductor - User Manual Verification 'Phase 3: End-to-End Integration, Documentation & Verification' (Protocol in workflow.md)
