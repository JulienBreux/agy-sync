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
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Syncer Engine Clear Operation' (Protocol in workflow.md)

## Phase 3: CLI Command & Confirmation Mechanism
- [ ] Task: Add `agy-sync clear` Command
    - [ ] Write failing tests in `cmd/clear_test.go` verifying interactive abort (answering "n" or typing unexpected input), interactive approval (answering "y"), and `--force` flag execution
    - [ ] Implement `cmd/clear.go` with Cobra command definition, interactive confirmation prompt reading from `os.Stdin`, `--force` / `-f`, `--conversation` / `-c`, and `--json` support
    - [ ] Register `clearCmd` in `cmd/root.go`
- [ ] Task: Conductor - User Manual Verification 'Phase 3: CLI Command & Confirmation Mechanism' (Protocol in workflow.md)

## Phase 4: Integration Testing, Quality Gates & Documentation
- [ ] Task: End-to-End Test and Verification
    - [ ] Add an end-to-end test in `test/e2e_test.go` verifying full database clear followed by verification that local files are untouched and Firestore is empty
    - [ ] Run `go test -race ./...`, `golangci-lint run`, and `go vet ./...` ensuring 0 warnings
- [ ] Task: Documentation Update
    - [ ] Update `README.md` with documentation for `agy-sync clear`, available flags, and confirmation safety behavior
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Integration Testing, Quality Gates & Documentation' (Protocol in workflow.md)
