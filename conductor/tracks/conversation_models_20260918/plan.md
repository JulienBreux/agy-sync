# Implementation Plan: Complete Conversation & Trajectory SQLite Models and Synchronization

## Phase 1: Extended Models & Database Table Structs
- [x] Task: Complete `models.Conversation` Summary Fields [3b7c1ed]
    - [x] Write failing unit tests in `pkg/models/models_test.go` for all 21 columns serialization (JSON & Firestore tags)
    - [x] Implement extended fields (`WorkspaceURIs`, `Status`, `Source`, `ProjectID`, `AgentName`, `ParentConversationID`, `NestingDepth`, `BattleID`, `WinningConversationID`, `NotFullyIdle`, `Killed`, `AppDataDir`, `GroupID`) in `pkg/models/models.go`
- [x] Task: Implement Trajectory Database Table Models [f7f9e1c]
    - [x] Write failing unit tests in `pkg/models/trajectory_test.go` for `TrajectoryMeta`, `ConversationDBStep`, `GenMetadata`, `ExecutorMetadata`, `ParentReference`, `TrajectoryMetadataBlob`, and `BattleModeInfo`
    - [x] Implement trajectory database model structs in `pkg/models/trajectory.go`
- [x] Task: Conductor - User Manual Verification 'Phase 1: Extended Models & Database Table Structs' (Protocol in workflow.md)

## Phase 2: Reconstructor Integration & Summary Mapping
- [x] Task: Update `internal/reconstructor` to Fully Map All 21 Columns [d8a9680]
    - [x] Write failing unit tests in `internal/reconstructor/summary_test.go` verifying read/write roundtrip of all 21 columns with `SummaryParams`
    - [x] Implement full 21-column queries and assignments in `internal/reconstructor/summary.go`
- [x] Task: Integrate Trajectory DB Models in `internal/reconstructor/conversation.go` [1c41a5d]
    - [x] Write failing unit tests in `internal/reconstructor/conversation_test.go` validating typed table schema and step mapping using `models.ConversationDBStep` and `models.TrajectoryMeta`
    - [x] Refactor `internal/reconstructor/conversation.go` to utilize typed trajectory models
- [x] Task: Conductor - User Manual Verification 'Phase 2: Reconstructor Integration & Summary Mapping' (Protocol in workflow.md)

## Phase 3: Push Pipeline Full Summary Extraction
- [x] Task: Extract and Push All Conversation Summary Attributes [67217db]
    - [x] Write failing unit tests in `internal/syncer/push_test.go` validating that all 21 columns from local `conversation_summaries.db` are extracted and stored into `models.Conversation` before Firestore upsert
    - [x] Update `internal/syncer/push.go` to populate all fields on `remoteConv` from `ReadLocalSummary` with strict title mirroring
- [x] Task: Conductor - User Manual Verification 'Phase 3: Push Pipeline Full Summary Extraction' (Protocol in workflow.md)

## Phase 4: Pull Pipeline Workspace URI Adaptation & Restoration
- [x] Task: Implement Workspace URI Path Adaptation Helper [ee194a0]
    - [x] Write failing unit tests in `internal/syncer/uri_adapter_test.go` verifying home prefix detection and replacement for `file:///Users/<user>/...` and `file:///home/<user>/...`
    - [x] Implement `AdaptWorkspaceURIs(uris []string, destHome string)` in `internal/syncer/uri_adapter.go`
- [x] Task: Integrate Full Summary Upsert & URI Adaptation in Pull Engine [5747a97]
    - [x] Write failing unit tests in `internal/syncer/pull_test.go` testing that pulling restores all 21 columns in `conversation_summaries.db` with adapted `workspace_uris`
    - [x] Update `internal/syncer/pull.go` to adapt `workspace_uris` and pass all 21 parameters to `reconstructor.UpsertSummary`
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Pull Pipeline Workspace URI Adaptation & Restoration' (Protocol in workflow.md)

## Phase 5: End-to-End Testing, Linting & Documentation
- [ ] Task: Multi-Machine Full Fidelity E2E Integration Test
    - [ ] Add an end-to-end integration test in `test/e2e_test.go` simulating Machine A (pushing with full 21 summary columns & trajectory tables) and Machine B (pulling with different user home path), asserting exact table content and adapted `workspace_uris`
    - [ ] Run `go test -race ./...`, `golangci-lint run`, and `go vet ./...` ensuring 0 warnings
- [ ] Task: Update Documentation
    - [ ] Update `README.md` and track documentation detailing full conversation summaries table synchronization and trajectory models
- [ ] Task: Conductor - User Manual Verification 'Phase 5: End-to-End Testing, Linting & Documentation' (Protocol in workflow.md)
