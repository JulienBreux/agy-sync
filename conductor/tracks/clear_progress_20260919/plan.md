# Implementation Plan: Display Progress for Database Clear Operation

## Phase 1: Syncer Engine Progress Event Architecture
- [ ] Task: Design and Implement Clear Progress Events in Syncer Engine
    - [ ] Define `ClearProgressEvent` and `ClearProgressFunc` types in `internal/syncer/clear.go`
    - [ ] Write failing unit tests in `internal/syncer/clear_test.go` verifying progress callbacks fire across discovery, deletion, and completion phases
    - [ ] Implement event dispatching in `Engine.Clear` for all-conversation and single-conversation deletion
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Syncer Engine Progress Event Architecture' (Protocol in workflow.md)

## Phase 2: Terminal Progress Display & Log Muting
- [ ] Task: Implement Terminal Progress Component
    - [ ] Implement `ProgressIndicator` in `internal/ui/progress.go` with support for dynamic spinners, progress counters, TTY detection, and non-TTY plain line printing
    - [ ] Write unit tests in `internal/ui/progress_test.go` testing TTY and non-TTY rendering modes
- [ ] Task: Implement Command Logger Muting
    - [ ] Add capability in `cmd/clear.go` to mute INFO-level log output during progress rendering while preserving output for `-v, --verbose` or `--log-level=debug`
    - [ ] Write tests verifying log filtering behavior during clear operations
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Terminal Progress Display & Log Muting' (Protocol in workflow.md)

## Phase 3: CLI Command Integration
- [ ] Task: Integrate Progress Display into `agy-sync clear`
    - [ ] Wire `ProgressIndicator` to `syncer.ClearOptions.OnProgress` in `cmd/clear.go`
    - [ ] Write unit tests in `cmd/clear_test.go` verifying output contains progress feedback in text mode and remains clean in `--json` mode
- [ ] Task: Conductor - User Manual Verification 'Phase 3: CLI Command Integration' (Protocol in workflow.md)

## Phase 4: Verification, Quality Gates & Documentation
- [ ] Task: End-to-End Testing and Quality Assurance
    - [ ] Add end-to-end integration test in `test/e2e_test.go` verifying progress display and clean completion
    - [ ] Run full test suite with race detector (`go test -race ./...`), `go vet ./...`, and `golangci-lint run`
- [ ] Task: Documentation Update
    - [ ] Update `README.md` documenting visual progress feedback during `agy-sync clear`
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Verification, Quality Gates & Documentation' (Protocol in workflow.md)
