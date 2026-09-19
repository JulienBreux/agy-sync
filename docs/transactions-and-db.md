# Transactions Audit & Database Operations

`agy-sync` provides built-in audit logging and administrative database management commands to inspect sync activity and manage cloud resources safely.

---

## 1. Transaction Audit Logging (`agy-sync transactions`)

Every synchronization event (push or pull of a conversation, transcript step, or artifact) is recorded in a local SQLite transaction audit database (`transactions.db`). This allows developers to trace exactly when and how data moved between workstations.

### Command Usage

```bash
# Display recent synchronization transactions (tabular output)
agy-sync transactions

# Filter by direction: IMPORT or EXPORT
agy-sync transactions --in
agy-sync transactions --out
agy-sync transactions --direction in

# Filter by entity type: conversation, artifact, or brain asset
agy-sync transactions --conv
agy-sync transactions --artifact
agy-sync transactions --brain
agy-sync transactions --type artifact

# Filter by conversation ID and limit results
agy-sync transactions -c 624296c6-d623-4c39-92d4-3906f8c07140 --limit 20

# Paginate audit log
agy-sync transactions --limit 10 --offset 20

# Output machine-readable JSON
agy-sync transactions --json
```

### Flags Reference

| Flag | Short | Description | Default |
| :--- | :--- | :--- | :--- |
| `--direction` | | Filter by direction: `'in'` (IMPORT) or `'out'` (EXPORT) | (all) |
| `--in` | | Convenience flag to filter to inbound (IMPORT) transactions | `false` |
| `--out` | | Convenience flag to filter to outbound (EXPORT) transactions | `false` |
| `--type` | | Filter by entity type: `'conv'`, `'artifact'`, or `'brain'` | (all) |
| `--conv` | | Convenience flag to filter to conversation transactions | `false` |
| `--artifact` | | Convenience flag to filter to artifact transactions | `false` |
| `--brain` | | Convenience flag to filter to brain transactions | `false` |
| `--conversation` | `-c` | Filter by specific conversation ID | (all) |
| `--limit` | `-n` | Maximum number of transactions to display | `50` |
| `--offset` | | Number of transactions to skip for pagination | `0` |
| `--db` | | Override path to transactions SQLite database | `~/.config/agy-sync/transactions.db` |
| `--json` | | Output transactions array in JSON format | `false` |

### Sample Terminal Output

```text
TIMESTAMP            ACTION    TYPE        CONVERSATION ID                       ENTITY                DETAILS
--------------------------------------------------------------------------------------------------------------
2026-09-19 08:35:10  EXPORT    conv        624296c6-d623-4c39-92d4-3906f8c07140  624296c6-...          steps: 42
2026-09-19 08:35:10  EXPORT    artifact    624296c6-d623-4c39-92d4-3906f8c07140  design.md             size: 1024 bytes
2026-09-19 08:35:10  EXPORT    brain       624296c6-d623-4c39-92d4-3906f8c07140  transcript.jsonl      +3 steps
2026-09-19 08:36:22  IMPORT    brain       624296c6-d623-4c39-92d4-3906f8c07140  transcript.jsonl      +3 steps
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
