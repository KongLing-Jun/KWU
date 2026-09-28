# Prototype Record

## Changes from parent

- Removed the redundant “综合研究” label from the conversation date row while retaining the timestamp.
- Added a shared `WidgetEnvelope` concept with stable widget identity, constrained widget type, and target surface.
- Wrapped the inline research conclusion and all four right-canvas widgets with the same generated-widget frame.
- Distinguished inline and canvas placement declaratively through `conversation-inline` and `research-canvas` surface values.

## Assumptions

- MCP composition tools should choose widget type, data and target surface, while the host owns trusted component implementations.
- A shared envelope is the smallest useful contract for demonstrating cross-surface composition in this UI-only prototype.
- Visual chrome does not need to expose whether a widget came from MCP; provenance can remain structural metadata.

## Evidence

- `shared-widget-surfaces.png`: browser screenshot showing the preserved inline conclusion and right-canvas widgets after shared framing.
- `verification.md`: automated and browser verification results.

## Findings

- The timestamp reads cleanly without the extra research-mode label.
- The inline conclusion preserves its previous layout while exposing generated widget identity and surface metadata.
- One inline widget and four canvas widgets now share the same envelope structure.
- The existing component styling can remain stable while the future MCP controls assembly declaratively.

## Known gaps

- Widget props are still local fixtures rather than a serialized and schema-validated MCP payload.
- A formal renderer registry, versioned schemas, action permissions and component-level error boundaries remain to be implemented.
- Streaming widget insertion, replacement and reordering are not modeled yet.

## Recommendation

Continue by extracting the envelopes into a versioned schema and renderer registry, then simulate MCP add/update/remove operations before integrating a real MCP transport.
