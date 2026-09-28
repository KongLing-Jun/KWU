# Prototype Record

## Changes from parent

- Replaced page-local widget constants with DashboardDocument v1 and WidgetEnvelope v1 contracts.
- Added host-style add, update, remove, move and attach-evidence reducers with revision CAS and operationId deduplication.
- Added a feature-local serialized DashboardStore adapter backed by browser localStorage.
- Added a finite renderer registry for research summary, trend chart, comparison bars and source coverage widgets.
- Moved both inline and canvas rendering onto the same document and registry.
- Changed the default business case from unsupported Amazon market claims to 1688 supply discovery plus an eBay US active-listing snapshot.
- Replaced exact monthly-sales, review-topic and brand-sales claims with scoped Mock samples, explicit limitations and a source coverage component.
- Preserved the catalog, empty input, capability-card transition, citation, scenario menu and collapsible canvas interactions.

## Assumptions

- The next uncertainty is the host/UI contract, not real marketplace connectivity.
- A browser-local serialized adapter is sufficient to test document restoration while keeping the future Go sidecar behind the same DashboardStore interface.
- Native renderers bundled with KWU are the appropriate first security boundary; arbitrary model-authored HTML, JavaScript and renderer URLs remain prohibited.
- 1688, eBay and Shopify should remain separate evidence roles rather than be normalized into a false universal market database.

## Evidence

- `.prototype-engineering/evidence/superpower-ecommerce-ui-v007/verification.md` records the successful production build and browser interaction checks.
- Browser inspection observed a complete document at revision 10, a citation mutation at revision 11, a partial document at revision 12 and revision 11 restoration after a full reload.
- Final reload produced no new browser console errors.

## Findings

- One serialized document can cleanly drive both conversation-inline and research-canvas surfaces without changing the existing KWU session implementation.
- The five proposed dashboard action shapes are sufficient for the current component lifecycle; their deterministic replay builds all declared scenarios.
- Source coverage works as a first-class UI component and prevents unavailable official metrics from being silently replaced by plausible-looking numbers.
- Keeping transient controls in React state while persisting only task facts produces the desired reload behavior: widgets restore, popovers and composer state do not.
- Existing card styling can be reused behind a registry without visible loss of structure.

## Known gaps

- Browser localStorage is not the intended production persistence layer and does not prove cross-process, cross-device or account-scoped durability.
- Runtime validation is structural TypeScript logic rather than generated JSON Schema validation.
- Session-to-task binding is fixed by prototype constants rather than resolved through the Go host.
- Real MCP transport, task progress, cancellation, audit logs and connector authorization are not exercised.
- All business values remain deterministic Mock data; platform feasibility is supported only by the separate research reports.
- Focused Vitest tests were updated but intentionally not run during this rapid-iteration pass.

## Recommendation

Continue with a host-sidecar prototype that implements the same DashboardStore contract on a task-scoped JSON file, resolves a real session binding, and exposes the five dashboard actions as Host-owned MCP tools. Keep data adapters mocked until that persistence and permission boundary is verified; then replace one read-only adapter at a time.
