# Repository Map

## Runtime and commands

KWU Desktop is a Wails shell around the shared Go kernel. The React frontend lives in `desktop/frontend`, uses pnpm, TypeScript and Vite, and is started through the repository-root `./dev` command. The active development instance exposes Vite at `http://127.0.0.1:5174/`. Frontend production validation is `pnpm build` from `desktop/frontend`.

## Relevant architecture

`desktop/frontend/src/App.tsx` owns the shell, Sidebar and top-level workspace selection. `desktop/frontend/src/app/useWorkspaceNavigation.ts` owns current workspace transitions. The existing prototype is isolated under `desktop/frontend/src/features/superpowers/`, with `SuperpowerWorkspace.tsx` holding catalog, conversation, deterministic scenarios, widget envelopes and the research canvas in one file. `superpowers.css` owns the feature styling.

The v006 prototype already established a shared envelope identity for `conversation-inline` and `research-canvas`, but its props, widget selection and rendering remain hardcoded inside the page component. It has no versioned document, renderer registry, mutation contract or persisted adapter.

## Existing state and persistence patterns

The prototype currently uses React component state for screen selection, demo scenario, selected capability, composer content, evidence citation and canvas collapse. This is appropriate for ephemeral presentation state. v007 will add a feature-local `DashboardStore` abstraction for business document state while leaving those transient controls in React state.

The first adapter will serialize DashboardDocument JSON to `localStorage`, namespaced by workspace/task/document. This is an explicit deterministic substitute for the future Go sidecar file store; no Wails bridge or Go package is changed in v007.

## Design and component patterns

The app uses CSS variables from `styles/tokens.css`, Lucide icons and feature-local React components. The current summary, trend, brand bars and canvas card styles can be reused by registered renderers. Existing DOM metadata (`data-widget-id`, generated type and surface) provides a stable inspection seam and will be expanded with status and version metadata.

## Data research inputs

The source reports under `l4p-docs/research/` establish three distinct roles: 1688 for supply discovery, eBay for current marketplace supply snapshots and Shopify for authorized merchant operations. They also establish that exact eBay monthly sales, eBay review text and all-market natural keyword trends are not available from the reviewed public official APIs. v007 must reflect those limitations in deterministic fixtures and a source coverage widget.

## Verification tools

The user requested rapid iteration without expanding the test loop. Verification for this version will therefore use TypeScript/build-level feedback only if needed to keep the page runnable, plus direct browser inspection of declared scenarios. Existing focused Vitest coverage remains available but is not expanded as part of v007.
