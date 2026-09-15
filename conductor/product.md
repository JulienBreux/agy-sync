# Initial Concept

A tool to convert Antigravity (AGY) conversations, transcripts, and artifacts into structured files on a firestore database asynchronously.
The idea is also to offer the capability to sync from another Antigravity machine.
The final idea is to sync the conversations between n machines.

# Product Definition: ayg-conv-to-fs

## Vision & Purpose
`ayg-conv-to-fs` is a high-performance synchronization and storage engine designed for Google Antigravity (AGY). It asynchronously ingests AGY conversation transcripts (`transcript.jsonl`), metadata, and generated artifacts, persists them into Google Cloud Firestore (with Cloud Storage integration for large assets), and enables seamless bidirectional synchronization across *n* developer machines.

## Core Problem
Antigravity stores session states, tool executions, and artifacts locally within `<appDataDir>/brain/<conversation-id>`. When switching between workstations or collaborating across machines, conversation history and context are fragmented. `ayg-conv-to-fs` bridges this gap by turning local conversation data into an interconnected, queryable, cloud-synchronized filesystem.

## Key Capabilities & Architecture
1. **Asynchronous Local Ingestion & Watcher:**
   - Monitors the Antigravity conversation directory (`<appDataDir>/brain/`) using non-blocking filesystem events.
   - Parses `transcript.jsonl` and full logs incrementally.
   - Normalizes steps (user inputs, planner reasoning, tool calls, and outputs) into structured entities.

2. **Bidirectional Multi-Machine Synchronization:**
   - Real-time Firestore snapshot listeners to detect updates originating from other registered machines.
   - Reconstructs remote conversations, logs, and artifacts accurately onto the local filesystem.
   - Machine identifier tagging to track sync states and prevent echo loops.

3. **Hybrid Scalable Storage:**
   - **Firestore:** Structured conversation metadata, step indices, tool summaries, and conversation hierarchy.
   - **Cloud Storage:** Offloading large artifacts (images, code snapshots, data exceeding 1MB limits).

4. **Append-Only Conflict Resolution:**
   - Immutable step sequences tagged with machine IDs, logical timestamps, and step indices, merging cleanly without data loss.

5. **CLI & Core Library:**
   - Ergonomic CLI (`ayg-sync watch`, `push`, `pull`, `status`) + clean modular library.
