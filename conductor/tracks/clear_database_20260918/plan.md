# Implementation Plan: Clear Firestore Database

## Phase 1: Firestore Repository Deletion Capabilities
- [x] Task: Implement In-Memory & Interface Deletion Methods [72e4f1d]
    - [x] Add `DeleteConversation(ctx context.Context, convID string) error` and `ClearAll(ctx context.Context) error` to `firestore.Repository`
    - [x] Write failing unit tests in `internal/firestore/memory_test.go` for in-memory conversation and subcollection deletion
    - [x] Implement `DeleteConversation` and `ClearAll` on `MemoryRepository` in `internal/firestore/memory.go`
- [x] Task: Implement Production Firestore Client Deletion [399135e]
    - [x] Write failing tests/mocks in `internal/firestore/client_test.go` verifying batch/recursive subcollection deletion
    - [x] Implement `DeleteConversation` and `ClearAll` in `internal/firestore/client.go` using Firestore batch deletes
- [x] Task: Conductor - User Manual Verification 'Phase 1: Firestore Repository Deletion Capabilities' (Protocol in workflow.md)

## Phase 2: Syncer Engine Clear Operation
- [x] Task: Implement Syncer Clear Logic [60dfaf8]
    - [x] Write failing unit tests in `internal/syncer/clear_test.go` covering full clear and single-conversation clear
    - [x] Implement `Clear(ctx context.Context, opts ClearOptions) (*ClearResult, error)` on `syncer.Engine` in `internal/syncer/clear.go`
- [x] Task: Conductor - User Manual Verification 'Phase 2: Syncer Engine Clear Operation' (Protocol in workflow.md)

## Phase 3: CLI Command & Confirmation Mechanism
- [x] Task: Add `agy-sync clear` Command [a03621e]
    - [x] Write failing tests in `cmd/clear_test.go` verifying interactive abort (answering "n" or typing unexpected input), interactive approval (answering "y"), and `--force` flag execution
    - [x] Implement `cmd/clear.go` with Cobra command definition, interactive confirmation prompt reading from `os.Stdin`, `--force` / `-f`, `--conversation` / `-c`, and `--json` support
    - [x] Register `clearCmd` in `cmd/root.go`
- [x] Task: Conductor - User Manual Verification 'Phase 3: CLI Command & Confirmation Mechanism' (Protocol in workflow.md)

## Phase 4: Integration Testing, Quality Gates & Documentation
- [x] Task: End-to-End Test and Verification [1dec7e3]
    - [x] Add end-to-end integration test in `test/e2e_test.go` verifying `agy-sync clear` with live Firestore emulator
    - [x] Run full test suite with race detector (`go test -race ./...`) and `golangci-lint run`, and `go vet ./...` ensuring 0 warnings
- [x] Task: Documentation Update [12565b0]
    - [x] Update `README.md` with documentation for `agy-sync clear`, available flags, and confirmation safety behavior
- [x] Task: Conductor - User Manual Verification 'Phase 4: Integration Testing, Quality Gates & Documentation' (Protocol in workflow.md)
