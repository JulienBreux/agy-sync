# Implementation Plan: Sync Transactions Command and Audit Logging

## Phase 1: Local Transaction Storage Engine
- [x] Task: Design and Implement SQLite Transaction Store [6f48870]
    - [x] Define `Transaction`, `Direction`, `EntityType`, and `Filter` models in `internal/transaction/transaction.go`
    - [x] Write failing unit tests in `internal/transaction/store_test.go` for initialization, insertion, and query filtering
    - [x] Implement SQLite store in `internal/transaction/store.go` with auto-migration, indexes, and thread-safe read/write operations
- [x] Task: Conductor - User Manual Verification 'Phase 1: Local Transaction Storage Engine' (Protocol in workflow.md)

## Phase 2: Transaction Recording Integration in Syncer Engine
- [ ] Task: Wire Transaction Recorder into Syncer Engine
    - [ ] Add `TransactionsDB` path to `config.Config` with default `~/.config/agy-sync/transactions.db`
    - [ ] Inject `transaction.Store` into `syncer.Engine`
    - [ ] Write failing unit tests in `internal/syncer/push_test.go` and `internal/syncer/pull_test.go` verifying transaction events are recorded
    - [ ] Implement transaction emission during push (export conv, artifact, brain) and pull (import conv, artifact, brain)
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Transaction Recording Integration in Syncer Engine' (Protocol in workflow.md)

## Phase 3: CLI Command `agy-sync transactions`
- [ ] Task: Implement `transactions` Cobra Command
    - [ ] Write failing unit tests in `cmd/transactions_test.go` verifying flag filtering (`--direction`, `--in`, `--out`, `--type`, `--conv`, `--artifact`, `--brain`, `-c/--conversation`, `--limit`) and `--json`
    - [ ] Implement `cmd/transactions.go` with Cobra command definition, tabular ASCII formatter, and JSON serializer
    - [ ] Register `transactionsCmd` in `cmd/root.go`
- [ ] Task: Conductor - User Manual Verification 'Phase 3: CLI Command agy-sync transactions' (Protocol in workflow.md)

## Phase 4: Integration Testing, Quality Gates & Documentation
- [ ] Task: End-to-End Testing and Quality Assurance
    - [ ] Add end-to-end integration test in `test/e2e_test.go` verifying transactions recorded during full multi-machine push and pull roundtrip
    - [ ] Run full test suite with race detector (`go test -race ./...`), `go vet ./...`, and `golangci-lint run`
- [ ] Task: Documentation Update
    - [ ] Update `README.md` documenting `agy-sync transactions` command, filtering flags, and output formats
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Integration Testing, Quality Gates & Documentation' (Protocol in workflow.md)
