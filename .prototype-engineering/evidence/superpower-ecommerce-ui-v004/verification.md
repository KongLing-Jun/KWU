# Verification

## Automated checks

- `pnpm exec vitest run --config vitest.config.ts src/vitest/superpower-workspace.vitest.test.tsx`: passed, 3 tests.
- `pnpm build`: passed, including CSS syntax, z-index, TypeScript, and Vite production build checks.
- No Go tests were run because this revision changes frontend and prototype-record files only.

## Browser verification

- Empty-input scenario shows the centered heading and all five capability cards.
- The compact capability launcher is absent before first submission.
- Selecting “市场机会”, entering a non-empty prompt, and submitting removes the centered chooser.
- Immediately after submission, the compact launcher includes `superpower-mode-chooser--settling`, the prompt is visible, and “正在启动市场机会” is rendered.
- After the transition, “市场机会” remains selected in the compact launcher and the evidence workspace remains blank.

## Visual evidence

- `empty-capability-cards.png`
- `capabilities-settled-after-submit.png`

## Notes

The production build reports the repository's existing non-blocking large-chunk warning.
