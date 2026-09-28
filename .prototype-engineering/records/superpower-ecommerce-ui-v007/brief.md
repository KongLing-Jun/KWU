# Prototype Brief

## Hypothesis

Replacing the page's static generated components with a versioned DashboardDocument, host-style mutation actions and a trusted widget registry will demonstrate the minimum safe contract for AI-composed inline and canvas UI, while a serialized adapter will show that task facts can survive a page reload without changing KWU's existing session services.

## Primary user flow

Open the cross-border ecommerce capability, review a completed 1688 supply plus eBay US marketplace study, inspect the source-coverage component, switch to partial/loading/empty deterministic scenarios, cite evidence, and reload the page to recover the current task document.

## Success criteria

- A single DashboardDocument contains both `conversation-inline` and `research-canvas` widgets.
- All widget rendering resolves through a finite registry with version, status and surface checks.
- Add, update, remove, move and attach-evidence actions enforce `baseRevision` and idempotent `operationId` behavior.
- Complete, partial, loading and empty documents share the same contract.
- The default case no longer presents unsupported Amazon, exact eBay monthly-sales, review-text or natural-keyword claims.
- A source-coverage widget exposes connected, restricted, inferred and unavailable evidence.
- The current task document can be serialized and restored through a feature-local store adapter.
- Existing catalog, composer, evidence citation, capability transition and canvas collapse interactions remain usable.

## Fidelity and data mode

High-fidelity local UI backed by deterministic DashboardDocument fixtures and a browser-local serialized JSON adapter standing in for the future Go sidecar store. All displayed values remain explicitly marked as prototype data.

## Out of scope

Real MCP transport, real platform APIs and OAuth, Go file persistence, SQLite, production session binding, package installation, remote renderers, arbitrary code generation, write-side commerce actions and multi-user collaboration remain out of scope.
