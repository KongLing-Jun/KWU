// Run: tsx src/__tests__/use-controller-meta.test.ts

import { foregroundRunningFromRuntimeMeta, initialState, metaFromTab, noticeNeedsReveal, reducer, sameMeta, shouldReconcileStaleTurn } from "../lib/useController";
import type { EffortInfo, Meta, TabMeta, WireUsage } from "../lib/types";

type LooseTabMeta = Omit<TabMeta, "toolApprovalMode"> & { toolApprovalMode?: TabMeta["toolApprovalMode"] | "" };

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

function meta(overrides: Partial<Meta> = {}): Meta {
  return {
    label: "DeepSeek-R1",
    ready: true,
    eventChannel: "events",
    cwd: "/repo",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    sessionPath: "/sessions/topic.jsonl",
    gitBranch: "main",
    autoApproveTools: false,
    bypass: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    goal: "",
    goalStatus: "stopped",
    ...overrides,
  };
}

function tab(overrides: Partial<LooseTabMeta> = {}): TabMeta {
  return {
    id: "tab-1",
    scope: "project",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    sessionPath: "/sessions/topic.jsonl",
    gitBranch: "main",
    topicId: "topic-1",
    topicTitle: "Topic",
    label: "DeepSeek-R1",
    ready: true,
    running: false,
    mode: "normal",
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    goal: "",
    goalStatus: "stopped",
    active: true,
    cwd: "/repo",
    ...overrides,
  } as TabMeta;
}

function usage(source: string): WireUsage {
  return {
    promptTokens: 100,
    completionTokens: 20,
    totalTokens: 120,
    cacheHitTokens: 80,
    cacheMissTokens: 20,
    sessionCacheHitTokens: 80,
    sessionCacheMissTokens: 20,
    source,
    cost: 0.001,
    currency: "$",
  };
}

function effort(current: string, supported = true): EffortInfo {
  return { supported, current, default: "auto", levels: supported ? ["auto", "low", "high"] : [] };
}

console.log("\nuse controller meta");

{
  eq(sameMeta(meta(), meta()), true, "identical meta is unchanged");
  eq(sameMeta(meta({ collaborationMode: "normal" }), meta({ collaborationMode: "plan" })), false, "collaboration mode changes invalidate meta equality");
  eq(sameMeta(meta({ workspacePath: "/repo" }), meta({ workspacePath: "/other" })), false, "workspace path changes invalidate meta equality");
  eq(sameMeta(meta({ sessionPath: "/sessions/a.jsonl" }), meta({ sessionPath: "/sessions/b.jsonl" })), false, "session identity changes invalidate meta equality");
  eq(sameMeta(meta({ gitBranch: "main" }), meta({ gitBranch: "feature" })), false, "git branch changes invalidate meta equality");
}

{
  const modelA = meta({ modelRef: "provider/a" });
  const modelB = meta({ modelRef: "provider/b" });
  const pendingA = reducer(initialState, {
    type: "composer_selection_start",
    seq: 1,
    modelRef: "provider/a",
    invalidate: true,
    meta: modelA,
  });
  eq(pendingA.composerSelection.phase, "pending", "selection start hides an incomplete model and effort pair");
  eq(pendingA.effort, undefined, "selection start clears the previous effort");

  const readyA = reducer(pendingA, {
    type: "composer_selection_ready",
    seq: 1,
    modelRef: "provider/a",
    meta: modelA,
    effort: effort("high"),
  });
  eq(readyA.composerSelection.phase, "ready", "matching model and effort publish one ready selection");
  eq(readyA.effort?.current, "high", "ready selection publishes its matching effort");

  const pendingB = reducer(readyA, { type: "optimistic_meta", meta: modelB });
  eq(pendingB.composerSelection.phase, "pending", "a changed optimistic model invalidates the old selection");
  eq(pendingB.effort, undefined, "a changed optimistic model cannot retain the old effort");
  const staleA = reducer(pendingB, {
    type: "composer_selection_ready",
    seq: 1,
    modelRef: "provider/a",
    meta: modelA,
    effort: effort("high"),
  });
  eq(staleA.meta?.modelRef, "provider/b", "an older selection sequence cannot overwrite a newer model");
  eq(staleA.composerSelection.phase, "pending", "an older selection sequence cannot settle the newer selection");

  const readyB = reducer(pendingB, {
    type: "composer_selection_ready",
    seq: pendingB.composerSelection.seq,
    modelRef: "provider/b",
    meta: modelB,
    effort: effort("low"),
  });
  const pendingC = reducer(readyB, {
    type: "composer_selection_start",
    seq: readyB.composerSelection.seq + 1,
    modelRef: "provider/c",
    invalidate: true,
  });
  const degradedC = reducer(pendingC, {
    type: "composer_selection_degraded",
    seq: pendingC.composerSelection.seq,
    modelRef: "provider/c",
    meta: meta({ modelRef: "provider/c" }),
  });
  eq(degradedC.composerSelection.phase, "degraded", "effort failure releases the model-only degraded state");
  eq(degradedC.effort, undefined, "degraded selection never reuses an older effort");

  const failedSwitch = reducer(readyB, {
    type: "composer_selection_start",
    seq: readyB.composerSelection.seq + 1,
    modelRef: "provider/failing",
    invalidate: true,
  });
  const restoredB = reducer(failedSwitch, {
    type: "composer_selection_restore",
    seq: failedSwitch.composerSelection.seq,
    meta: readyB.meta,
    effort: readyB.effort,
    selection: readyB.composerSelection,
  });
  eq(restoredB.meta?.modelRef, "provider/b", "failed model switch restores the prior model");
  eq(restoredB.effort?.current, "low", "failed model switch restores the prior effort");
  eq(restoredB.composerSelection.seq, failedSwitch.composerSelection.seq, "restored selection keeps the monotonic request sequence");

  const updatingEffort = reducer(readyB, {
    type: "composer_effort_update_start",
    seq: readyB.composerSelection.seq + 1,
    modelRef: "provider/b",
    effort: effort("high"),
  });
  eq(updatingEffort.composerSelection.phase, "updating-effort", "effort update has a distinct non-blanking phase");
  eq(updatingEffort.meta?.modelRef, "provider/b", "effort update keeps the confirmed model");
  eq(updatingEffort.effort?.current, "high", "effort update publishes the optimistic level immediately");
  const confirmedEffort = reducer(updatingEffort, {
    type: "composer_effort_update_done",
    seq: updatingEffort.composerSelection.seq,
  });
  eq(confirmedEffort.composerSelection.phase, "ready", "effort confirmation returns to ready without replacing the pair");
}

{
  const preserved = metaFromTab(tab({ toolApprovalMode: "" }), meta({ toolApprovalMode: "auto", autoApproveTools: false }));
  eq(preserved.toolApprovalMode, "auto", "blank tab snapshot preserves explicit auto approval mode");
  eq(preserved.autoApproveTools, false, "blank tab snapshot does not silently resurrect yolo approval");
}

{
  const started = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  const rendered = reducer(started, { type: "event", e: { kind: "message", text: "done", reasoning: "" } });
  eq(rendered.running, true, "message without turn_done leaves local runtime marked running");
  eq(rendered.turnActive, true, "message without turn_done still belongs to an active turn");
  eq(rendered.live, undefined, "final message closes the live stream before turn_done");
  eq(shouldReconcileStaleTurn(rendered, 1_000, 31_000), true, "stale completed stream still reconciles missed turn_done");
  eq(shouldReconcileStaleTurn(rendered, 1_000, 20_000), false, "fresh completed stream waits before reconciling");
  eq(shouldReconcileStaleTurn({ ...rendered, turnActive: false }, 1_000, 31_000), false, "local pending send before turn_started does not reconcile");
}

{
  const hidden = '<system-prompt source="expert" hidden="true">secret</system-prompt>\n\nvisible goal';
  const next = reducer(initialState, { type: "event", e: { kind: "notice", level: "info", text: `goal set: ${hidden}` } });
  const notice = next.items.find((item) => item.kind === "notice");
  eq(notice?.kind === "notice" ? notice.text : "", "goal set: visible goal", "hidden expert prompt does not render in notices");
}

{
  const hidden = '<system-prompt source="expert" hidden="true" id="data-analysis-team" name="数据分析专家团"';
  const next = reducer(initialState, { type: "event", e: { kind: "notice", level: "info", text: `目标已设置 → ${hidden}` } });
  const notice = next.items.find((item) => item.kind === "notice");
  eq(notice?.kind === "notice" ? notice.text : "", "目标已设置 → ", "truncated hidden expert prompt does not render in notices");
}

{
  const hidden = '<system-prompt source="expert" hidden="true">internal expert prompt</system-prompt>\n\nnetwork error';
  const pending = reducer(initialState, { type: "user", text: "visible question", seq: 0 });
  const failedSend = reducer(pending, { type: "send_failed", error: hidden });
  const notice = failedSend.items.find((item) => item.kind === "notice");
  eq(notice?.kind === "notice" ? notice.text : "", "network error", "send errors hide embedded expert prompt context");
}

{
  const started = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  const withTool = reducer(started, { type: "event", e: { kind: "tool_dispatch", tool: { id: "tool-1", name: "automation", args: "{}", readOnly: true } } });
  const done = reducer(withTool, { type: "event", e: { kind: "turn_done" } });
  const tool = done.items.find((item) => item.kind === "tool");
  eq(done.running, false, "turn_done clears the composer running status after automation output");
  eq(done.turnActive, false, "turn_done clears the active turn after automation output");
  eq(done.cancellable, false, "turn_done clears the stop action after automation output");
  eq(tool?.kind === "tool" ? tool.status : "", "stopped", "turn_done stops any running automation tool card");
  eq(done.turnStartAt, 0, "turn_done clears the active turn start");
  eq(done.turnTokens, 0, "turn_done clears the active turn tokens");
}

{
  const started = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  const canceling = reducer(started, { type: "cancel_requested" });
  const interrupted = reducer(canceling, { type: "event", e: { kind: "turn_done", err: "context canceled" } });
  const interruptionNotice = interrupted.items.find((item) => item.kind === "notice");
  eq(interruptionNotice?.kind === "notice" ? interruptionNotice.text : "", "本轮已中断", "user cancellation is localized immediately");
  eq(interrupted.items.some((item) => item.kind === "notice" && item.text === "context canceled"), false, "user cancellation does not leak context canceled");
  eq(interrupted.running, false, "localized cancellation still settles the turn");
  eq(interrupted.cancelRequested, false, "localized cancellation clears the request marker");

  const unexpectedCancellation = reducer(started, { type: "event", e: { kind: "turn_done", err: "context canceled" } });
  const unexpectedNotice = unexpectedCancellation.items.find((item) => item.kind === "notice");
  eq(unexpectedNotice?.kind === "notice" ? unexpectedNotice.text : "", "context canceled", "cancellation without a user request remains diagnostic");

  const explicitInterruption = reducer(started, { type: "event", e: { kind: "turn_done", interrupted: true } });
  const explicitNotice = explicitInterruption.items.find((item) => item.kind === "notice");
  eq(explicitNotice?.kind === "notice" ? explicitNotice.text : "", "本轮已中断", "explicit host interruption renders without local cancel state or error text");
  eq(explicitNotice?.kind === "notice" ? noticeNeedsReveal(explicitNotice) : false, true, "explicit host interruption requests a one-time transcript reveal");

  const realFailure = reducer(canceling, { type: "event", e: { kind: "turn_done", err: "provider settlement failed" } });
  const failureNotice = realFailure.items.find((item) => item.kind === "notice");
  eq(failureNotice?.kind === "notice" ? failureNotice.text : "", "provider settlement failed", "non-cancellation failure remains visible while stopping");
  eq(failureNotice?.kind === "notice" ? noticeNeedsReveal(failureNotice) : false, false, "ordinary failures do not steal the transcript scroll position");
}

{
  const current = { ...initialState, running: true, turnActive: true, turnStartAt: 2000, turnTokens: 20 };
  const statusBeforeContext = reducer(initialState, { type: "backend_status", running: true, cancellable: true });
  eq(statusBeforeContext.turnStartAt, 0, "running backend status does not invent an active turn start");
  eq(statusBeforeContext.turnTokens, 0, "running backend status does not invent active turn tokens");
  const restoredAfterStatus = reducer(statusBeforeContext, {
    type: "context",
    context: { used: 10, window: 100, sessionTokens: 50, activeTurnStartedAt: 1000, activeTurnCompletionTokens: 40 },
  });
  eq(restoredAfterStatus.turnStartAt, 1000, "context restores the turn start after backend status");
  eq(restoredAfterStatus.turnTokens, 40, "context restores turn tokens after backend status");
  const restoredDirectlyFromStatus = reducer(initialState, {
    type: "backend_status",
    running: true,
    cancellable: true,
    activeTurnStartedAt: 1000,
    activeTurnCompletionTokens: 40,
  });
  eq(restoredDirectlyFromStatus.turnStartAt, 1000, "runtime status restores the authoritative turn start");
  eq(restoredDirectlyFromStatus.turnTokens, 40, "runtime status restores authoritative turn tokens");
  const sameRuntimeTurn = reducer(restoredDirectlyFromStatus, {
    type: "backend_status",
    running: true,
    cancellable: true,
    activeTurnStartedAt: 1000,
    activeTurnCompletionTokens: 60,
  });
  eq(sameRuntimeTurn.turnTokens, 60, "same runtime turn merges newly completed tokens");
  const newerRuntimeTurn = reducer(restoredDirectlyFromStatus, {
    type: "backend_status",
    running: true,
    cancellable: true,
    activeTurnStartedAt: 2000,
    activeTurnCompletionTokens: 3,
  });
  eq(newerRuntimeTurn.turnStartAt, 2000, "newer runtime turn replaces a stale hydrated turn");
  eq(newerRuntimeTurn.turnTokens, 3, "newer runtime turn cannot inherit stale tokens");

  const oldSnapshot = reducer(current, {
    type: "context",
    context: { used: 10, window: 100, sessionTokens: 50, activeTurnStartedAt: 1000, activeTurnCompletionTokens: 40 },
  });
  eq(oldSnapshot.turnStartAt, 2000, "older context snapshot does not replace a realtime turn");
  eq(oldSnapshot.turnTokens, 20, "older context snapshot does not replace realtime turn tokens");

  const sameTurn = reducer(current, {
    type: "context",
    context: { used: 10, window: 100, sessionTokens: 60, activeTurnStartedAt: 2000, activeTurnCompletionTokens: 30 },
  });
  eq(sameTurn.turnTokens, 30, "same-turn context snapshot merges tokens monotonically");
  const sameTurnOlderTokens = reducer(sameTurn, {
    type: "context",
    context: { used: 10, window: 100, sessionTokens: 60, activeTurnStartedAt: 2000, activeTurnCompletionTokens: 25 },
  });
  eq(sameTurnOlderTokens.turnTokens, 30, "same-turn older token count cannot move backwards");

  const newTurn = reducer(current, {
    type: "context",
    context: { used: 10, window: 100, sessionTokens: 70, activeTurnStartedAt: 3000, activeTurnCompletionTokens: 5 },
  });
  eq(newTurn.turnStartAt, 3000, "newer context snapshot replaces the turn start");
  eq(newTurn.turnTokens, 5, "newer context snapshot replaces the turn tokens");

  const stopped = reducer(current, { type: "backend_status", running: false });
  eq(stopped.turnStartAt, 0, "stopped backend clears the active turn start");
  eq(stopped.turnTokens, 0, "stopped backend clears the active turn tokens");

  const hydratedReset = reducer(current, { type: "hydrate_reset" });
  eq(hydratedReset.turnStartAt, 2000, "transcript hydration reset preserves the active turn start");
  eq(hydratedReset.turnTokens, 20, "transcript hydration reset preserves active turn tokens");
  const newSessionReset = reducer(current, { type: "reset" });
  eq(newSessionReset.turnStartAt, 0, "new-session reset still clears the active turn start");
  eq(newSessionReset.turnTokens, 0, "new-session reset still clears active turn tokens");
}

{
  const started = reducer(initialState, {
    type: "event",
    e: { kind: "turn_started", activeTurnStartedAt: 1000 },
  });
  eq(started.turnStartAt, 1000, "turn_started uses the backend turn identity");
  const withUsage = reducer(started, {
    type: "event",
    e: {
      kind: "usage",
      activeTurnStartedAt: 1000,
      activeTurnCompletionTokens: 30,
      usage: usage("executor"),
    },
  });
  eq(withUsage.turnTokens, 30, "live usage uses the backend cumulative turn total");
  const duplicateUsage = reducer(withUsage, {
    type: "event",
    e: {
      kind: "usage",
      activeTurnStartedAt: 1000,
      activeTurnCompletionTokens: 30,
      usage: usage("executor"),
    },
  });
  eq(duplicateUsage.turnTokens, 30, "duplicate cumulative usage cannot double-count tokens");
}

{
  const started = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  const waiting = reducer(started, { type: "event", e: { kind: "approval_request", approval: { id: "1", tool: "bash", subject: "go test" } } });
  eq(waiting.running, true, "approval prompt keeps the turn running");
  eq(waiting.pendingPrompt, true, "approval prompt marks pendingPrompt");
  eq(waiting.cancellable, true, "approval prompt remains cancellable");

  const canceling = reducer(waiting, { type: "cancel_requested" });
  eq(canceling.approval, undefined, "cancel_requested clears approval prompt locally");
  eq(canceling.pendingPrompt, false, "cancel_requested clears pendingPrompt locally");
  eq(canceling.cancelRequested, true, "cancel_requested marks cancelling");
  eq(canceling.running, true, "cancel_requested waits for backend turn_done before idling");
  const stalePrompt = reducer(canceling, { type: "event", e: { kind: "approval_request", approval: { id: "late", tool: "bash", subject: "sleep" } } });
  eq(stalePrompt.approval, undefined, "late approval after cancel_requested stays hidden");

  const backgroundOnly = reducer(initialState, { type: "backend_status", running: false, backgroundJobs: 1, cancellable: false });
  eq(backgroundOnly.running, false, "background jobs alone do not make the composer runstatus active");
  eq(backgroundOnly.backgroundJobs, 1, "backend_status stores background job count");
  eq(backgroundOnly.cancellable, false, "background jobs alone are not foreground-cancellable");

  const omittedCancellableBackgroundOnly = reducer(initialState, { type: "backend_status", running: true, backgroundJobs: 1 });
  eq(omittedCancellableBackgroundOnly.running, false, "missing cancellable does not promote background-only metadata");
  eq(omittedCancellableBackgroundOnly.cancellable, false, "missing cancellable stays non-cancellable with background-only metadata");
  eq(foregroundRunningFromRuntimeMeta({ running: true }), true, "legacy running metadata remains foreground-running");
  eq(foregroundRunningFromRuntimeMeta({ running: true, pendingPrompt: true, backgroundJobs: 1 }), true, "pending prompts remain foreground-running");
  eq(foregroundRunningFromRuntimeMeta({ running: true, backgroundJobs: 1 }), false, "background jobs without cancellable are background-only");
}

{
  const idleExecutor = reducer(
    { ...initialState, context: { used: 0, window: 200, sessionTokens: 0 } },
    { type: "event", e: { kind: "usage", usage: usage("executor") } },
  );
  eq(idleExecutor.sessionTokens, 0, "executor usage outside a turn does not inflate session tokens");
  eq(idleExecutor.context.used, 0, "executor usage outside a turn does not refresh context used tokens");

  const idleHelper = reducer(initialState, { type: "event", e: { kind: "usage", usage: usage("classifier") } });
  eq(idleHelper.sessionTokens, 0, "helper usage outside a turn does not inflate session tokens");
  eq(idleHelper.sessionCost, 0, "helper usage outside a turn does not inflate session cost");

  const pendingClassifier = reducer(
    { ...initialState, running: true, context: { used: 0, window: 200, sessionTokens: 0 } },
    { type: "event", e: { kind: "usage", usage: usage("classifier") } },
  );
  eq(pendingClassifier.sessionTokens, 120, "classifier usage while send is running counts toward session tokens");
  eq(pendingClassifier.sessionCost, 0.001, "classifier usage while send is running counts toward session cost");
  eq(pendingClassifier.context.used, 0, "classifier usage while send is running does not refresh context used tokens");

  const active = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  const activeHelper = reducer(active, { type: "event", e: { kind: "usage", usage: usage("subagent") } });
  eq(activeHelper.sessionTokens, 120, "helper usage inside a turn still counts toward session tokens");
  eq(activeHelper.sessionCost, 0.001, "helper usage inside a turn still counts toward session cost");
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
