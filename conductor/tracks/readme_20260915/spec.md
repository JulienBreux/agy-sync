# Specification: Clean Project Documentation (`README.md`)

## 1. Overview
Create a comprehensive, production-grade `README.md` for `agy-sync`. The documentation will provide clear project vision, architectural deep dives, CLI command manuals, configuration references, and testing/development workflows.

## 2. Requirements & Sections
### 2.1 Header & Overview
- Clear title, badges (Go version, License: Apache 2.0, Architecture).
- High-level value proposition: asynchronous ingestion of Antigravity transcripts (`transcript.jsonl`) and artifacts to Google Cloud Firestore, with multi-machine sync and loop prevention.

### 2.2 Architecture & Data Flow
- Mermaid sequence/flow diagram illustrating local file watcher (`fsnotify`), push streaming, Firestore collections (`/conversations/{id}`, `/steps`, `/artifacts`), and remote pulling by secondary machines.
- Explanation of the append-only log model and machine ID loop-prevention mechanism.

### 2.3 CLI Reference & Quick Start
- Installation & compilation instructions (`go build -o bin/agy-sync .`).
- Detailed command reference with arguments, flags, and formatted terminal examples:
  - `agy-sync init`: Configuration initialization (`--project-id`, `--database-id`, `--machine-id`).
  - `agy-sync push`: One-off incremental sync.
  - `agy-sync pull [conversation-id]`: Reconstruction of local transcripts and artifact trees.
  - `agy-sync watch`: Background real-time sync daemon.
  - `agy-sync status`: Progress inspection (local vs remote) with `--json` output.

### 2.4 Configuration Guide
- Configuration schema (`~/.config/agy-sync/config.yaml`).
- Environment variable overrides (`AGY_SYNC_*`).
- Google Cloud Application Default Credentials (ADC) setup.

### 2.5 Developer & Testing Guide
- Local development workflow: building, unit tests, emulator integration, and `golangci-lint`.
- Firestore emulator setup instructions (`FIRESTORE_EMULATOR_HOST`).

### 2.6 Licensing
- Apache 2.0 License reference.

## 3. Acceptance Criteria
- [ ] Root `README.md` is created with all sections and valid Mermaid diagrams.
- [ ] All CLI commands, options, and environment variables match current codebase.
- [ ] All code blocks and syntax are verified and render cleanly.
- [ ] Out of scope: Modifying application code or CLI runtime behavior.
