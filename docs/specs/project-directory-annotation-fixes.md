# Project directory annotation fixes

## Objective

Bring the Desktop project directory in line with annotations 4, 7, and 8: use the supplied monochrome collaboration artwork, expose per-project rename and delete actions, and provide exactly five complete project templates with the supplied agent instructions.

## Technology and existing contracts

- React 19, TypeScript, Vite, and the existing project feature CSS.
- Lucide icons and the current Desktop design tokens.
- Existing `ProjectGateway` operations remain authoritative for create, patch, and delete; no new backend endpoint or storage format is introduced.
- Template instructions continue to be sent through `createCloudProject` as `agentInstruction`.

## User experience

- The directory hero displays the supplied wide black-and-white team illustration without circular cropping.
- Each project card has a trailing menu button. The menu exposes Rename and Delete, closes on Escape/outside click, restores focus, and does not open the project accidentally.
- Rename uses the existing edit dialog and optimistic-version backend mutation.
- Delete uses the existing destructive confirmation dialog. Deleting a non-open project keeps the current workspace untouched; deleting the open project exits that workspace.
- The template collection contains Product requirements, Market research, Team knowledge base, Project delivery, and Bug tracking/testing acceptance, each carrying the exact supplied instruction.
- Existing loading, empty, unavailable, stale, and error states remain intact.

## Structure

- `desktop/frontend/src/features/projects/ui/CloudProjectsSectionImpl.tsx`: directory composition and card actions.
- `desktop/frontend/src/features/projects/ui/useProjectLifecycleController.ts`: dialog target state.
- `desktop/frontend/src/features/projects/model/projectTemplateInstructions.ts`: complete template instruction source of truth.
- `desktop/frontend/src/features/projects/projects.css`: reference-matched layout and interaction styling.
- `desktop/frontend/src/assets/projects/`: supplied collaboration image.
- `desktop/frontend/src/__tests__/`: behavior and rendering contracts.

## Commands

From `desktop/frontend`:

```powershell
pnpm.cmd test:all
pnpm.cmd test:vitest
pnpm.cmd build
```

## Boundaries

- Preserve existing Cloud/Admin endpoints, profiles, authentication, project data, and runtime configuration.
- Do not add speculative backend APIs or alter public gateway signatures.
- Do not replace the supplied artwork with generated or CSS-drawn imagery.
- Do not push or open a PR.

## Success criteria

- The supplied illustration and card/menu composition visually match the references at Desktop widths and degrade to one column on narrow widths.
- Mouse and keyboard users can rename/delete any listed project safely.
- Exactly five templates are rendered and their complete instructions reach the existing create mutation.
- Relevant tests, type checks, and build pass.
- Browser QA records a passing comparison against both supplied references.

## Open questions

None. Existing product wording and backend contracts determine the implementation; the supplied references determine the visual target.
