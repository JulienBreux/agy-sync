# agy-sync

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Production%20Ready-green.svg)](#)

> **Asynchronous bidirectional synchronization and cloud storage engine for Google Antigravity (AGY) conversations, transcripts, and artifacts.**

---

## Overview

**`agy-sync`** is a high-performance Go CLI and background daemon designed to synchronize Google Antigravity (AGY) sessions across multiple developer workstations in real-time using Google Cloud Firestore.

Antigravity stores session states, tool executions, and generated artifacts locally within `<appDataDir>/brain/<conversation-id>`. When switching between laptops, workstations, or remote environments, conversation history and context are fragmented. `agy-sync` bridges this gap by transforming local session data into an interconnected, queryable, cloud-synchronized file system.

### Key Capabilities

- **Bidirectional Multi-Machine Sync:** Keep *n* machines synchronized in real-time. Changes written on Machine Alpha stream to Firestore and are automatically pulled onto Machine Beta.
- **Append-Only Step Log:** Monotonic step indices (`step_000001`, `step_000002`, ...) prevent merge conflicts, data loss, or out-of-order writes.
- **Loop Prevention:** Every sync payload is tagged with the origin machine's unique `MachineID`. Machines automatically ignore echoes of their own updates.
- **Real-time Filesystem Watcher:** Non-blocking `fsnotify` watcher detects incremental transcript appends (`transcript.jsonl`) and new artifact files with sub-second latency and debouncing.
- **Byte-Offset Incremental Streaming:** Efficiently reads only newly appended JSONL bytes without re-parsing entire conversation histories.
- **Artifact Hash Deduplication:** SHA256 checksum comparison avoids redundant network uploads for unchanged artifacts.
- **Production-Grade Observability:** Leveled logging, structured error messages with user remediation hints, and full `--json` output support for scripting and automation.

---

## Architecture & Data Flow

### Multi-Machine Synchronization Sequence

```mermaid
sequenceDiagram
    autonumber
    actor DevA as Developer (Machine Alpha)
    participant WatcherA as agy-sync (Machine Alpha)
    participant FS as Google Cloud Firestore
    participant DaemonB as agy-sync (Machine Beta)
    actor DevB as Developer (Machine Beta)

    DevA->>WatcherA: Appends step to transcript.jsonl
    WatcherA->>FS: BulkWriter push: step + updated SourceMachine ("Alpha")
    Note over FS: Document /conversations/{id}<br/>Subcollection /steps/{step_index}
    DaemonB->>FS: Poll / Real-time listener detects change
    DaemonB->>DaemonB: Check SourceMachine != "Beta" (Loop Prevention: PASS)
    DaemonB->>DaemonB: Stream new steps & reconstruct transcript.jsonl
    DevB->>DaemonB: Local brain updated with Machine Alpha's context
```

### Ingestion & Watcher Engine Flow

```mermaid
flowchart TD
    A["File System Event (fsnotify)"] --> B{"Event Path Match?"}
    B -->|"transcript.jsonl modified"| C["Read from byte offset"]
    B -->|"New artifact file created"| D["Compute SHA256 checksum"]
    B -->|"Other / system-generated"| E["Ignore (.system_generated)"]

    C --> F["Parse JSONL Step Record"]
    D --> G{"SHA256 Changed?"}
    G -->|"No"| H["Skip upload"]
    G -->|"Yes"| I["Queue Firestore Artifact write"]

    F --> J["Firestore BulkWriter (Throttled Batch)"]
    I --> J
    J --> K["Update /conversations/{id} Metadata (SourceMachine, LastSyncedStep)"]
```

---

## Firestore Data Schema

`agy-sync` structures conversations in Google Cloud Firestore using a hierarchical subcollection layout:

```
conversations/
└── {conversation_id}
    ├── metadata: { id, title, created_at, updated_at, last_synced_step, source_machine }
    ├── steps/
    │   ├── step_000001: { step_index, source, type, status, content, thinking, tool_calls, machine_id, timestamp }
    │   └── step_000002: { ... }
    └── artifacts/
        ├── {artifact_hash_or_name}: { path, mime_type, size_bytes, sha256, content_blob, updated_at }
        └── ...
```

### Collections & Documents

| Path | Purpose | Key Attributes |
| :--- | :--- | :--- |
| `/conversations/{id}` | Conversation parent document | `last_synced_step`, `source_machine`, `updated_at` |
| `/conversations/{id}/steps/{stepIndex}` | Immutable transcript turn | `step_index`, `type`, `source`, `content`, `machine_id` |
| `/conversations/{id}/artifacts/{id}` | User/planner generated artifacts | `path`, `sha256`, `size_bytes`, `content` |

---
