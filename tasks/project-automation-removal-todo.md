# Project Automation Removal Checklist

- [x] Desktop project UI contains only activity, work items and assets.
- [x] Desktop frontend contains no project automation types, gateway or bridge methods.
- [x] Desktop Go exposes no project automation Wails methods or schedule runner.
- [x] Shared automation remains personal/global only.
- [x] Cloud project routes and application interfaces contain no automation endpoints.
- [x] Cloud project models, store and governance contain no automation projection.
- [x] Forward migration removes `project_automation_runs` and `project_automations`.
- [x] Cloud admin project UI contains no automation counts or governance section.
- [x] Project automation E2E scripts and active contract fixtures are removed.
- [x] Residual scan finds only historical documentation/migrations where retention is intentional.
- [x] Desktop frontend typecheck, focused tests and build pass; two unrelated account contract assertions remain in the legacy script suite.
- [x] Desktop Go focused packages and the full desktop module pass; the wider root suite was stopped after extended inactivity with no failure output.
- [x] Cloud backend vet, compile-only suite and focused migration/contract tests pass; database integration tests require `TEST_DATABASE_URL`.
- [x] Cloud admin tests, typecheck and build pass.
