# DEF-008 — L21 first-turn scene commit fails on locked session metadata

- First run: L21, `desktop-design-empty-state`, repeat 4, low effort
- Reproduction runs: one so far
- Classification: B (Agent runtime persistence and error classification)
- Severity: severe run failure; no model request, artifact, or final
- Status: open evidence record; no product or runtime repair in this task

## Symptom and strict judgment

The first Desktop turn failed 18.942 seconds after send. The user-visible message said the model stream disconnected after retry attempts, but its nested cause was local session persistence:

`save scene session: open <session>.jsonl.meta: The process cannot access the file because it is being used by another process.`

The composer returned to idle with no requested HTML/CSS, no assistant final, no approvals, and no recorded model request. The session remained in `scene_state: draft`; JSONL contains only the system record. Telemetry and the offline extractor both report zero tokens, model calls, tools, and model duration. This run therefore fails model delivery quality and Desktop completion stability and counts as one failed sample in the 30-run matrix. It is not retried or reclassified as recovered.

## Reproduction

1. In the persistent dev environment, create/open the frozen L21 workspace in the Wails Desktop.
2. Select Design and fixed low effort.
3. Send the frozen empty-state prompt as the first turn.
4. Observe the failure before any assistant output or approval.
5. Inspect the session sidecars: `.jsonl.meta` remains `scene_state: draft`, telemetry is zero, and the workspace contains only the two original inputs plus expected `.clouvia` topic metadata.

## Evidence

- Runtime record: `observations/L21.json`
- Extracted zero metrics: `stats/L21.json`
- Desktop screenshots: `screenshots/L21/pre-send.jpg`, `after-send.jpg`, `runtime-failure-meta-lock.jpg`
- Session sidecars and turn record: `logs/L21/`
- Frozen workspace and unchanged input hashes: `workspaces/L21/`
- Failed frozen grade: `runs/L21/grade.json`

## Read-only source investigation

- `internal/control/task_lifecycle.go:25-29` commits a draft scene synchronously before dispatching the root runner, so this failure occurs before the provider call.
- `internal/control/task_lifecycle.go:72-86` replaces the in-memory session, then persists the session and metadata; a persistence error is wrapped as `save scene session`.
- `internal/agent/branch.go:266-327` writes `.jsonl.meta` via a temporary file and `fileutil.ReplaceFile`.
- `internal/fileutil/atomicwrite.go:83-104` already recognizes transient Windows locks and retries rename/copy replacement, but all attempts failed in this run.
- `internal/provider/retry.go:165-185` treats any error satisfying `net.Error` as a connection reset. A wrapped Windows `syscall.Errno` can satisfy that interface, allowing this local filesystem error to be mislabeled by `internal/control/errmsg.go:21-26` as a model-stream disconnection.

Only one `kwu-desktop.exe` process existed when checked after the failure. That rules out a second visible Desktop instance but does not identify the lock owner; antivirus, indexing, a concurrent in-process reader/writer, or another filter driver remain possible. Confidence is high that the fatal path and message are B-class runtime defects, and low on the external lock owner.

## Cost and completion impact

- prompt/completion/cache/total tokens: 0 / 0 / 0 / 0
- model calls / tool calls / approvals: 0 / 0 / 0
- todo steps / `todo_write` / `complete_step`: 0 / 0 / 0
- model duration / approval wait: 0ms / 0ms
- user-visible failure: 18.942s after send
- artifact quality: fail; runtime normal end: fail; realtime final: fail

## Suggested follow-up boundary

A separate approved fix task should reproduce the file-lock race with targeted Windows instrumentation, prove the lock owner/concurrent access path, and then assess stronger serialization or lock-tolerant metadata persistence. Independently, connection-reset classification should reject non-network filesystem errors so Desktop reports an accurate persistence failure. No code, prompt, or runtime configuration is changed by this benchmark task.
