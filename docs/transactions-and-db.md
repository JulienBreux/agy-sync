# Transactions Audit & Database Operations

`agy-sync` provides built-in audit logging and administrative database management commands to inspect sync activity and manage cloud resources safely.

---

## 1. Transaction Audit Logging (`agy-sync transactions`)

Every synchronization event (push or pull of a conversation, transcript step, or artifact) is recorded in a local SQLite transaction audit database (`transactions.db`). This allows developers to trace exactly when and how data moved between workstations.

### Command Usage

```bash
# View recent sync transactions in tabular format
agy-sync transactions

# Filter by direction: incoming (pulled from cloud)
agy-sync transactions --direction in

# Filter by direction: outgoing (pushed to cloud)
agy-sync transactions --direction out

# Filter by entity type (conversation, artifact, or brain)
agy-sync transactions --entity-type artifact

# Trace all events for a specific conversation
agy-sync transactions --conversation-id 624296c6-d623-4c39-92d4-3906f8c07140

# Paginate audit log
agy-sync transactions --limit 10 --offset 20

# Output structured JSON for log collectors or scripts
agy-sync transactions --json
```

### Flags Reference

| Flag | Short | Description | Default |
| :--- | :--- | :--- | :--- |
| `--direction` | `-d` | Filter by sync direction (`in` or `out`) | (all) |
| `--entity-type` | `-e` | Filter by entity type (`conversation`, `artifact`, `brain`) | (all) |
| `--conversation-id` | `-c` | Filter transactions for a single conversation UUID | (all) |
| `--limit` | `-l` | Maximum number of transactions to display | `50` |
| `--offset` | `-o` | Number of transactions to skip for pagination | `0` |
| `--db` | | Path to SQLite transactions database file | `~/.gemini/antigravity-cli/transactions.db` |
| `--json` | | Output transactions array in JSON format | `false` |

### Sample Terminal Output

```
+----+---------------------+-----------+--------------+--------------------------------------+------------------------------+
| ID | TIMESTAMP (UTC)     | DIRECTION | ENTITY TYPE  | CONVERSATION ID                      | DETAILS                      |
+----+---------------------+-----------+--------------+--------------------------------------+------------------------------+
| 42 | 2026-09-19 10:14:02 | OUT       | conversation | 624296c6-d623-4c39-92d4-3906f8c07140 | Pushed 3 steps               |
| 41 | 2026-09-19 10:14:03 | OUT       | artifact     | 624296c6-d623-4c39-92d4-3906f8c07140 | Pushed plan.md (14.2 KB)     |
| 40 | 2026-09-19 10:12:11 | IN        | conversation | a1b2c3d4-e5f6-7890-1234-567890abcdef | Pulled 1 step from macbook-2 |
+----+---------------------+-----------+--------------+--------------------------------------+------------------------------+
Showing 3 of 42 transactions.
```

---

## 2. Remote Database Cleanup (`agy-sync clear`)

During development, end-to-end testing, or environment resets, developers may need to purge synchronized Antigravity conversations from Google Cloud Firestore.

The `clear` command safely removes remote conversation documents and associated step sub-collections while preventing accidental data loss through multiple protection layers.

> [!WARNING]
> `agy-sync clear` deletes synchronized data from **Firestore**. It does not delete local files in your brain directory unless explicitly re-synced.

### Safety Safeguards

1. **Interactive Confirmation**: When run interactively, the CLI requires the user to type `yes` to confirm deletion.
2. **Dry-Run Preview (`--dry-run`)**: Scans Firestore and reports the exact number of conversations and sub-collections that would be deleted without performing any modifications.
3. **CI/Automation Flag (`--force`)**: Bypasses interactive prompts for automated testing pipelines and scripts.

### Command Usage

```bash
# Preview what would be deleted without modifying Firestore
agy-sync clear --dry-run

# Interactive deletion (prompts: "Type 'yes' to proceed: ")
agy-sync clear

# Non-interactive deletion (for automated test scripts)
agy-sync clear --force
```

### Flags Reference

| Flag | Short | Description | Default |
| :--- | :--- | :--- | :--- |
| `--dry-run` | | Scan and preview documents to be removed without deleting | `false` |
| `--force` | `-f` | Bypass interactive confirmation prompt | `false` |
| `--config` | | Custom path to configuration file | `~/.gemini/antigravity-cli/config.yaml` |

### Sample Outputs

#### Dry-Run Preview
```
Scanning Firestore for synchronized conversations...
Found 14 conversation documents and 82 trajectory steps in project 'my-gcp-project'.
[DRY RUN] 14 conversations would be deleted. No data was modified.
```

#### Interactive Execution
```
Scanning Firestore for synchronized conversations...
Found 14 conversation documents.
Are you sure you want to delete all 14 remote conversations?
This action cannot be undone. Type 'yes' to proceed: yes
Deleting 14 conversations from Firestore...
✓ Successfully cleared 14 conversations from Firestore.
```
