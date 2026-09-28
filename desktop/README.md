# 云智企办 Desktop (Wails shell)

A native desktop window around the Clouvia Go kernel (`kwu` module). The same
transport-agnostic `control.Controller` that backs the chat TUI and the HTTP/SSE
server is bound **directly** to a React webview — Go methods in, typed events
out, no HTTP hop.

```
┌─────────────────────────────────────────────────────────────┐
│  webview (React + TS, Vite)                                  │
│    bridge.ts ──calls──▶ window.go.main.App.{Submit,Cancel,…} │
│    bridge.ts ◀─events── window.runtime.EventsOn("agent:event")│
└───────────────▲───────────────────────────┬─────────────────┘
        bound methods                  runtime.EventsEmit
┌───────────────┴───────────────────────────▼─────────────────┐
│  desktop/app.go   App (bound)  +  eventSink (event.Sink)     │
│  desktop/main.go  Wails options, window, embed frontend/dist │
└───────────────▲───────────────────────────┬─────────────────┘
       commands │                            │ typed event stream
┌───────────────┴────────────────────────────▼────────────────┐
│  internal/boot.Build → internal/control.Controller (kernel)  │
│  (same assembly the CLI uses: providers, tools, gate, …)     │
└──────────────────────────────────────────────────────────────┘
```

## Why a nested module

`desktop/` is its own Go module (`module kwu/desktop`, `replace kwu =>
../`). That keeps the CGO + WebKit desktop build entirely separate from the CLI's
`CGO_ENABLED=0` single-static-binary guarantee: the parent module's `go build /
vet / test ./...` skip this directory, while the import path stays under
`kwu/` so it can still import the `kwu/internal/*` kernel.

## Prerequisites

- Go (matches the parent module).
- Node + **pnpm** (`npm i -g pnpm`).
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Platform webview libs: macOS ships WebKit; Windows needs the Edge **WebView2**
  runtime; Linux needs `libgtk-3-dev` plus WebKitGTK. The default build links
  against **WebKitGTK 4.0**; distros that only ship **4.1** (Fedora 40+, Ubuntu
  24.04+, Arch) build with `-tags webkit2_41` — see [Build](#build). Run
  `wails doctor` to verify.

## Develop

```sh
./dev                 # hot-reloads Go + frontend (Vite dev server)
```

For browser-based development and testing, open the `vite` URL printed by
`./dev` (for example, `http://127.0.0.1:5291/`). Vite proxies `/wails` HTTP and
WebSocket traffic to the printed Wails dev-server port, so the browser still
uses the real Go bindings. The Wails dev-server port is an internal transport
endpoint and should not be used as the browser entry point.

Development builds keep all private user data separate from an installed
release. The resolved `KWU_CLOUD_BASE_URL` is normalized and hashed, so every
cloud environment gets a stable profile across branches, worktrees, and working
directories. On Windows the default profile lives below
`%APPDATA%\kwu-dev\environments\<id>` (with cache data below
`%LOCALAPPDATA%`; WebView2 data is a sibling of that environment's cache); on macOS and Linux it lives below
`~/.kwu-dev/environments/<id>` (with cache data below the OS cache directory).
Set `KWU_DEV_PROFILE_ROOT` to an absolute directory when an integration run
needs a disposable location; the desktop refuses production data directories
and records the normalized cloud identity in `dev-environment.json`.

Direct `wails dev` is also supported and uses the same cloud-environment
profile initialization.

### Integrated three-repository runtime

Run the canonical Cloud, Admin, and Desktop stack from the Desktop repository:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\reasonix-dev.ps1 up
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\reasonix-dev.ps1 verify
```

The launcher resolves matching dated worktrees, compiles Cloud, injects one
canonical Cloud URL into Admin and Desktop, and writes only non-secret process
metadata to `%LOCALAPPDATA%\Reasonix\dev\runtime.json`. Desktop checks the
manifest URL and Cloud database identity before constructing authenticated
state. Use `status` for a verified inventory and `down` for scoped shutdown;
ownership mismatches fail closed and do not stop the foreign process.

Frontend-only iteration without the Go side:

```sh
cd desktop/frontend
pnpm install
pnpm dev             # opens in a plain browser; bridge.ts uses the dev mock
```

In a plain browser the native bindings are absent, so `bridge.ts` falls back to a
**mock** that streams a canned turn (text + one `edit_file` tool call) through the
exact same event contract — so layout, streaming, markdown, tool cards, and the
diff seam can all be built without rebuilding Go.

### Frontend UI review checklist

For anchored menus, dropdowns, tooltips, and other portaled UI, review both the
component code and the CSS positioning contract:

- If a component uses `createPortal` plus `getBoundingClientRect()`, it must
  handle scrollable ancestors, window resize, and `visualViewport` changes.
- Add a focused regression test when changing shared positioning primitives such
  as `AnchoredPopover`, not only the specific menu that exposed the bug.
- Exercise at least one scrollable container path, such as Settings content, when
  manually checking dropdown or popover changes.

## Build

```sh
cd desktop
wails build          # → build/bin/kwu-desktop(.app/.exe); release packaging renames it to 云智企办
```

**Linux on WebKitGTK 4.1 only** (Fedora 40+, Ubuntu 24.04+, Arch — no
`webkit2gtk-4.0` package): pass the Wails build tag so cgo links against 4.1.

```sh
wails build -tags webkit2_41
wails dev   -tags webkit2_41   # same tag for hot-reload
```

Fedora deps: `sudo dnf install webkit2gtk4.1-devel gtk3-devel`.

`frontend/dist` is generated by the build (it's git-ignored except for a
`.gitkeep` that keeps the Go `//go:embed all:frontend/dist` compilable on a fresh
checkout). A bare `go build` without a prior `pnpm build` produces a blank window.

## Releases & auto-update

Production desktop releases follow the Actions-first, manually approved process
in the [desktop release manual](../docs/releases/desktop.md). An annotated
`desktop-vX.Y.Z` source tag builds a complete Draft Release; a separate manual
workflow signs and publishes it after review. The two-machine local process remains
available for Actions outages and can take over the same Draft. Historical
`manual-desktop-vX.Y.Z` tags are read-only compatibility inputs.

Phase one publishes only Windows amd64/arm64 installers and one macOS universal
DMG. Linux and portable ZIP artifacts are not published.

The macOS release bundle is `云智企办.app`, with bundle identifier
`ai.clouvia.desktop` and executable `Contents/MacOS/clouvia-desktop`. The DMG
verification gate checks those identities, both numeric bundle version fields,
and the Intel + Apple Silicon slices before upload.

macOS and Windows desktop builds use only the canonical user-data roots;
former-product data is left in place but is not read, migrated, or deleted.
Linux desktop and CLI compatibility remain unchanged.

The app checks the configured update manifest on startup and shows an update
banner when a newer version is published; **Settings → Software update** has a
manual check. Stable builds poll the owned OSS manifest first and GitHub Releases
as a fallback. When an update exists, the app also fetches the matching public
cloud release record for Markdown release notes and the macOS manual-download
URL. Cloud metadata failure never blocks manifest-based update detection.

Self-update behavior by platform:

- **Windows** — download, verify the minisign signature, then run the per-user
  NSIS installer (no admin rights needed).
- **macOS** — *not* self-updating yet. The build is unsigned/un-notarized, so an
  in-place swap would be blocked by Gatekeeper; the banner opens the matching OSS
  universal DMG, with the GitHub Latest Release page as fallback.

### Unsigned builds — first launch

There are no Apple/Windows code-signing certificates yet, so a downloaded build
trips the OS gatekeepers on first run:

- **macOS** — open `Clouvia-darwin-universal.dmg` and drag 云智企办 into
  Applications. Gatekeeper may then report the app "is damaged" or is from an
  unidentified developer; clear the quarantine attribute and open it:
  ```sh
  xattr -dr com.apple.quarantine /Applications/云智企办.app
  ```
- **Windows** — SmartScreen shows "Windows protected your PC". Click *More info →
  Run anyway*.

When Developer ID / Authenticode certificates are added, the release workflow's
`HAS_APPLE_CERT` gate flips to the signed path and these steps go away.

### Verifying a download

Artifacts are signed with minisign (public key ID `E33B7107401DB17B`). The `.minisig`
signature sits next to each artifact in the release; verify with the
[minisign](https://jedisct1.github.io/minisign/) CLI:

```sh
minisign -Vm Clouvia-darwin-universal.dmg \
  -P RWR7sR1AB3E74wIqiyRDnAuF8ygAm3BRY/oySqdbI/99aexhhBMc6nb0
```

## Editor seam (Monaco / CodeMirror)

Code and diff rendering go through two components with stable prop contracts and a
lazy boundary, so a heavy editor stays out of the initial bundle and dropping one
in is a one-line change — no consumer touches:

| Component | Props | Default impl | Upgrade |
|---|---|---|---|
| `components/CodeViewer.tsx` | `EditorProps` | `editors/PlainCode.tsx` (`<pre>`) | swap the lazy import for `editors/MonacoCode` or `editors/CodeMirrorCode` |
| `components/DiffView.tsx` | `DiffProps` | `editors/PlainDiff.tsx` (LCS line diff) | swap for `editors/MonacoDiff` or `editors/CodeMirrorMerge` |

```sh
# Monaco
pnpm add @monaco-editor/react monaco-editor
# or CodeMirror 6
pnpm add @uiw/react-codemirror @codemirror/lang-javascript @codemirror/merge
```

Then add `editors/MonacoCode.tsx` (default-export a component taking
`EditorProps`) and point `CodeViewer.tsx`'s `lazy(() => import(...))` at it.
`ToolCard` already routes `edit_file` calls' `old_string`/`new_string` through
`DiffView`, and `Markdown` routes fenced code blocks through `CodeViewer`, so
both seams light up everywhere at once.

Markdown itself is currently minimal (fenced code + plain text). Upgrade path:
`pnpm add react-markdown remark-gfm` and render in `components/Markdown.tsx`,
keeping fenced code delegated to `CodeViewer`.

## Multi-platform adaptation

Wails is the right shell for a Go kernel (no sidecar), but a Go+webview stack uses
the **native** webview per OS, so the rough edges are platform-specific. What's
handled here, and what to reach for if a target misbehaves:

- **Linux / WebKitGTK** is the one real pain point — rendering varies by distro &
  GPU driver. `main.go` keeps `WebviewGpuPolicy: OnDemand` when a DRI render node
  is usable, and falls back to `Never` for xrdp/headless/software-rendered sessions
  that cannot access `/dev/dri`. If artifacts persist, launch with
  `WEBKIT_DISABLE_COMPOSITING_MODE=1`. Test on at least one GTK target before release;
  the CSS deliberately avoids `backdrop-filter`/blur (slow & inconsistent there).
  - **Wayland + NVIDIA**: On KDE Plasma Wayland with NVIDIA GPUs, WebKitGTK can
    crash at startup (`Error 71: Protocol error`) due to an upstream WebKit
    explicit-sync bug (WebKit #280210, #317089, NVIDIA/egl-wayland #179).
    云智企办 automatically sets `__NV_DISABLE_EXPLICIT_SYNC=1` when it detects
    Wayland + NVIDIA GPU. To opt out, set `__NV_DISABLE_EXPLICIT_SYNC=0`.
    Alternative fallbacks: `WEBKIT_DISABLE_DMABUF_RENDERER=1` (poor performance)
    or `GDK_BACKEND=x11` (forces XWayland).
- **Windows / WebView2** — `Theme: SystemDefault` follows the OS light/dark
  setting; the installer embeds the WebView2 bootstrapper. Canary builds disable
  WebView2 GPU acceleration by default to smoke-test blank-window reports; set
  `KWU_DESKTOP_DISABLE_WEBVIEW2_GPU=1` or `0` to force the fallback on or
  off.
- **macOS / WebKit** — inset/hidden title bar (`TitleBarHiddenInset`); the CSS
  marks the top bar as an OS drag region (`--wails-draggable: drag`) and leaves
  room for the traffic lights.
- **Theming** — colors are CSS variables gated on `prefers-color-scheme`, which all
  three webviews honor, so the UI follows the OS theme without native glue.
- **Fonts / offline** — system font stack only; no web-font fetches, so first paint
  is instant and identical offline.
- **First paint** — the window background is set to the dark shell color so there's
  no white flash before CSS loads (most visible on WebKitGTK).

## Files

```
desktop/
  main.go            Wails options, window, embed frontend/dist
  app.go             App (bound command surface) + eventSink (event.Sink → webview)
  tabs.go            shared eventwire payload + desktop tab routing fields
  wails.json         Wails project config (pnpm install/build/dev)
  frontend/
    src/
      lib/
        types.ts         wire contract (mirrors internal/eventwire/wire.go)
        bridge.ts        window.go/window.runtime wrapper + browser dev mock
        useController.ts event-stream reducer + command surface (the hook)
      components/
        Transcript, Message, ToolCard, Composer, ApprovalModal, ContextGauge,
        Markdown, CodeViewer, DiffView
        editors/  PlainCode, PlainDiff   ← editor seam impls (swap targets)
```

## Telemetry and diagnostics

Anonymous launch telemetry and aggregate quality metrics are disabled by
default. The endpoints currently shipped with the desktop use reserved
`.example` hosts and are rejected at runtime, so no telemetry, metrics, crash,
performance-pressure, or bot diagnostic payload is sent. A Go panic may be
written temporarily to the local `crash-pending.json`; while reporting remains
unavailable, the next production launch discards that file without sending it.
Development builds never send launch telemetry.

Settings > Updates intentionally exposes no telemetry or metrics switch. The
`telemetry` and `metrics` fields under `[desktop]` remain configuration
compatibility inputs, but enabling them cannot bypass endpoint validation. If a
future production build supplies a real reporting endpoint, data is sent only
after explicit configuration opt-in and remains limited to the documented
anonymous install metadata or aggregate signal/bucket counts; conversations,
prompts, API keys, paths, base URLs, tool output, and file contents are excluded.

Crash, performance-pressure, and bot diagnostic interfaces are local copy-only
and do not expose a "Send report" action.

Memory v5 has been retired. The desktop no longer exposes its settings or
workflow metrics, and the runtime no longer reads or writes compiler state.
Legacy transcripts remain readable.
