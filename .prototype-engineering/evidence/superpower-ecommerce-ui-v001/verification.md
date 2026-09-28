# Verification Evidence

Date: 2026-08-21

## Frontend production build

Command run from `desktop/frontend`:

```text
pnpm build
```

Result: passed. CSS syntax and z-index checks passed, TypeScript emitted no errors, and Vite completed the production build. The new feature remains a lazy-loaded `superpowers` chunk. Vite reported the repository's non-blocking large-chunk warning.

## Focused UI tests

Command run from `desktop/frontend`:

```text
pnpm exec vitest run --config vitest.config.ts src/vitest/superpower-workspace.vitest.test.tsx
```

Result: passed — 1 test file, 2 tests.

## In-app browser scenarios

Tested at `http://127.0.0.1:5174/`:

- Sidebar “超能力” opens the “选择你的超能力” catalog.
- “跨境电商超能力” opens the split research/evidence workspace.
- Product evidence can be cited into a deterministic follow-up.
- Removing all saved products exposes the intended empty state.
- Selecting the partial-data scenario exposes the intended degraded state.
- Browser console contained no warnings or errors during the walkthrough.
- A desktop viewport override confirmed no document-level horizontal overflow at 1280×800; the override was reset afterward.
