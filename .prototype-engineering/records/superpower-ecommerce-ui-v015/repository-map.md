# Repository Map

## Runtime and commands

KWU Desktop is a Wails shell around the shared Go kernel. `desktop/app.go` is the exported command surface, `desktop/tabs.go` owns active workspace/session identity, and `desktop/frontend` is the React/Vite client. Targeted Desktop Go checks run from `desktop`; frontend production validation is `pnpm build` from `desktop/frontend`.

## Existing prototype

The feature is isolated under `desktop/frontend/src/features/superpowers/`. v008 added a Desktop-owned task JSON sidecar, hashed tab binding, strict dashboard actions and a browser fallback. The same document projects into conversation-inline and research-canvas surfaces through a finite registry.

## Desktop integration seams

`App.activeTab()` exposes the active `WorkspaceTab`; a tab carries workspace root, topic ID and current session path. `App.currentProfilePathResolver(workspaceRoot).MemoryUserDir()` resolves the profile-owned state root. Existing Desktop code uses private directories and atomic temporary-file rename patterns. `desktop/frontend/src/lib/bridge.ts` provides the handwritten TypeScript interface for Wails methods, while feature-specific wrappers can detect the Wails runtime and keep a browser fallback.

## Persistence ownership

The session remains an observer and routing context. A task-scoped sidecar owns dashboard facts. A separate binding sidecar maps a hashed active-session identity to `{capabilityId, workspaceId, taskId, documentId}` without changing the core session schema. Dashboard files live below the profile state root and are addressed only through validated identifiers.

## v009 change surface

v009 adds one deterministic capability-tool adapter that behaves like the future MCP boundary: a named tool produces one declared dashboard action, delegates validation/persistence to the existing Host action API, and emits a document-changed event. The frontend subscribes, reloads the JSON, and renders the newly added registered widget. A floating demo control invokes the adapter without entering the normal product layout.

## v010 change surface

`WidgetRenderer.tsx` is the common seam for all four registered widget types. Citation is currently a `TrendChartProps.citable` exception, and the conversation renderer does not pass the citation callback to inline widgets. v010 removes that business-props flag, places a shared citation action in every widget header, and routes inline and canvas widgets through the same callback.

## v011 change surface

The user rejected explicit citation buttons. v011 keeps citation at the common `WidgetRenderer` seam but changes the input model to card-level hover affordance plus `contextmenu` selection. `EcommerceWorkspace` owns the single selected widget ID so removing or replacing the composer reference updates card state consistently.

## v012 change surface

Research follow-ups need evidence synthesis rather than a single active source. v012 replaces the scalar selected-widget/reference state with an ordered citation collection. Right-click becomes a toggle, composer references render as independently removable chips, and each submitted follow-up captures an immutable snapshot of the selected sources so later selection changes do not rewrite conversation history.

## v013 change surface

The feature currently captures citation labels only inside component state. The next reusable seam belongs in `runtime/`: a pure request builder can combine the active DashboardDocument, selected widget IDs, research mode and user question into a bounded protocol object. This keeps raw widget props out of model context, preserves document/widget revisions and evidence IDs, and leaves the eventual conversation transport behind an explicit adapter boundary.

## v014 change surface

`runtime/followUpRequest.ts` now defines a compact transport envelope but execution is still represented by static answer copy. v014 moves evidence resolution and answer execution behind feature-owned interfaces. A task-scoped deterministic catalog stands in for future artifact storage, a context assembler labels resolved material as untrusted evidence rather than instructions, and a mock execution adapter produces citation-aware answer blocks consumed by the existing conversation UI.

## v015 closure surface

The remaining locally verifiable loop spans three existing seams: Desktop sidecar storage beside `dashboards/`, the shared App `handleSend`/controller path, and `data-widget-id` registry wrappers. v015 persists a validated EvidenceDocument under the bound capability task, injects an optional existing-session execution callback into the isolated superpower workspace, observes the active session's assistant stream without modifying session schemas, and lets numbered answer sources reveal and focus their originating cards. Pure Vite keeps the deterministic adapter as an explicit preview fallback.

## Security and concurrency constraints

Identifiers are allow-listed and bounded; callers cannot supply paths. JSON payload and widget counts are bounded. Widget type/surface/version combinations remain finite. Writes are serialized in-process and use temp-file sync plus atomic rename. Actions require the bound active scope, matching base revision and unique operation ID.

## Verification

Because v009 changes Desktop Go and frontend code, validation is proportional: targeted tests for the mock tool/action composition, `pnpm build`, and runtime source/document inspection. Real MCP protocol transport, model execution, marketplace APIs, credentials and commerce writes remain outside the boundary.
