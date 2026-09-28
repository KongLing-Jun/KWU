# Project conversation failure diagnostics

Project send failures are recorded automatically through the existing local
error journal. No developer tools or diagnostic environment switch is needed.
This change adds observations; it does not fix the intermittent conversation
failure, change user messages, add retries, or upload logs.

## Collect and search

After installing a build containing this patch, reproduce the failure and keep
the toast's diagnostic ID and the local `diagnostics` directory. On Windows the
default directory is `%LOCALAPPDATA%\Clouvia\diagnostics`; a configured cache
root changes that location. Existing diagnostics collection remains usable.

Search a collected directory in PowerShell, replacing the example ID:

```powershell
$diagnosticsRoot = 'C:\Users\admin\Downloads\diagnostics'
$toastId = 'A0B1C2D3E4'
Get-ChildItem -LiteralPath $diagnosticsRoot -Recurse -File -Filter '*.log' |
  Select-String -SimpleMatch -Pattern "client_diagnostic_id=$toastId"
```

The journal still generates the leading log ID. `client_diagnostic_id` is the
separate, validated ID shown in the frontend toast. Different toast IDs remain
independently loggable; duplicate delivery of one record remains suppressed.
Persistence uses the existing bounded nonblocking queue, so saturation, shutdown,
or an unwritable destination can still lose a record.

## Interpret the record

- The `PROJECT_COMPOSER_*` code and trailing `phase` preserve the existing error
  stage. `step` identifies the operation or check reached inside that stage.
- `route` is `fresh` or `pinned` at submission entry; it does not imply first or
  second conversation. `phase_at_start` is also captured before asynchronous work.
- `list_tabs=failed` distinguishes a rejected list request from a successful
  lookup with `tab_found=false`. Missing boolean fields mean unobserved.
- `reason` is a fixed failed-check classification; `cause_type` is the original
  exception's allowlisted type. Neither is inferred from exception text.
- A `frame` contains category, ordinal, line, column, and a production asset
  hash. Only frames matching the running diagnostics module's bundle are kept.
  Missing frames do not mean there was no exception.
- `build` is a validated source revision prefix (up to 12 hex characters).
  Preserve the exact release assets and existing private sourcemap archive;
  resolve frame hashes against those assets before using numeric locations.

The new payload excludes exception messages, arbitrary objects, prompt text,
project/account/session identifiers, source URLs, paths, and function names.
Both TypeScript and Go independently project and validate the extension.

## Validation

Existing summary-based dedupe already includes the correlation ID, so no extra
registry or journal behavior change is needed.

Go tests exercise actual persisted output, scope and enum rejection, optional
fields, privacy canaries, bounds, and ID dedupe. Frontend verification consists of
the required production build and source/control-flow review, not a runtime
reproduction of the customer's Windows ARM64 incident. Generated Wails bindings
remain ignored build artifacts and are regenerated from the Go DTO.
