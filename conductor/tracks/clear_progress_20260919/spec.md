# Specification: Display Progress for Database Clear Operation

## 1. Overview
When clearing remote Firestore data via `agy-sync clear`, users currently see generic standard logger output (`INFO Clearing all conversations...`) followed by a silent pause while remote documents and subcollections are deleted. This lack of feedback can lead to uncertainty about whether the operation is running or frozen. 

This feature replaces raw log output during `agy-sync clear` with an interactive, reassuring terminal progress display featuring live spinners, real-time counters (`[X/N]`), conversation IDs, and subcollection statistics.

## 2. Functional Requirements
1. **Interactive Progress Display:**
   - For TTY terminals (non-`--json` and interactive runs), render a dynamic spinner and progress indicator.
   - Distinct phases:
     - **Discovery Phase:** Display `⠋ Scanning remote Firestore database...` followed by `Found N conversation(s) to clear`.
     - **Deletion Phase:** For each conversation, display dynamic progress:
       `⠋ [X/N] Deleting conversation <id> (<steps> steps, <artifacts> artifacts, <chunks> db chunks)...`
       Transitioning to `✔ [X/N] Deleted conversation <id>`.
     - **Completion Phase:** Display final summary: `✔ Successfully purged N conversation(s) and associated subcollections from Firestore.`
2. **Targeted Deletion (`--conversation <id>`):**
   - When clearing a single conversation, display targeted progress:
     `⠋ Deleting conversation <id>...` followed by `✔ Successfully cleared conversation <id>`.
3. **Log Output Management:**
   - Mute standard `INFO` level logs from reaching stderr during interactive progress mode to keep terminal output clean and focused.
   - If `-v, --verbose` or `--log-level=debug` is specified, preserve full debug logging.
4. **CI & Non-TTY Compatibility:**
   - When stdout is not a TTY (e.g., piped to a file, running in CI/CD automation) or when `--json` is supplied, disable interactive ANSI animation.
   - For non-TTY environments, emit clean sequential status lines without carriage-return animation.
   - For `--json` mode, output strictly valid JSON upon completion without any progress decoration.
5. **Syncer Engine Progress Protocol:**
   - Extend `syncer.ClearOptions` with a progress callback `OnProgress func(event ClearProgressEvent)` so progress reporting is decoupled from the terminal UI logic.

## 3. Acceptance Criteria
- `agy-sync clear` provides real-time visual progress showing discovery count and per-conversation deletion progress.
- Raw INFO logs do not pollute the interactive terminal during progress execution.
- `--verbose` retains full debug log streams.
- `--json` produces pure, parseable JSON without ANSI codes.
- Non-TTY / automated environments output clean plain-text line-based updates without ANSI artifacts.
- Unit and E2E tests verify progress callback invocation and output formatting.
