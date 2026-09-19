# Specification: Cloud Setup and Environment Verification Command (`agy-sync setup`)

## 1. Overview
Before running `agy-sync`, developers and automated environments must verify and prepare their Google Cloud Platform (GCP) prerequisites: active Application Default Credentials (ADC), target project access, required service APIs enabled (Firestore, Cloud Resource Manager, Service Usage, and Cloud Storage), and Firestore database existence/readiness.

This track introduces the `agy-sync setup` command. It inspects local credentials and configuration, queries GCP to verify required APIs and permissions, confirms or provisions the target Firestore database (in Native mode), and checks Google Cloud Storage access for artifact offloading. It includes a `--dry-run` mode to only inspect and report access without making any mutations, provides interactive authentication triggers or actionable remediation instructions if credentials or permissions are missing, and formats results with a rich visual status checklist and `--json` support.

## 2. Functional Requirements
1. **Authentication & Credential Check:**
   - Detect and validate Google Cloud Application Default Credentials (ADC) via `google.golang.org/api` / `golang.org/x/oauth2/google`.
   - If missing or expired:
     - In interactive terminal mode: offer to launch `gcloud auth application-default login` directly.
     - In non-interactive/CI mode: fail gracefully with remediation advice (`gcloud auth application-default login` or setting `GOOGLE_APPLICATION_CREDENTIALS`).
2. **Project & API Enablement Verification:**
   - Verify access to the configured `project_id` using Cloud Resource Manager.
   - Check status of required APIs:
     - `firestore.googleapis.com` (Google Cloud Firestore API)
     - `cloudresourcemanager.googleapis.com` (Cloud Resource Manager API)
     - `storage.googleapis.com` (Cloud Storage API)
   - If an API is disabled:
     - In standard mode without `--dry-run`: offer remediation (`gcloud services enable <service>`) or programmatic enablement if authorized.
3. **Firestore Database Verification & Creation:**
   - Verify existence and operational state of the target Firestore database (`(default)` or custom).
   - If missing:
     - With `--dry-run`: report database as missing `[!] Database does not exist (dry-run, skipping creation)`.
     - In standard mode: prompt for confirmation before creation (unless `--yes` / `-y` is provided).
     - Upon confirmation or `--yes`: provision Firestore database in Native mode (default location `nam5` or user-specified).
   - Verify read/write permissions on the target database by testing connectivity and token authorization.
4. **Cloud Storage Verification:**
   - Verify Cloud Storage API access and check bucket access/permissions for artifact offloading if configured.
5. **Local Environment & Configuration Check:**
   - Verify local configuration file (`~/.config/agy-sync/config.yaml`), Antigravity brain directory (`~/.gemini/antigravity-cli/brain`), conversations directory, and SQLite databases readability/writeability.
6. **CLI Command Interface (`agy-sync setup`):**
   - **Command:** `agy-sync setup` (Aliases: `check`, `doctor`) under `setup` group.
   - **Flags:**
     - `--dry-run`: Read-only verification pass; do not create database, enable APIs, or modify local files.
     - `-y, --yes`: Non-interactive auto-confirmation for database creation and API enablement.
     - `--project-id <string>`: Override target GCP Project ID.
     - `--database-id <string>`: Override target Firestore Database ID.
     - `--location <string>`: Target Firestore database region/location for creation (default: `nam5`).
     - `--json`: Output full structured diagnostic report in JSON.
7. **Output Presentation:**
   - Rich visual checklist rendered to terminal with status indicators (`[✓] PASS`, `[✗] FAIL`, `[!] WARN`, `[?] PENDING`), step durations, and clear troubleshooting remediation hints.
   - Machine-readable JSON output schema when `--json` is specified.

## 3. Acceptance Criteria
- `agy-sync setup` successfully diagnoses all 5 verification pillars: ADC authentication, project accessibility, API enablement, Firestore database existence/access, and Cloud Storage access.
- `--dry-run` performs pure read-only validation without prompting or triggering any resource creation.
- If database is missing in standard mode, interactive prompt asks before creation; `--yes` skips prompt and creates database.
- If credentials are missing, terminal offers interactive login or provides clear remediation command.
- `--json` outputs clean, well-typed JSON diagnostic payload with exit code 0 on all checks passed and exit code 1 on failures.
- Unit and mock integration tests cover all check scenarios (success, auth failure, missing APIs, missing database, dry-run).
- Full test suite passes with `-race` and 0 linter issues.
