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

## Security and concurrency constraints

Identifiers are allow-listed and bounded; callers cannot supply paths. JSON payload and widget counts are bounded. Widget type/surface/version combinations remain finite. Writes are serialized in-process and use temp-file sync plus atomic rename. Actions require the bound active scope, matching base revision and unique operation ID.

## Verification

Because v009 changes Desktop Go and frontend code, validation is proportional: targeted tests for the mock tool/action composition, `pnpm build`, and runtime source/document inspection. Real MCP protocol transport, model execution, marketplace APIs, credentials and commerce writes remain outside the boundary.
