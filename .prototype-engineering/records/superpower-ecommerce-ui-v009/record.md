# Prototype Record

## Changes from parent

v009 adds a finite Mock MCP-style tool adapter, a Host commit event, a frontend task-document subscription and a floating prototype trigger. The tool adds a labeled MOQ comparison widget only through the existing Host action and JSON sidecar path.

## Assumptions

- One fixed tool is enough to test the architectural seam before implementing MCP protocol transport.
- Wails process-local events are sufficient for mounted-view refresh; JSON remains the recovery source of truth.
- A stable operation ID is appropriate because this tool represents one durable research artifact per task.

## Evidence

- `verification.md` records targeted Go behavior, frontend build, regenerated Wails binding and live served-module checks.
- The tool fixture and UI label explicitly identify their data as Mock.
- The execution plan is stored in `l4p-docs/06-mock-mcp-dashboard-loop-plan.md`.

## Findings

- The proposed separation works end to end: a capability adapter can compose a business action without knowing paths or UI implementation.
- Host events can remain hints; the frontend reloads the canonical DashboardDocument rather than trusting event payloads or tool output as state.
- Fixed operation IDs provide useful artifact-level idempotency and avoid duplicate components on retry.
- The existing registry is sufficient to render a newly tool-generated component without adding arbitrary code execution.

## Known gaps

- This is not an MCP protocol implementation and does not exercise subprocess lifecycle, tools/list, tools/call or authentication.
- There is no durable tool-run/progress/cancel record.
- Event delivery is process-local and not ordered across multiple processes.
- The mock tool currently has one fixed recipe and one deterministic dataset.
- Visual interaction was not automatically captured in this run.

## Recommendation

Continue. The next version should introduce a capability package manifest and a real local MCP adapter boundary with only `dashboard.*`/`artifact.*` tools, while initially keeping the MCP server deterministic and offline.
