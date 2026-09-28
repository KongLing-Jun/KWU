# Agent Instructions

Before opening or updating a PR, sync the branch with the latest `main-v3` and resolve any conflicts.

## Engineering constraints

- New functionality must not introduce locks. Resolve concurrency issues with atomic data structures or atomic operations.

## Common checks

Run these checks for every PR:

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
```

Use the complete NUL-delimited path list from `git diff --name-only -z origin/main-v3...HEAD` when classifying the change.

The executable source of truth is:

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

`make pr-ready` runs the same command. Inspect the classification summary before
publishing. Gate-infrastructure changes run the fast routing-contract test
instead of forcing unrelated Root Go, Desktop Go, and frontend suites.

## Proportional validation

Run Go tests only for affected packages.

- For each directly changed Go package, run `go test`. Expand the scope to affected dependents when exported APIs, shared contracts, registration, initialization, or cross-package behavior changes. Use a subsystem pattern such as `./internal/skill/...` when several coupled packages are affected.
- During iteration, an exact test is acceptable; before PR handoff, validate at least the affected package scope.
- For nested modules such as `desktop`, run checks from that module.
- If neither Root Go nor Desktop Go files or module files changed, do not run Go tests.
- Frontend changes run `pnpm build`; frontend tests are not part of the PR gate.

## When broader Go checks are required

Root Go changes use targeted package checks by default, including changes to a
single core-runtime file. Expand to `./...` when any of the following applies:

- root `go.mod` or `go.sum` changes;
- more than 4 core packages or 16 core Go files change;
- more than 12 Root Go packages or 30 Root Go files change.

Desktop Go changes also use targeted package checks by default. Expand the
Desktop module to `./...` when `desktop/go.mod` or `desktop/go.sum` changes, or
when more than 5 Desktop packages or 20 Desktop Go files change.

All frontend changes run `pnpm build` only. They do not run `pnpm test:all`,
`pnpm test:typecheck`, or mapped frontend tests. Windows installer changes
continue to run the installer contract because it is a release contract rather
than a general frontend test.

Do not open or update a PR if a required change-related check fails. Report the exact commands run and any checks intentionally skipped, with the reason.

## Stable and frozen surfaces

Stable packages are frozen by default: when neither their files nor a shared
contract they depend on changes, their tests are not part of the PR gate.
Directly changing a frozen package runs that package's checks. Dependency-file
or sufficiently broad changes expand to the broader suite. Gate infrastructure
runs `scripts/pr-gate.test.mjs`; frontend changes run a production build only.

The path classification lives in `scripts/pr-gate.mjs` and is covered by
`scripts/pr-gate.test.mjs`. Update both when changing the routing contract.

Full frontend tests remain available outside the PR gate. `go test ./...` is
reserved for dependency or broad Go changes and the nightly Go regression
workflow.
