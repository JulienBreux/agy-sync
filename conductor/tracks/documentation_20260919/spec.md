# Specification: Comprehensive Documentation & Feature Guides (`docs/`)

## 1. Overview
Revamp the repository documentation by delivering a modern, high-signal, developer-friendly `README.md` and an organized, deep-dive `docs/` directory covering all primary features of `agy-sync`. The documentation will provide immediate onboarding for new users while serving as an authoritative reference for advanced multi-machine setups, background daemon operations, transactions auditing, and cloud diagnostics.

## 2. Functional Requirements

### 2.1 Top-Level README (`README.md`)
- **Hero & Value Proposition**: Concise problem statement, key features, and badges.
- **3-Step Quickstart**: Fast path from binary installation (`go install .`) to setup and running background sync.
- **Architecture Overview**: Visual Mermaid diagram illustrating Antigravity local brain, watcher, Firestore cloud store, and SQLite reconstruction.
- **Command Cheatsheet**: Clean table summarizing all subcommands (`init`, `setup`, `start`, `stop`, `status`, `push`, `pull`, `transactions`, `clear`, `version`) with links to detailed feature guides.
- **Configuration Overview**: Explaining `config.yaml` options and environment variables (`AGY_SYNC_*`).

### 2.2 Deep-Dive Feature Guides (`docs/`)
- **`docs/README.md`**: Central documentation hub, navigation index, and architecture reference.
- **`docs/sync.md`**:
  - Bidirectional sync concepts (push vs. pull vs. background daemon).
  - Local SQLite reconstruction (`conversations.db`, `conversation_summaries.db`, trajectories).
  - Conflict resolution model (append-only step sequences, machine-ID tagging).
  - Selective push/pull by conversation ID.
- **`docs/daemon.md`**:
  - Background daemon architecture, PID file, log file, and state file management.
  - `start`, `stop`, and `status` command mechanics.
  - Interactive terminal viewport pager (`agy-sync status --full`) and keyboard controls.
  - File watcher (`fsnotify`) event debouncing and continuous background sync.
- **`docs/setup-and-cloud.md`**:
  - Prerequisites and GCP Authentication (Application Default Credentials / ADC).
  - `agy-sync setup` automated diagnostics (IAM check, API enablement, Firestore database creation).
  - Safe dry-run mode (`--dry-run`).
- **`docs/transactions-and-db.md`**:
  - Transaction audit trail logging in SQLite.
  - Inspecting sync events with `agy-sync transactions` (direction, entity, conversation filters, JSON output).
  - Database cleanup with `agy-sync clear` (safety prompts, dry-run, `--force` flag).

## 3. Non-Functional Requirements
- **Visuals & Layout**: GitHub Flavored Markdown alerts (`> [!NOTE]`, `> [!TIP]`, `> [!IMPORTANT]`), clean tables, and Mermaid diagrams.
- **Practical Copy-Paste Examples**: Realistic CLI invocations with flags and actual output snippets.
- **Link Integrity**: 100% verified relative cross-links across all docs.

## 4. Out of Scope
- Code modifications to the Go CLI or background engine.
- External documentation hosting frameworks (e.g. Docusaurus, MkDocs).
