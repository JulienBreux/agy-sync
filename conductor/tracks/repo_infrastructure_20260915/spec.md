# Specification: Import & Adapt Repository Automation and Community Assets from `JulienBreux/run-cli`

## 1. Overview
Import, customize, and integrate standard open-source governance, CI/CD automation, build tooling, and release pipelines from `https://github.com/JulienBreux/run-cli` into `agy-sync`. All configurations will be fully tailored to `agy-sync`, Go 1.27+, and Google Antigravity session synchronization.

## 2. Requirements & Deliverables

### 2.1 Community & Governance Documentation
- `CODE_OF_CONDUCT.md`: Contributor Covenant Code of Conduct adapted for Julien Breux / `agy-sync`.
- `CONTRIBUTING.md`: Contribution guide including code standards, TDD workflow, commit formatting, and testing guidelines.
- `SECURITY.md`: Vulnerability reporting process and supported version matrices.
- `SUPPORT.md`: Getting help, bug reporting, and discussion channels.
- `MAINTAINERS.md`: Project maintainers listing.

### 2.2 Developer Tooling & Build Automation
- `.editorconfig`: Standard indentation, line endings, and file formatting rules.
- `Makefile`: Build, test, lint, clean, run, and docker build targets adapted for `agy-sync` and `./bin/agy-sync`.
- `Dockerfile`: Multi-stage Docker build producing a minimal, secure static container image for `agy-sync`.
- `.goreleaser.yaml`: Multi-platform cross-compilation matrix (Darwin, Linux, Windows for `amd64` and `arm64`), archive packaging, checksum generation, and homebrew/docker release configurations.
- `codecov.yml`: Code coverage thresholds, target percentages (>80%), and status check rules.

### 2.3 GitHub Actions & Templates
- `.github/workflows/test.yml`: Continuous Integration workflow running on push and PR (Go 1.27+, `go test -v -race -coverprofile`, `golangci-lint`, and Codecov upload).
- `.github/workflows/release.yml`: Automated semantic release pipeline triggered on Git tags via GoReleaser.
- `.github/pull_request_template.md`: Structured PR template with verification checklists and Conductor alignment.
- `.github/ISSUE_TEMPLATE/bug_report.md`: Issue template for reporting bugs with environment details.
- `.github/ISSUE_TEMPLATE/feature_request.md`: Issue template for proposing enhancements.

## 3. Acceptance Criteria
- [ ] All community documents (`CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SECURITY.md`, `SUPPORT.md`, `MAINTAINERS.md`) exist at root with `agy-sync` branding.
- [ ] `.editorconfig`, `codecov.yml`, `.goreleaser.yaml`, and `Dockerfile` are created and valid.
- [ ] `Makefile` targets (`make build`, `make test`, `make lint`, `make docker`) execute cleanly and match project binaries.
- [ ] GitHub workflows and templates in `.github/` are configured and syntactically valid YAML.
- [ ] All references to `run-cli` are replaced with `agy-sync`.
