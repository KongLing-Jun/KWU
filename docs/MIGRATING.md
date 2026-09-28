# Migrating from the legacy 0.x line

Clouvia 1.x is a ground-up Go rewrite of the legacy TypeScript/Node product.
It is a new codebase rather than an in-place runtime upgrade.

## Version lines

| | Legacy line | Current line |
| --- | --- | --- |
| Language | TypeScript / Node.js | Go core, Wails + React desktop |
| Branch | [`v1`](https://github.com/xiaoxiaotest9527-bit/Reasonix/tree/v1) | `main-v3` |
| Versions | `0.x` | `1.x` |
| User-facing brand | Earlier product names | 云智企办 (Clouvia) |
| Compatible CLI/module name | `suiyuan` / earlier layouts | `kwu` |
| Project config | Legacy JSON/TOML layouts | `clouvia.toml` |

Do not reuse a legacy executable and assume it is the current Go runtime. Check:

```sh
kwu --version
```

## Install or build the current line

Desktop packages are produced by the repository's desktop release workflow.
Release-specific changes are recorded in [`docs/releases/`](releases/README.md).

To build the CLI from source:

```sh
git clone https://github.com/xiaoxiaotest9527-bit/Reasonix.git
cd Reasonix
git switch main-v3
make build
```

The resulting compatibility CLI is `bin/kwu` (or `bin/kwu.exe` on Windows).
The old npm instructions from the 0.x line are historical and are not the
authority for current desktop installation.

## Configuration and user data

New profiles use:

- `~/.clouvia` on macOS and Linux;
- `%APPDATA%\Clouvia` on Windows;
- `clouvia.toml` for new project configuration;
- `.clouvia/` for new project-owned state.

Legacy homes, configuration filenames, and environment aliases remain readable
where compatibility is explicitly implemented. Migration is non-destructive;
do not delete the legacy directory until the current CLI and desktop app have
both been verified.

See [Configuration Paths](CONFIG_PATHS.md) for canonical paths, precedence,
credential storage, compatibility aliases, automatic migration, and the
interactive `/migrate` command.

## What to verify

After migration:

1. Run `kwu doctor`.
2. Confirm the expected profile and provider configuration.
3. Send a small model request.
4. Verify sessions, skills, plugins/MCP servers, and desktop login.
5. Keep the old data backup until all required workflows have passed.

The current product guide is [GUIDE.md](GUIDE.md); the Chinese guide is
[GUIDE.zh-CN.md](GUIDE.zh-CN.md).
