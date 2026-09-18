# Specification: Clear Firestore Database

## 1. Overview
The `clear` command allows developers and automated test suites to purge remote Firestore data stored by `agy-sync`. This is especially crucial for integration testing, QA environments, or resetting state after development iterations without manually deleting Firestore collections via the Google Cloud Console.

## 2. Functional Requirements
1. **CLI Command:**
   - Add a top-level command `agy-sync clear [flags]`.
   - Flags:
     - `-f, --force`: Bypass interactive confirmation prompt.
     - `-c, --conversation <string>`: Clear only a single specific conversation ID and all its subcollections (`steps`, `artifacts`, `db_chunks`). If omitted, all conversations and their subcollections are cleared.
     - Global flags support: `--json`, `--project-id`, `--database-id`, `--config`, `-v, --verbose`.

2. **Confirmation Mechanism:**
   - When run interactively without `--force` / `-f`:
     - Display a warning indicating the targeted Google Cloud project ID and Firestore database ID, and whether all conversations or a specific conversation will be deleted.
     - Prompt: `Are you sure you want to clear Firestore data for project '<project-id>'? [y/N]: `
     - If input is `y` or `yes` (case-insensitive), proceed with deletion.
     - If input is anything else or EOF/interrupted, abort immediately with `Operation cancelled.` without modifying Firestore.
   - When `--force` / `-f` is passed:
     - Immediately proceed with deletion without prompting (ideal for test setups and CI scripts).

3. **Remote Deletion Engine:**
   - Extend `firestore.Repository` interface with:
     - `DeleteConversation(ctx context.Context, convID string) error`: Deletes a conversation document and batches deletion of all its subcollection documents (`steps`, `artifacts`, `db_chunks`).
     - `ClearAll(ctx context.Context) error`: Iterates over all conversations and deletes them along with all subcollections.
   - Implement these methods in:
     - `internal/firestore/client.go` (production Firestore client utilizing bulk/batch deletion)
     - `internal/firestore/memory.go` (in-memory mock repository for tests)
   - Expose clear functionality in `internal/syncer`:
     - `Engine.Clear(ctx context.Context, opts ClearOptions) (*ClearResult, error)`
     - Return statistics such as `ConversationsDeleted`.

4. **Safety & Scope:**
   - **Remote Firestore Only:** Never touch, delete, or modify local files (e.g. `~/.gemini/antigravity-cli/brain`, local SQLite databases, or local transcripts).

5. **Observability & Output:**
   - Support `--json` flag outputting `{ "conversations_deleted": N, "status": "success" }`.
   - Log informative messages detailing progress and results.

## 3. Acceptance Criteria
- `agy-sync clear` prompts for confirmation and aborts if not confirmed.
- `agy-sync clear --force` purges all conversations and subcollections without prompting.
- `agy-sync clear --conversation <id> --force` purges only the specified conversation.
- Local brain and SQLite files remain untouched.
- Unit and integration tests cover interactive abort, interactive confirm, force flag, and partial conversation deletion.
