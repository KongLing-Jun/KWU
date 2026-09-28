# Prototype Brief

## Hypothesis

A centered card-based capability chooser will make the pre-research state actionable, while moving the selected capabilities into a compact composer launcher after submission preserves context without occupying the conversation canvas.

## Primary user flow

Open the empty-input demo, choose one of five centered research capability cards, type a question, and submit it. The cards transition into the compact launcher above the composer while the submitted message and deterministic research-start state enter the conversation.

## Success criteria

- Empty state shows centered guidance and five selectable cards.
- Selected card is visually and accessibly identifiable.
- Bottom compact launcher is absent before the first submission.
- Submitting non-empty text displays the question and mock research-start state.
- Compact capabilities animate into place above the composer after submission.

## Fidelity and data mode

High-fidelity local interaction and CSS motion using deterministic in-memory state.

## Out of scope

Real research execution, streaming, persistence, and final motion tuning remain out of scope.
