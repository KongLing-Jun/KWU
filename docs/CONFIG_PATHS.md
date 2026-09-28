# Configuration Paths

Clouvia uses one user-facing home directory for global configuration and
user-owned state. The CLI and desktop app share the same profile.

## Clouvia Home

| Platform | Default home |
| --- | --- |
| macOS | `~/.clouvia` |
| Linux | `~/.clouvia` |
| Windows | `%APPDATA%\Clouvia` |

Set `CLOUVIA_HOME` only for tests, CI, portable installations, or isolated
development profiles. `KWU_HOME`, `SUIYUAN_HOME`, and `REASONIX_HOME` remain
recognized as legacy aliases.

## Contents

| Data | Path |
| --- | --- |
| Global config | `<Clouvia home>/config.toml` |
| Provider credentials | `<Clouvia home>/.env` |
| Legacy credentials import source | `<Clouvia home>/credentials` |
| Global slash commands | `<Clouvia home>/commands/` |
| Global skills | `<Clouvia home>/skills/` |
| Project-local MCP trust store | `<Clouvia home>/trust.json` |
| Sessions | `<Clouvia home>/sessions/` |
| Archives | `<Clouvia home>/archive/` |
| Memory | `<Clouvia home>/memory/` and `<Clouvia home>/projects/` |

New project-local configuration is named `clouvia.toml`. Existing
`kwu.toml`, `suiyuan.toml`, and `reasonix.toml` files are still read for
compatibility. New project-owned commands, skills, attachments, and
AutoResearch state use the `.clouvia/` directory.

## Resolution Order

```text
CLI flag
> project ./clouvia.toml (or a recognized legacy filename)
> active-profile <Clouvia home>/config.toml
> built-in defaults
```

User/global-only fields are not overridden by project configuration.

## Credentials

Provider entries use `api_key_env` to name a secret:

```toml
[[providers]]
name = "example"
kind = "openai"
base_url = "https://api.example.com/v1"
model = "example-model"
api_key_env = "KWU_API_KEY"
```

The value is stored in the active profile's `<Clouvia home>/.env`:

```dotenv
KWU_API_KEY=replace-with-your-key
```

The `KWU_*` key prefix remains valid because provider and cloud contracts keep
their compatibility identifiers. Path overrides use the canonical
`CLOUVIA_HOME`, `CLOUVIA_STATE_HOME`, and `CLOUVIA_CACHE_HOME` names.

Project `.env` files are not a provider-key fallback. They are only used for
workspace-scoped MCP/plugin `${VAR}` expansion, and stable Clouvia/KWU control
variables are not imported from them.

## Migration

Clouvia checks supported legacy homes and filenames when the canonical profile
does not already contain the corresponding data. Migration is non-destructive:
legacy files remain in place after compatible data is copied or converted.

Supported legacy migration runs automatically for normal CLI startup paths. In
an interactive TUI session, it can also be requested explicitly with:

```sh
/migrate
```

Before manual migration:

1. Back up the legacy directory.
2. Do not overwrite an existing canonical `config.toml` or `.env`.
3. Run `kwu doctor` and a simple model request after migration.
4. Keep the backup until desktop login, sessions, skills, and plugins
   have all been verified.

For the current Chinese reference, see
[CONFIG_PATHS.zh-CN.md](CONFIG_PATHS.zh-CN.md).
