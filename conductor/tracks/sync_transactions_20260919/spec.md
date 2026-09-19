# Specification: Sync Transactions Command and Audit Logging

## 1. Overview
As `agy-sync` synchronizes Antigravity conversations, artifacts, and brain transcripts between local machines and Google Cloud Firestore, developers need visibility into what data has been imported (pulled) or exported (pushed).

This feature introduces a persistent transaction log and the `agy-sync transactions` CLI command to display an audit trail of synchronization transactions with support for granular filtering by direction (`in`/`out`), entity type (`conv`/`artifact`/`brain`), and conversation ID.

## 2. Functional Requirements
1. **Transaction Storage Engine:**
   - Store sync transactions locally in an embedded SQLite database (`~/.config/agy-sync/transactions.db` by default, configurable via config file or flags).
   - Schema tracks: `id`, `timestamp`, `direction` (`in` for IMPORT, `out` for EXPORT), `entity_type` (`conv`, `artifact`, `brain`), `conversation_id`, `entity_id` (e.g. artifact filename or step range), `details` (e.g., bytes, steps count), and `status` (`success`, `error`).
2. **Automated Transaction Recording:**
   - On `Push` (or daemon export):
     - Record `EXPORT conv <id>` when conversation metadata is created/updated.
     - Record `EXPORT artifact <filename>` when artifacts are uploaded.
     - Record `EXPORT brain transcript.jsonl (+N steps)` when transcript steps or SQLite chunks are exported.
   - On `Pull` / `SyncRemoteChanges` (or daemon import):
     - Record `IMPORT conv <id>` when conversation session is pulled.
     - Record `IMPORT artifact <filename>` when artifacts are downloaded.
     - Record `IMPORT brain transcript.jsonl (+N steps)` when transcript turns or SQLite databases are reconstructed locally.
3. **`agy-sync transactions` CLI Command:**
   - **Flags:**
     - `--direction <in|out>` (or convenience `--in` / `--out` flags) to filter by data flow direction.
     - `--type <conv|artifact|brain>` (or `--conv`, `--artifact`, `--brain`) to filter by entity type.
     - `-c, --conversation <id>` to filter transactions for a specific conversation.
     - `--limit <int>` (default: 50, maximum: 1000) to limit output count.
     - `--json`: Emit structured JSON output for automation and piping.
4. **Output Presentation:**
   - Formatted terminal table showing `TIMESTAMP`, `DIRECTION` (`IMPORT`/`EXPORT`), `TYPE` (`conv`/`artifact`/`brain`), `CONVERSATION ID`, and `DETAILS`.
   - Pure JSON output when `--json` flag is provided.

## 3. Acceptance Criteria
- Running `agy-sync transactions` displays recent sync activity in a readable tabular format.
- Filtering by direction (`in`/`out`), entity type (`conv`/`artifact`/`brain`), and conversation works accurately.
- Push, pull, and daemon processes record transaction entries automatically upon successful operations.
- `--json` emits valid JSON without terminal ANSI codes.
- Transaction store operations are thread-safe and non-blocking to sync performance.
- Full test suite passes with `-race` and 0 linter issues.
