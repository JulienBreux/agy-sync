# Implementation Plan: Status Command Conversation Viewport Pagination & Default Summary (`--full`)

## Phase 1: Viewport Pager & Navigation Engine (`internal/pager/`)
- [ ] Task: Implement Viewport-Aware Terminal Paginator
    - [ ] Write unit tests in `internal/pager/pager_test.go` verifying terminal height calculations, page size derivation, page bounds clipping, row selection, and key event mapping
    - [ ] Implement `Pager` in `internal/pager/pager.go` supporting dynamic viewport calculation, raw mode handling, keyboard navigation (arrows, j/k, h/l, q/Esc), and fallback for non-TTY
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Viewport Pager & Navigation Engine' (Protocol in workflow.md)

## Phase 2: Status Command Integration & Flag Handling (`cmd/status.go`)
- [ ] Task: Implement `--full` Flag and Default Compact Status
    - [ ] Write unit tests in `cmd/status_test.go` verifying default output omits conversation table, `--full` flag includes conversations, `--json` omits/includes conversations array, and non-TTY streaming
    - [ ] Update `cmd/status.go` adding `--full` flag, default compact overview with hint, JSON conditional field rendering, and interactive pager invocation
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Status Command Integration & Flag Handling' (Protocol in workflow.md)

## Phase 3: Integration Testing, Quality Gates & Documentation
- [ ] Task: End-to-End Verification & Quality Gates
    - [ ] Add integration tests in `test/status_test.go` verifying full status execution, `--full` piped output, and JSON validation
    - [ ] Run full project test suite with `-race`, `go vet ./...`, and `golangci-lint run ./...`
- [ ] Task: Documentation Update
    - [ ] Update `README.md` with `--full` flag, default summary behavior, navigation key bindings, and sample outputs
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Integration Testing, Quality Gates & Documentation' (Protocol in workflow.md)
