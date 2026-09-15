# Implementation Plan: MVP Core Ingestion & Bidirectional Firestore Sync

## Phase 1: Scaffolding, Configuration & CLI Foundation
- [x] Task: Go Module Setup & Project Structure [c64d4b7]
  - [x] Initialize `go.mod` (Go 1.23+) and establish package structure (`cmd/`, `pkg/config/`, `pkg/parser/`, `pkg/firestore/`, `pkg/syncer/`)
  - [x] Configure `golangci-lint` and test harnesses
- [ ] Task: Configuration & `init` Command (TDD)
  - [ ] Write unit tests for config load, validation, and defaults (`pkg/config/config_test.go`)
  - [ ] Implement Viper-based config manager (`pkg/config/config.go`)
  - [ ] Implement Cobra root and `init` command (`cmd/init.go`)
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)

## Phase 2: Ingestion & JSONL Transcript Parser
- [ ] Task: Transcript Parsing & Step Extraction (TDD)
  - [ ] Write unit tests for parsing AGY JSONL transcript entries, step indices, tool calls, and truncated fields (`pkg/parser/parser_test.go`)
  - [ ] Implement incremental JSONL scanner with byte-offset tracking (`pkg/parser/parser.go`)
  - [ ] Implement models for Conversation, Step, ToolCall, and Artifact (`pkg/models/models.go`)
- [ ] Task: Brain Directory Discovery (TDD)
  - [ ] Write unit tests for scanning `~/.gemini/antigravity-cli/brain/` (`pkg/discovery/discovery_test.go`)
  - [ ] Implement directory walker identifying active conversations and artifacts (`pkg/discovery/discovery.go`)
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)

## Phase 3: Firestore Cloud Storage & Data Layer
- [ ] Task: Firestore Repository (TDD with Emulator)
  - [ ] Write tests using Firestore emulator for conversation upsert and step append (`pkg/firestore/client_test.go`)
  - [ ] Implement Firestore client initialization with ADC (`pkg/firestore/client.go`)
  - [ ] Implement idempotent batch step write operations with monotonic indexing (`pkg/firestore/steps.go`)
  - [ ] Implement remote step retrieval and query methods (`pkg/firestore/fetch.go`)
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)

## Phase 4: Synchronization Commands (`push` & `pull`)
- [ ] Task: Push Command Implementation (TDD)
  - [ ] Write tests for incremental push logic (local -> Firestore) (`pkg/syncer/push_test.go`)
  - [ ] Implement sync engine push mechanism with state tracking (`pkg/syncer/push.go`)
  - [ ] Implement Cobra `push` command (`cmd/push.go`)
- [ ] Task: Pull Command Implementation (TDD)
  - [ ] Write tests for reconstructing local `transcript.jsonl` and directory tree from Firestore (`pkg/syncer/pull_test.go`)
  - [ ] Implement pull engine and local filesystem writer (`pkg/syncer/pull.go`)
  - [ ] Implement Cobra `pull` command (`cmd/pull.go`)
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)

## Phase 5: Real-time Filesystem Watcher & Remote Listener (`watch`)
- [ ] Task: Local Filesystem Event Watcher (TDD)
  - [ ] Write tests for `fsnotify` event debouncing and filtering (`pkg/watcher/watcher_test.go`)
  - [ ] Implement debounced watcher monitoring `transcript.jsonl` modifications (`pkg/watcher/watcher.go`)
- [ ] Task: Firestore Snapshot Listener (TDD)
  - [ ] Write tests for handling remote Firestore snapshot change streams (`pkg/syncer/listener_test.go`)
  - [ ] Implement remote listener with machine ID loop prevention (`pkg/syncer/listener.go`)
- [ ] Task: Cobra `watch` Command
  - [ ] Integrate watcher + listener into unified daemon command (`cmd/watch.go`)
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)

## Phase 6: End-to-End Integration & Hardening
- [ ] Task: Full Round-Trip E2E Test
  - [ ] Execute multi-machine simulated sync test using Firestore emulator
  - [ ] Verify exact line-by-line fidelity of reconstructed `transcript.jsonl`
- [ ] Task: Observability, Error Handling & CLI Polishing
  - [ ] Verify `--json` output flag on status and inspect commands
  - [ ] Verify standard POSIX exit codes and user remediation hints
- [ ] Task: Phase Verification & Checkpoint (Refer to workflow.md)
