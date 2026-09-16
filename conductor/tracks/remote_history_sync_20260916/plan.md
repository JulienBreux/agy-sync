# Implementation Plan: Remote History, Steps, and Title Synchronization

## Phase 1: DB Chunk Model & Firestore Repository
- [x] Task: DBChunk model and Conversation metadata fields [5a929d1]
    - [x] Write failing unit tests in `pkg/models/models_test.go` for DBChunk and Conversation DB metadata fields (`DBSHA256`, `DBSizeBytes`, `DBChunksCount`, `RawSummary`, `LastUserInputTime`)
    - [x] Define `DBChunk` struct in `pkg/models/models.go` and add DB & Summary fields to `Conversation`
- [x] Task: Firestore Repository DB Chunks & Summary Methods [5a929d1]
    - [x] Write failing unit tests in `internal/firestore/repository_test.go` for `SaveDBChunks`, `GetDBChunks`, and extended `UpsertConversation` with summary fields
    - [x] Implement `SaveDBChunks`, `GetDBChunks`, and update conversation Firestore mapping in `internal/firestore/repository.go`
- [x] Task: Conductor - User Manual Verification 'Phase 1: DB Chunk Model & Firestore Repository' (Protocol in workflow.md)

## Phase 2: SQLite Snapshot (`VACUUM INTO`) & Local Summary Extraction
- [x] Task: Implement SQLite Snapshot via `VACUUM INTO` [ab29867]
    - [x] Write failing unit tests in `internal/reconstructor/snapshot_test.go` verifying clean `.db` snapshot creation from an active/WAL database
    - [x] Implement `SnapshotConversationDB` in `internal/reconstructor/snapshot.go` with SHA256 checksum and temp file cleanup
- [x] Task: Extract Local Conversation Summaries from `conversation_summaries.db` [ab29867]
    - [x] Write failing unit tests in `internal/reconstructor/summary_test.go` for reading summary rows and defaulting title to preview
    - [x] Implement `ReadLocalSummary` in `internal/reconstructor/summary.go` returning title, preview, timestamps, step count, and raw_summary blob
- [x] Task: Conductor - User Manual Verification 'Phase 2: SQLite Snapshot (VACUUM INTO) & Local Summary Extraction' (Protocol in workflow.md)

## Phase 3: Push Pipeline Integration with Chunked Database Sync
- [x] Task: Integrate Chunked DB Upload into Push Engine [ab51417]
    - [x] Write failing unit tests in `internal/syncer/push_test.go` verifying that local conversation SQLite snapshot is chunked and uploaded, and metadata contains non-empty title/preview/timestamps
    - [x] Update `PushEngine` in `internal/syncer/push.go` to extract summary, snapshot DB, chunk into 512KB slices, and upload via `repo.SaveDBChunks`
- [x] Task: Conductor - User Manual Verification 'Phase 3: Push Pipeline Integration with Chunked Database Sync' (Protocol in workflow.md)

## Phase 4: Pull Pipeline Integration & Database Reassembly
- [x] Task: Integrate DB Chunk Download, Reassembly & Summary Upsert into Pull Engine [a0e280b]
    - [x] Write failing unit tests in `internal/syncer/pull_test.go` verifying chunk download, SHA256 verification, atomic `.db` write, and summary upsert
    - [x] Implement chunk download, reassembly, and summary upsert in `internal/syncer/pull.go` with fallback to log reconstruction if no chunks exist
- [x] Task: Conductor - User Manual Verification 'Phase 4: Pull Pipeline Integration & Database Reassembly' (Protocol in workflow.md)

## Phase 5: Quality, End-to-End Verification & Documentation
- [x] Task: End-to-End Test for Full Remote Sync Roundtrip [2d05419]
    - [x] Add an end-to-end integration test in `test/e2e_test.go` verifying that pushing on Machine A (with SQLite .db and summaries) and pulling on Machine B restores exact `.db` binary, summary title, and steps
    - [x] Run `go test -race ./...`, `golangci-lint run`, `go fmt ./...`, and `go vet ./...` ensuring 0 warnings
- [x] Task: Update Documentation [2d05419]
    - [x] Update `README.md` and track documentation detailing chunked database sync and summary title synchronization
- [x] Task: Conductor - User Manual Verification 'Phase 5: Quality, End-to-End Verification & Documentation' (Protocol in workflow.md)
