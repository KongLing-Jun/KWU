# Prototype Record

## Changes from parent

v011 replaces all explicit citation buttons with a card-level hover and right-click interaction. It adds animated hover/selected overlays, one selected widget ID, scoped native-menu prevention and composer-driven selection clearing.

## Assumptions

- The current target environment is mouse-first.
- A short “右键引用” hover hint makes the hidden interaction discoverable.
- One persistent selected card maps cleanly to the current single-reference composer.

## Evidence

- `verification.md` records production build, served-module inspection, removal of old button code and whitespace validation.
- The user explicitly specified hover feedback, fade animation and right-click selection.

## Findings

- Citation can remain a uniform WidgetRenderer capability without occupying card header layout.
- `contextmenu` is the correct scoped browser event and coexists with the app's broader Wails context-menu suppression.
- Keeping selected identity outside the document correctly treats selection as ephemeral UI state, while attach-evidence remains durable business state.

## Known gaps

- No touch or keyboard equivalent exists yet.
- The hover hint may need further visual tuning after direct inspection.
- Selection is not restored across page reload, by design.
- Automated visual capture was unavailable.

## Recommendation

Continue. Validate the hover strength and hint placement in the browser, then evolve the composer reference into a typed citation payload before connecting it to the MCP runtime.
