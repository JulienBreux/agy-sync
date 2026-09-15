# Product Guidelines: ayg-conv-to-fs

## 1. CLI Experience & Design Philosophy
- **Human-Centric Terminal Output:** Clean, unobtrusive progress indicators, concise colored status updates, and graceful spinners during network calls.
- **Machine Readability:** Global `--json` flag on query/status commands for easy automation and scripting.
- **Predictable Exit Codes:** Standard POSIX exit codes (0 for success, 1 for runtime error, 2 for config/auth error).

## 2. Observability & Logging Standards
- **Structured Leveled Logging:** `DEBUG`, `INFO`, `WARN`, `ERROR`.
- **Dual Destination:** Formatted console output for interactive runs, rotating log files for background daemon mode.
- **Correlation & Machine IDs:** Every sync batch carries a machine ID and transaction timestamp for unambiguous tracing.

## 3. Reliability & Resilience Principles
- **Offline-First Resilience:** Queue pending local transcript changes when disconnected from Firestore.
- **Exponential Backoff & Retries:** Automatic retries with jitter on transient network failures.
- **Actionable Remediation:** Clean user-facing error explanations detailing the fix rather than raw stack traces.
- **Strict Idempotency:** Sync operations must be safe to re-run repeatedly without duplicate steps or file corruption.

## 4. Security & Privacy Defaults
- **Standard GCP ADC:** Application Default Credentials (`GOOGLE_APPLICATION_CREDENTIALS` / gcloud auth) with zero local secret leakage.
- **Data Protection:** Support optional redaction of sensitive credentials or tokens in transcripts prior to cloud storage upload.
