# Specification: Complete Conversation & Trajectory SQLite Models and Synchronization

## Overview
When synchronizing Antigravity sessions across workstations, users experienced incomplete conversation history and missing conversation titles in the `agy` CLI and IDE UI.
1. `conversation_summaries.db` contains 21 columns, but `models.Conversation` in `agy-sync` only stored a fraction of them (omitting `workspace_uris`, `status`, `source`, `project_id`, `agent_name`, `parent_conversation_id`, `nesting_depth`, `battle_id`, `winning_conversation_id`, `not_fully_idle`, `killed`, `app_data_dir`, and `group_id`). Without `workspace_uris` and project metadata in `conversation_summaries.db`, Antigravity cannot match conversations to active workspace folders on the target machine.
2. Individual conversation databases (`~/.gemini/antigravity-cli/conversations/<id>.db`) store trajectory metadata, raw step blobs, generation metadata, executor metadata, parent references, and trajectory metadata blobs. Currently, these tables (`trajectory_meta`, `steps`, `gen_metadata`, `executor_metadata`, `parent_references`, `trajectory_metadata_blob`, `battle_mode_infos`) lack explicit, typed Go models in `pkg/models`.

This track completes the Go model definitions for both `conversation_summaries` and `conversations/<id>.db`, ensures full 1:1 synchronization between SQLite and Firestore, provides path adaptation for `workspace_uris` across different machines, and guarantees exact session fidelity and title display.

---

## Functional Requirements

### 1. Complete `Conversation` Summary Model (`pkg/models/models.go`)
- Extend `models.Conversation` with all 21 columns present in `conversation_summaries.db`:
  - `WorkspaceURIs` (`[]string`, `json:"workspace_uris,omitempty" firestore:"workspace_uris,omitempty"`)
  - `Status` (`string`, `json:"status,omitempty" firestore:"status,omitempty"`)
  - `Source` (`string`, `json:"source,omitempty" firestore:"source,omitempty"`)
  - `ProjectID` (`string`, `json:"project_id,omitempty" firestore:"project_id,omitempty"`)
  - `AgentName` (`string`, `json:"agent_name,omitempty" firestore:"agent_name,omitempty"`)
  - `ParentConversationID` (`string`, `json:"parent_conversation_id,omitempty" firestore:"parent_conversation_id,omitempty"`)
  - `NestingDepth` (`int`, `json:"nesting_depth,omitempty" firestore:"nesting_depth,omitempty"`)
  - `BattleID` (`string`, `json:"battle_id,omitempty" firestore:"battle_id,omitempty"`)
  - `WinningConversationID` (`string`, `json:"winning_conversation_id,omitempty" firestore:"winning_conversation_id,omitempty"`)
  - `NotFullyIdle` (`bool`, `json:"not_fully_idle,omitempty" firestore:"not_fully_idle,omitempty"`)
  - `Killed` (`bool`, `json:"killed,omitempty" firestore:"killed,omitempty"`)
  - `AppDataDir` (`string`, `json:"app_data_dir,omitempty" firestore:"app_data_dir,omitempty"`)
  - `GroupID` (`string`, `json:"group_id,omitempty" firestore:"group_id,omitempty"`)
- Retain all existing fields (`ID`, `Title`, `Preview`, `StepCount`, `CreatedAt`, `UpdatedAt`, `LastSyncedStep`, `SourceMachine`, `DBSHA256`, `DBSizeBytes`, `DBChunksCount`, `LastUserInputTime`, `LastUserInputStepIndex`, `RawSummary`).

### 2. Conversation Database Models (`pkg/models/trajectory.go`)
Model all tables from `conversations/<id>.db`:
- **`TrajectoryMeta`**:
  - `TrajectoryID` (`string`, `json:"trajectory_id" firestore:"trajectory_id"`)
  - `CascadeID` (`string`, `json:"cascade_id" firestore:"cascade_id"`)
  - `TrajectoryType` (`int`, `json:"trajectory_type" firestore:"trajectory_type"`)
  - `Source` (`int`, `json:"source" firestore:"source"`)
- **`ConversationDBStep`**:
  - `Idx` (`int`, `json:"idx" firestore:"idx"`)
  - `StepType` (`int`, `json:"step_type" firestore:"step_type"`)
  - `Status` (`int`, `json:"status" firestore:"status"`)
  - `HasSubtrajectory` (`bool`, `json:"has_subtrajectory" firestore:"has_subtrajectory"`)
  - `Metadata` (`[]byte`, `json:"metadata,omitempty" firestore:"metadata,omitempty"`)
  - `ErrorDetails` (`[]byte`, `json:"error_details,omitempty" firestore:"error_details,omitempty"`)
  - `Permissions` (`[]byte`, `json:"permissions,omitempty" firestore:"permissions,omitempty"`)
  - `TaskDetails` (`[]byte`, `json:"task_details,omitempty" firestore:"task_details,omitempty"`)
  - `RenderInfo` (`[]byte`, `json:"render_info,omitempty" firestore:"render_info,omitempty"`)
  - `StepPayload` (`[]byte`, `json:"step_payload,omitempty" firestore:"step_payload,omitempty"`)
  - `StepFormat` (`int`, `json:"step_format" firestore:"step_format"`)
- **`GenMetadata`**:
  - `Idx` (`int`, `json:"idx" firestore:"idx"`)
  - `Data` (`[]byte`, `json:"data,omitempty" firestore:"data,omitempty"`)
  - `Size` (`int`, `json:"size" firestore:"size"`)
- **`ExecutorMetadata`**:
  - `Idx` (`int`, `json:"idx" firestore:"idx"`)
  - `Data` (`[]byte`, `json:"data,omitempty" firestore:"data,omitempty"`)
- **`ParentReference`**:
  - `Idx` (`int`, `json:"idx" firestore:"idx"`)
  - `Data` (`[]byte`, `json:"data,omitempty" firestore:"data,omitempty"`)
- **`TrajectoryMetadataBlob`**:
  - `ID` (`string`, `json:"id" firestore:"id"`)
  - `Data` (`[]byte`, `json:"data,omitempty" firestore:"data,omitempty"`)
- **`BattleModeInfo`**:
  - `Idx` (`int`, `json:"idx" firestore:"idx"`)
  - `Data` (`[]byte`, `json:"data,omitempty" firestore:"data,omitempty"`)

### 3. Push Extraction & Persistence (`internal/syncer/push.go`)
- Query local `conversation_summaries.db` via `reconstructor.ReadLocalSummary` and copy all 21 columns into `models.Conversation`.
- Maintain strict mirroring for `title` and `preview` matching local SQLite state.
- Upsert the completed `models.Conversation` record to Firestore.

### 4. Workspace URI Adaptation & Complete Restoration on Pull (`internal/syncer/pull.go`)
- On pull, rewrite `workspace_uris` to match the target machine's home directory:
  - Detect `file:///Users/<user>/...` or `file:///home/<user>/...` paths and adapt the home prefix to the current machine's home directory path.
- Populate all 21 fields into `reconstructor.SummaryParams` when calling `UpsertSummary`.
- Safe defaults: If pulling legacy Firestore records lacking new fields, gracefully use safe zero values without clobbering existing local SQLite records.

### 5. Reconstructor Integration (`internal/reconstructor/`)
- Ensure `internal/reconstructor` uses typed models for conversation databases and summary tables.
- Keep table creation schemas and indexes completely aligned with Antigravity SQLite standards.

---

## Non-Functional Requirements
- **Data Integrity & Byte Parity:** Exact preservation of binary blobs (`raw_summary`, step payloads, trajectory metadata blobs).
- **Idempotency & Reentrancy:** Multi-machine updates merge cleanly without overwriting newer edits or creating sync loops.
- **Performance:** SQLite operations remain transactional and fast.

---

## Acceptance Criteria
1. `models.Conversation` defines all 21 columns of `conversation_summaries` with full json/firestore tags.
2. `pkg/models` contains typed definitions for `TrajectoryMeta`, `ConversationDBStep`, `GenMetadata`, `ExecutorMetadata`, `ParentReference`, `TrajectoryMetadataBlob`, and `BattleModeInfo`.
3. `agy-sync push` writes all conversation summary attributes to Firestore.
4. `agy-sync pull` on a new machine populates `conversation_summaries.db` with all 21 fields, including adapted `workspace_uris` that match the target machine's home directory.
5. In the destination machine's `agy` CLI and IDE UI, conversation titles and histories display properly under the current workspace.
6. All existing and new tests pass (`go test ./...` and `golangci-lint run`).

---

## Out of Scope
- Modifying closed-source Antigravity CLI binary internals.
- Direct synchronization of non-conversation SQLite databases.
