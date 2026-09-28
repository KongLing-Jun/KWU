# DEF-009 — A06 final rendered while Desktop stayed in stop/loading state

- First run: A06, `desktop-design-empty-state`, auto effort, repeat 2
- Reproduction runs: one observed instance
- Classification: C (Desktop projection/completion state)
- Severity: completion-tail latency; no artifact or realtime-final failure
- Status: open evidence record; no product or runtime repair in this task

## Symptom and judgment

The accepted final message persisted at `2026-08-12T20:18:05.2328555Z` and was visibly rendered in the active Desktop session at `20:18:13.054Z`. The composer still showed the stop/loading state. The first observation with the same final and an idle composer was `20:18:39.726Z`, 34.493 seconds after persistence and 26.672 seconds after the final was already visible.

No session switch was required and no duplicate final appeared. Therefore model quality, normal end, and current-session realtime final all pass. This does not meet the locked failure rule, which requires the final to remain unavailable until a session switch, but it is retained as a Desktop completion-state latency defect.

## Reproduction

1. Open the A06 fixture in the persistent dev Wails Desktop.
2. Select Design, `gpt-5.6-luna`, and automatic effort.
3. Send the frozen empty-state prompt and approve four requested calls once.
4. Observe the final content rendered with the stop/loading control still present.
5. Continue observing the same session without switching; the control eventually returns to idle while the final remains unchanged.

## Evidence

- Runtime record: `observations/A06.json`
- Running-final screenshot: `runs/A06-final-candidate-loading.jpg`
- Accepted and stable screenshots: `runs/A06/final-first-visible.jpg`, `runs/A06/final-stable.jpg`
- Session log and telemetry: `logs/A06/`
- Extracted metrics: `stats/A06.json`

## Suggested follow-up boundary

A separate task can instrument the interval between final persistence, final projection, runner completion, and composer-state reset to determine whether the tail is backend finalization or frontend state projection. No code, prompt, or runtime configuration is changed here.
