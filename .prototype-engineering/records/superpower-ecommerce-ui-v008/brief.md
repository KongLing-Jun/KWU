# Prototype Brief

## Hypothesis

A non-invasive Desktop sidecar can make AI-composed dashboard documents durable across sessions without changing the existing conversation service or allowing MCP/model output to manipulate UI or arbitrary files directly.

## Primary user flow

The user opens the cross-border ecommerce capability. The host binds the active session to a stable capability task, ensures the task's dashboard JSON exists, and renders it through the trusted registry. A controlled action such as attaching evidence updates the JSON with revision/idempotency checks. Reopening from another bound session restores the same task document. Standalone browser preview continues through the local fallback.

## Success criteria

- Desktop persists one task-scoped DashboardDocument under the active profile state root.
- Core session records and existing conversation services are unchanged.
- Only an exact active-session binding can load or mutate its declared document.
- Controlled add/update/remove/move/attach-evidence actions survive reload and reject stale revisions, duplicate operations and invalid widgets.
- The existing UI renders Desktop state when available and visibly reports browser fallback otherwise.
- Partial/loading/empty demo scenarios do not corrupt the durable complete-research document.

## Fidelity and data mode

High-fidelity local integration prototype with deterministic marketplace fixtures. Persistence, binding, validation and rendering are real; marketplace APIs, model reasoning and MCP transport remain substituted.

## Out of scope

Production authentication, multi-process locking, cloud sync, package installation, credentials, real Shopify/eBay/1688 calls, arbitrary AI component source, generalized workflow execution, and write-side commerce operations.
