import type { ClientErrorFrame } from "./bridge";
import { crashFrameMetadata } from "./errorFrames";

declare const __BUILD_COMMIT__: string;

export const PROJECT_DIAGNOSTIC_CODES = {
  "prepare-tab": "PROJECT_COMPOSER_PREPARE_FAILED",
  "create-session": "PROJECT_COMPOSER_SESSION_CREATE_FAILED",
  "reconcile-session": "PROJECT_COMPOSER_SESSION_RECONCILE_FAILED",
  "activate-session": "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED",
  "apply-profile": "PROJECT_COMPOSER_PROFILE_FAILED",
  "resolve-capabilities": "PROJECT_COMPOSER_CAPABILITIES_FAILED",
  "send-message": "PROJECT_COMPOSER_SEND_FAILED",
} as const;

type Stage = keyof typeof PROJECT_DIAGNOSTIC_CODES;
const PREPARE_STEPS = ["prepare-tab", "activate-expert", "ensure-blank-tab", "verify-adapter", "set-model", "set-effort", "load-tab-state", "model-admission"] as const;
const PROFILE_STEPS = ["set-collaboration-mode", "set-tool-approval-mode", "set-token-mode", "set-goal"] as const;
const STAGE_STEPS = {
  "prepare-tab": [...PREPARE_STEPS, "refresh-tabs"],
  "create-session": ["create-session"],
  "reconcile-session": ["verify-session", "refresh-tabs", "verify-target"],
  "activate-session": ["verify-pinned", "refresh-tabs", "verify-target", "switch-tab", "steer", "run-shell", "connect-command", "management-command"],
  "apply-profile": PROFILE_STEPS,
  "resolve-capabilities": ["resolve-capabilities"],
  "send-message": ["send-message"],
} as const;
export type ProjectComposerDiagnosticStep = typeof STAGE_STEPS[Stage][number];
const REASON_STEPS = {
  "not-pinned": ["verify-pinned"],
  "tab-missing": ["prepare-tab", "verify-target"],
  "path-mismatch": ["verify-target"],
  "snapshot-mismatch": ["verify-session"],
  "scope-changed": ["verify-adapter"],
  "model-blocked": ["model-admission"],
} as const;
type Reason = "exception" | keyof typeof REASON_STEPS;
const PHASES = ["draft", "preparing-tab", "creating-session", "pinned-unsent", "sending", "sent"] as const;
const CAUSE_TYPES = ["Error", "TypeError", "ReferenceError", "RangeError", "SyntaxError", "URIError", "EvalError", "AggregateError", "NonError", "unknown"] as const;
const OBSERVATIONS = ["hasTab", "hasSession", "adapterCurrent", "tabFound", "pathMatches"] as const;
const PINNED_STEPS = ["verify-pinned", "steer", "run-shell", "connect-command", "management-command"] as const;

export interface ProjectComposerAttempt {
  route: "fresh" | "pinned";
  phaseAtStart: typeof PHASES[number];
  executionTarget: "local" | "cloud";
  step: ProjectComposerDiagnosticStep;
  reason: Reason;
  listTabsOutcome: "not-called" | "succeeded" | "failed";
  hasTab?: boolean;
  hasSession?: boolean;
  adapterCurrent?: boolean;
  tabFound?: boolean;
  pathMatches?: boolean;
}

export interface ProjectComposerFrame extends ClientErrorFrame {
  assetHash?: string;
}

export interface ProjectComposerDiagnostic extends ProjectComposerAttempt {
  version: 1;
  stage: Stage;
  causeType: typeof CAUSE_TYPES[number];
  buildCommit?: string;
  frames?: ProjectComposerFrame[];
}

export function createProjectComposerAttempt(adapter: {
  phase: ProjectComposerAttempt["phaseAtStart"];
  executionTarget: ProjectComposerAttempt["executionTarget"];
  tabId: string;
  sessionPath: string;
}): ProjectComposerAttempt {
  return {
    route: adapter.sessionPath ? "pinned" : "fresh",
    phaseAtStart: adapter.phase,
    executionTarget: adapter.executionTarget,
    step: adapter.sessionPath ? "verify-pinned" : "prepare-tab",
    reason: "exception",
    listTabsOutcome: "not-called",
    hasTab: Boolean(adapter.tabId),
    hasSession: Boolean(adapter.sessionPath),
  };
}

function includes(values: readonly string[], value: unknown): value is string {
  return typeof value === "string" && values.includes(value);
}

// Retain only locations in this running bundle; arbitrary stack URLs never
// become asset identities. Other chunks and unsupported engines omit frames.
function originalFrames(error: Error): ProjectComposerFrame[] | undefined {
  const moduleURL = new URL(import.meta.url);
  const assetHash = /^\/assets\/[A-Za-z0-9_-]+-([A-Za-z0-9_-]{8})\.js$/.exec(moduleURL.pathname)?.[1];
  if (!assetHash || typeof error.stack !== "string") return undefined;
  const frames: ProjectComposerFrame[] = [];
  for (const line of error.stack.slice(0, 8192).split("\n")) {
    const location = /(?:\(|@|\s)((?:https?|wails|file):\/\/[^\s()]+):\d+:\d+\)?\s*$/.exec(line);
    if (!location || location[1] !== moduleURL.href) continue;
    const frame = crashFrameMetadata(line)?.[0];
    if (!frame) continue;
    frames.push({ ...frame, ordinal: frames.length, assetHash });
    if (frames.length === 4) break;
  }
  return frames.length ? frames : undefined;
}

export function snapshotProjectComposerDiagnostic(
  stage: Stage,
  originalError: unknown,
  attempt?: ProjectComposerAttempt,
): ProjectComposerDiagnostic | undefined {
  if (!attempt) return undefined;
  try {
    const causeType = originalError instanceof Error
      ? (includes(CAUSE_TYPES, originalError.name) ? originalError.name as ProjectComposerDiagnostic["causeType"] : "unknown")
      : "NonError";
    const buildCommit = typeof __BUILD_COMMIT__ === "string" ? __BUILD_COMMIT__ : "dev";
    return safeProjectComposerDiagnostic(PROJECT_DIAGNOSTIC_CODES[stage], {
      ...attempt,
      version: 1,
      stage,
      causeType,
      buildCommit,
      frames: originalError instanceof Error ? originalFrames(originalError) : undefined,
    });
  } catch {
    // Metadata access is best-effort, even for errors with throwing getters.
    return safeProjectComposerDiagnostic(PROJECT_DIAGNOSTIC_CODES[stage], { ...attempt, version: 1, stage, causeType: "unknown" });
  }
}

export function safeProjectComposerDiagnostic(code: string | undefined, value: unknown): ProjectComposerDiagnostic | undefined {
  try {
    if (!value || typeof value !== "object") return undefined;
    const record = value as Record<string, unknown>;
    const stage = record.stage as Stage;
    if (record.version !== 1 || !Object.prototype.hasOwnProperty.call(PROJECT_DIAGNOSTIC_CODES, stage) || PROJECT_DIAGNOSTIC_CODES[stage] !== code) return undefined;
    if (!includes(STAGE_STEPS[stage], record.step) || !includes(PHASES, record.phaseAtStart)) return undefined;
    if (record.route !== "fresh" && record.route !== "pinned") return undefined;
    if (record.executionTarget !== "local" && record.executionTarget !== "cloud") return undefined;
    if (!includes(["not-called", "succeeded", "failed"], record.listTabsOutcome)) return undefined;
    if (record.reason !== "exception" && (!Object.prototype.hasOwnProperty.call(REASON_STEPS, record.reason as string) || !includes(REASON_STEPS[record.reason as keyof typeof REASON_STEPS], record.step))) return undefined;
    if (record.route === "fresh" && includes(PINNED_STEPS, record.step)) return undefined;
    if (record.route === "pinned" && stage === "prepare-tab") return undefined;
    if (record.listTabsOutcome === "succeeded" && record.pathMatches === true && record.tabFound === false) return undefined;
    const safe: ProjectComposerDiagnostic = {
      version: 1,
      stage,
      route: record.route,
      phaseAtStart: record.phaseAtStart as ProjectComposerAttempt["phaseAtStart"],
      executionTarget: record.executionTarget,
      step: record.step as ProjectComposerDiagnosticStep,
      reason: record.reason as Reason,
      listTabsOutcome: record.listTabsOutcome as ProjectComposerAttempt["listTabsOutcome"],
      causeType: includes(CAUSE_TYPES, record.causeType) ? record.causeType as ProjectComposerDiagnostic["causeType"] : "unknown",
    };
    for (const key of OBSERVATIONS) {
      if ((key === "tabFound" || key === "pathMatches") && record.listTabsOutcome !== "succeeded") continue;
      if (typeof record[key] === "boolean") safe[key] = record[key];
    }
    if (typeof record.buildCommit === "string" && /^(?:[a-fA-F0-9]{7,40}|dev)$/.test(record.buildCommit)) safe.buildCommit = record.buildCommit;
    if (Array.isArray(record.frames)) {
      const frames: ProjectComposerFrame[] = [];
      for (const frame of record.frames.slice(0, 4)) {
        if (!frame || typeof frame !== "object") continue;
        if (![frame.ordinal, frame.line, frame.column].every((value) => Number.isSafeInteger(value) && value >= 0 && value <= 1_000_000_000) || frame.ordinal > 63) continue;
        if (typeof frame.assetHash !== "string" || !/^[A-Za-z0-9_-]{8}$/.test(frame.assetHash)) continue;
        frames.push({
          category: includes(["app", "framework", "runtime", "unknown"], frame.category) ? frame.category as ClientErrorFrame["category"] : "unknown",
          ordinal: frame.ordinal, line: frame.line, column: frame.column, assetHash: frame.assetHash,
        });
      }
      if (frames.length) safe.frames = frames;
    }
    return safe;
  } catch {
    return undefined;
  }
}
