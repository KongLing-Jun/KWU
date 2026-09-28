# DEF-004 — Computer Use could not inspect localhost in Windows Edge

- Scope: supplemental L03 artifact visual acceptance
- Classification: E (test infrastructure)
- Severity: blocks native Edge inspection through Computer Use; no Wails Desktop execution impact
- Status: deterministic across four attempts in this task

## Symptom

Computer Use could enumerate and activate Edge windows, but state capture stopped because it could not determine the current Windows browser URL with enough confidence to enforce policy. The same stop occurred for a `file://` app-mode window, a localhost app-mode window, and a normal Edge localhost window with a visible address bar.

## Reproduction

1. Serve L03 at `http://127.0.0.1:8766/empty-state.html` and confirm HTTP 200 outside the UI.
2. Open it in Edge as app mode or a normal new window.
3. Enumerate the exact returned Edge window and activate it with Computer Use.
4. Call window-state capture; Computer Use ends the turn with a URL-confidence safety stop.

## Evidence

- The localhost response was HTTP 200 and contained the exact heading.
- Returned Edge window IDs during the attempts: `307234814`, `79693654`, and `241436730`.
- Supplemental acceptance was completed with the in-app Browser and Playwright; screenshots are under `visual/L03/` and `output/playwright/`.
- Real Reasonix execution and final-state acceptance remained Wails Desktop + Computer Use.

## Assessment and workaround

This is external to Reasonix. It must not be counted as a Desktop Agent completion failure. The accepted workaround is to keep Computer Use as the Wails acceptance surface and use the built-in Browser/Playwright only for the generated HTML artifact's rendered visual checks.
