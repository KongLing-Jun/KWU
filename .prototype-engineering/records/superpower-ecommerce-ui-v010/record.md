# Prototype Record

## Changes from parent

v010 removes the trend-specific `citable` flag and makes citation a uniform renderer capability. All four registered widget types expose the same action in both inline and canvas surfaces, with loading-state disabling and a widget-revision fallback.

## Assumptions

- A generated component is the smallest useful citation unit in this prototype.
- One composer reference at a time remains sufficient.
- The first evidence reference is the preferred display label when several exist.

## Evidence

- `verification.md` records the production build, whitespace check, served-module inspection and removal of the special-case flag.
- The user's browser annotation identified the inconsistent single-widget behavior.

## Findings

- Citation belongs to the host renderer contract, not business props supplied by data or AI.
- The same WidgetEnvelope identity and revision provide a safe fallback when source evidence is not yet attached.
- Passing one handler through both renderer surfaces keeps the behavior consistent without changing each widget schema.

## Known gaps

- Citation remains single-select and text-only.
- It does not yet open the underlying evidence or preserve a visual component snapshot.
- There is no keyboard shortcut or multi-reference composer model.
- Automated visual capture was unavailable.

## Recommendation

Continue. The next interaction refinement should evolve the composer reference into a typed citation object so follow-up requests can carry widgetId, documentRevision and evidenceRefs to the eventual capability MCP runtime.
