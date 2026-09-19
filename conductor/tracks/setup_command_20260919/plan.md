# Implementation Plan: Cloud Setup and Environment Verification Command (`agy-sync setup`)

## Phase 1: GCP Diagnostic and Management Core Client (`internal/setup/`)
- [x] Task: Design and Implement Setup Checker Interfaces and GCP Diagnostics [e0d5b5a]
    - [x] Define `CheckResult`, `CheckStatus`, `SetupReport`, and `Checker` interfaces in `internal/setup/types.go`
    - [x] Write failing unit tests in `internal/setup/checker_test.go` covering credential validation, project access, API enablement, database verification, and dry-run mode
    - [x] Implement `GCPChecker` in `internal/setup/checker.go` checking ADC credentials, Cloud Resource Manager project access, Service Usage API status, and Firestore database existence
- [x] Task: Conductor - User Manual Verification 'Phase 1: GCP Diagnostic and Management Core Client' (Protocol in workflow.md)

## Phase 2: Firestore Database Creation and Mutation Engine (`internal/setup/`)
- [x] Task: Implement Database Provisioning and Service Enablement [a19aeae]
    - [x] Write failing unit tests in `internal/setup/provision_test.go` for database creation flow, interactive confirmation callbacks, `--dry-run` guardrails, and error handling
    - [x] Implement `Provisioner` in `internal/setup/provision.go` utilizing Firestore Admin API to create Native mode Firestore database and optional service API enablement
- [x] Task: Conductor - User Manual Verification 'Phase 2: Firestore Database Creation and Mutation Engine' (Protocol in workflow.md)

## Phase 3: CLI Command `agy-sync setup` (`cmd/setup.go`)
- [x] Task: Implement `setup` Cobra Command and Visual Checklist Formatter [9b252d3]
    - [x] Write failing unit tests in `cmd/setup_test.go` verifying flag handling (`--dry-run`, `--yes`, `--project-id`, `--database-id`, `--location`), interactive prompts, exit codes, and `--json` format
    - [x] Implement `cmd/setup.go` with Cobra command definition, interactive login trigger, and visual checklist ASCII renderer with status badges (`[✓]`, `[✗]`, `[!]`) and remediation hints
    - [x] Register `setupCmd` in `cmd/root.go` under `setup` group
- [x] Task: Conductor - User Manual Verification 'Phase 3: CLI Command agy-sync setup' (Protocol in workflow.md)

## Phase 4: Integration Testing, Quality Gates & Documentation
- [ ] Task: End-to-End Verification & Quality Gates
    - [ ] Add integration tests in `test/setup_test.go` verifying full setup workflow with dry-run, mock/emulator, and JSON validation
    - [ ] Run full project test suite with `-race`, `go vet ./...`, and `golangci-lint run ./...`
- [ ] Task: Documentation Update
    - [ ] Update `README.md` documenting `agy-sync setup` command, `--dry-run`, prerequisites, and sample output
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Integration Testing, Quality Gates & Documentation' (Protocol in workflow.md)
