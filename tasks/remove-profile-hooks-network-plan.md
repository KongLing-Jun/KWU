# Implementation Plan: Remove Profile Learning, Shell Hooks, and Network Proxy Settings

## Scope

Remove the Desktop user-facing and runtime features named 用户画像, 自动化钩子, and 网络设置. Preserve account/profile identity isolation, project Automation, Cloud/MaaS/model HTTP infrastructure, web_fetch, updater traffic, sandbox network policy, and Admin/Cloud infrastructure.

Cloud and Admin contain no matching business API or PostgreSQL table. No database table is eligible for deletion.

## Dependency order

1. Remove the three Settings surfaces and obsolete frontend contracts.
2. Remove learned-profile runtime, management APIs, config, and tests.
3. Move shared project trust out of the hook package, then remove shell-hook runtime/API/CLI code.
4. Remove user-configurable proxy contracts while retaining automatic network infrastructure.
5. Remove owned local feature data at exact validated paths; never remove account `profiles/<profile_id>` roots.
6. Run boundary, frontend, Go, and repository guard checks.

## Safety decisions

- `ProfileID`, `ProfilePathResolver`, composer profiles, dev profiles, and Cloud bindings are identity/session infrastructure and stay.
- `project_automations` and `project_automation_runs` stay; they are not shell hooks.
- Project/MCP trust remains explicit and backward-compatible with existing trust files.
- `netclient`, `sysproxy`, Cloud/MaaS/model clients, updater, web_fetch, and sandbox network policy stay.
- Local cleanup targets only learned-profile directories and top-level hook/network configuration owned by these removed features.

## Verification checkpoints

- Frontend: removed tabs/commands/bridges are absent; typecheck, targeted tests, and production build pass.
- Profile: no learning/selection/management writer remains; account profile isolation tests pass.
- Hooks: no external shell hook executes; local MCP still requires explicit trust and can be enabled.
- Network: no proxy editor/API/config is exposed; core Cloud/model/network routing tests pass.
- Data: exact targets are enumerated before deletion; PostgreSQL schema audit confirms no matching table.
- Final: `go test ./...`, `go vet ./...`, code guard, diff checks, and focused residual-symbol scans.

## Risks

- Removing hook trust directly could weaken or permanently block local MCP startup; migrate trust first.
- Removing the base network layer would break core product functions; remove only user proxy configuration.
- The word “profile” is overloaded; only learned JSONL profile data is removed.
- Existing unrelated frontend test-type errors must be reported separately if still present.
