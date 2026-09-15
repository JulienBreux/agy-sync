# Implementation Plan - Split watch command into start, stop, and enhanced status daemon commands

## Phase 1: Daemon Controller Package (`pkg/daemon`) [checkpoint: bea18e2]
- [x] Task: TDD - Daemon Controller Logic [bea18e2]
  - [x] Write unit tests for PID file read/write, stale PID detection, and process liveness checking in `pkg/daemon/daemon_test.go`
  - [x] Implement `Daemon` manager in `pkg/daemon/daemon.go` with PID file tracking, log redirection, and SIGTERM process termination
- [x] Task: Conductor - User Manual Verification 'Phase 1: Daemon Controller Package' (Protocol in workflow.md) [bea18e2]

## Phase 2: CLI Commands (`start`, `stop`, enhanced `status`) and Removal of `watch` [checkpoint: 87a352f]
- [x] Task: TDD - `start` and `stop` CLI Commands [87a352f]
  - [x] Write unit tests in `cmd/start_test.go` and `cmd/stop_test.go`
  - [x] Implement `cmd/start.go` with default background fork and `-f, --foreground` flag
  - [x] Implement `cmd/stop.go` with graceful termination and PID cleanup
- [x] Task: TDD - Enhanced `status` Command & `watch` Removal [87a352f]
  - [x] Update `cmd/status.go` and `cmd/status_test.go` to report daemon state (status, PID, log path) and sync metrics in table/JSON
  - [x] Remove `cmd/watch.go` and `cmd/watch_test.go`, unbinding from `cmd/root.go`
- [x] Task: Conductor - User Manual Verification 'Phase 2: CLI Commands and Watch Removal' (Protocol in workflow.md) [87a352f]

## Phase 3: End-to-End Integration, Documentation & Verification [checkpoint: efa5453]
- [x] Task: E2E Integration Test for Daemon Lifecycle [efa5453]
  - [x] Implement lifecycle integration test in `cmd/lifecycle_test.go` (`start` -> check running -> `status` -> `stop` -> check dead)
- [x] Task: Update Project Documentation [efa5453]
  - [x] Update `README.md` to document `start`, `stop`, enhanced `status`, and remove references to `watch`
  - [x] Verify `golangci-lint run ./...` and `CI=true go test -v -cover ./...`
- [x] Task: Conductor - User Manual Verification 'Phase 3: End-to-End Integration, Documentation & Verification' (Protocol in workflow.md) [efa5453]

