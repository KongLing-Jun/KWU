# Prototype Brief

## Hypothesis

Multi-source card selections can safely enter an eventual model workflow through a small, versioned request envelope that carries provenance identifiers instead of serializing arbitrary generated component content.

## Primary user flow

The user selects several generated cards and submits a follow-up. The UI builds a request containing the question, capability/task/document identity, document revision, ordered widget identities and bounded evidence references; the submitted message exposes source count and revision while the deterministic prototype response consumes the same request snapshot.

## Success criteria

- The request contract is versioned and independent from React component state.
- Each citation preserves widget type/version/surface/revision plus its evidence IDs.
- Raw widget props are excluded from the request.
- Requests cap sources and evidence references deterministically and report omissions.
- Submitted follow-ups render from the immutable request snapshot.

## Fidelity and data mode

High-fidelity request composition using deterministic DashboardDocument fixtures. The actual model/session transport is represented by the existing local response UI and remains outside the prototype boundary.

## Out of scope

Actual model execution, evidence artifact resolution, provider-specific token counting, session persistence, permissions enforcement, streaming, cancellation, retries and production request IDs.
