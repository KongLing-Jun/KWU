# Verification

## Automated checks

- `cd desktop/frontend && pnpm build` — passed CSS syntax checks, z-index checks, TypeScript compilation, and Vite production bundling.
- `git diff --check` — passed.

## Protocol probes

- A bundled Node probe built a request from the complete deterministic document with two selected widgets: schema v1, two ordered citations, document revision 10, no omissions, and no raw `props` field.
- An oversized probe used one missing widget, seven valid widgets, six evidence references per widget and a 4,100-character question. The builder emitted five valid citations after the missing entry occupied the six-item selection window, four evidence IDs per citation, a 4,000-character question, and explicit omission counts `{questionChars:100,citations:2,evidenceRefs:10,missingWidgets:1}`.
- Live Vite source exposes request construction and submitted-message request metadata (`data-request-schema`, document revision, citation count).

## Rapid-iteration constraint

- The focused UI test was updated for request schema and citation-count metadata but was intentionally not executed because the user requested no test run during rapid iteration.
