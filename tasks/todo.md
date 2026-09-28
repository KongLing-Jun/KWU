# Unified Development Cloud Environment Tasks

Status: Implementation in progress; migration `00090` approved and applied.

Specification: `docs/specs/unified-development-cloud-environment.md`

Plan: `tasks/plan.md`

## Task 1: Unify Cloud configuration sources

**Description:** Add one resolver with explicit managed path, process environment, user-level machine configuration, checked-in development defaults, and built-in fallback precedence. Stop treating the worktree `.env` as an implicit configuration source.

**Acceptance criteria:**

- [x] A fresh worktree without `.env` loads safe development defaults.
- [x] Process values override user-level and checked-in values; explicit managed configuration resolves predictably for CI/deployment.
- [x] A detected legacy `.env` causes a redacted migration/conflict diagnostic instead of silent loading.

**Verification:**

- [x] `go test ./internal/config -count=1`
- [x] `git diff --check`

**Dependencies:** None

**Files likely touched:**

- `Reasonix-Cloud/internal/config/config.go`
- `Reasonix-Cloud/internal/config/config_test.go`
- `Reasonix-Cloud/config/development.yaml`

**Estimated scope:** Medium, 3 files

## Task 2: Provide one-time user configuration setup

**Description:** Add a command that validates or migrates an existing `.env` into the user-level configuration location without printing secrets. It must use restrictive file permissions and require explicit input rather than scanning unrelated worktrees.

**Acceptance criteria:**

- [x] The command imports one explicitly named legacy file into the user-level configuration path.
- [x] Existing destination data is not overwritten without an explicit replace flag.
- [x] Output reports only source/destination metadata and validation status, never values.

**Verification:**

- [x] `go test ./cmd/configuredev -count=1`
- [x] A fixture containing sentinel credentials does not leak the sentinel into stdout/stderr.

**Dependencies:** Task 1

**Files likely touched:**

- `Reasonix-Cloud/cmd/configuredev/main.go`
- `Reasonix-Cloud/cmd/configuredev/main_test.go`
- `Reasonix-Cloud/internal/config/config.go`
- `Reasonix-Cloud/internal/config/config_test.go`

**Estimated scope:** Medium, 4 files

## Task 3: Converge Cloud database lifecycle commands

**Description:** Make migration and local database lifecycle commands use the unified resolver instead of calling `LoadDotEnv(".env")` independently.

**Acceptance criteria:**

- [x] Migration, development database, bootstrap, and invariant commands resolve the same database target as the server.
- [x] No command silently reads a worktree `.env`.
- [x] Existing safety checks for destructive database operations remain intact.

**Verification:**

- [x] `go test ./cmd/migrate ./cmd/devdb ./cmd/bootstrapdevdb ./cmd/projectinvariants -count=1`
- [x] Target parsing and pre-connection safety tests succeed with no implicit `.env` load.

**Dependencies:** Task 1

**Files likely touched:**

- `Reasonix-Cloud/cmd/migrate/main.go`
- `Reasonix-Cloud/cmd/devdb/main.go`
- `Reasonix-Cloud/cmd/bootstrapdevdb/main.go`
- `Reasonix-Cloud/cmd/projectinvariants/main.go`

**Estimated scope:** Medium, 4 files

### Checkpoint A: Cloud configuration foundation

- [x] Tasks 1-3 tests pass.
- [ ] A clean temporary Cloud worktree has no `.env` dependency.
- [x] No credential sentinel appears in logs, diagnostics, Git diff, or generated artifacts.

## Task 4: Converge Cloud support commands

**Description:** Make administrative and development support commands use the same resolver while preserving each command's explicit flags and existing behavior.

**Acceptance criteria:**

- [x] Dev token, CSV SMS, and admin creation commands use the unified configuration path.
- [x] An explicit command-level configuration file remains explicit and does not restore automatic `.env` discovery.
- [x] Help text documents the user-level configuration behavior.

**Verification:**

- [x] `go test ./cmd/devtoken ./cmd/smssendcsv ./cmd/createadmin -count=1`
- [x] `go vet ./cmd/devtoken ./cmd/smssendcsv ./cmd/createadmin`

**Dependencies:** Tasks 1-2

**Files likely touched:**

- `Reasonix-Cloud/cmd/devtoken/main.go`
- `Reasonix-Cloud/cmd/smssendcsv/main.go`
- `Reasonix-Cloud/cmd/createadmin/main.go`
- `Reasonix-Cloud/cmd/createadmin/main_test.go`

**Estimated scope:** Medium, 4 files

## Task 5: Identify a durable Cloud environment identity

**Description:** Inspect existing authoritative database metadata and prove whether it can supply a stable non-secret environment identifier. Record the chosen source and collision/lifecycle behavior before adding an API.

**Acceptance criteria:**

- [x] The identifier is stable across Cloud process restarts and worktree changes.
- [x] Two different databases cannot be accepted as the same environment by configuration alone.
- [x] If no existing durable identifier is suitable, implementation pauses before creating a migration.

**Verification:**

- [x] Add a focused test or read-only database query proving stability and mismatch behavior.
- [x] Update the ADR/spec with the finding that a dedicated singleton identity migration is required.

**Dependencies:** Task 1

**Files likely touched:**

- `Reasonix-Cloud/internal/database/postgres.go`
- `Reasonix-Cloud/internal/database/postgres_test.go`
- `Reasonix/docs/decisions/0001-unified-development-cloud-environment.md`

**Estimated scope:** Medium, 3 files

## Task 6: Enforce and expose Cloud environment identity

**Description:** Compare the expected shared-environment identity against authoritative database state before serving authenticated APIs, then expose only the stable identifier through a minimal non-secret endpoint or health metadata.

**Acceptance criteria:**

- [x] Correct identity reaches ready state; missing or mismatched identity fails closed before authenticated routes are served.
- [x] The public response contains the identity and schema version only, with no database/configuration details.
- [x] Local disposable development remains possible under its explicitly selected profile.

**Verification:**

- [x] `go test ./internal/config ./internal/http -count=1`
- [x] Negative test supplies the wrong expected identity and observes startup/readiness rejection.

**Dependencies:** Task 5

**Files likely touched:**

- `Reasonix-Cloud/internal/config/config.go`
- `Reasonix-Cloud/internal/config/config_test.go`
- `Reasonix-Cloud/internal/http/router.go`
- `Reasonix-Cloud/internal/http/router_test.go`

**Estimated scope:** Medium, 4 files

### Checkpoint B: Cloud identity contract

- [x] Tasks 4-6 tests pass.
- [x] Server, worker, migration, and support commands agree on the configuration source.
- [x] Correct and incorrect identity integration cases behave as specified.
- [x] No database migration was added without separate approval.

## Task 7: Define the runtime manifest and status command

**Description:** Create the canonical PowerShell launcher skeleton and an allowlisted, atomic, non-secret runtime manifest. Implement dynamic worktree/revision resolution and read-only `status` behavior first.

**Acceptance criteria:**

- [ ] `status` reports selected repository roots, branches, commits, normalized origins, identities, PIDs, ports, and start times.
- [ ] Manifest writes are atomic and reject unknown or secret-like fields.
- [ ] Malformed, stale, or path-escaping manifest data is rejected without affecting processes.

**Verification:**

- [ ] `pwsh -NoProfile -File .\scripts\reasonix-dev-test.ps1 -Case Manifest`
- [ ] A sentinel secret-field fixture is rejected and not echoed.

**Dependencies:** Task 6

**Files likely touched:**

- `Reasonix/scripts/reasonix-dev.ps1`
- `Reasonix/scripts/reasonix-dev-test.ps1`

**Estimated scope:** Small, 2 files

## Task 8: Implement owned process lifecycle

**Description:** Add `up`, `verify`, and `down` using the manifest contract. Start replacement processes without disrupting the current stack, validate command line/worktree/listener ownership, and perform only a verified scoped cutover.

**Acceptance criteria:**

- [ ] A foreign listener or stale PID blocks startup and is never stopped.
- [ ] `down` terminates only a process tree whose PID, start time, command line, worktree, and listener agree with the manifest.
- [ ] The current runtime remains usable until replacement verification passes.

**Verification:**

- [ ] `pwsh -NoProfile -File .\scripts\reasonix-dev-test.ps1 -Case Lifecycle`
- [ ] Negative test starts a foreign listener and proves it survives failed `up` and `down`.

**Dependencies:** Task 7

**Files likely touched:**

- `Reasonix/scripts/reasonix-dev.ps1`
- `Reasonix/scripts/reasonix-dev-test.ps1`
- `Reasonix/scripts/start-integration-20260809.ps1`

**Estimated scope:** Medium, 3 files

### Checkpoint C: Launcher ownership

- [ ] Tasks 7-8 contract tests pass.
- [ ] `status` is truthful for both running and stale manifests.
- [ ] Foreign-process negative tests pass.
- [ ] Manifest and launcher logs contain no secret values.

## Task 9: Bind Admin to the manifest Cloud

**Description:** Make integrated Admin development resolve its Vite proxy from the verified runtime contract while leaving production same-origin behavior unchanged.

**Acceptance criteria:**

- [ ] Integrated Admin proxy uses the manifest Cloud URL rather than the committed 8080 default.
- [ ] Missing, malformed, or mismatched runtime input fails startup instead of falling back silently.
- [ ] Production build continues to request same-origin `/api/v1`.

**Verification:**

- [ ] `pnpm test`
- [ ] `pnpm build`
- [ ] Acceptance fixture proves proxy success while `127.0.0.1:8080` is not listening.

**Dependencies:** Task 7

**Files likely touched:**

- `naive-ui-admin/vite.config.ts`
- `naive-ui-admin/build/utils.ts`
- `naive-ui-admin/tests/api-routing-config.test.ts`
- `naive-ui-admin/.env.development`

**Estimated scope:** Medium, 4 files

## Task 10: Bind Desktop to Cloud URL and environment identity

**Description:** Extend Desktop development profile validation to compare the runtime manifest identity with Cloud before authenticated state is initialized, while preserving existing profile roots and URL-based isolation.

**Acceptance criteria:**

- [ ] Manifest URL, resolved runtime URL, profile marker URL, and Cloud-reported identity must agree.
- [ ] A mismatch fails before Cloud credentials, billing, cache, or authenticated project state is constructed.
- [ ] Existing profile paths are preserved and no profile data is copied or deleted.

**Verification:**

- [ ] From `Reasonix/desktop`: `go test . -count=1 -run 'Test(ResolveCloudBaseURL|DevProfile|Environment|NewAppMarkerConflict)'`
- [ ] Negative identity fixture proves fail-closed initialization.

**Dependencies:** Tasks 6-7

**Files likely touched:**

- `Reasonix/desktop/dev_profile.go`
- `Reasonix/desktop/dev_profile_test.go`
- `Reasonix/desktop/cloud_runtime.go`
- `Reasonix/desktop/cloud_runtime_real_test.go`

**Estimated scope:** Medium, 4 files

### Checkpoint D: Three-client binding contract

- [ ] Tasks 9-10 tests and builds pass.
- [ ] Admin and Desktop resolve exactly one normalized Cloud URL.
- [ ] URL and identity mismatch tests fail before authenticated use.
- [ ] Existing Desktop profile artifacts remain unchanged.

## Task 11: Update developer workflow documentation

**Description:** Replace per-worktree `.env` quick-start guidance with one-time machine provisioning and the canonical launcher commands in all three repositories.

**Acceptance criteria:**

- [ ] New developers have one documented setup path and no instruction to copy `.env`.
- [ ] Documentation distinguishes local non-secret defaults from the approved shared PostgreSQL profile.
- [ ] Troubleshooting covers legacy `.env`, inherited override, wrong identity, stale manifest, and foreign port errors.

**Verification:**

- [ ] Search confirms active quick-start instructions do not require `Copy-Item .env.example .env`.
- [ ] All documented commands match implemented `--help` or PowerShell parameter contracts.

**Dependencies:** Tasks 1-10

**Files likely touched:**

- `Reasonix/README.md`
- `Reasonix-Cloud/README.md`
- `naive-ui-admin/README.md`

**Estimated scope:** Medium, 3 files

## Task 12: Perform clean-worktree cutover and full verification

**Description:** Build from the three integration branches, provision the approved user-level Cloud configuration without exposing credentials, start the replacement stack, execute positive and negative gates, then stop only the verified old runtime and reopen Desktop/Admin in the Codex browser.

**Acceptance criteria:**

- [ ] Cloud health/readiness, Admin proxy, Desktop native window/profile/project/expert flows, ports, PIDs, worktrees, commits, URL, and identity all agree.
- [ ] A deliberately wrong identity and foreign listener both fail closed.
- [ ] No `.env`, credential, secret-bearing manifest, pushed branch, or pull request is created.

**Verification:**

- [ ] Cloud: `go test ./internal/config ./internal/http -count=1` and `go vet ./...`
- [ ] Admin: `pnpm test` and `pnpm build`
- [ ] Desktop: affected Go tests, `go run ./scripts/code_guard`, baseline diff checks, `go vet ./...`, and `git diff --check`
- [ ] Live: `pwsh -NoProfile -File .\scripts\reasonix-dev.ps1 verify`
- [ ] Browser: Desktop and Admin tabs load from the verified replacement processes.

**Dependencies:** Tasks 1-11

**Files likely touched:**

- `Reasonix/tasks/todo.md`
- Runtime artifacts outside repositories under the verified user/runtime location only

**Estimated scope:** Small repository diff; operational verification task

## Final Review

- [ ] Every task acceptance criterion is satisfied.
- [ ] Every claimed check has an observed successful result.
- [ ] Final diffs contain only request-scoped changes and no secrets.
- [ ] Existing user data and unrelated processes remain intact.
- [ ] Branches remain local and no PR is created.
