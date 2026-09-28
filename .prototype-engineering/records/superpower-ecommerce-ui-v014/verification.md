# Verification

## Automated checks

- `cd desktop/frontend && pnpm build` — passed CSS syntax checks, z-index checks, TypeScript compilation, and Vite production bundling after the final context assembly change.
- `git diff --check` — passed.

## Execution probes

- A two-source request resolved four artifacts, reported one deliberately unknown evidence ID, produced two source-specific answer blocks plus one synthesis block, and mapped citation orders as `[1]`, `[2]`, then `[1,2]`.
- The same valid evidence IDs under a mismatched task scope produced `insufficient-evidence`.
- A request with no citations produced `insufficient-evidence` without source claims.
- A synthetic 7,000-character artifact caused context truncation while retaining the untrusted-evidence instruction and valid JSON payload structure.
- Live Vite source serves the resolver/execution adapter and citation-aware answer/source rendering.

## Rapid-iteration constraint

- The focused UI test was updated for asynchronous answer sources but was intentionally not executed because the user requested no test run during rapid iteration.
