# Changelog

All notable changes to the Go line (Clouvia 1.0+, with the compatible `kwu`
CLI/module name) are recorded here. The legacy
`0.x` TypeScript history lives on the legacy `v1` branch.

## Unreleased

## 1.3.0

- Cloud project collaboration and automation, account plans and credit recharge,
  the integrated model ecosystem, experimental cross-border ecommerce research,
  connector recovery, persistent diagnostics, and broad session, authentication,
  expert, project, and desktop interaction fixes. See
  [desktop release notes](docs/releases/v1.3.0.md).
- Agent runtime defaults now leave both executor and dedicated planner tool-call
  rounds unlimited (`max_steps = 0`, `planner_max_steps = 0`). Step limits now
  come from the user/global config only; project `clouvia.toml` (and recognized
  legacy project-config filenames) does not override them.

## 1.2.1

- Reliable completion for chat tasks that mention file-like outputs,
  capability-center navigation and responsive-layout stability, and
  cross-platform installation of remote HTTPS MCP connectors. See
  [desktop release notes](docs/releases/v1.2.1.md).

## 1.2.0

- Desktop automation, cloud project workflows, the public skills marketplace,
  managed connectors, composer controls, and broad session, expert, model, and
  responsive-layout fixes. See [desktop release notes](docs/releases/v1.2.0.md).

## 1.1.2

- Conversation and workspace list expansion, cloud expert favorites, media
  understanding, video-generation progress, shared model-catalog state, and
  broad desktop visual and interaction fixes. See
  [desktop release notes](docs/releases/v1.1.2.md).

## 1.1.1

- Windows installer and uninstaller reliability, expert capability menus,
  narrow-window layouts, ARM64 message-channel rendering, localized UI copy,
  and model-service error clarity. See
  [desktop release notes](docs/releases/v1.1.1.md).

## 1.1.0

- Password reset, user feedback, unified composer capability selection, account
  validation and localization, model-marketplace clarity, and desktop
  update/runtime resilience. See [desktop release notes](docs/releases/v1.1.0.md).

## 1.0.6

- Account, subscription, balance, model-selection, cross-client synchronization,
  and Windows single-instance fixes. See [desktop release notes](docs/releases/v1.0.6.md).

## 1.0.5

- Skill/MCP selection, cloud experts, media generation, session loading,
  Feishu synchronization, automation refresh, and account-center improvements.
  See [desktop release notes](docs/releases/v1.0.5.md).

## 1.0.4

- Platform-aware update detection and multi-platform release-candidate handling.
  See [desktop release notes](docs/releases/v1.0.4.md).

## 1.0.3

- Workbench, subscription, session, model, user-data migration, and desktop
  stability improvements. See [desktop release notes](docs/releases/v1.0.3.md).

## 1.0.2

- In-app updates and installation reliability improvements. See
  [desktop release notes](docs/releases/v1.0.2.md).

## 1.0.1

- In-app release notes, update fallback, account/model/automation/expert UX,
  and macOS Universal packaging. See [desktop release notes](docs/releases/v1.0.1.md).

## 1.0.0 - 2026-06-03

First stable release — a **ground-up rewrite in Go**. Not an upgrade of the `0.x`
TypeScript line; a new codebase that became the default development line.
Active development now targets `main-v3`.

### Highlights

- **Go kernel**: a single static binary (CGO-free), cross-compiled for
  darwin/linux/windows on amd64 + arm64. Distributed via npm (the package wraps
  the native binary), Homebrew tap, and release archives;
  no Node runtime needed to run it.
- **Agent core**: the loop, built-in tools (read/write/edit/multi_edit/glob/grep/
  ls/bash/web_fetch/todo_write), permission gate, sandboxed bash, and the
  DeepSeek prefix-cache–oriented design.
- **Subagents**: `task` plus explore/research/review/security_review skill agents.
- **Skills & hooks**: Claude-Code-style skills (`internal/skill`) and hooks
  (`internal/hook`), symlink-aware and slash-integrated.
- **MCP client**: connect external servers over stdio / Streamable HTTP; reads
  `[[plugins]]` and a Claude-Code `.mcp.json`.
- **Code intelligence via CodeGraph**: a tree-sitter symbol/call graph
  (`codegraph_*` tools) replaces embedding semantic search — no embedding service
  or API cost. Fetched into a local cache on first use (or `kwu codegraph
  install`) and indexed in the background, so installs and startup stay fast.
- **Plan mode** with evidence-backed step sign-off (`complete_step`).
- **Memory**: `KWU.md` hierarchy + auto-memory, folded into the cache-stable
  prefix.
- **ACP** (`kwu acp`) and an HTTP/SSE server frontend; desktop app (Wails).

### Fixed

- **File encoding support restored** — GBK/GB18030 (and other non-UTF-8) files
  can now be read, edited, and grepped correctly. The v2 rewrite had dropped
  v1's encoding detection; files in CJK Windows charsets were silently misread
  or rejected as binary. The read/edit/write round-trip now preserves the
  original file encoding. (#2637)

### Notes

- Versions: the legacy TypeScript line stays in `0.x`; the Go line starts at
  `1.0.0`. See [docs/MIGRATING.md](docs/MIGRATING.md).
- Release archives ship a bare binary; CodeGraph is fetched on first use. Windows
  support for the fetched runtime is unverified — install `codegraph` on PATH if
  the auto-fetch doesn't resolve there.
