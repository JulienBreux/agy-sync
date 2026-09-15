# Specification: Modernize Go Application Architecture, CLI Ergonomics & Best Practices

## 1. Overview
Modernize `agy-sync` to adhere to modern Go (1.27+) idiomatic patterns, standard library capabilities, and enterprise CLI design. This includes migrating to standard `log/slog` structured logging, encapsulating domain packages into `internal/`, adopting `signal.NotifyContext` and `errgroup`, adding a comprehensive `version` command with build metadata, organizing Cobra command groups, and standardizing CLI exit codes.

## 2. Functional Requirements

### 2.1 Codebase Package Architecture & Encapsulation
- Restructure repository layout to move domain-specific packages to `internal/`:
  - `internal/daemon`: Process lifecycle, PID file, signal dispatch.
  - `internal/syncer`: Sync engine and Firestore synchronization logic.
  - `internal/watcher`: Filesystem debounced watcher.
  - `internal/parser`: Transcript and JSONL parser.
  - `internal/discovery`: Local brain discovery utilities.
  - `internal/firestore`: Firestore client and repository implementations.
- Retain shared configuration and models in public `pkg/`:
  - `pkg/config`: User configuration loading and validation.
  - `pkg/models`: Shared domain models and entities.

### 2.2 Standard Structured Logging (`log/slog`)
- Implement `internal/logger` wrapping Go's standard `log/slog`.
- Support `--log-level` flag (`debug`, `info`, `warn`, `error`), defaulting to `info` (or `debug` when `--verbose` / `-v` is provided).
- Support dual format:
  - Text handler by default for interactive CLI human readability.
  - JSON handler (`slog.NewJSONHandler`) when `--json` or `--log-format=json` is active, or when running in background daemon mode.
- Replace ad-hoc `fmt.Println` and `cmd.Print*` in background engine/watcher components with contextual structured logging (`slog.InfoContext`, `slog.ErrorContext`, `slog.DebugContext`).

### 2.3 Context & Concurrency Modernization
- Replace custom signal loops in `cmd/root.go` / `cmd/start.go` with `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` ensuring unified graceful shutdown.
- Refactor multi-artifact sync in `syncer` to utilize `golang.org/x/sync/errgroup` with bounded parallelism (e.g. max 5 concurrent file transfers).

### 2.4 CLI Ergonomics, Versioning & Command Grouping
- **`agy-sync version` Command:**
  - Print version, git commit, build date, Go runtime version, OS/Arch.
  - Integrate with `runtime/debug.ReadBuildInfo()` for embedded Go module info, with ldflags fallbacks.
  - Support `--json` output format for scripting and CI pipelines.
- **Cobra Command Groups:**
  - Group commands in help output:
    - *Daemon Management:* `start`, `stop`, `status`
    - *Data Synchronization:* `push`, `pull`
    - *Configuration & Setup:* `init`
- **Status Reporting Enhancements:**
  - Record and display the last polling / sync timestamp (`LastPolledAt` / `LastSyncedAt`) in `agy-sync status` (human-readable table and `--json` format).
- **Standardized Exit Codes (`internal/exitcode`):**
  - Define semantic exit constants: `Success (0)`, `GeneralError (1)`, `UsageError (2)`, `ConfigError (3)`, `DaemonError (4)`.

## 3. Acceptance Criteria
- [ ] Internal packages moved to `internal/` with all import paths cleanly updated.
- [ ] `log/slog` active across all packages with configurable levels and text/JSON format.
- [ ] `agy-sync version` outputs detailed build information in text and JSON.
- [ ] Cobra help displays organized command groups.
- [ ] Artifact uploads/downloads utilize `errgroup` concurrency.
- [ ] Root command uses `signal.NotifyContext`.
- [ ] Standardized exit codes applied across all commands.
- [ ] 100% passing tests with statement coverage maintained (>80%) and `golangci-lint` passes with 0 issues.
