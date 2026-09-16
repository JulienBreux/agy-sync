# Implementation Plan: Remote History, Steps, and Title Synchronization

## Phase 1: DB Chunk Model & Firestore Repository
- [ ] Task: DBChunk model and Conversation metadata fields
    - [ ] Write failing unit tests in `internal/models/conversation_test.go` and `internal/models/chunk_test.go` for DBChunk and Conversation DB metadata fields (`DBSHA256`, `DBSizeBytes`, `DBChunksCount`, `RawSummary`, `LastUserInputTime`)
    - [ ] Define `DBChunk` struct in `internal/models/chunk.go` and add DB & Summary fields to `internal/models/conversation.go`
- [ ] Task: Firestore Repository DB Chunks & Summary Methods
    - [ ] Write failing unit tests in `internal/repository/firestore_test.go` for `SaveDBChunks`, `GetDBChunks`, and extended `UpsertConversation` with summary fields
    - [ ] Implement `SaveDBChunks`, `GetDBChunks`, and update conversation Firestore mapping in `internal/repository/firestore.go`
- [ ] Task: Conductor - User Manual Verification 'Phase 1: DB Chunk Model & Firestore Repository' (Protocol in workflow.md)

## Phase 2: SQLite Snapshot (`VACUUM INTO`) & Local Summary Extraction
- [ ] Task: Implement SQLite Snapshot via `VACUUM INTO`
    - [ ] Write failing unit tests in `internal/reconstructor/snapshot_test.go` verifying clean `.db` snapshot creation from an active/WAL database
    - [ ] Implement `SnapshotConversationDB` in `internal/reconstructor/snapshot.go` with SHA256 checksum and temp file cleanup
- [ ] Task: Extract Local Conversation Summaries from `conversation_summaries.db`
    - [ ] Write failing unit tests in `internal/reconstructor/summary_test.go` for reading summary rows and defaulting title to preview
    - [ ] Implement `ReadLocalSummary` in `internal/reconstructor/summary.go` returning title, preview, timestamps, step count, and raw_summary blob
- [ ] Task: Conductor - User Manual Verification 'Phase 2: SQLite Snapshot (VACUUM INTO) & Local Summary Extraction' (Protocol in workflow.md)

## Phase 3: Push Pipeline Integration with Chunked Database Sync
- [ ] Task: Integrate Chunked DB Upload into Push Engine
    - [ ] Write failing unit tests in `internal/syncer/push_test.go` verifying `.db` snapshot, chunking (512KB), chunk upload, and summary population on `push`
    - [ ] Implement chunking logic and wire `SnapshotConversationDB` and `ReadLocalSummary` into `pushConversation` in `internal/syncer/push.go`
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Push Pipeline Integration with Chunked Database Sync' (Protocol in workflow.md)

## Phase 4: Pull Pipeline Integration & Database Reassembly
- [ ] Task: Integrate DB Chunk Download, Reassembly & Summary Upsert into Pull Engine
    - [ ] Write failing unit tests in `internal/syncer/pull_test.go` verifying chunk download, SHA256 verification, atomic `.db` write, and summary upsert
    - [ ] Implement chunk download, reassembly, and summary upsert in `internal/syncer/pull.go` with fallback to log reconstruction if no chunks exist
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Pull Pipeline Integration & Database Reassembly' (Protocol in workflow.md)

## Phase 5: Quality, End-to-End Verification & Documentation
- [ ] Task: End-to-End Test for Full Remote Sync Roundtrip
    - [ ] Write or extend E2E tests in `test/e2e_test.go` verifying that pushing a conversation with `.db` and pulling it on a simulated remote node reassembles the identical `.db` and populates `conversation_summaries.db` with title, preview, and timestamps
    - [ ] Run `go test -race ./...`, `golangci-lint run`, `go fmt ./...`, and `go vet ./...` ensuring 0 warnings
- [ ] Task: Update Documentation
    - [ ] Update `README.md` and track documentation detailing chunked database sync and summary title synchronization
- [ ] Task: Conductor - User Manual Verification 'Phase 5: Quality, End-to-End Verification & Documentation' (Protocol in workflow.md)
