# Prototype Brief

## Hypothesis

A capability-specific MCP-style tool can request a Host-owned dashboard action and cause every bound UI surface to update from persisted JSON, without granting the tool direct UI or filesystem access.

## Primary user flow

The user opens the complete cross-border research scenario and triggers the floating “Mock MCP 生成供给卡” control. The deterministic tool adapter composes an `add_widget` action, the Host validates and persists it, emits `capability:dashboard-changed`, and the frontend reloads the task document and renders the new comparison component.

## Success criteria

- The mock tool is selected by a finite tool name and accepts no path or executable UI source.
- It delegates mutation to the same revision/idempotency action path intended for real MCP tools.
- The resulting widget survives reload in the task JSON.
- The UI updates via Host event subscription, not by trusting the tool response as UI state.
- Replaying the tool is idempotent.
- Unsupported tools and stale actions do not mutate the document.

## Fidelity and data mode

Real local Host persistence/event integration with a deterministic MCP and research-data substitute.

## Out of scope

MCP protocol process transport, model calls, tool discovery handshake, streaming progress, real connector data, credentials, arbitrary component schemas, cloud sync and commerce write operations.
