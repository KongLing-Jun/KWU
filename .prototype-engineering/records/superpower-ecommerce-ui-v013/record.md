# Prototype Record

## Changes from parent

Added a pure, versioned ResearchFollowUpRequest builder and made submitted follow-ups store that request rather than UI-only question/citation labels. Requests carry task/document/widget provenance and bounded evidence IDs while excluding raw widget props.

## Assumptions

- Evidence IDs can be resolved later within the active capability task.
- Six sources with four evidence references each is a useful initial context budget for the prototype.
- Character limits are an acceptable deterministic proxy until the selected model/provider tokenizer is available.

## Evidence

- Frontend production build passed.
- Normal and oversized protocol probes produced the expected version, ordering, limits and omission metadata.
- The normal request contained no raw widget `props`.
- Live Vite source serves the request builder integration.
- Diff whitespace validation passed.

## Findings

The DashboardDocument already contains enough stable identity and provenance to form a compact follow-up envelope. Keeping evidence resolution outside the request avoids duplicating arbitrary AI-generated UI payloads into model context and creates a clear permission/token-control seam.

## Known gaps

- No real model/session execution adapter exists yet.
- Evidence IDs are not resolved to text or structured facts.
- Limits use characters/counts rather than provider tokenization.
- Request persistence, permission checks, streaming, cancellation and retries are absent.
- Focused UI tests were updated but not run during this rapid iteration.

## Recommendation

Continue by implementing the execution adapter boundary: resolve authorized evidence artifacts, assemble a bounded hidden context block, pass the visible question through the existing conversation service, and stream an answer whose citations map back to request citation order.
