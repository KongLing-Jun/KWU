# v015 verification evidence

Verified on 2026-08-21.

## Automated checks

- `cd desktop/frontend && pnpm build` — passed TypeScript validation and Vite production build.
- `cd desktop && go test . -run '^TestCapability(Dashboard|Evidence)'` — passed all directly affected Desktop dashboard/evidence tests.
- `git diff --check` — passed.

The broader `cd desktop && go test .` command was also sampled and failed in pre-existing session reservation and WorkBuddy project workspace-policy tests. None of the failures referenced the capability dashboard/evidence implementation. The proportional affected-package regex above passed.

## Browser interaction evidence

The running Vite page at `http://127.0.0.1:5174/` was exercised through the in-app browser without invoking the frontend test suite:

- Opened 超能力 → 跨境电商超能力.
- Right-click selected `ebay-active-listing-snapshot` and `crossborder-source-coverage`; both appeared in the selected-reference list.
- Submitted `结合这两个来源，为什么值得继续做小规模验证？`.
- Follow-up reported `data-execution-mode="preview"` and visibly labelled the answer `浏览器预览`.
- The answer retained two numbered sources and preserved the Mock/insufficient-evidence qualification.
- Collapsed the research canvas, clicked source `[1]`, and observed the canvas return with `data-source-focused="true"` on `ebay-active-listing-snapshot`.
- Browser console error log was empty.

## Desktop-only verification contract

The production build statically includes the Desktop session adapter and Wails evidence methods. A configured Desktop session is required for the `真实会话` path; Vite intentionally cannot claim that path. The manual Desktop acceptance steps are recorded in `l4p-docs/07-real-business-loop-validation.md` for the user to validate with their configured model and profile state.
