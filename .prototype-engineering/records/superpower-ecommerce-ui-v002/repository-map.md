# Repository Map

## Runtime and commands

The shipped Desktop application is a Wails shell around the shared Go kernel. The React frontend lives in `desktop/frontend`, uses pnpm and Vite, and is started with the repository-root `./dev` command. The current development instance exposes Vite at `http://127.0.0.1:5174/` and the Wails transport at `http://127.0.0.1:34174/`. Production frontend verification uses `pnpm build` from `desktop/frontend`.

## Relevant architecture

`desktop/frontend/src/App.tsx` owns the application shell, sidebar and top-level workspace selection. The `WorkspaceView` union and `useWorkspaceNavigation` centralize top-level view transitions. Feature workspaces are implemented below `desktop/frontend/src/features/`; CSS is composed through `src/styles.css`. The prototype can remain isolated as a new `features/superpowers` workspace while reusing the existing shell and sidebar.

## Existing design and implementation patterns

The active Creation layout renders primary feature buttons through `sidebar-feature-zone__item`, including selected and compact-sidebar states. Existing full-page workspaces render inside the shared `<main id="main-workspace">` surface and use CSS variables from `styles/tokens.css` for light/dark themes. Lucide supplies product-shell icons. Focused component tests use Vitest, jsdom and Testing Library under `src/vitest/`.

## Data and integration boundaries

The proposed surface is presentation-only. Real market data, model execution, MCP calls, authentication, persistence and export stay outside the experiment. Deterministic in-component fixtures will provide catalog cards, research evidence, product rows, task progress and follow-up messages. No Go bridge changes are required.

## Verification tools

Use a focused Vitest interaction test for the two-level navigation and evidence citation flow, `pnpm build` for the required frontend gate, and the running Wails/Vite application for browser screenshots at desktop and narrower widths. Repository-wide PR classification is provided by `node scripts/pr-gate.mjs --base origin/main-v3`.
