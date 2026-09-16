# Specification: Remote History, Steps, and Title Synchronization

## 1. Overview
When synchronizing Antigravity (`agy`) conversations across multiple workstations (e.g., from macOS to Google Cloud Shell), users experienced two critical issues:
1. **Missing Conversation Titles and Relative Timestamps:** The `agy` interactive conversation picker displayed raw UUIDs (e.g., `167c7a40-ee25-44dc-b215-e2640c21d11e`) without human-readable titles and without relative timestamps (such as "1m ago").
2. **Missing Steps and Session History:** Opening or resuming a conversation (`agy --conversation=<uuid>`) presented an empty conversation without previous user inputs, agent reasoning, or tool outputs.

### Root Causes
- **Title & Summary Absence:** `push` created remote `models.Conversation` records with an empty `Title: ""` and did not read the local `conversation_summaries.db`. When `pull` ran on remote machines, `conversation_summaries.db` was populated with an empty title and missing `raw_summary` protobuf blob. When `title` is empty, `agy` falls back to displaying the UUID.
- **Protobuf Binary Format Mismatch:** Resuming a conversation in `agy` reads `~/.gemini/antigravity-cli/conversations/<id>.db`. The `steps` table contains a `step_payload` column that `agy` strictly deserializes as a compiled Google Protobuf binary message (`*gemini_coder_go_proto.Step`). The previous synthetic reconstructor inserted JSON strings (`{"step_index": ...}`), causing `agy`'s protobuf deserializer to fail and drop all steps.

### Solution Strategy
Directly synchronize the authentic `conversations/<id>.db` file using SQLite `VACUUM INTO` (which safely flushes WAL journal entries into a clean, compact, consistent standalone `.db` file without holding locks on active sessions) and transfer it chunked in 512KB pieces via Firestore subcollections. In parallel, read and synchronize full summary metadata (`title`, `preview`, timestamps, `step_count`, and `raw_summary` blob) from `conversation_summaries.db`.

---

## 2. Functional Requirements

### FR1: Local Database Snapshotting via `VACUUM INTO`
- Implement an SQLite snapshot helper that takes the active path of `conversations/<id>.db` and executes `VACUUM INTO <tempPath>`.
- `VACUUM INTO` automatically folds active WAL logs into a single standalone `.db` file without blocking concurrent readers or writers.
- Compute the SHA256 checksum and total byte size of the clean snapshot.

### FR2: Chunked Database Firestore Storage
- Define `models.DBChunk` with fields:
  - `ChunkIndex int`
  - `TotalChunks int`
  - `SizeBytes int`
  - `SHA256 string`
  - `Data []byte` (512 KB per chunk)
- Extend `FirestoreRepository` with:
  - `SaveDBChunks(ctx context.Context, conversationID string, chunks []models.DBChunk) error`
  - `GetDBChunks(ctx context.Context, conversationID string) ([]models.DBChunk, error)`
- Update `models.Conversation` metadata to include:
  - `DBSHA256 string`
  - `DBSizeBytes int64`
  - `DBChunksCount int`

### FR3: Summary Metadata Synchronization
- In `internal/syncer/push.go`, query the local `conversation_summaries.db` for the conversation record.
- Extract `title`, `preview`, `step_count`, `last_modified_time`, `last_user_input_time`, `last_user_input_step_index`, and `raw_summary` blob.
- If `title` is empty, automatically set `title = preview` (falling back to first user step in `transcript.jsonl` if both are empty), ensuring `agy` displays a descriptive title.
- Store summary metadata in `models.Conversation` in Firestore.

### FR4: Chunked Database Reassembly & Summary Upsert on Pull
- In `internal/syncer/pull.go`, check if `remoteConv` has associated database chunks.
- If chunks exist, download concurrently, reassemble the binary file, verify the SHA256 hash against `remoteConv.DBSHA256`, and write atomically to `~/.gemini/antigravity-cli/conversations/<id>.db`.
- Upsert the complete record into the destination machine's `conversation_summaries.db` with non-empty `title`, `preview`, `last_user_input_time`, and `raw_summary`.
- Gracefully fall back to synthetic `ReconstructConversationDB` only if no remote database chunks exist.

---

## 3. Non-Functional Requirements

### NFR1: Data Integrity & Cross-Platform Compatibility
- Verify SHA256 checksums before finalizing any reassembled `.db` file.
- Maintain 100% SQLite binary compatibility across macOS (arm64/x86_64) and Linux (x86_64/arm64).

### NFR2: Performance & Scalability
- Split large databases into 512KB chunks to safely stay under Firestore's 1MB document limit.
- Concurrent chunk upload and download with bounded worker pools (limit: 5).

### NFR3: Code Quality & Concurrency Safety
- All operations must pass `go test -race ./...` with zero race conditions.
- Zero warnings from `golangci-lint run`.

---

## 4. Acceptance Criteria
1. `push` snapshots `conversations/<id>.db` cleanly and transfers chunks to Firestore.
2. `push` captures non-empty `title`, `preview`, timestamps, and `raw_summary` from `conversation_summaries.db`.
3. `pull` downloads chunks, verifies SHA256, and writes identical `conversations/<id>.db` to disk.
4. `pull` populates `conversation_summaries.db` such that `agy` shows human-readable titles and relative timestamps ("1m ago").
5. Resuming a conversation in `agy` (`agy --conversation=<uuid>`) renders all previous steps and interaction history correctly.
6. Existing tests and new unit/integration tests pass cleanly.

---

## 5. Out of Scope
- Modifying Antigravity CLI binary (`/Users/julienbreux/.local/bin/agy`).
- Offloading to Cloud Storage bucket (Firestore chunking provides self-contained storage for conversations up to several tens of megabytes).
