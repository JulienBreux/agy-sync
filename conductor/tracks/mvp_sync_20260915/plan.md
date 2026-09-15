# Implementation Plan: MVP Core Ingestion & Bidirectional Firestore Sync

## Phase 1: Scaffolding, Configuration & CLI Foundation [checkpoint: fe6122c]
- [x] Task: Go Module Setup & Project Structure [c64d4b7]
  - [x] Initialize `go.mod` (Go 1.23+) and establish package structure (`cmd/`, `pkg/config/`, `pkg/parser/`, `pkg/firestore/`, `pkg/syncer/`)
  - [x] Configure `golangci-lint` and test harnesses
- [x] Task: Configuration & `init` Command (TDD) [fe6122c]
  - [x] Write unit tests for config load, validation, and defaults (`pkg/config/config_test.go`)
  - [x] Implement Viper-based config manager (`pkg/config/config.go`)
  - [x] Implement Cobra root and `init` command (`cmd/init.go`)
- [x] Task: Phase Verification & Checkpoint (Refer to workflow.md) [fe6122c]

## Phase 2: Ingestion & JSONL Transcript Parser [checkpoint: 5bd4209]
- [x] Task: Transcript Parsing & Step Extraction (TDD) [e369413]
  - [x] Write unit tests for parsing AGY JSONL transcript entries, step indices, tool calls, and truncated fields (`pkg/parser/parser_test.go`)
  - [x] Implement incremental JSONL scanner with byte-offset tracking (`pkg/parser/parser.go`)
  - [x] Implement models for Conversation, Step, ToolCall, and Artifact (`pkg/models/models.go`)
- [x] Task: Brain Directory Discovery (TDD) [5bd4209]
  - [x] Write unit tests for scanning `~/.gemini/antigravity-cli/brain/` (`pkg/discovery/discovery_test.go`)
  - [x] Implement directory walker identifying active conversations and artifacts (`pkg/discovery/discovery.go`)
- [x] Task: Phase Verification & Checkpoint (Refer to workflow.md) [5bd4209]

## Phase 3: Firestore Cloud Storage & Data Layer [checkpoint: 2330823]
- [x] Task: Firestore Repository (TDD with Emulator) [2330823]
  - [x] Write tests using Firestore emulator for conversation upsert and step append (`pkg/firestore/client_test.go`)
  - [x] Implement Firestore client initialization with ADC (`pkg/firestore/client.go`)
  - [x] Implement idempotent batch step write operations with monotonic indexing (`pkg/firestore/steps.go`)
  - [x] Implement remote step retrieval and query methods (`pkg/firestore/fetch.go`)
- [x] Task: Phase Verification & Checkpoint (Refer to workflow.md) [2330823]

## Phase 4: Synchronization Commands (`push` & `pull`) [checkpoint: fb82a3f]
- [x] Task: Push Command Implementation (TDD) [99c9a17]
  - [x] Write tests for incremental push logic (local -> Firestore) (`pkg/syncer/push_test.go`)
  - [x] Implement sync engine push mechanism with state tracking (`pkg/syncer/push.go`)
  - [x] Implement Cobra `push` command (`cmd/push.go`)
- [x] Task: Pull Command Implementation (TDD) [fb82a3f]
  - [x] Write tests for reconstructing local `transcript.jsonl` and directory tree from Firestore (`pkg/syncer/pull_test.go`)
  - [x] Implement pull engine and local filesystem writer (`pkg/syncer/pull.go`)
  - [x] Implement Cobra `pull` command (`cmd/pull.go`)
- [x] Task: Phase Verification & Checkpoint (Refer to workflow.md) [fb82a3f]

## Phase 5: Real-time Filesystem Watcher & Remote Listener (`watch`)
- [x] Task: Local Filesystem Event Watcher (TDD) [1207c2f]
  - [x] Write tests for `fsnotify` event debouncing and filtering (`pkg/watcher/watcher_test.go`)
  - [x] Implement debounced watcher monitoring `transcript.jsonl` modifications (`pkg/watcher/watcher.go`)
- [x] Task: Firestore Snapshot Listener (TDD) [b3e83a9]
  - [x] Write tests for handling remote Firestore snapshot change streams (`pkg/syncer/listener_test.go`)
  - [x] Implement remote listener with machine ID loop prevention (`pkg/syncer/listener.go`)
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
