# Prototype Record

## Changes from parent

Added a task-scoped evidence resolver, deterministic evidence catalog, bounded hidden-context assembler, replaceable research execution interface, deterministic prototype adapter, and citation-aware answer/source rendering. The feature remains isolated from the existing session service.

## Assumptions

- Evidence artifacts can later be persisted and resolved under the same capability/workspace/task scope.
- Model adapters can consume a hidden evidence context plus the visible question while preserving the v1 request identity.
- Numbered citations at answer-block granularity are sufficient for the next UI iteration.

## Evidence

- Frontend production build passed.
- Multi-source, partially unresolved, empty, scope-mismatch and oversized-context probes produced the expected outcomes.
- Oversized evidence retained a valid bounded JSON payload rather than a partially cut JSON value.
- Live Vite source includes resolver, adapter and source rendering.
- Diff whitespace validation passed.

## Findings

The superpower workspace can own an execution sidecar without changing the core session schema or service. Stable evidence IDs provide the join between generated cards, task artifacts, hidden model context and answer citations. Treating resolved content explicitly as untrusted data creates a necessary prompt-injection boundary before real MCP sources are connected.

## Known gaps

- Execution is deterministic and non-streaming; no real model or existing session adapter is connected.
- The evidence catalog is an in-memory prototype fixture rather than the persisted runtime workspace.
- Production ACLs, artifact integrity hashes, provider tokenization, retries and cancellation are absent.
- Source markers do not yet navigate back to or highlight their originating card.
- Focused UI tests were updated but not run during rapid iteration.

## Recommendation

Continue with source navigation and artifact persistence before connecting a real model: clicking `[n]` should reveal/highlight the card, while the resolver should read versioned task artifacts from the capability runtime workspace through the Host boundary.
