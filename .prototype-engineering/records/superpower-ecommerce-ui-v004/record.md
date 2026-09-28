# Prototype Record

## Changes from parent

- Replaced the blank conversation canvas with a centered, five-card research capability chooser.
- Added a selected capability state and connected it to first-request submission.
- Added a deterministic first-request response so the prototype can demonstrate the transition without a backend.
- Mounted the compact capability launcher only after first submission and applied a settling animation as it moves above the composer.

## Assumptions

- Selecting a capability before typing is the intended empty-state path.
- A deterministic “starting research” response is sufficient to validate the UI transition.
- The evidence workspace should remain visibly empty until real research output exists.

## Evidence

- `empty-capability-cards.png`: centered card chooser before first submission.
- `capabilities-settled-after-submit.png`: selected capability in the compact launcher after submission.
- `verification.md`: focused test, production build, and browser verification results.

## Findings

- The centered chooser gives the otherwise empty canvas a clear primary action without competing with the composer.
- The selected card is exposed both visually and through `aria-pressed`.
- The compact launcher is absent before submission and appears with the expected settling class immediately after submission.
- The submitted prompt and “正在启动市场机会” state render while the evidence workspace stays empty.

## Known gaps

- The research-start state is deterministic and does not stream or persist.
- Motion timing has only been visually checked at the current desktop viewport; additional responsive tuning may be needed.
- The floating demo control remains prototype-only and is not part of the product information architecture.

## Recommendation

Continue with this interaction model. The next prototype should connect the selected capability to real orchestration states while retaining the centered pre-submit chooser and compact post-submit launcher.
