# DEF-003 — L03 local validation required three bash attempts

- Run: L03, `desktop-design-empty-state`, repeat 1, low effort
- Classification: B / D / E
- Severity: medium efficiency impact; no final artifact-quality impact
- Status: reproduced in L15, L16, and A01; all four referenced runs recovered in the same Desktop session

## Symptom

The first bash validation performed correct static checks but launched a temporary HTTP server whose process did not exit as expected; the tool timed out after 120 seconds. The second bounded Python server attempt exited immediately because `Path.read_text()` used the Windows GBK default on UTF-8 input. The third attempt explicitly selected UTF-8, returned HTTP 200, and passed all static checks.

## Reproduction

1. In the real Wails Desktop run, approve the combined static-check plus background `python -m http.server` command.
2. Observe the tool remain running until the 2-minute host timeout.
3. Approve the bounded in-process HTTP retry without explicit encoding and observe `UnicodeDecodeError` from the GBK decoder.
4. Approve the otherwise identical retry with `encoding='utf-8'`; all checks and the local request pass.

## Evidence

- Parent session: `logs/L03/20260812-153240.137917100-gpt-gpt-5.6-luna.jsonl`
- Statistics: `stats/L03.json` (`bash` 3; no duplicate tool-call signature)
- Desktop screenshots: `screenshots/L03/`
- Final validation output is retained in the parent JSONL.
- Reproduction run L15: `logs/L15/20260812-175311.136223300-gpt-gpt-5.6-luna.jsonl` (first structural assertion failed with the same Windows GBK `UnicodeDecodeError`; explicit UTF-8 retry passed). L15's bounded HTTP server itself returned 200 and exited normally.
- Reproduction run L16: `logs/L16/20260812-180709.748037000-gpt-gpt-5.6-luna.jsonl` (first Markdown content check again used `Path.read_text()` with the Windows default, failed with `UnicodeDecodeError`, and passed after an explicit UTF-8 retry).

## Assessment

This is not a Reasonix product-code defect proven by the sample. The model selected a fragile cross-platform background-process pattern, while the Windows tool environment and default text encoding amplified the recovery cost. Future prompt or tool guidance could favor a bounded in-process server and explicit UTF-8 on Windows, but this test does not change prompts or runtime configuration.

## Suggested follow-up

The default-encoding portion is now recurrent across L03, L15, and L16. A later independent task should separate model/tool guidance (D/E) from host process-cleanup behavior (B) using a minimal runtime reproduction; this evaluation task does not change prompts or runtime configuration.
