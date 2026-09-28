# Prototype Record

## Changes from parent

- Added a Desktop-owned, task-scoped `EvidenceDocument` sidecar with atomic first-install, strict scope/schema/content/provenance checks and read-only load methods.
- Added a feature-owned evidence resolver that builds bounded untrusted context and separates clean visible text from hidden submit text.
- Adapted the isolated superpower workspace to the existing App session controller instead of adding a second conversation service.
- Projected live/final assistant messages into the matching capability follow-up.
- Made numbered answer sources navigate to, reveal, focus and transiently highlight the originating generated widget.
- Kept pure Vite execution as a visibly labelled deterministic browser preview.

## Assumptions

- The active Desktop tab is the intended conversation context for a superpower follow-up.
- Only one capability follow-up can be submitted while the current session is running; the existing controller remains the concurrency owner.
- Persisted evidence is data, not executable instruction, and external connectors will preserve the same provenance contract.
- Marketplace credentials and production approvals are unavailable during this prototype, so built-in commercial metrics remain Mock.

## Evidence

- `pnpm build` completed TypeScript checks and the production Vite build.
- `go test . -run '^TestCapability(Dashboard|Evidence)'` passed the affected Desktop persistence/action package tests.
- Static source inspection confirms Desktop-only session adapter injection, clean display/hidden submit separation and source-to-widget navigation.
- `l4p-docs/07-real-business-loop-validation.md` records the reproducible Desktop and browser validation paths.

## Findings

- The existing session controller is sufficient for model routing, streaming and persisted turns; the capability only needs an optional adapter and does not need a parallel session system.
- Dashboard and evidence JSON can remain independent sidecars while sharing the same validated task binding.
- A source list generated from the immutable follow-up request preserves navigation even when later card selections change.
- Browser preview can remain useful for visual iteration if it is explicitly distinguished from Desktop model execution and durable Host state.

## Known gaps

- No live Shopify, eBay or 1688 connector, OAuth flow, production approval or commerce write exists in this boundary.
- The built-in evidence metrics are Mock; official-research artifacts describe API capability only.
- Model-authored inline `[n]` markers are rendered as plain response text; the clickable source list is the supported navigation surface.
- Cloud sync, multi-user ACLs, artifact refresh/upsert, expiry and external connector audit are not implemented.
- A full pre-existing Desktop package run currently has unrelated failures in session reservation and WorkBuddy project workspace-policy tests; the directly affected capability tests pass.

## Recommendation

Treat v015 as the completed local workflow experiment and productionize the architecture in a separate hardening stream. The next uncertainty is external marketplace authorization and ingestion, not the local evidence-to-model-to-source loop.

## Productionization follow-up — 2026-08-25

- Replaced seeded dashboard/evidence startup with empty task-scoped documents.
- Removed the floating Seed/MCP control, deterministic browser persistence, Mock widgets and prototype research catalog from production UI paths.
- Added Host-side Shopify Storefront MCP projection from successful tool results into `live-public` evidence and a registered `product.catalog` widget.
- Preserved untruncated tool output only inside the Host event object; frontend event conversion continues to expose only bounded model/display output.
- Added first-load migration that removes legacy Mock/official-research seed artifacts, their widgets and prototype operation IDs.
- Kept connector configuration in the existing connector surface and the normal shared composer; the superpower workspace no longer owns credential or MCP installation forms.
- Exposed the shared session adapter's runtime-unavailable reason above the reused composer, so a disabled send action is no longer silent when the model service or restored session runtime is unavailable.

Validation: frontend production build passed; Desktop package tests passed; focused `internal/agent` truncation tests plus `internal/event` and `internal/eventwire` passed. The full `internal/agent` package still contains an unrelated temporary-path normalization failure in `TestSubagentArtifactSnapshotExcludesHiddenToolSupportFiles`.

## Productionization follow-up — 2026-08-26: open-box runtime

- Added a capability-owned, non-persistent managed MCP registration for Shopify Global Catalog at `https://catalog.shopify.com/api/ucp/mcp`.
- The connector is attached only to `crossborder-commerce` turns and is not written into user `config.toml`; ordinary chats remain unchanged.
- The integration is remote Streamable HTTP and therefore adds no local Node/npm runtime dependency.
- Declared the verified `search_catalog` operation as trusted read-only so the default approval mode does not interrupt catalog research.
- Added a hidden capability contract that supplies the official UCP agent profile, constrains data semantics, and tells the model that successful catalog results are projected into the right dashboard by the Host rather than by file tools.
- Preserved MCP `structuredContent` for the model and the Host-side product projection when a server omits duplicate text content.
- Removed decorative Sparkles icons from AI reply rows while preserving research-mode icons.

Live validation: with the manually configured Shopify connector removed from the active dev profile, a newly-created capability session accepted the plain-language request “帮我找 3 款网球拍，列出价格和商品链接，并把结果整理到右侧看板。” It automatically called `mcp__shopify-global-catalog__search_catalog`, completed without an approval prompt or file write, rendered the model answer, and projected three live-public product cards into the right dashboard.

## Productionization follow-up — 2026-08-26: session and aggregate dashboards

- Preserved the existing `primary` document as the intentional cross-session aggregate dashboard.
- Added a Host-assigned `sessionDocumentId` to the sidecar binding with deterministic migration for existing bindings.
- Routed MCP projections and restored capability presentation to the session document by default while allowing the bound aggregate document through the same validated Host boundary.
- Added explicit “本次会话 / 聚合看板” scope switching, hover promotion into the aggregate view, empty/loading/error states and reload on sidebar navigation.
- Kept task evidence shared so promoted widgets preserve their evidence references without duplicating artifacts.
- Hid the now-implicit Shopify Global Catalog connector selection from the reused superpower Composer while leaving ordinary Composer connector behavior unchanged.

Validation: frontend production build passed; focused Desktop capability dashboard/session/evidence tests passed. Existing aggregate JSON was preserved in place and no user document was rewritten during implementation.

## Productionization follow-up — 2026-08-26: smart chart transformations

- Added schema-validated `chart.bar`, `chart.histogram` and `chart.donut` widgets to the shared dashboard registry.
- Added deterministic transformations from persisted `product.catalog` widgets into product-price comparison, automatic price buckets and merchant-domain distribution.
- Routed generated charts through the existing `dashboard.add_widget` Host action instead of keeping them in transient component state.
- Added an explicit “组织为图表” menu and matching natural-language intent detection in the reused Composer.
- Decoupled local dashboard restoration from remote MCP runtime readiness so persisted work remains visible during connector outages.
- Preserved data semantics: catalog prices are snapshots, merchant distribution is not market share, and none of the generated charts imply sales volume.

Live validation used three persisted Shopify Global Catalog widgets with 11 deduplicated products. All three chart forms rendered, the aggregate document reached six widgets, and all charts restored after leaving and reopening the capability task.

## Productionization follow-up — 2026-08-26: AI-owned board controls

- Removed the manual “组织为图表” menu; chart composition remains driven from the shared AI conversation workflow.
- Added a uniform top-right close action to every generated card and routed removal through the existing revision-checked `dashboard.remove_widget` action.
- Added a left-click detail dialog that reuses the same Widget JSON and renderer, includes evidence references, and preserves internal link behavior.
- Kept right-click multi-source citation independent from left-click detail inspection.
- Clearing a card also clears its transient citation/detail selection but does not delete shared task evidence or a same-id card in the other dashboard scope.

Browser verification confirmed that the manual chart menu is absent, seven persisted cards exposed close actions, a product card opened a modal with its three live catalog items and evidence reference, right-click selected a donut chart without opening the modal, and removing one duplicate derived bar chart persisted the aggregate count from seven back to six.
