# Implementation Plan - Import & Adapt Automation and Community Assets from `JulienBreux/run-cli`

## Phase 1: Community & Governance Documentation [checkpoint: 2a21810]
- [x] Task: Import and Adapt Community Files [2a21810]
  - [x] Fetch and adapt `CODE_OF_CONDUCT.md`, `MAINTAINERS.md`, `SECURITY.md`, and `SUPPORT.md`
  - [x] Adapt `CONTRIBUTING.md` tailored for `agy-sync` (TDD, Go 1.27+, commit conventions)
- [x] Task: Conductor - User Manual Verification 'Phase 1: Community & Governance Documentation' (Protocol in workflow.md) [2a21810]

## Phase 2: Build & Packaging Tooling
- [x] Task: Import and Adapt Developer Tooling [1e263e3]
  - [x] Create `.editorconfig` and `codecov.yml`
  - [x] Adapt `Makefile` for `agy-sync` (`build`, `test`, `lint`, `cover`, `docker`, `clean`)
  - [x] Adapt multi-stage `Dockerfile` compiling minimal static `agy-sync` binary
  - [x] Adapt `.goreleaser.yaml` for multi-architecture binary builds (`amd64`, `arm64`)
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Build & Packaging Tooling' (Protocol in workflow.md)

## Phase 3: GitHub Workflows, Templates & Verification
- [ ] Task: Import and Adapt GitHub Workflows & Templates
  - [ ] Create `.github/ISSUE_TEMPLATE/bug_report.md` and `feature_request.md`
  - [ ] Create `.github/pull_request_template.md`
  - [ ] Adapt `.github/workflows/test.yml` (Go 1.27, linting, tests, Codecov)
  - [ ] Adapt `.github/workflows/release.yml` (GoReleaser tag release)
- [ ] Task: Validate Local Tooling Execution
  - [ ] Verify `make build`, `make test`, and `make lint` execute successfully
- [ ] Task: Conductor - User Manual Verification 'Phase 3: GitHub Workflows, Templates & Verification' (Protocol in workflow.md)
