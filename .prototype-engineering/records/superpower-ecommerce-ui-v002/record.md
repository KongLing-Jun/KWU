# Prototype Record

## Changes from parent

- Removed the full-width ecommerce top bar.
- Consolidated back navigation, ecommerce capability identity, case title, and research status into the conversation header.
- Moved research shortcut chips above the composer and removed the instructional placeholder copy.
- Added a disabled empty-send state and a deterministic “空输入” demo scenario.
- Replaced the layout-bound scenario select with a bottom-right floating pill and popover.
- Added collapse/restore controls for the evidence workspace; the conversation expands to the full available width when evidence is hidden.
- Preserved the production-only context-menu suppression while keeping native right-click available in development builds.

## Assumptions

- The floating scenario control is prototype tooling and should not consume normal product-layout space.
- “空输入” means an empty question, reference, and follow-up state with the send action disabled.
- Collapsing evidence should preserve its active tab and in-memory selections for restoration.

## Evidence

- Focused Vitest suite passed: 1 file, 3 tests.
- Frontend production build passed after the layout revision.
- In-app browser walkthrough verified the empty-input demo, popover, shortcut order, evidence collapse/restore, evidence citation, follow-up submission, and zero console warnings/errors.
- Browser geometry confirmed the collapsed conversation expands to the full 1274 px content width and the restore pill no longer overlaps the research-state badge.
- Screenshots are stored under `.prototype-engineering/evidence/superpower-ecommerce-ui-v002/`.

## Findings

- Consolidating capability identity into the conversation header removes a redundant chrome layer and makes more vertical space available for research content.
- The shortcut row reads as part of composing intent when placed immediately above the input.
- A floating demo control keeps prototype-only state switching visually separate from the intended production workspace.
- Evidence collapse is useful for long-form follow-up work and does not require changing the research data model.

## Known gaps

- The demo popover does not yet close on outside click or Escape.
- Collapse state, demo scenario, references, and messages remain in React memory only.
- The prototype still uses deterministic fixtures and CSS product artwork.
- Final animation, keyboard behavior, and mobile interaction design remain unvalidated.

## Recommendation

Continue this direction for the next UI review. Validate the floating demo-control location and the consolidated header at the team’s target minimum desktop window before connecting real research services.
