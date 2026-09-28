# DEF-007 — L18 operations dashboard visually overflows desktop and narrow screens

- First run: L18, `desktop-design-operations-dashboard`, repeat 3, low effort
- Reproduction runs: L18 only; L24 repeat 4 did not reproduce
- Classification: D (model delivery variance), with an E-class static-grader coverage gap
- Severity: material artifact-quality failure; Desktop runtime completion remains successful
- Status: open evidence record; no product or artifact repair in this task

## Symptom and strict judgment

The model delivered all three required files and the structural grader passed 14/14 checks. A real Playwright render disproved the claimed responsive behavior:

- at 320px, `document.documentElement.scrollWidth` was 954px for `window.innerWidth` 320px;
- the mobile `.content-grid` computed to 936px and rendered far outside the viewport;
- at 1440px, the embedded SVG image overflowed its trend panel and overlapped the adjacent recent-incidents panel;
- the Desktop final nevertheless claimed that narrow-screen layout was supported.

Therefore L18 is a model-delivery quality failure and a false validation claim. It is not a Wails completion-state failure: the accepted final appeared and remained stable in the active session without switching.

## Reproduction

1. Open `workspaces/L18/operations-dashboard.html` in Chrome.
2. At 1440×900, inspect the trend panel and observe the embedded dashboard image crossing into the recent-incidents panel.
3. Set the viewport to 320×800.
4. Observe horizontal overflow, then evaluate `window.innerWidth`, `document.documentElement.scrollWidth`, and the computed grid columns.
5. Compare the HTML `<img src="dashboard-visual.svg">` with CSS rule `.trend svg`; the rule does not target the delivered `img` element.

## Evidence

- Desktop runtime record: `observations/L18.json`
- Frozen grade: `runs/L18/grade.json`
- Playwright metrics: `visual/L18/playwright-observation.json`
- Rendered evidence: `visual/L18/desktop-light.png`, `mobile-light.png`, and `desktop-focus.png`
- Desktop screenshots: `screenshots/L18/`
- Parent and child logs: `logs/L18/`
- Workspace: `workspaces/L18/`

## Cost and completion impact

- 286,018 total tokens: 259,676 parent and 26,342 child
- 20 model calls: 14 parent and 6 child
- 26 tool calls; 3 nonconsecutive duplicate signatures
- 4 approvals; 820ms observed approval wait
- 3 todo steps, 2 `todo_write` calls, 4 `complete_step` calls with 1 rejection
- 211.641s model duration; 273.885s to first observed Desktop final
- normal runtime end and realtime Desktop final: pass
- artifact structure: pass; visual/end-to-end quality: fail

## Assessment and follow-up boundary

The user prompt explicitly required narrow-screen support, so the primary cause is not missing prompt direction. The artifact's selector mismatch is concrete model-output evidence. The frozen static grader checks that a width media query exists but cannot prove rendered containment; that is a secondary E-class coverage gap, mitigated in this benchmark by mandatory Playwright acceptance.

No fix is applied here. L24 used the same prompt and low effort but did not reproduce the failure: Playwright measured no desktop
panel overlap and `scrollWidth=320` at a 320px viewport, with all visible elements contained. This makes DEF-007 observed model
delivery variance rather than a deterministic runtime defect. The L18 result remains failed; a later successful repeat does not
retroactively change its visual judgment. Any future product change to improve model guidance or the frozen grader must be handled
in a separate approved task.
