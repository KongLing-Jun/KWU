# Verification

## Automated checks

- `cd desktop/frontend && pnpm build` — passed CSS syntax checks, z-index checks, TypeScript compilation, and Vite production bundling.
- `git diff --check` — passed.

## Runtime source inspection

- The live Vite server at `http://127.0.0.1:5174/` serves the multi-citation collection, right-click toggle, independently removable chips, per-message citation snapshots, and selected-card hint.
- Repository search confirms the former scalar `selectedWidgetId`, `reference`, and `referenceLabel` state no longer exists in the feature.

## Rapid-iteration constraint

- The focused interaction test was updated to cover two simultaneous citations, submission snapshot stability, and single-source toggling, but was intentionally not executed because the user requested no test run during rapid iteration.
