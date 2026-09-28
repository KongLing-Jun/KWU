import { useCallback, useEffect, useState } from "react";
import {
  AppWindow,
  BarChart3,
  Blocks,
  Bot,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  Code2,
  Coffee,
  FileText,
  FolderKanban,
  Globe2,
  Image,
  LayoutTemplate,
  Palette,
  Presentation,
  SearchCheck,
  Smartphone,
  WalletCards,
  Workflow,
  type LucideIcon,
} from "lucide-react";
import { BRAND_DISPLAY_NAME } from "../lib/brand";
import { useT } from "../lib/i18n";
import type { Scene, SceneState } from "../lib/types";
import { normalizeScenePresentation } from "../lib/scenePresentation";
import type { DictKey } from "../locales/zh";
import { useHorizontalDragScroll } from "../hooks/useHorizontalDragScroll";

const logoSymbol = new URL("../assets/brand-symbol-hollow.png", import.meta.url).href;
const SCENARIO_SCROLL_STEP = 200;

export type SceneSelectionAvailability = "hidden" | "disabled" | "enabled";

export type CapabilityWelcomePresentation = {
  label: string;
  title: string;
  description: string;
  prompts: Array<{
    id: string;
    label: string;
    prompt: string;
  }>;
};

// Welcome is the empty-state landing: a one-liner, the input affordances
// (/ commands, @ files, Enter), and a few clickable example prompts that send
// immediately so a first turn is one click away.

type CreationModeId = "office" | "code" | "design";

function sceneForCreationMode(mode: CreationModeId): Scene {
  return mode === "code" ? "coding" : mode;
}

function creationModeForScene(scene: Scene | undefined): CreationModeId {
  return scene === "coding" ? "code" : scene ?? "office";
}

type CreationMode = {
  id: CreationModeId;
  icon: LucideIcon;
  labelKey: "welcome.creation.modeOffice" | "welcome.creation.modeCode" | "welcome.creation.modeDesign";
  titleKey: "welcome.creation.titleOffice" | "welcome.creation.titleCode" | "welcome.creation.title";
};

type CreationScenario = {
  id: string;
  icon: LucideIcon;
  labelKey: DictKey;
  promptKey: DictKey;
  skillCandidates: readonly string[];
};

export type CreationScenarioSkill = {
  name: string;
  label: string;
  candidates: readonly string[];
};

const CREATION_MODES: CreationMode[] = [
  {
    id: "office",
    icon: Coffee,
    labelKey: "welcome.creation.modeOffice",
    titleKey: "welcome.creation.titleOffice",
  },
  {
    id: "code",
    icon: Code2,
    labelKey: "welcome.creation.modeCode",
    titleKey: "welcome.creation.titleCode",
  },
  {
    id: "design",
    icon: Palette,
    labelKey: "welcome.creation.modeDesign",
    titleKey: "welcome.creation.title",
  },
];

function BrandLockup({ label = BRAND_DISPLAY_NAME }: { label?: string }) {
  return (
    <span className="welcome__brand">
      <img src={logoSymbol} className="welcome__brand-logo" alt="" aria-hidden="true" draggable={false} />
      <span className="welcome__brand-name">{label}</span>
    </span>
  );
}

const CREATION_SCENARIOS: Record<CreationModeId, CreationScenario[]> = {
  office: [
    { id: "document", icon: FileText, labelKey: "welcome.creation.scenarioDocument", promptKey: "welcome.creation.scenarioDocumentPrompt", skillCandidates: ["documents", "document-processing", "document", "word", "docx", "pdf"] },
    { id: "finance", icon: WalletCards, labelKey: "welcome.creation.scenarioFinance", promptKey: "welcome.creation.scenarioFinancePrompt", skillCandidates: ["financial-analysis", "finance", "spreadsheets", "spreadsheet", "excel"] },
    { id: "data", icon: BarChart3, labelKey: "welcome.creation.scenarioData", promptKey: "welcome.creation.scenarioDataPrompt", skillCandidates: ["data-analysis", "data-visualization", "visualize", "spreadsheets", "spreadsheet"] },
    { id: "meeting", icon: ClipboardList, labelKey: "welcome.creation.scenarioMeeting", promptKey: "welcome.creation.scenarioMeetingPrompt", skillCandidates: ["meeting-notes", "meeting-minutes", "meeting", "documents"] },
    { id: "project", icon: FolderKanban, labelKey: "welcome.creation.scenarioProject", promptKey: "welcome.creation.scenarioProjectPrompt", skillCandidates: ["project-planning", "project-management", "planning"] },
    { id: "research", icon: SearchCheck, labelKey: "welcome.creation.scenarioResearch", promptKey: "welcome.creation.scenarioResearchPrompt", skillCandidates: ["deep-research", "web-research", "research"] },
    { id: "automation", icon: Workflow, labelKey: "welcome.creation.scenarioAutomation", promptKey: "welcome.creation.scenarioAutomationPrompt", skillCandidates: ["workflow-automation", "automation", "workflows"] },
  ],
  code: [
    { id: "code-daily", icon: Code2, labelKey: "welcome.creation.scenarioCodeDaily", promptKey: "welcome.creation.scenarioCodeDailyPrompt", skillCandidates: ["software-development", "coding", "developer"] },
    { id: "code-website", icon: Globe2, labelKey: "welcome.creation.scenarioCodeWebsite", promptKey: "welcome.creation.scenarioCodeWebsitePrompt", skillCandidates: ["sites-building", "website-development", "web-development", "frontend"] },
    { id: "code-agent", icon: Bot, labelKey: "welcome.creation.scenarioCodeAgent", promptKey: "welcome.creation.scenarioCodeAgentPrompt", skillCandidates: ["agent-development", "agent-builder", "agents"] },
    { id: "code-skill", icon: Blocks, labelKey: "welcome.creation.scenarioCodeSkill", promptKey: "welcome.creation.scenarioCodeSkillPrompt", skillCandidates: ["skill-creator", "skill-development"] },
    { id: "code-cicd", icon: Workflow, labelKey: "welcome.creation.scenarioCodeCiCd", promptKey: "welcome.creation.scenarioCodeCiCdPrompt", skillCandidates: ["ci-cd", "cicd", "devops"] },
    { id: "code-docs", icon: FileText, labelKey: "welcome.creation.scenarioCodeDocs", promptKey: "welcome.creation.scenarioCodeDocsPrompt", skillCandidates: ["technical-writing", "documentation", "documents"] },
  ],
  design: [
    { id: "design-website", icon: LayoutTemplate, labelKey: "welcome.creation.scenarioDesignWebsite", promptKey: "welcome.creation.scenarioDesignWebsitePrompt", skillCandidates: ["website-design", "web-design", "product-design", "image-to-code"] },
    { id: "design-ppt", icon: Presentation, labelKey: "welcome.creation.scenarioDesignPpt", promptKey: "welcome.creation.scenarioDesignPptPrompt", skillCandidates: ["presentations", "presentation", "ppt-design", "powerpoint"] },
    { id: "design-poster", icon: Image, labelKey: "welcome.creation.scenarioDesignPoster", promptKey: "welcome.creation.scenarioDesignPosterPrompt", skillCandidates: ["poster-design", "graphic-design", "imagegen"] },
    { id: "design-mobile", icon: Smartphone, labelKey: "welcome.creation.scenarioDesignMobile", promptKey: "welcome.creation.scenarioDesignMobilePrompt", skillCandidates: ["mobile-app-design", "product-design", "image-to-code"] },
    { id: "design-system", icon: Blocks, labelKey: "welcome.creation.scenarioDesignSystem", promptKey: "welcome.creation.scenarioDesignSystemPrompt", skillCandidates: ["design-system", "product-design"] },
    { id: "design-webapp", icon: AppWindow, labelKey: "welcome.creation.scenarioDesignWebApp", promptKey: "welcome.creation.scenarioDesignWebAppPrompt", skillCandidates: ["web-app-design", "product-design", "image-to-code"] },
  ],
};

export function Welcome({
  onPrompt,
  onSelectScenario = onPrompt,
  scene,
  sceneState,
  sceneSelectionAvailability,
  onSelectScene,
  variant = "default",
  capabilityPresentation,
}: {
  onPrompt: (text: string) => void;
  onSelectScenario?: (text: string, skill?: CreationScenarioSkill) => void | Promise<void>;
  scene?: Scene;
  sceneState?: SceneState;
  sceneSelectionAvailability?: SceneSelectionAvailability;
  onSelectScene?: (scene: Scene) => Promise<void>;
  variant?: "default" | "creation";
  capabilityPresentation?: CapabilityWelcomePresentation;
}) {
  const t = useT();
  const scenePresentation = normalizeScenePresentation(scene, sceneState);
  const creationMode = creationModeForScene(scenePresentation.scene);
  const [pendingMode, setPendingMode] = useState<CreationModeId | null>(null);
  const [pendingScenario, setPendingScenario] = useState<CreationScenario["id"] | null>(null);
  const [selectedScenario, setSelectedScenario] = useState<CreationScenario["id"] | null>(null);
  const scenarioListRef = useHorizontalDragScroll(creationMode);
  const [scenarioScroll, setScenarioScroll] = useState({ canScrollLeft: false, canScrollRight: false });
  const syncScenarioScroll = useCallback(() => {
    const list = scenarioListRef.current;
    if (!list) return;
    const next = {
      canScrollLeft: list.scrollLeft > 1,
      canScrollRight: list.scrollLeft < list.scrollWidth - list.clientWidth - 1,
    };
    setScenarioScroll((current) => current.canScrollLeft === next.canScrollLeft && current.canScrollRight === next.canScrollRight
      ? current
      : next);
  }, [scenarioListRef]);
  const scrollScenarios = useCallback((direction: "left" | "right") => {
    scenarioListRef.current?.scrollBy({
      left: direction === "left" ? -SCENARIO_SCROLL_STEP : SCENARIO_SCROLL_STEP,
      behavior: "smooth",
    });
  }, [scenarioListRef]);

  useEffect(() => {
    if (variant !== "creation") return;
    const list = scenarioListRef.current;
    if (!list) return;
    list.scrollLeft = 0;
    syncScenarioScroll();
  }, [creationMode, scenarioListRef, syncScenarioScroll, variant]);

  useEffect(() => {
    if (variant !== "creation") return;
    const list = scenarioListRef.current;
    if (!list) return;
    const frame = window.requestAnimationFrame(syncScenarioScroll);
    list.addEventListener("scroll", syncScenarioScroll, { passive: true });
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(syncScenarioScroll);
    observer?.observe(list);
    return () => {
      window.cancelAnimationFrame(frame);
      list.removeEventListener("scroll", syncScenarioScroll);
      observer?.disconnect();
    };
  }, [creationMode, scenarioListRef, syncScenarioScroll, variant]);
  const resolvedSceneSelectionAvailability = sceneSelectionAvailability
    ?? (onSelectScene ? scenePresentation.sceneState === "locked" ? "disabled" : "enabled" : "hidden");
  const sceneInteractionDisabled = resolvedSceneSelectionAvailability === "disabled" || pendingMode !== null || pendingScenario !== null;

  if (capabilityPresentation) {
    return (
      <div className="welcome welcome--creation welcome--capability">
        <BrandLockup label={capabilityPresentation.label} />
        <h2 className="welcome-creation__headline">{capabilityPresentation.title}</h2>
        <p className="welcome-capability__description">{capabilityPresentation.description}</p>
        <section className="welcome-creation__scenarios welcome-capability__prompts" aria-label="推荐研究任务">
          <div className="welcome-creation__scenario-list" role="group" aria-label="推荐研究任务">
            {capabilityPresentation.prompts.map((prompt) => (
              <button
                key={prompt.id}
                type="button"
                className="welcome-creation__scenario"
                aria-busy={pendingScenario === prompt.id}
                disabled={pendingScenario !== null}
                onClick={async () => {
                  setPendingScenario(prompt.id);
                  try {
                    await onPrompt(prompt.prompt);
                  } finally {
                    setPendingScenario(null);
                  }
                }}
              >
                <SearchCheck size={15} aria-hidden="true" />
                <span>{prompt.label}</span>
              </button>
            ))}
          </div>
        </section>
      </div>
    );
  }

  if (variant === "creation") {
    const activeMode = CREATION_MODES.find((mode) => mode.id === creationMode) ?? CREATION_MODES[0];
    const scenarios = CREATION_SCENARIOS[creationMode];
    const renderScenario = (scenario: CreationScenario) => {
      const Icon = scenario.icon;
      const active = selectedScenario === scenario.id;
      return (
        <button
          key={scenario.id}
          type="button"
          className={`welcome-creation__scenario${active ? " welcome-creation__scenario--active" : ""}`}
          aria-pressed={active}
          aria-busy={pendingScenario === scenario.id}
          disabled={sceneInteractionDisabled}
          onClick={async () => {
            setSelectedScenario(scenario.id);
            setPendingScenario(scenario.id);
            try {
              await onSelectScenario(t(scenario.promptKey), {
                name: scenario.id,
                label: t(scenario.labelKey),
                candidates: scenario.skillCandidates,
              });
            } finally {
              setPendingScenario(null);
            }
          }}
        >
          <Icon size={15} aria-hidden="true" />
          <span>{t(scenario.labelKey)}</span>
        </button>
      );
    };
    return (
      <div className="welcome welcome--creation">
        <BrandLockup />
        <h2 className="welcome-creation__headline">
          {t(activeMode.titleKey)}
        </h2>
        {resolvedSceneSelectionAvailability !== "hidden" && onSelectScene ? <div className="welcome-creation__modes" role="group" aria-label={t("welcome.creation.modeGroupLabel")}>
          {CREATION_MODES.map((mode) => {
            const Icon = mode.icon;
            const active = creationMode === mode.id;
            return (
              <button
                key={mode.id}
                type="button"
                className={`welcome-creation__mode${active ? " welcome-creation__mode--active" : ""}`}
                aria-pressed={active}
                disabled={sceneInteractionDisabled}
                onClick={() => {
                  setSelectedScenario(null);
                  setPendingMode(mode.id);
                  void onSelectScene(sceneForCreationMode(mode.id)).finally(() => setPendingMode(null));
                }}
              >
                <Icon size={16} strokeWidth={2.2} aria-hidden="true" />
                <span>{t(mode.labelKey)}</span>
              </button>
            );
          })}
        </div> : null}
        <section className="welcome-creation__scenarios" aria-labelledby="welcome-creation-scenarios-title">
          <h3 id="welcome-creation-scenarios-title" className="sr-only">
            {t("welcome.creation.scenarioGroupLabel")}
          </h3>
          <div className="welcome-creation__scenario-strip">
            <span className="welcome-creation__scenario-arrow-slot welcome-creation__scenario-arrow-slot--left">
              {scenarioScroll.canScrollLeft ? (
                <button
                  type="button"
                  className="welcome-creation__scenario-arrow welcome-creation__scenario-arrow--left"
                  aria-label={t("welcome.creation.scenarioPrevious")}
                  onClick={() => scrollScenarios("left")}
                >
                  <ChevronLeft size={14} aria-hidden="true" />
                </button>
              ) : null}
            </span>
            <div className={`welcome-creation__scenario-viewport${scenarioScroll.canScrollLeft ? " welcome-creation__scenario-viewport--can-scroll-left" : ""}${scenarioScroll.canScrollRight ? " welcome-creation__scenario-viewport--can-scroll-right" : ""}`}>
              <div ref={scenarioListRef} className="welcome-creation__scenario-list" role="group" aria-label={t("welcome.creation.scenarioGroupLabel")}>
                {scenarios.map(renderScenario)}
              </div>
            </div>
            <span className="welcome-creation__scenario-arrow-slot welcome-creation__scenario-arrow-slot--right">
              {scenarioScroll.canScrollRight ? (
                <button
                  type="button"
                  className="welcome-creation__scenario-arrow welcome-creation__scenario-arrow--right"
                  aria-label={t("welcome.creation.scenarioNext")}
                  onClick={() => scrollScenarios("right")}
                >
                  <ChevronRight size={14} aria-hidden="true" />
                </button>
              ) : null}
            </span>
          </div>
        </section>
      </div>
    );
  }

  const examples = [t("welcome.ex1"), t("welcome.ex2"), t("welcome.ex3"), t("welcome.ex4")];
  return (
    <div className="welcome welcome--brand">
      <BrandLockup />
      <h2 className="welcome__title">{t("welcome.title")}</h2>
      <div className="welcome__tag">{t("welcome.tagline")}</div>

      <div className="welcome__hints">
        <span>
          <kbd>/</kbd> {t("welcome.hintCommands")}
        </span>
        <span>
          <kbd>@</kbd> {t("welcome.hintFiles")}
        </span>
        <span>
          <kbd>⏎</kbd> {t("welcome.hintSend")}
        </span>
      </div>

      <div className="welcome__examples">
        {examples.map((ex) => (
          <button key={ex} className="welcome__ex" onClick={() => onPrompt(ex)}>
            {ex}
          </button>
        ))}
      </div>
    </div>
  );
}
