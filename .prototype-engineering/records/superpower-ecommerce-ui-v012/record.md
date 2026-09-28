# Prototype Record

## Changes from parent

The single selected widget/reference string was replaced by an ordered citation collection. Right-click now toggles individual cards, the composer renders multiple removable chips, and submitted follow-ups own immutable citation snapshots.

## Assumptions

- Research questions commonly require evidence from multiple generated components.
- Right-clicking an already selected card is an understandable inverse action when the hover hint explicitly says it will cancel.
- Draft citation selection may remain ephemeral while durable evidence attachment stays in the DashboardDocument action path.

## Evidence

- Frontend production build passed.
- Live Vite source contains multi-citation collection and submission snapshot behavior.
- Static search confirms scalar citation state was removed.
- Diff whitespace validation passed.

## Findings

The common WidgetRenderer seam supports multi-source selection without adding business-specific controls to component implementations. Separating draft citation state from submitted follow-up snapshots prevents later selection changes from rewriting conversation history.

## Known gaps

- Keyboard and touch citation alternatives are not designed.
- Draft selection is not persisted across reloads or sessions.
- Citation ordering follows selection order and cannot be rearranged.
- The existing dashboard attach action records evidence linkage but there is no durable detach action for ephemeral UI deselection.
- Focused tests were updated but not run during this rapid iteration.

## Recommendation

Continue with the multi-source interaction. The next prototype should decide how selected widget IDs and evidence references are serialized into the actual model request while preserving source provenance and token limits.
