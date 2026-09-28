# Prototype Brief

## Hypothesis

Generated research cards can act as a lightweight multi-source evidence picker when right-click toggles each source and the composer exposes the accumulated selection without adding permanent controls to every card.

## Primary user flow

The user hovers over several generated cards, right-clicks each relevant source, reviews removable reference chips above the composer, submits a follow-up with all selected sources, and can then change the selection without changing the already submitted message.

## Success criteria

- More than one ready widget can remain selected across conversation and canvas surfaces.
- Right-clicking an already selected widget removes only that citation.
- Every selected source appears as its own removable composer chip.
- A submitted follow-up displays the exact citation set captured at submission time.
- Loading widgets remain non-selectable and no explicit citation button returns.

## Fidelity and data mode

High-fidelity frontend interaction using the existing deterministic DashboardDocument fixtures and Host action path. Citation selection is ephemeral UI state; evidence attachment continues through the existing dashboard action contract.

## Out of scope

Keyboard/touch alternatives, citation ordering controls, persisted draft selections, citation-level model prompting, source previews, and production conflict resolution.
