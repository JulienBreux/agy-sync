# Implementation Plan - Modernize Go Application Architecture, CLI Ergonomics & Best Practices

## Phase 1: Codebase Package Restructuring & Encapsulation
- [x] Task: TDD - Reorganize domain packages into `internal/`
  - [x] Relocate `pkg/daemon`, `pkg/syncer`, `pkg/watcher`, `pkg/parser`, `pkg/discovery`, and `pkg/firestore` into `internal/`
  - [x] Update all import paths across `cmd/`, `internal/`, and `test/`
  - [x] Validate full test suite passes with reorganized package structure
- [x] Task: Conductor - User Manual Verification 'Phase 1: Codebase Package Restructuring & Encapsulation' (Protocol in workflow.md)

## Phase 2: Structured Logging (`log/slog`) & Concurrency Modernization
- [x] Task: TDD - Implement Structured Logging (`internal/logger`)
  - [x] Write unit tests for logger configuration, log level parsing, and text/JSON handler selection in `internal/logger/logger_test.go`
  - [x] Implement `internal/logger/logger.go` wrapping `log/slog`
  - [x] Inject structured logging into `syncer`, `watcher`, `daemon`, and `cmd/`
- [x] Task: TDD - Context & Concurrency Modernization
  - [x] Refactor artifact syncing in `internal/syncer` to use `golang.org/x/sync/errgroup` with bounded parallelism
  - [x] Adopt `signal.NotifyContext` in CLI execution for graceful interruption
- [x] Task: Conductor - User Manual Verification 'Phase 2: Structured Logging & Concurrency Modernization' (Protocol in workflow.md)

## Phase 3: CLI Ergonomics, Version Command, Grouping & Exit Codes
- [x] Task: TDD - Standardized Exit Codes & `version` Command
  - [x] Define semantic exit codes in `internal/exitcode/exitcode.go` and tests
  - [x] Implement `cmd/version.go` with build metadata (`debug.ReadBuildInfo()` & ldflags) and tests in `cmd/version_test.go`
- [x] Task: TDD - Cobra Command Grouping, Help UX & Status Last Polling Date
  - [x] Add Cobra command groups: Daemon Management, Data Synchronization, Configuration & Setup
  - [x] Record last polling date in daemon/sync state and display it in `agy-sync status` output (text and JSON)
  - [x] Verify help text grouping and autocompletion output
- [x] Task: Conductor - User Manual Verification 'Phase 3: CLI Ergonomics, Version Command, Grouping & Exit Codes' (Protocol in workflow.md)

## Phase 4: Full System Verification & Documentation Update
- [x] Task: Full System Validation & Documentation Sync
  - [x] Verify `make lint`, `make test`, and `make build` pass cleanly
  - [x] Update `README.md` to document the new `version` command and grouped help layout
- [~] Task: Conductor - User Manual Verification 'Phase 4: Full System Verification & Documentation Update' (Protocol in workflow.md)
