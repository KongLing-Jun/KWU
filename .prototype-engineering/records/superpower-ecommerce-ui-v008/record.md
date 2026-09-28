# Prototype Record

## Changes from parent

v008 replaces the complete-research document's browser-only persistence with a Desktop Host sidecar when the new Wails methods are present. It adds a separate session-to-task binding, a task-scoped JSON store, controlled dashboard mutations and an explicit browser/older-host fallback. Partial, loading and empty scenarios are now isolated projections that cannot overwrite the durable task document.

## Assumptions

- A persisted Desktop tab ID is a sufficiently stable prototype session-surface identity and does not change when a blank tab later receives a session path.
- One active session binds to one task per capability in this version.
- Single-process serialization is sufficient for the local prototype.
- Deterministic v007 fixtures remain the correct initial seed until connector/MCP work begins.

## Evidence

- `verification.md` records targeted sidecar tests, Desktop module compilation, frontend production build, whitespace validation and live Vite source availability.
- Wails dev regenerated the new bindings after the Go command surface changed.
- `05-desktop-sidecar-prototype-plan.md` captures the agreed ownership, API, file layout and next-stage MCP seam.

## Findings

- The capability can persist dashboard facts without modifying core session JSONL/schema or existing conversation services.
- Returning JSON strings across the Wails boundary avoids binding the flexible widget `props` union to generated Go/TypeScript model classes.
- The same five-action contract works on both sides: the current UI can use the Go Host now and future MCP tools can request the same actions later.
- Stable task IDs allow different sessions to bind to the same durable document while each binding remains independently authorized.
- Keeping demo scenarios out of the durable store prevents prototype controls from becoming accidental business-state mutations.

## Known gaps

- No real MCP server or platform connector invokes the Host actions yet.
- The prototype validates widget identity, type, version, surface, status and props discriminator, but not every nested props field against a generated JSON Schema.
- Multi-process locking, event subscriptions, cloud sync, migration and crash-recovery journals are not implemented.
- The current in-app browser control was unavailable for automated click/screenshot QA; manual visual feedback remains the next UI loop.
- Browser fallback is retained only for preview/older-host resilience and is not a second production source of truth.

## Recommendation

Continue. The next bounded version should expose the Host-owned artifact/dashboard operations through a capability-specific MCP/skill package and replace one deterministic research action with a mocked end-to-end MCP call, while keeping all real marketplace credentials and network writes out of scope.
