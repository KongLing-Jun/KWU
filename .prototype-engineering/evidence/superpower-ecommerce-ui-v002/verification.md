# Verification Evidence

Date: 2026-08-21

## Focused UI tests

`pnpm exec vitest run --config vitest.config.ts src/vitest/superpower-workspace.vitest.test.tsx`

Result: passed — 1 file, 3 tests. Coverage includes the two-level entry flow, evidence follow-up, saved-product empty state, empty-input demo, and evidence collapse/restore.

## Frontend build

`pnpm build`

Result: passed — CSS checks, z-index token checks, TypeScript, and the Vite production build completed successfully. The existing non-blocking large-chunk warning remains.

## Browser walkthrough

- Confirmed the full-width ecommerce top bar is absent.
- Confirmed back navigation and ecommerce identity render inside the conversation header.
- Confirmed research shortcuts render above the empty composer and the placeholder is blank.
- Confirmed the floating demo popover exposes 完整研究、研究中、部分数据、空输入.
- Confirmed 空输入 clears question/reference/follow-ups and disables send.
- Confirmed collapsing evidence expands the conversation to the full content width and restoration preserves the workspace.
- Confirmed evidence citation and follow-up submission still work.
- Browser console contained no warnings or errors.
