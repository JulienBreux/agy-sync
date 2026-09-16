# Implementation Plan: Antigravity SQLite Database Reconstruction & Synchronization

## Phase 1: SQLite Storage Driver & Configuration
- [x] Task: Add SQLite dependency and configuration fields [e0ea5e1]
    - [x] Add `modernc.org/sqlite` pure-Go driver dependency to `go.mod`
    - [x] Write failing unit tests in `pkg/config/config_test.go` for `ConversationsDir`, `SummariesDB`, and `NoDBSync`
    - [x] Implement configuration defaults and validation in `pkg/config/config.go`
- [x] Task: Conductor - User Manual Verification 'Phase 1: SQLite Storage Driver & Configuration' (Protocol in workflow.md)

## Phase 2: Conversation & Summary SQLite Reconstructor Package
- [ ] Task: Implement `internal/reconstructor` for individual conversation databases (`conversations/<id>.db`)
    - [ ] Write failing unit tests for schema creation and step insertion in `internal/reconstructor/conversation_test.go`
    - [ ] Implement `ReconstructConversationDB` with WAL mode, busy timeout, table creation, and step upserts
- [ ] Task: Implement `internal/reconstructor` for global summaries (`conversation_summaries.db`)
    - [ ] Write failing unit tests for summary upserts and metadata parsing in `internal/reconstructor/summary_test.go`
    - [ ] Implement `UpsertSummary` with title extraction and workspace URI preservation
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Conversation & Summary SQLite Reconstructor Package' (Protocol in workflow.md)

## Phase 3: Integration into Pull, Syncer Listener & CLI Commands
- [ ] Task: Integrate Reconstructor into Engine Pull & Syncer Listener
    - [ ] Write failing tests in `internal/syncer/pull_test.go` and `internal/syncer/listener_test.go` verifying SQLite DB creation on pull
    - [ ] Wire reconstructor into `Pull` and `OnConversationUpdated` respecting `NoDBSync`
- [ ] Task: Add CLI flags and update Status command
    - [ ] Add `--no-db-sync`, `--conversations-dir`, `--summaries-db` flags to `cmd/pull.go` and `cmd/start.go`
    - [ ] Update `cmd/status.go` to display SQLite database sync status and test status output
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Integration into Pull, Syncer Listener & CLI Commands' (Protocol in workflow.md)

## Phase 4: Modern Go Quality, Idiomatic Standards & Linting
- [ ] Task: Ensure Modern Go Idioms, Context Propagation & Structured Logging
    - [ ] Audit reconstructor and syncer changes for modern Go practices (`log/slog`, `context.Context`, `errors.Join` / `%w`, `slices`/`maps`)
    - [ ] Run `golangci-lint run` and address all static analysis and lint warnings
    - [ ] Run `go fmt ./...` and `go vet ./...`
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Modern Go Quality, Idiomatic Standards & Linting' (Protocol in workflow.md)

## Phase 5: End-to-End Testing & Documentation
- [ ] Task: End-to-End Test and Documentation
    - [ ] Extend `test/e2e_test.go` to verify full multi-machine pull restores both `brain/` and `conversations/<id>.db`
    - [ ] Update `README.md` and documentation with SQLite synchronization details
- [ ] Task: Conductor - User Manual Verification 'Phase 5: End-to-End Testing & Documentation' (Protocol in workflow.md)
