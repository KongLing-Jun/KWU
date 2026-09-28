# Repository Map

## Runtime and commands

KWU Desktop is a Wails shell around the shared Go kernel. `desktop/app.go` is the exported command surface, `desktop/tabs.go` owns active workspace/session identity, and `desktop/frontend` is the React/Vite client. Targeted Desktop Go checks run from `desktop`; frontend production validation is `pnpm build` from `desktop/frontend`.

## Existing prototype

The v007 prototype is isolated under `desktop/frontend/src/features/superpowers/`. It already has versioned `DashboardDocument`/`WidgetEnvelope` contracts, a trusted widget registry, host-style actions with revision and operation-id checks, deterministic research scenarios, and a browser-local `DashboardStore`. The same document projects into conversation-inline and research-canvas surfaces.

## Desktop integration seams

`App.activeTab()` exposes the active `WorkspaceTab`; a tab carries workspace root, topic ID and current session path. `App.currentProfilePathResolver(workspaceRoot).MemoryUserDir()` resolves the profile-owned state root. Existing Desktop code uses private directories and atomic temporary-file rename patterns. `desktop/frontend/src/lib/bridge.ts` provides the handwritten TypeScript interface for Wails methods, while feature-specific wrappers can detect the Wails runtime and keep a browser fallback.

## Persistence ownership

The session remains an observer and routing context. A task-scoped sidecar owns dashboard facts. A separate binding sidecar maps a hashed active-session identity to `{capabilityId, workspaceId, taskId, documentId}` without changing the core session schema. Dashboard files live below the profile state root and are addressed only through validated identifiers.

## v008 change surface

v008 adds a narrowly scoped Desktop Go store and three host commands: ensure a session/task binding, ensure/load a dashboard document, and apply one controlled dashboard action. The frontend prefers this Desktop bridge when available and falls back to the existing local adapter in a standalone browser. Demo-only partial/loading/empty scenarios remain deterministic projections and are not allowed to overwrite the task's durable ready document.

## Security and concurrency constraints

Identifiers are allow-listed and bounded; callers cannot supply paths. JSON payload and widget counts are bounded. Widget type/surface/version combinations remain finite. Writes are serialized in-process and use temp-file sync plus atomic rename. Actions require the bound active scope, matching base revision and unique operation ID.

## Verification

Because v008 changes Desktop Go and frontend code, validation is proportional: targeted Go tests for the new store/command package surface, `pnpm build`, and direct runtime inspection. Real marketplace APIs, real MCP transport, credentials and commerce write operations remain mocked or unavailable.
