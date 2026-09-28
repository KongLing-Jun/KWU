# Prototype Brief

## Hypothesis

A capability-owned evidence resolver and execution adapter can turn the bounded follow-up request into citation-aware answers without coupling the superpower workspace to the existing session implementation.

## Primary user flow

The user selects cards and submits a question. The execution pipeline resolves only evidence IDs authorized for the active capability/task, assembles bounded hidden evidence context, invokes a replaceable adapter, and renders answer blocks plus numbered source mappings back to the selected cards.

## Success criteria

- Resolver output is scoped to capability/workspace/task and reports unresolved IDs.
- Hidden context marks evidence content as untrusted data and remains bounded.
- The execution adapter consumes the same v1 request/context contract intended for a future real model adapter.
- Answer blocks carry citation orders that map to selected cards.
- Empty and partially unresolved evidence produce explicit, non-fabricated responses.

## Fidelity and data mode

High-fidelity local execution boundary using deterministic artifact records and a deterministic mock model adapter. No existing session data or service is mutated.

## Out of scope

Real model calls, provider credentials, streaming transport, session persistence, MCP artifact storage, production ACLs, provider tokenization, retries and cancellation.
