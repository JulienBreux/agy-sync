# Repository Guidelines for agy-sync

## 1. Hermetic Testing & Mock Factory Invariant
- **No Ambient Credentials in Tests:** Tests in all packages (including external `test/` packages executing `cmd.Execute()`) must NEVER rely on ambient developer credentials (e.g., GCP Application Default Credentials / ADC).
- **Hermetic Isolation:** All tests must pass in isolated, credential-less environments like GitHub Actions CI/CD.
- **Factory Stubs:** When testing CLI commands or packages interacting with external services (such as Firestore), use exported factory injectors (e.g., `cmd.SetFirestoreClientFactory(...)` with `defer cmd.ResetFirestoreClientFactory()`) and provide in-memory implementations (e.g., `firestore.NewMemoryRepository()`).

## 2. Terminal Raw Mode & TUI Formatting
- **CRLF Line Endings in Raw Mode:** When placing a terminal in raw mode for interactive pagers or TUIs, always translate newlines (`\n`) to CRLF (`\r\n`) (e.g., using `pager.ToCRLF`) to prevent diagonal staircasing.
- **Column Alignment & Cursor Padding:** When rendering selectable rows, ensure that active cursor indicators (e.g., `> `) and unselected row prefixes (e.g., `  `) have identical character widths so table columns remain strictly aligned across all rows.
