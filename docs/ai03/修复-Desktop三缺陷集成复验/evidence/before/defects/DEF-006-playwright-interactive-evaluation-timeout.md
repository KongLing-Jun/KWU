# DEF-006 — Supplemental Playwright interactive evaluation timed out after focus capture

- Scope: supplemental L15 artifact visual acceptance
- Classification: E (test infrastructure)
- Severity: blocks computed-style/console collection through the custom interactive script; CLI screenshots and Wails Desktop acceptance remain available
- Status: reproduced twice in L15 with fresh Playwright headless Chrome profiles

## Symptom

The Node REPL package entry first failed with an export-compatibility error. The Playwright CLI successfully rendered desktop, mobile, and dark screenshots. A CommonJS Playwright script then launched the installed Chrome channel, loaded the local artifact, pressed Tab, and saved the visible focus screenshot, but stalled on the subsequent `page.evaluate` stage until the 60s and 30s bounded shell timeouts.

## Reproduction

1. Open L15 `empty-state.html` with Playwright 1.62.1 and the installed Chrome channel.
2. Load the local `file://` artifact, press Tab, and save a full-page screenshot; this succeeds.
3. Request subsequent DOM/computed-style metrics through the custom Node script.
4. Observe no completion before the bounded timeout. Repeat with one page and sequential viewport/media emulation; the same stall occurs after the focus screenshot.

## Evidence

- Successful CLI screenshots: `visual/L15/desktop-light.png`, `mobile-light.png`, and `desktop-dark.png`.
- Successful interactive focus output: `visual/L15/desktop-focus.png`.
- Script: `visual/L15/playwright-observe.cjs`.
- Observation: `visual/L15/playwright-observation.json`.
- Stale headless parents were identified by their unique `playwright_chromiumdev_profile-*` command lines and terminated after each timeout; the user's pre-existing Chrome window/profile was not targeted.

## Assessment and workaround

This is external to Reasonix and is not a Desktop Agent failure. L15 visual acceptance relies on successful Playwright CLI renders, the actual focus screenshot, frozen grader/static assertions, and direct visual inspection. A later infrastructure task can isolate the REPL export mismatch and `page.evaluate` stall.
