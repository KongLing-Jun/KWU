# Implementation Plan: Remove Project Automation

## Overview

Remove project-scoped automation from the desktop, cloud API, database schema, and cloud admin UI while preserving personal/global automation. Historical migrations remain immutable; a forward migration removes deployed project-automation tables and obsolete idempotency rows.

## Architecture Decisions

- Project automation is identified by project-specific UI, Wails methods, cloud routes, `project_automations` / `project_automation_runs`, and `Task.CloudProject` governance.
- Personal automation, its scheduler, archive, history, and global sidebar remain supported.
- Removal is performed from consumers toward foundations, with compilation and focused tests after each slice.

## Task List

### Phase 1: Desktop surface

- Remove project automation tabs, cards, dialogs, state, types, gateway and bridge methods.
- Delete the project task/run panel and its project-only tests.
- Verify project UI tests, TypeScript typecheck and production build.

### Phase 2: Desktop runtime

- Delete project automation Wails endpoints, capability persistence, scheduling runner and run journal.
- Remove project-only branches from the shared automation service while preserving personal automation.
- Verify focused Go tests and `go test ./desktop ./internal/automation ./internal/project`.

### Phase 3: Cloud backend

- Remove project automation routes, request/response models, service and store queries.
- Remove automation projections from project admin governance and deletion flows.
- Add a forward migration that drops project automation tables and retired idempotency rows.
- Verify migration, project service and router tests.

### Phase 4: Cloud admin and cleanup

- Remove project automation counts and governance UI from the admin repository.
- Remove project-automation E2E scripts and current contract references.
- Scan all three repositories for executable project automation remnants.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Personal automation is removed accidentally | High | Keep global automation tests running after every desktop runtime slice. |
| Deployed cloud databases retain obsolete data | High | Use a forward migration; do not rewrite migration 00081. |
| Old clients call removed routes | Medium | Contract removal is deliberate; deployment must coordinate desktop and cloud versions. |
| Project deletion or admin queries reference dropped tables | High | Remove every query before the migration and test both clean and upgraded schemas. |

## Acceptance Criteria

- No project UI exposes automation creation, scheduling or run history.
- No Wails or cloud API exposes project automation methods/routes.
- Current cloud schema contains no project automation tables.
- Personal/global automation remains buildable and tested.
- Three repositories build and their relevant tests pass, apart from documented unrelated baseline failures.

