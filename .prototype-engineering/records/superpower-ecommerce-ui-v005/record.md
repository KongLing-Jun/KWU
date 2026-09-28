# Prototype Record

## Changes from parent

- Removed the fixed research-workspace header, title, marketplace badge, and view navigation from the right panel.
- Reframed the right panel as a headerless AI-generated component canvas.
- Marked the four deterministic research cards as generated widget instances for the prototype renderer boundary.
- Replaced the header collapse action with an unobtrusive divider-edge control and retained compact restore behavior.
- Removed the now-inaccessible product-table and saved-product presentation from this prototype version.

## Assumptions

- AI output should determine which widgets appear and in what order; the surrounding canvas should not impose product-specific tabs.
- Deterministic widget markup is sufficient to validate the intended container and renderer contract before a real schema exists.
- Evidence actions belong to individual widgets rather than a global panel navigation model.

## Evidence

- `generated-component-canvas.png`: browser screenshot of the headerless generated canvas.
- `verification.md`: automated and browser verification results.

## Findings

- Removing 96 pixels of fixed header/navigation lets the generated result begin 12 pixels from the canvas top.
- All four research widgets remain legible and evidence citation still works without global navigation.
- The divider-edge collapse action preserves the earlier collapsible-panel requirement without reintroducing a header.
- The empty-input scenario continues to present a truly blank right canvas.

## Known gaps

- The generated widget specification is represented by deterministic local markup, not a serialized runtime schema.
- Runtime component validation, permissioning, sandboxing, and error isolation are not implemented.
- Product tables and saved candidates need to return later as AI-selected widget types rather than fixed tabs.

## Recommendation

Continue with the headerless canvas. Define a constrained declarative widget schema and renderer registry next; avoid model-authored executable UI code.
