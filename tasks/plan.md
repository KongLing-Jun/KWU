# Implementation Plan: Unified Development Cloud Environment

## Overview

Replace three independent development configuration paths with one verified contract. Cloud first gains a deterministic, no-worktree-`.env` configuration resolver and a stable environment identity. A canonical Desktop-owned launcher then publishes a non-secret runtime manifest. Admin and Desktop consume the same verified Cloud URL and identity before the launcher cuts over from the existing runtime.

Specification: `docs/specs/unified-development-cloud-environment.md`

Decision: `docs/decisions/0001-unified-development-cloud-environment.md`

## Dependency Graph

```text
Cloud configuration resolver
    |
    +-- user-level secure configuration discovery
    +-- checked-in non-secret development defaults
    +-- legacy .env conflict/migration behavior
    |
Cloud environment identity contract
    |
Runtime manifest schema and canonical launcher
    |
    +-- Admin Vite proxy binding
    +-- Desktop URL and identity binding
    |
Ownership/status/shutdown verification
    |
Fresh-worktree and negative-mismatch live cutover gate
```

## Architecture Decisions

- Cloud owns bootstrap configuration and authoritative environment identity.
- The Desktop repository owns cross-repository orchestration because it already contains the integrated Wails launcher and process-ownership checks.
- The runtime manifest is discovery metadata only and contains no secrets.
- Child processes receive one canonical normalized Cloud URL; they do not independently select defaults during integrated startup.
- Existing profile roots are preserved. Environment identity augments URL-based isolation and must not silently migrate user data.
- Current processes remain running until the replacement stack passes the full verification checkpoint.

## Implementation Order

### Phase 1: Cloud configuration foundation

1. Encode precedence and no-`.env` behavior in focused failing tests.
2. Add checked-in non-secret development defaults and user-level config discovery without adding dependencies.
3. Route server, worker, migration, token, and database helper entrypoints through the same resolver.
4. Add redacted legacy `.env` detection and a one-time migration/setup command.

Checkpoint:

- Fresh Cloud worktree loads with no `.env` using safe local defaults.
- The approved shared profile loads only through user-level configuration or process secret injection.
- Tests prove process override, user config, checked-in defaults, and legacy conflict behavior.
- No output contains secret values.

### Phase 2: Authoritative environment identity

1. Inspect existing metadata tables for a stable environment identifier before proposing any schema change.
2. Expose a minimal unauthenticated, non-secret identity alongside health/readiness or a dedicated development metadata endpoint.
3. Refuse shared/remote database startup when the configured expected identity does not match the authoritative value.

Checkpoint:

- Correct environment identity starts normally.
- Missing or incorrect identity fails before serving authenticated APIs.
- HTTP tests confirm that no configuration or credential material is exposed.

If no existing durable identifier can be reused and a migration is required, pause for user approval before changing schema.

### Phase 3: Canonical launcher and runtime manifest

1. Replace the dated integration script with `scripts/reasonix-dev.ps1` supporting `up`, `status`, `verify`, and `down`.
2. Resolve selected worktrees and commits dynamically, normalize the Cloud URL once, and validate port ownership before launch.
3. Write the manifest atomically under the user/runtime scope with a secret-field denial test.
4. Preserve the current runtime until all replacement health and ownership checks pass, then perform scoped cutover.

Checkpoint:

- Contract tests cover manifest schema, atomic replacement, stale PID rejection, and scoped shutdown.
- `status` reports the selected roots, revisions, endpoints, identities, and owned PIDs without credentials.
- A foreign process on any required port prevents startup and is not terminated.

### Phase 4: Admin and Desktop consumers

1. Make integrated Admin startup derive `VITE_PROXY` from the verified manifest while keeping production same-origin routing unchanged.
2. Make Desktop integrated startup verify manifest URL and Cloud environment identity before initializing authenticated Cloud state.
3. Reject inherited endpoint/profile overrides in the canonical path; keep explicit opt-in hooks only for isolated tests.

Checkpoint:

- Admin proxy reaches the manifest Cloud when port 8080 is not listening.
- Desktop marker, actual Cloud requests, manifest URL, and environment identity agree.
- Deliberate URL or identity mismatches fail closed and do not construct authenticated state.

### Phase 5: Fresh-worktree cutover and documentation

1. Update the three repository quick-start documents to use the canonical workflow and explain one-time machine provisioning.
2. Start from clean worktrees with no `.env`, build the new Cloud binary, and run the full integrated stack.
3. Verify health/readiness, Admin proxy routes, Desktop window/profile/project/expert flows, PID/port ownership, and secret absence.
4. Stop only the old verified runtime after replacement validation, then reopen Desktop and Admin in the Codex browser.

Final checkpoint:

- All specification success criteria pass.
- Relevant tests, builds, vet, code guard, and diff checks pass.
- All three worktrees contain only scoped changes and no credential files.
- Branches remain local unless separately authorized.

## Vertical Slices

Each implementation slice must touch no more than five files where practical and leave its repository buildable:

1. Cloud resolver contract and tests.
2. Cloud legacy migration/setup behavior and tests.
3. Cloud identity contract and mismatch tests.
4. Launcher manifest/status contract and tests.
5. Launcher process lifecycle and ownership tests.
6. Admin manifest proxy and build tests.
7. Desktop manifest identity verification and Go tests.
8. Full-stack cutover, docs, and final gates.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Existing `.env` precedence is relied on by commands | High | Encode every entrypoint in tests, provide explicit migration/setup, and fail with source-only diagnostics |
| Same URL points to another database | High | Compare configured and authoritative stable environment identities before serving |
| Runtime manifest leaks secrets | High | Allowlist fields, atomic write, restrictive permissions, automated secret-field/content checks |
| Launcher stops unrelated processes | High | Require manifest PID, start time, command line, worktree root, and listener ownership agreement |
| Desktop unexpectedly selects another profile | High | Preserve normalized URL behavior and add identity validation without moving profile roots |
| Production/CI config behavior regresses | High | Keep explicit managed config and process injection first-class; add precedence regression tests |
| Existing runtime becomes unavailable during rollout | Medium | Build and verify replacement on non-conflicting ports or in a staged cutover before stopping owned processes |

## Parallelization

- Read-only investigation and test inventory may run in parallel.
- Cloud resolver and identity work are sequential because identity depends on resolved configuration.
- Admin and Desktop consumers may proceed independently only after the manifest and identity contracts are fixed.
- Runtime cutover and browser validation are sequential and owned by one operator.

## Resolved Questions

- No existing Cloud metadata row was a durable database identity. The user separately approved migration `00090`, which now owns the singleton environment UUID.
