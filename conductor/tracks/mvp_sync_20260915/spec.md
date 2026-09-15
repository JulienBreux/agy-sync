# Specification: MVP Core Ingestion & Bidirectional Firestore Sync

## 1. Overview
The MVP of `agy-sync` delivers a standalone, compiled Go CLI (`agy-sync`) that ingests local Antigravity (AGY) conversations, transcripts (`transcript.jsonl`), and artifacts, parses them into structured records, and enables bidirectional multi-machine synchronization via Google Cloud Firestore.

## 2. Functional Requirements
### 2.1 Configuration & Discovery
- **Initialization (`agy-sync init`):** Configures GCP Project ID, Firestore DB, machine ID, stored in `~/.config/agy-sync/config.yaml`.
- **Discovery Engine:** Automatically scans `~/.gemini/antigravity-cli/brain/` with `--dir` and `--conversation-id` overrides.

### 2.2 Ingestion & Parser
- **JSONL Step Parser:** Parses `transcript.jsonl` steps (index, timestamp, source, type, content, tool_calls, thinking).
- **Incremental Line Tracker:** Tracks byte/line offsets to stream only new steps without redundant reads.

### 2.3 Firestore Schema
- `/conversations/{id}`: Conversation metadata, last synced step, machine ID, timestamps.
- `/conversations/{id}/steps/{stepIndex}`: Immutable step records, tool calls, and model outputs.
- `/conversations/{id}/artifacts/{artifactId}`: Artifact metadata, hash, and content.

### 2.4 CLI Commands
- `agy-sync init`: Configure project, credentials, and machine ID.
- `agy-sync push`: One-off sync of local steps to Firestore.
- `agy-sync pull`: Reconstruct conversation and transcripts from Firestore locally.
- `agy-sync watch`: Continuous daemon with `fsnotify` file watcher and Firestore real-time listeners.

## 3. Quality & Non-Functional Requirements
- **Idempotency:** Monotonic step indices guarantee no duplicate steps or data loss.
- **Coverage:** >80% unit/integration test coverage using standard Go testing and Firestore Emulator.
- **Security:** GCP ADC authentication with zero secret leakage.

## 4. Acceptance Criteria
- [ ] CLI compiles cleanly into a single static binary.
- [ ] `push` and `pull` demonstrate exact round-trip fidelity for AGY `transcript.jsonl`.
- [ ] `watch` reflects new local steps in Firestore within 2 seconds.
- [ ] All tests pass with >80% code coverage.
