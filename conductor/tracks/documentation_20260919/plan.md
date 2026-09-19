# Implementation Plan: Comprehensive Documentation & Feature Guides (`docs/`)

## Phase 1: Deep-Dive Feature Guides (`docs/`)
- [ ] Task: Create Documentation Hub and Sync Guide
    - [ ] Create `docs/README.md` as the master index and navigation hub for all feature guides
    - [ ] Create `docs/sync.md` detailing bidirectional synchronization, push/pull, conflict resolution, SQLite reconstruction (`conversations.db`, `conversation_summaries.db`), and Mermaid sync workflow
- [ ] Task: Create Daemon and Terminal Viewport Guide
    - [ ] Create `docs/daemon.md` detailing background daemon lifecycle (`start`, `stop`, `status`), file watching, polling, and interactive terminal pager controls (`--full`)
- [ ] Task: Create Cloud Setup, Diagnostics, and Database Operations Guides
    - [ ] Create `docs/setup-and-cloud.md` detailing Google Cloud authentication (ADC), `setup` command diagnostics, Firestore provisioning, and dry-run mode
    - [ ] Create `docs/transactions-and-db.md` detailing SQLite transaction audit logs, filtering options, and database clearing operations (`clear` with interactive/force protection)
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Deep-Dive Feature Guides (`docs/`)' (Protocol in workflow.md)

## Phase 2: Modernized Top-Level README (`README.md`) & Quality Verification
- [ ] Task: Overhaul Top-Level README
    - [ ] Rewrite `README.md` to feature a concise hero pitch, 3-step quickstart, high-level Mermaid architecture diagram, CLI subcommand matrix linking to `docs/`, and configuration table
- [ ] Task: Cross-Link & Markdown Quality Gates
    - [ ] Validate all relative markdown links between `README.md` and `docs/*.md`
    - [ ] Verify command syntax, flags, and outputs match the latest CLI implementation
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Modernized Top-Level README (`README.md`) & Quality Verification' (Protocol in workflow.md)
