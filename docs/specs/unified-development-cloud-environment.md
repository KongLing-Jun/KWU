# Spec: Unified Development Cloud Environment

## Status

Specification, implementation plan, task list, and migration `00090` approved on 2026-08-11.

## Assumptions

1. The standard integrated development topology is a locally compiled Cloud process at `http://127.0.0.1:9090`, backed by the team PostgreSQL environment and local Redis.
2. A one-time user-level secure configuration is acceptable; per-repository and per-worktree `.env` files are not.
3. Runtime discovery files must contain no credentials, tokens, database URLs, or external-provider secrets.
4. Existing Desktop user data must remain isolated by Cloud environment, and a changed database behind the same URL must be detected before authenticated state is used.
5. Existing processes remain available until the replacement stack passes its cutover checks.
6. This work remains local until the user separately authorizes pushes or pull requests.

## Objective

Provide one repeatable development command that starts Cloud, Admin, and Desktop from the selected worktrees without requiring `.env` files and without allowing any client to silently bind to a different Cloud or database environment.

Developers should be able to provision machine-level secrets once, switch worktrees, and start the integrated stack without copying configuration. A mismatched endpoint, database environment, inherited override, stale runtime manifest, or foreign port owner must fail closed with a diagnostic that does not disclose secrets.

## Tech Stack

- Desktop: Go, Wails, PowerShell integration launcher, Vite frontend.
- Cloud: Go, PostgreSQL, Redis, PowerShell developer tooling.
- Admin: Vue 3, TypeScript, Vite proxy.
- Configuration artifacts: user-level secure configuration plus a non-secret JSON runtime manifest.

No new third-party dependency is required.

## Commands

### Development

```powershell
# From the Desktop worktree. The launcher resolves sibling worktrees,
# starts the three processes, writes the non-secret manifest, and verifies cutover.
pwsh -NoProfile -File .\scripts\reasonix-dev.ps1 up

# Inspect resolved worktrees, commits, PIDs, ports, URLs, and environment identity.
pwsh -NoProfile -File .\scripts\reasonix-dev.ps1 status

# Stop only processes proven to belong to the runtime manifest.
pwsh -NoProfile -File .\scripts\reasonix-dev.ps1 down
```

### Focused validation

```powershell
# Cloud worktree
go test ./internal/config -count=1
go test ./internal/http -count=1
go vet ./...

# Desktop worktree
Set-Location .\desktop
go test . -count=1 -run 'Test(ResolveCloudBaseURL|DevProfile|Environment)'
Set-Location ..

# Admin worktree
pnpm test
pnpm build

# Cross-repository runtime gate
pwsh -NoProfile -File .\scripts\reasonix-dev.ps1 verify
```

### Desktop repository handoff gate

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
go vet ./...
```

## Project Structure

```text
Reasonix/
  docs/specs/                         Cross-repository behavior specification
  docs/decisions/                     Architecture decision records
  scripts/reasonix-dev.ps1            Canonical integrated development lifecycle
  desktop/                            Cloud discovery and profile binding checks
  tasks/plan.md                        Current reviewed implementation plan
  tasks/todo.md                        Approved implementation slices

Reasonix-Cloud/
  config/development.yaml             Checked-in non-secret local defaults
  internal/config/                    Unified configuration resolver
  internal/http/                      Public non-secret environment identity
  scripts/                            One-time user configuration migration/setup

naive-ui-admin/
  build/                              Runtime-manifest-aware Vite proxy resolution
  tests/                              Proxy and wrong-environment contract tests
```

## Code Style

Resolve one canonical value, validate it at the boundary, and pass the validated value downstream. Do not let each child process independently infer the Cloud endpoint.

```powershell
$runtime = Resolve-ReasonixRuntime -ManifestPath $manifestPath
Assert-ReasonixCloudOwner -Runtime $runtime
Start-ReasonixAdmin -CloudBaseUrl $runtime.cloud_base_url
Start-ReasonixDesktop -CloudBaseUrl $runtime.cloud_base_url -EnvironmentId $runtime.environment_id
```

Go and TypeScript code must follow nearby naming, error handling, formatting, and test patterns. Exported configuration fields require useful comments. Error messages may identify configuration source and profile names but never secret values.

## Configuration Contract

### Cloud bootstrap precedence

1. Explicit config path intended for CI or managed deployment.
2. Process environment for deployment-time overrides and secret injection.
3. One user-level machine configuration shared by all worktrees.
4. Checked-in non-secret development defaults.
5. Built-in conservative defaults.

The standard launcher must not automatically read a worktree `.env`. A legacy `.env` is detected and reported as a migration input or conflict, not silently consumed.

### Runtime manifest

The launcher writes a user/runtime-scoped JSON document containing only:

- schema version and generation identifier;
- normalized Cloud, Admin, and Desktop origins;
- stable Cloud environment identity;
- selected worktree roots, branches, and Git commits;
- owned PIDs, ports, and process start times.

The manifest must never contain database URLs, credentials, tokens, cookies, provider keys, or captured environment-variable values.

### Environment identity

Cloud exposes a stable non-secret environment identifier derived from authoritative database state. The launcher, Desktop profile marker, and health verification compare this identifier. A missing or mismatched identifier prevents authenticated Cloud initialization and cutover.

## Testing Strategy

- Cloud unit tests cover configuration precedence, user-level discovery, legacy `.env` refusal, redacted diagnostics, and remote-database environment mismatch.
- Cloud HTTP tests cover the non-secret environment identity response without exposing configuration.
- Desktop Go tests cover manifest discovery, URL and identity agreement, stable profile selection, and fail-closed mismatch behavior.
- Admin tests prove the Vite proxy targets the manifest Cloud even when port 8080 is unavailable, and refuses malformed or stale manifests.
- PowerShell contract tests cover child-process environment derivation, port ownership, stale PID rejection, scoped shutdown, and manifest secret-field denial.
- The live gate verifies Cloud health/readiness, Admin proxy routing, Desktop window/profile marker, actual request target, PID ownership, and the negative wrong-environment case.

## Boundaries

### Always

- Preserve dirty or unrelated worktrees and processes.
- Validate worktree, commit, PID tree, listener, URL, and environment identity before cutover or shutdown.
- Keep secrets outside Git and redact configuration diagnostics.
- Preserve explicit CI and production configuration behavior.
- Keep the current runtime available until replacement verification passes.

### Ask first

- Changing the standard topology away from local Cloud on port 9090, team PostgreSQL, or local Redis.
- Adding a database migration or a third-party secret-store dependency.
- Migrating or deleting an existing Desktop profile.
- Pushing branches, creating pull requests, or changing CI.

### Never

- Commit or print credentials.
- Copy `.env` into a new worktree as a startup step.
- Silently override a configured Cloud endpoint.
- Treat a responding Vite page as proof that Desktop or Cloud is correctly bound.
- Stop a process that is not proven to belong to the selected runtime manifest.

## Success Criteria

1. A clean worktree with no `.env` can start Cloud, Admin, and Desktop after one machine-level provisioning step.
2. Admin and Desktop use the exact normalized Cloud URL recorded by the runtime manifest.
3. Cloud connects only when the expected database environment identity matches the authoritative identity.
4. Inherited or legacy Cloud overrides, stale manifests, wrong database identities, and foreign port owners fail before cutover.
5. Switching worktrees does not require copying configuration and does not unexpectedly change the Desktop profile for the same environment.
6. The runtime manifest and logs pass an automated secret-field/content check.
7. `up`, `status`, `verify`, and scoped `down` work on Windows PowerShell.
8. Relevant Cloud, Desktop, Admin, build, vet, guard, and live integration checks pass.

## Resolved Questions

- Migration `00090` was separately approved and adds the singleton database-owned Cloud environment identity discovered to be necessary during Task 5.
