# Wails Browser Development Runtime

Wails 2.15.0 compiles its browser reconnect overlay with Svelte 5 but instantiates
it with `new Overlay`. Browser startup consequently throws from `/wails/ipc.js`.
The repository dev launchers use `scripts/prepare-wails-dev.mjs` to apply the
Svelte `mount` API fix to an isolated copy of the verified Wails dependency.

Preparation requires Node, npm, Go, and PowerShell 7 on Windows. It uses the
upstream npm lockfile and only builds `internal/frontend/runtime/dev`. Native
IPC and production runtime assets are checked for unexpected changes. The Go
module cache and tracked module files are never patched.

For direct PowerShell development:

```powershell
$env:GOWORK = (& node ./scripts/prepare-wails-dev.mjs).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Wails runtime preparation failed' }
Push-Location desktop
try { wails dev -nosyncgomod } finally { Pop-Location; Remove-Item Env:GOWORK }
```

The generated workspace, dependency copy, and provenance report are under
`.codex-run/wails-dev/runtime-*`. Keep that directory for the entire watcher
session so Go hot rebuilds retain the fix. Preparation deliberately rejects
pre-existing Go workspaces and unexpected Wails versions or source hashes.
After stopping its watcher, the corresponding generated directory may be removed.

Production builds must run without the temporary `GOWORK` setting. This fix does
not change tool-approval timing or suppress application error reporting.

When Wails fixes this upstream, verify browser cold load/reload, real bindings,
backend events, and repeated reconnects, then remove the preparation calls,
script, tests, and this note together. Do not update pinned hashes without
reviewing the new upstream entry point and rebuilt browser IPC.
