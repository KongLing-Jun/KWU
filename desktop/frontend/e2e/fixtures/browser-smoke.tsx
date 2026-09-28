import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import { HistoryLoadErrorBanner } from "../../src/components/HistoryLoadErrorBanner";
import { SendAvailabilityBanner } from "../../src/components/SendAvailabilityBanner";
import { Composer } from "../../src/components/Composer";
import { LocaleProvider } from "../../src/lib/i18n";
import { sendAvailabilityAllowsControllerActions } from "../../src/lib/sendAvailability";
import { ToastProvider } from "../../src/lib/toast";
import type { CollaborationMode, TabSendAvailability } from "../../src/lib/types";
import "../../src/styles.css";
import "./browser-smoke.css";

const workspaceStorageKey = "reasonix-browser-smoke-workspace";
const scenario = new URLSearchParams(window.location.search).get("scenario");

if (scenario?.startsWith("composer-")) {
  localStorage.setItem("kwu.layoutPreferences.v1", JSON.stringify({ sizes: { composerHeight: 104 } }));
  window.go = {
    main: {
      App: {
        Commands: async () => [],
        ListProjectTree: async () => [
          { key: "cloud:project-fixture", kind: "cloud_project", label: "Fixture project", cloudProjectId: "project-fixture" },
        ],
        Models: async () => [],
        ModelsForTab: async () => [],
      },
    },
  } as unknown as typeof window.go;
}

function initialAvailability(): TabSendAvailability {
  const scenario = new URLSearchParams(window.location.search).get("scenario");
  if (scenario === "auth") {
    return {
      state: "subscription_auth_required",
      action: "login",
      retryable: true,
      detail: "synthetic_auth_required",
    };
  }
  if (scenario === "offline") {
    return {
      state: "service_unavailable",
      action: "retry",
      retryable: true,
      detail: "synthetic_offline",
    };
  }
  return { state: "ready", retryable: false };
}

function BrowserSmokeFixture() {
  const [availability] = useState<TabSendAvailability>(initialAvailability);
  const [activity, setActivity] = useState("none");
  const [workspaceDraft, setWorkspaceDraft] = useState("");
  const [workspace, setWorkspace] = useState(() => localStorage.getItem(workspaceStorageKey) || "No workspace restored");
  const [selectedFile, setSelectedFile] = useState("No file selected");
  const sendingAllowed = sendAvailabilityAllowsControllerActions(availability);

  const record = (value: string) => setActivity(value);
  const saveWorkspace = () => {
    localStorage.setItem(workspaceStorageKey, workspaceDraft);
    setWorkspace(workspaceDraft);
  };

  return (
    <main className="browser-smoke">
      <header>
        <h1>Reasonix browser smoke</h1>
        <p>Test-only Vite fixture composed from production UI units.</p>
      </header>

      <nav aria-label="Primary" className="browser-smoke__nav">
        <button type="button" onClick={() => record("new-conversation")}>New conversation</button>
        <button type="button" onClick={() => record("projects")}>Projects</button>
        <button type="button" onClick={() => record("account")}>Account</button>
      </nav>

      <SendAvailabilityBanner
        availability={availability}
        hasHistory
        onLogin={() => record("login")}
        onOpenAccount={() => record("account")}
        onOpenPayments={() => record("payments")}
        onConfigureBYOK={() => record("configure-model")}
        onRetry={() => record("retry")}
        onSwitchModel={() => record("switch-model")}
      />

      {scenario === "history-error" ? <HistoryLoadErrorBanner error="synthetic disk error" /> : null}

      <section aria-labelledby="composer-heading" className="browser-smoke__composer">
        <h2 id="composer-heading">Composer</h2>
        <label htmlFor="message">Message</label>
        <textarea id="message" disabled={!sendingAllowed} />
        <button type="button" disabled={!sendingAllowed}>Send</button>
      </section>

      <section aria-labelledby="workspace-heading">
        <h2 id="workspace-heading">Workspace restore</h2>
        <p data-testid="restored-workspace">{workspace}</p>
        <label htmlFor="workspace-name">Workspace name</label>
        <input
          id="workspace-name"
          value={workspaceDraft}
          onChange={(event) => setWorkspaceDraft(event.currentTarget.value)}
        />
        <button type="button" onClick={saveWorkspace}>Save workspace</button>
      </section>

      <section aria-labelledby="file-heading">
        <h2 id="file-heading">File preparation</h2>
        <label htmlFor="fixture-file">Choose a local file</label>
        <input
          id="fixture-file"
          type="file"
          onChange={(event) => setSelectedFile(event.currentTarget.files?.[0]?.name || "No file selected")}
        />
        <output aria-live="polite">{selectedFile}</output>
      </section>

      <output aria-label="Last action" data-testid="last-action">{activity}</output>
    </main>
  );
}

function ComposerFollowupFixture() {
  const [sent, setSent] = useState(false);
  return (
    <div className="app app--creation browser-smoke-followup">
      <div className={`chat-pane${sent ? "" : " chat-pane--creation-empty"}`}>
        <div className="footer">
          <Composer
            running={false}
            collaborationMode="normal"
            toolApprovalMode="ask"
            tokenMode="full"
            modelLabel="DeepSeek-R1"
            modelCatalog={{ state: "ready", routes: [] }}
            selectedConnectors={["openaiDeveloperDocs"]}
            onClearConnector={() => {}}
            onSend={async () => setSent(true)}
            onCancel={() => undefined}
            onCycleMode={() => {}}
            onSetMode={() => {}}
            onSetCollaborationMode={() => {}}
            onSetToolApprovalMode={() => {}}
            onToggleYoloApprovalMode={() => {}}
            onClearGoal={() => {}}
            onSwitchModel={() => {}}
            onSetEffort={() => {}}
            onSetTokenMode={() => {}}
          />
        </div>
      </div>
      <output data-testid="followup-state">{sent ? "sent" : "draft"}</output>
    </div>
  );
}

function ComposerExpertMenuFixture() {
  const [expertCleared, setExpertCleared] = useState(false);
  const expert = {
    id: "finance-team",
    name: "Finance Team",
    role: "Operating analysis",
    mode: "expert" as const,
    prompt: "Act as the finance expert.",
  };
  return (
    <div className="app app--creation browser-smoke-followup">
      <div className="chat-pane chat-pane--creation-empty">
        <div className="footer" style={{ position: "fixed", right: 40, bottom: 40, left: 40 }}>
          <Composer
            running={false}
            collaborationMode="normal"
            toolApprovalMode="ask"
            tokenMode="full"
            modelLabel="DeepSeek-R1"
            modelCatalog={{ state: "ready", routes: [] }}
            activeExpert={expertCleared ? null : expert}
            recentExperts={[expert]}
            onOpenExpertDirectory={() => {}}
            onClearExpert={() => setExpertCleared(true)}
            onSelectRecentExpert={() => {}}
            onSend={() => {}}
            onCancel={() => undefined}
            onCycleMode={() => {}}
            onSetMode={() => {}}
            onSetCollaborationMode={() => {}}
            onSetToolApprovalMode={() => {}}
            onToggleYoloApprovalMode={() => {}}
            onClearGoal={() => {}}
            onSwitchModel={() => {}}
            onSetEffort={() => {}}
            onSetTokenMode={() => {}}
          />
        </div>
      </div>
      <output data-testid="expert-state">{expertCleared ? "cleared" : "active"}</output>
    </div>
  );
}

function ComposerCollaborationChipFixture() {
  const [collaborationMode, setCollaborationMode] = useState<CollaborationMode>("plan");
  return (
    <div className="app app--creation browser-smoke-followup">
      <div className="chat-pane chat-pane--creation-empty">
        <div className="footer" style={{ position: "fixed", right: 40, bottom: 40, left: 40 }}>
          <Composer
            running={false}
            collaborationMode={collaborationMode}
            toolApprovalMode="ask"
            tokenMode="full"
            modelLabel="DeepSeek-R1"
            modelCatalog={{ state: "ready", routes: [] }}
            onSend={() => {}}
            onCancel={() => undefined}
            onCycleMode={() => {}}
            onSetMode={() => {}}
            onSetCollaborationMode={setCollaborationMode}
            onSetToolApprovalMode={() => {}}
            onToggleYoloApprovalMode={() => {}}
            onClearGoal={() => {}}
            onSwitchModel={() => {}}
            onSetEffort={() => {}}
            onSetTokenMode={() => {}}
          />
        </div>
      </div>
      <output data-testid="collaboration-mode">{collaborationMode}</output>
    </div>
  );
}

const manyFixtureConnectors = Array.from({ length: 18 }, (_, index) => `fixture-connector-${index + 1}`);

function ComposerProjectResizeFixture() {
  const [selectedConnectors, setSelectedConnectors] = useState<string[]>([]);
  return (
    <div className="app app--creation browser-smoke-project-composer">
      <div className="browser-smoke-project-composer__controls">
        <button type="button" onClick={() => setSelectedConnectors(["openaiDeveloperDocs"])}>Add one connector</button>
        <button type="button" onClick={() => setSelectedConnectors(manyFixtureConnectors)}>Add many connectors</button>
        <button type="button" onClick={() => setSelectedConnectors([])}>Remove connectors</button>
      </div>
      <footer className="cloud-projects__composer" data-project-composer data-project-composer-reused="true">
        <div className="session-composer-region session-composer-region--project" id="project-composer-input">
          <div className="session-composer-slot">
            <div className="session-composer-content">
              <Composer
                running={false}
                collaborationMode="normal"
                toolApprovalMode="ask"
                tokenMode="full"
                modelLabel="Auto"
                modelCatalog={{ state: "ready", routes: [] }}
                selectedConnectors={selectedConnectors}
                onClearConnector={(name) => setSelectedConnectors((current) => current.filter((item) => item !== name))}
                onSend={() => {}}
                onCancel={() => undefined}
                onCycleMode={() => {}}
                onSetMode={() => {}}
                onSetCollaborationMode={() => {}}
                onSetToolApprovalMode={() => {}}
                onToggleYoloApprovalMode={() => {}}
                onClearGoal={() => {}}
                onSwitchModel={() => {}}
                onSetEffort={() => {}}
                onSetTokenMode={() => {}}
                projectPresentation={{
                  scopeKey: "fixture-account/project-fixture",
                  executionTarget: "cloud",
                  workspacePath: "",
                  cloudProjectId: "project-fixture",
                  workspaceLocked: false,
                  onExecutionTargetChange: () => {},
                  onWorkspacePathChange: () => {},
                  onCloudProjectIdChange: () => {},
                }}
              />
            </div>
          </div>
        </div>
      </footer>
      <output data-testid="connector-count">{selectedConnectors.length}</output>
    </div>
  );
}

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");
createRoot(root).render(
  <StrictMode>
    <LocaleProvider>
      <ToastProvider>
        {scenario === "composer-followup"
          ? <ComposerFollowupFixture />
          : scenario === "composer-expert-menu"
            ? <ComposerExpertMenuFixture />
            : scenario === "composer-collaboration-chip"
              ? <ComposerCollaborationChipFixture />
              : scenario === "composer-project-resize"
                ? <ComposerProjectResizeFixture />
            : <BrowserSmokeFixture />}
      </ToastProvider>
    </LocaleProvider>
  </StrictMode>,
);
