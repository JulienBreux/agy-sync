# Technology Stack: agy-sync

## 1. Core Language & Runtime
- **Language:** Go (Golang) 1.23+
- **Rationale:** Compiled static binary, zero runtime dependencies, minimal memory footprint for background daemons, and native concurrency for streaming sync.

## 2. CLI & Configuration
- **CLI Engine:** `spf13/cobra` (structured subcommands, flags, auto-generated shell completions).
- **Configuration:** `spf13/viper` (supports config file, flags, and `AGY_SYNC_*` environment variables).

## 3. Cloud & Data Layer
- **Database:** Google Cloud Firestore (`cloud.google.com/go/firestore`) for conversation documents, steps, metadata, and real-time snapshot listeners.
- **Object Storage:** Google Cloud Storage (`cloud.google.com/go/storage`) for large artifacts and binary blobs (>1MB).
- **Auth:** Standard Google Cloud Application Default Credentials (ADC).

## 4. Filesystem Watcher & Event Loop
- **Watcher:** `github.com/fsnotify/fsnotify` for non-blocking, low-overhead filesystem event notifications.
- **Ingestion Pipeline:** Buffered channel-based debouncing and worker pool for reliable batch writes.

## 5. Testing & Tooling
- **Testing:** Go standard `testing` + `github.com/stretchr/testify`.
- **Local Testing:** Google Cloud Firestore Emulator for offline, hermetic integration test suites.
- **Linting:** `golangci-lint` + `gofmt`.
