import type { DictKey, Translator } from "../../lib/i18n";

export type RuntimeEventPresentationKind = "notice" | "phase";

type Vars = Record<string, string | number>;

function translated(t: Translator, key: DictKey, vars?: Vars): string {
  return t(key, vars);
}

function exactNotice(text: string): DictKey | undefined {
  switch (text) {
    case "response truncated: hit max output tokens": return "runtimeNotice.responseMaxTokens";
    case "response blocked by content filter": return "runtimeNotice.responseContentFilter";
    case "response truncated: model repetition detected": return "runtimeNotice.responseRepetition";
    case "executor answered without taking any action; nudging it to use its tools": return "runtimeNotice.executorContinuing";
    case "task_tree planning failed: planner returned no tree.": return "runtimeNotice.taskTreePlanningEmpty";
    case "Deprecated [agent].memory_compiler was removed; Memory v5 no longer runs or writes state, while old transcripts remain readable.": return "runtimeNotice.memoryCompilerRemoved";
    case "context canceled": return "runtimeNotice.turnCanceled";
    case "context deadline exceeded": return "runtimeNotice.turnTimedOut";
    case "auto plan: task looks multi-step; drafting a plan first": return "runtimeNotice.autoPlanStarted";
    case "auto plan classifier failed; falling back to heuristic": return "runtimeNotice.autoPlanFallback";
    case "no active plan with todos to execute": return "runtimeNotice.noActivePlan";
    case "prometheus: starting planning interview": return "runtimeNotice.planningInterview";
    case "goal intercept: hard requirement cannot be overridden; satisfy it before signaling [goal:complete]": return "runtimeNotice.goalHardRequirement";
    case "goal intercept: incomplete todos remain (override with a second [goal:complete])": return "runtimeNotice.goalTodosRemain";
    case "compacted": return "runtimeNotice.compacted";
    case "new session": return "runtimeNotice.newSession";
    case "context cleared": return "runtimeNotice.contextCleared";
    case "migration rescue: checking legacy config and credentials": return "runtimeNotice.migrationCheckingConfig";
    case "migration rescue: current config is already present or no legacy config was found": return "runtimeNotice.migrationConfigCurrent";
    case "migration rescue: scanning legacy memory": return "runtimeNotice.migrationScanningMemory";
    case "migration rescue: scanning legacy sessions": return "runtimeNotice.migrationScanningSessions";
    case "migration rescue: no legacy memory needed migration": return "runtimeNotice.migrationNoMemory";
    case "migration rescue: no legacy sessions needed migration": return "runtimeNotice.migrationNoSessions";
    case "migration rescue complete: no legacy data needed migration": return "runtimeNotice.migrationNothingToDo";
    default: return undefined;
  }
}

function phaseText(text: string, t: Translator): string {
  let match = /^(.*\S) · (planning|executing)$/.exec(text);
  if (match) {
    return translated(t, match[2] === "planning" ? "runtimePhase.planning" : "runtimePhase.executing", { source: match[1] });
  }

  const treeStatus: Record<string, DictKey> = {
    planning: "runtimePhase.taskTreePlanning",
    tree_created: "runtimePhase.taskTreeCreated",
    executing: "runtimePhase.taskTreeExecuting",
    completed: "runtimePhase.taskTreeCompleted",
    failed: "runtimePhase.taskTreeFailed",
    blocked: "runtimePhase.taskTreeBlocked",
    skipped: "runtimePhase.taskTreeSkipped",
  };
  match = /^task_tree: (planning|tree_created|executing|completed|failed|blocked|skipped)$/.exec(text);
  if (match) return translated(t, treeStatus[match[1]]);

  match = /^task_tree: node_dispatch \S+ (.+\S)$/.exec(text);
  if (match) return translated(t, "runtimePhase.taskNodeDispatch", { node: match[1] });

  const nodeStatus: Record<string, DictKey> = {
    started: "runtimePhase.taskNodeStarted",
    completed: "runtimePhase.taskNodeCompleted",
    failed: "runtimePhase.taskNodeFailed",
    blocked: "runtimePhase.taskNodeBlocked",
    skipped: "runtimePhase.taskNodeSkipped",
  };
  match = /^task_tree: node_(started|completed|failed|blocked|skipped) (.+\S)$/.exec(text);
  if (match) return translated(t, nodeStatus[match[1]], { node: match[2] });

  return text;
}

function backgroundJobText(text: string, t: Translator): string | undefined {
  let match = /^background ([A-Za-z0-9_-]+) may be stalled: .+$/.exec(text);
  if (match) {
    return translated(t, match[1] === "task" ? "runtimeNotice.taskStalled" : "runtimeNotice.backgroundStalled");
  }
  match = /^background ([A-Za-z0-9_-]+) (started|finished|failed|killed): .+$/.exec(text);
  if (!match) return undefined;
  const [, kind, status] = match;
  const taskKeys: Record<string, DictKey> = {
    started: "runtimeNotice.taskStarted",
    finished: "runtimeNotice.taskFinished",
    failed: "runtimeNotice.taskFailed",
    killed: "runtimeNotice.taskKilled",
  };
  const commandKeys: Record<string, DictKey> = {
    started: "runtimeNotice.commandStarted",
    finished: "runtimeNotice.commandFinished",
    failed: "runtimeNotice.commandFailed",
    killed: "runtimeNotice.commandKilled",
  };
  const backgroundKeys: Record<string, DictKey> = {
    started: "runtimeNotice.backgroundStarted",
    finished: "runtimeNotice.backgroundFinished",
    failed: "runtimeNotice.backgroundFailed",
    killed: "runtimeNotice.backgroundKilled",
  };
  const keys = kind === "task" ? taskKeys : (kind === "bash" || kind === "shell") ? commandKeys : backgroundKeys;
  return translated(t, keys[status]);
}

function noticeText(text: string, t: Translator): string {
  const exact = exactNotice(text);
  if (exact) return translated(t, exact);

  const background = backgroundJobText(text, t);
  if (background) return background;

  let match = /^final-answer readiness blocked: .+$/.exec(text);
  if (match) return translated(t, "runtimeNotice.readinessContinuing");
  match = /^empty final answer blocked: .+ returned no visible answer text \(finish=.+, reasoning=\d+ chars\); retrying$/.exec(text);
  if (match) return translated(t, "runtimeNotice.emptyFinalRetrying");
  match = /^budget \([^\r\n]+\) exhausted: one grace round to finalize$/.exec(text);
  if (match) return translated(t, "runtimeNotice.toolBudgetFinalizing");
  match = /^loop guard: .+ failed \d+× the same way — nudging the model to change approach$/.exec(text);
  if (match) return translated(t, "runtimeNotice.loopGuard");
  match = /^tool output truncated: (\d+) of (\d+) bytes elided$/.exec(text);
  if (match) return translated(t, "runtimeNotice.toolOutputTruncated", { omitted: match[1], total: match[2] });

  match = /^context reached (\d+(?:\.\d+)?)% of window; keeping cache-first prefix until compact threshold (\d+(?:\.\d+)?)%$/.exec(text);
  if (match) return translated(t, "runtimeNotice.contextApproachingLimit", { used: match[1], threshold: match[2] });
  match = /^pruned (\d+) stale tool results \(~\d+ tokens est\.\) before compaction$/.exec(text);
  if (match) return translated(t, "runtimeNotice.staleToolsPruned", { count: match[1] });
  match = /^context_window=\d+ is too small for compaction to help \(.+\); raise context_window or shrink tool output\. Auto-compaction paused until the prompt drops\.$/.exec(text);
  if (match) return translated(t, "runtimeNotice.compactionWindowTooSmall");
  match = /^compaction skipped: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.compactionSkipped", { detail: match[1] });
  match = /^compaction summary unavailable \((.+)\); folded mechanically$/.exec(text);
  if (match) return translated(t, "runtimeNotice.compactionMechanical", { detail: match[1] });
  match = /^summarized (\d+) later messages → summary$/.exec(text);
  if (match) return translated(t, "runtimeNotice.summarizedLater", { count: match[1] });
  match = /^summarized (\d+) earlier messages → summary$/.exec(text);
  if (match) return translated(t, "runtimeNotice.summarizedEarlier", { count: match[1] });

  match = /^job artifact migration failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.jobArtifactMigrationFailed", { detail: match[1] });
  match = /^task_tree execution failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.taskTreeExecutionFailed", { detail: match[1] });
  match = /^task_tree node (\S+) (failed|blocked): (.+)$/.exec(text);
  if (match) {
    return translated(t, match[2] === "failed" ? "runtimeNotice.taskNodeFailed" : "runtimeNotice.taskNodeBlocked", { node: match[1], detail: match[3] });
  }

  match = /^guardian_model "(.+)" not found — guardian disabled$/.exec(text);
  if (match) return translated(t, "runtimeNotice.guardianModelMissing", { model: match[1] });
  match = /^guardian construction failed: (.+) — guardian disabled$/.exec(text);
  if (match) return translated(t, "runtimeNotice.guardianInitFailed", { detail: match[1] });
  match = /^guardian enabled(?: ·|;) model=(.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.guardianEnabled", { model: match[1] });

  match = /^this project defines local MCP servers but is not trusted, so auto-start is disabled for: (.+); run \/mcp trust-project and restart the session before auto-starting them$/.exec(text);
  if (match) return translated(t, "runtimeNotice.mcpUntrusted", { servers: match[1] });
	match = /^mcp \S+: .*connector is temporarily unavailable\. (?:please retry the current operation later|retry it from the connectors page)\.?$/i.exec(text);
	if (match) return translated(t, "runtimeNotice.connectorTemporarilyUnavailable");
	match = /^mcp \S+: .*connector authorization is required\. reconnect it from the connectors page\.?$/i.exec(text);
	if (match) return translated(t, "runtimeNotice.connectorReauthorizationRequired");
	match = /^mcp \S+: .*connector OAuth credential is unavailable$/i.exec(text);
	if (match) return translated(t, "runtimeNotice.connectorTemporarilyUnavailable");
  match = /^mcp (\S+): (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.mcpFailed", { server: match[1], detail: match[2] });
  match = /^(\d+) MCP server\(s\) failed to start: (.+?)(?: \(\+(\d+) more\))? — run \/mcp for details$/.exec(text);
  if (match) {
    const servers = match[3]
      ? translated(t, "runtimeNotice.mcpStartupFailedMore", { servers: match[2], more: match[3] })
      : match[2];
    return translated(t, "runtimeNotice.mcpStartupFailed", { count: match[1], servers });
  }
  match = /^MCP server "(.+)" connected but exposed no tools\.$/.exec(text);
  if (match) return translated(t, "runtimeNotice.mcpNoTools", { server: match[1] });
  match = /^enabled MCP server "(.+)" tools: (.+)\.$/.exec(text);
  if (match) return translated(t, "runtimeNotice.mcpToolsEnabled", { server: match[1], tools: match[2] });
  match = /^legacy user-data migration failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.legacyMigrationFailed", { detail: match[1] });
  match = /^migrated your previous configuration and data: (.+) → (.+?)(?: \(\d+ MCP server\(s\)\))?(?:; API key saved to .+ credentials store)?\. The old files were left untouched\.(?:\n[\s\S]*)?$/.exec(text);
  if (match) return translated(t, "runtimeNotice.legacyMigrationDone", { from: match[1], to: match[2] });
  match = /^imported (\d+) past session\(s\) from (.+) — resume them with --resume or the history panel$/.exec(text);
  if (match) return translated(t, "runtimeNotice.sessionsImported", { count: match[1], source: match[2] });
  match = /^Deprecated \[agent\]\.memory_compiler .+: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.memoryCompilerMigrationFailed", { detail: match[1] });
  match = /^cleanup-pending reconciliation failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.cleanupReconciliationFailed", { detail: match[1] });
  match = /^mark cleanup pending failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.cleanupMarkFailed", { detail: match[1] });
  match = /^clear session cleanup failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.sessionCleanupFailed", { detail: match[1] });

  match = /^model "(.+)" is no longer available; switched to (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.modelFallback", { model: match[1], fallback: match[2] });
  match = /^effort is not configurable for (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.effortUnsupported", { model: match[1] });
  match = /^effort for (.+) set to (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.effortSet", { model: match[1], effort: match[2] });

  match = /^rewound conversation to turn (\d+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.conversationRewound", { turn: match[1] });
  match = /^forked conversation at turn (\d+) into a new session$/.exec(text);
  if (match) return translated(t, "runtimeNotice.conversationForked", { turn: match[1] });

  match = /^final-answer readiness failed \d+ times: .+$/.exec(text);
  if (match) return translated(t, "runtimeNotice.readinessStopped");
  match = /^model finished without a visible final answer \d+ times$/.exec(text);
  if (match) return translated(t, "runtimeNotice.emptyFinalStopped");
  match = /^paused after \d+ tool-call rounds \(.+\) — the work so far is saved; send another message to continue, or set .+ higher or to 0 for no limit$/.exec(text);
  if (match) return translated(t, "runtimeNotice.toolBudgetStopped");

  match = /^invalid timezone "(.+)"; falling back$/.exec(text);
  if (match) return translated(t, "runtimeNotice.invalidTimezone", { timezone: match[1] });
  match = /^plan_mode_allowed_tools ignored known blocked entries: (.+); this setting only declares extra read-only custom tools and cannot unlock known blocked tools or unsafe bash$/.exec(text);
  if (match) return translated(t, "runtimeNotice.planModeToolsIgnored", { tools: match[1] });
  match = /^model "(.+)" is selected but its API key (\S+) is not set for (.+) - requests will fail until you set it$/.exec(text);
  if (match) return translated(t, "runtimeNotice.apiKeyMissing", { model: match[1], key: match[2], owner: match[3] });
  match = /^auto plan classifier: .+$/.exec(text);
  if (match) return translated(t, "runtimeNotice.autoPlanClassified");
  match = /^auto plan classifier failed; falling back to heuristic: .+$/.exec(text);
  if (match) return translated(t, "runtimeNotice.autoPlanFallback");
  match = /^plan-exec: detected (\d+) modules — (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.planModulesDetected", { count: match[1], modules: match[2] });
  match = /^plan-exec: dispatching (\d+) plan steps \(strict=(true|false)\)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.planStepsDispatching", { count: match[1] });
  match = /^compaction failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.compactionFailed", { detail: match[1] });
  match = /^new session failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.newSessionFailed", { detail: match[1] });
  match = /^clear context failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.clearContextFailed", { detail: match[1] });
  match = /^rewound code to turn (\d+) — (\d+) file\(s\) restored, (\d+) removed$/.exec(text);
  if (match) return translated(t, "runtimeNotice.codeRewound", { turn: match[1], restored: match[2], removed: match[3] });
  match = /^created branch (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.branchCreated", { branch: match[1] });
  match = /^switched to branch (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.branchSwitched", { branch: match[1] });
  match = /^resumed after .+ idle \(provider cache expired\) — elided (\d+) stale tool results to cheapen the cold restart$/.exec(text);
  if (match) return translated(t, "runtimeNotice.sessionResumed", { count: match[1] });
  match = /^(load user experts|load expert library state|reload experts|save expert usage|save expert metadata|load expert metadata|restore no-expert metadata|restore expert metadata) failed: (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.expertStateFailed", { detail: match[2] });
  match = /^effort for (.+): (.+) \(default: (.+); options: (.+)\)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.effortStatus", { model: match[1], effort: match[2], default: match[3], options: match[4] });
  match = /^imported (\d+) memory file\(s\) from (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.memoryImported", { count: match[1], source: match[2] });
  match = /^migration rescue: (?:config migration warning|skipped memory from .+|skipped .+): (.+)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.migrationWarning", { detail: match[1] });
  match = /^migration rescue completed with (\d+) warning\(s\): imported (\d+) memory file\(s\) and (\d+) past session\(s\)$/.exec(text);
  if (match) return translated(t, "runtimeNotice.migrationCompletedWithWarnings", { warnings: match[1], memory: match[2], sessions: match[3] });

  return text;
}

export function presentRuntimeEventText(kind: RuntimeEventPresentationKind, text: string, t: Translator): string {
  return kind === "phase" ? phaseText(text, t) : noticeText(text, t);
}

export function presentCompactionTrigger(trigger: string, t: Translator): string {
  switch (trigger) {
    case "auto": return translated(t, "compaction.triggerAuto");
    case "manual": return translated(t, "compaction.triggerManual");
    default: return trigger;
  }
}
