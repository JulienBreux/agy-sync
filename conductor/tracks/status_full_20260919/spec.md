# Specification: Status Command Conversation Viewport Pagination & Default Summary (`--full`)

## 1. Overview
The `agy-sync status` command currently displays daemon health and configuration, followed immediately by an unpaginated, full table of all local conversations found in the brain directory. In workspaces with dozens or hundreds of conversation transcripts, this fills or overflows the terminal scrollback buffer and makes checking overall synchronization health slow and cluttered.

This track updates `agy-sync status` to hide the granular conversations table by default, presenting a clean, compact overview of daemon status, cloud project, local paths, and total session count with an actionable hint. When `--full` is passed, `agy-sync status` displays the conversations list. If executed in an interactive terminal (TTY), it launches a viewport-aware paginated terminal viewer with keyboard arrow navigation (`Up`/`Down` or `j`/`k` for scrolling rows, `Left`/`Right` or `h`/`l` for previous/next page, `q` or `Esc` to exit). When executed in non-TTY environments (pipes, redirection, CI), it outputs the continuous tabular list cleanly. In `--json` mode, detailed conversations are omitted by default unless `--full` is specified.

## 2. Functional Requirements
1. **Default Status Behavior (Compact Summary):**
   - By default (when `--full` is not provided), `agy-sync status` outputs:
     - Header banner (`Antigravity Sync Status`)
     - Daemon status (RUNNING / STOPPED, PID, Last Polling, Daemon Log)
     - GCP Project ID, Machine ID
     - Brain Directory, Conversations Dir, Summaries DB, SQLite DB Sync
     - Total sessions count (`Sessions Found: X`)
     - An informative hint: `Run 'agy-sync status --full' to inspect conversations.`
   - The individual conversations table is completely omitted in default mode.
2. **`--full` Flag Behavior:**
   - Add flag `--full` (`bool`, default `false`) to `agy-sync status`.
   - When `--full` is provided:
     - In an interactive terminal TTY:
       - Detect terminal viewport dimensions (height and width).
       - Launch an interactive paginated viewport pager for conversations.
       - Calculate page size dynamically based on available terminal height.
       - Display a sticky header/footer with page navigation info (e.g. `Page 1 of 5 (1-10 of 48 conversations) | [↑/↓] Row  [←/→] Page  [q] Quit`).
       - Respond smoothly to arrow keys (`Up`, `Down`, `Left`, `Right`), Vim-style keys (`k`, `j`, `h`, `l`), `PageUp`, `PageDown`, `Home`, `End`, and exit on `q`, `Esc`, or `Ctrl+C`.
     - In non-interactive environments (piped stdout, non-TTY, CI, `--no-pager` or `TERM=dumb`):
       - Print the complete conversation table continuously without blocking.
3. **JSON Output (`--json`):**
   - When `globalOpts.JSON` is true:
     - If `--full` is NOT provided: omit the `conversations` array while keeping `conversations_count: X`.
     - If `--full` IS provided: include the complete `conversations` array.
4. **Filtering Support:**
   - Retain existing `-c, --conversation <id>` flag: when `-c` is passed with `--full`, filter the paginated view to the specified conversation ID.

## 3. Acceptance Criteria
- `agy-sync status` without `--full` does NOT display the conversation table; displays clean summary and session count with the `--full` hint.
- `agy-sync status --full` in interactive TTY renders interactive viewport-aware paginator with arrow navigation and pagination footer.
- Pressing `Left`/`Right` or `h`/`l` navigates pages; `Up`/`Down` navigates rows; `q`/`Esc` exits cleanly restoring terminal state.
- `agy-sync status --full` when piped outputs standard full tabular output without hanging or raw ANSI control codes.
- `agy-sync status --json` omits `conversations` array by default; `agy-sync status --full --json` includes `conversations`.
- Unit tests verify flag handling, viewport calculation, page slicing, JSON omission/inclusion, and keyboard event dispatch.
- Full test suite passes with `go test -race ./...` and `golangci-lint run ./...` with 0 issues.
