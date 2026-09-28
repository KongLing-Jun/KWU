# Remove Profile Learning, Shell Hooks, and Network Proxy Settings

- [x] Remove Profile/Hooks/Network Settings navigation, pages, commands, DTOs, bridge methods, styles, locale keys, and feature tests.
- [x] Remove learned-profile config, boot/controller runtime, Desktop Wails API, package, and feature tests.
- [x] Extract project/MCP trust from `internal/hook`, retain backward-compatible trust data, and add a non-hook trust entry point.
- [x] Remove shell-hook agent/controller/boot/CLI runtime and feature tests.
- [x] Remove user proxy config from frontend/Desktop/config rendering while retaining automatic network infrastructure.
- [x] Audit PostgreSQL schema and confirm no matching tables/data are present.
- [x] Enumerate and remove only owned learned-profile/hook/network local data targets.
- [x] Run focused frontend and Go tests after each slice.
- [x] Run full build/test/vet/code-guard and review residual references and final diff (frontend full suite still reports two unrelated stale account-workspace source-contract assertions).
