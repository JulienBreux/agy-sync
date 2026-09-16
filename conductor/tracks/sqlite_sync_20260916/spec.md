# Specification: Antigravity SQLite Database Reconstruction & Synchronization (`conversations/` & `conversation_summaries.db`)

## 1. Overview
Currently, `agy-sync` synchronizes the `brain/` directory (`transcript.jsonl` and generated artifacts), enabling rich cloud archiving and step inspection in Firestore. However, the Antigravity CLI (`agy resume`) and IDE UI rely on local SQLite databases located at `~/.gemini/antigravity-cli/conversations/<id>.db` and `~/.gemini/antigravity-cli/conversation_summaries.db`. Without these database entries, sessions pulled from remote machines do not automatically appear in the local Antigravity session selector or resume interface.

This feature introduces automatic local SQLite database reconstruction and synchronization upon pull/sync. It reconstructs or updates the conversation's SQLite database (`conversations/<id>.db`) and registers the session in `conversation_summaries.db`, enabling seamless resume across multiple workstations.

## 2. Functional Requirements
1. **SQLite Database Reconstructor (`internal/reconstructor/`):**
   - Provide a safe SQLite client/manager that interacts with Antigravity SQLite databases using WAL mode and a busy timeout (5000ms) to prevent locking conflicts with active Antigravity processes.
   - Reconstruct or populate `conversations/<conversation-id>.db`:
     - Ensure schema tables exist (`trajectory_meta`, `steps`, `gen_metadata`, etc.).
     - Insert or update step records (`steps` table) based on the parsed transcript (`transcript.jsonl`).
     - Populate trajectory metadata (`trajectory_meta`).
   - Reconstruct or update `conversation_summaries.db`:
     - Upsert conversation summary records (`conversation_id`, `title`, `step_count`, `last_modified_time`, `workspace_uris`, `status`).
     - Derive summary fields (extract conversation title from prompt/metadata, step count from parsed steps, workspace URI from config/metadata).
2. **Integration into Pull and Sync Engine:**
   - On `agy-sync pull` and remote syncer listener updates, after writing `brain/` files, invoke the reconstructor to build or update `conversations/<id>.db` and `conversation_summaries.db`.
   - Provide transactional updates so partially pulled states do not corrupt existing SQLite databases.
3. **Configuration & CLI Controls:**
   - Add configuration flag/setting `no_db_sync` / `--no-db-sync` (default: `false`, i.e., SQLite sync enabled by default).
   - Add configuration options for `conversations_dir` (default: `~/.gemini/antigravity-cli/conversations`) and `summaries_db` (default: `~/.gemini/antigravity-cli/conversation_summaries.db`).
4. **Status & CLI Visibility:**
   - Update `agy-sync status` to report SQLite database presence and step count alongside local/remote steps.

## 3. Non-Functional Requirements
- **Concurrency & Safety:** Safe handling of active Antigravity processes using SQLite `_busy_timeout=5000` and `_journal_mode=WAL`.
- **Zero CGo Dependency:** Use a pure Go SQLite driver (`modernc.org/sqlite`) to maintain frictionless cross-platform builds without requiring GCC/CGO across macOS, Linux, and Windows.
- **High Test Coverage:** Comprehensive unit tests with in-memory and temporary SQLite databases adhering to the project's >80% coverage standard.
- **Modern Go Standards:** Enforce modern Go idioms (`log/slog`, `cmp.Or`, `slices`, `context.Context` propagation, standard error handling, and strict linting).

## 4. Acceptance Criteria
- Pulling a remote conversation creates `conversations/<conversation-id>.db` with valid schema and step records if it did not exist.
- Pulling an updated conversation updates `conversation_summaries.db` with the correct title, step count, and last modified timestamp.
- `agy-sync pull --no-db-sync` skips SQLite modification and only updates `brain/`.
- Concurrency handled gracefully with busy-timeout and WAL mode.
- All new code has unit tests and passes linting and full test suites.

## 5. Out of Scope
- Reverse sync (ingesting binary protobuf blobs directly from `conversations/<id>.db` into Firestore without JSONL transcripts).
- Modifying Antigravity CLI's binary internal schemas or hooks beyond the standard SQLite tables.
