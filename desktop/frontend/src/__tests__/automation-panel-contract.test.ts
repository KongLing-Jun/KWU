// Run: tsx src/__tests__/automation-panel-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const panelSource = readFileSync(resolve(testDir, "../features/automation/heartbeat/HeartbeatPanel.tsx"), "utf8");
const heartbeatCss = readFileSync(resolve(testDir, "../features/automation/heartbeat/heartbeat.css"), "utf8");
const heartbeatBridgeSource = readFileSync(resolve(testDir, "../features/automation/heartbeat/heartbeat.bridge.ts"), "utf8");
const typesSource = readFileSync(resolve(testDir, "../features/automation/heartbeat/heartbeat.types.ts"), "utf8");
const bridgeSource = readFileSync(resolve(testDir, "../lib/bridge.ts"), "utf8");
const localeSource = readFileSync(resolve(testDir, "../locales/zh.ts"), "utf8");
const appSource = readFileSync(resolve(testDir, "../../../app.go"), "utf8");
const automationSource = readFileSync(resolve(testDir, "../../../automation.go"), "utf8");
const templateSource = readFileSync(resolve(testDir, "../../../../internal/automation/templates.go"), "utf8");
const builtinTemplateSource =
  templateSource.match(/var defaultTaskTemplates = \[\]TaskTemplate\{([\s\S]*?)\n\}/)?.[1] || "";
const alignedActionCss = heartbeatCss.slice(heartbeatCss.lastIndexOf(".heartbeat-editor__actions {"));

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nautomation panel contract");

ok(
  /onDelete=\{\(\) => void handleDelete\(task\.id\)\}/.test(panelSource) &&
    /const \[confirmingDelete, setConfirmingDelete\] = useState\(false\)/.test(panelSource) &&
    /heartbeat-card__open-btn--danger/.test(panelSource) &&
    /heartbeat-card__confirm/.test(panelSource) &&
    /setConfirmingDelete\(false\);\s*onDelete\(\);/.test(panelSource) &&
    !/window\.confirm\(t\("heartbeat\.confirmDelete"/.test(panelSource) &&
    /\.heartbeat-card__confirm \{[\s\S]*color-mix\(in srgb, var\(--err\)/.test(heartbeatCss) &&
    /<Trash2 size=\{13\}/.test(panelSource),
  "task cards expose a themed inline delete confirmation wired to persisted task removal",
);

ok(
  !/function heartbeatNextRunAt\(/.test(panelSource) &&
    /nextRunAt=\{task\.nextRunAt\}/.test(panelSource) &&
    /const nextRunLabel = quotaPaused \? "套餐额度暂停" : backendNextRunLabel\(nextRunAt/.test(panelSource) &&
    /const handleToggle[\s\S]{0,460}const refreshedTasks = await heartbeatListTasks\(\)/.test(panelSource) &&
    /const handleSaveEdit[\s\S]{0,700}const refreshedTasks = await heartbeatListTasks\(\)/.test(panelSource),
  "task cards display only the backend-owned nextRunAt value",
);

ok(
  /automationListRunsV2\(\{ limit: RUN_HISTORY_PAGE_SIZE \}, cursor\)/.test(panelSource) &&
    /setWorkspaceRuns\(\(previous\) => cursor \? \[\.\.\.previous, \.\.\.page\.items\] : page\.items\)/.test(panelSource) &&
    /加载更多运行记录/.test(panelSource) &&
    !/const workspaceRuns = Object\.values\(runsByTask\)\s*\.flat\(\)/.test(panelSource),
  "run history uses one global cursor stream and exposes load-more",
);

ok(
  /subscribeAutomationChanges\(\(kind\) => \{\s*invalidations\.add\(kind\)/.test(panelSource) &&
    /window\.setTimeout\(flushInvalidations, 50\)/.test(panelSource) &&
    /const panelGeneration = panelGenerationRef\.current/.test(panelSource) &&
    /panelRequestIsCurrent\(panelGeneration\)/.test(panelSource) &&
    /await loadTasks\(panelGeneration\)/.test(panelSource) &&
    /await loadWorkspaceRuns\("", panelGeneration\)/.test(panelSource) &&
    /\["automation:task_changed", "task"\]/.test(heartbeatBridgeSource) &&
    /\["automation:run_changed", "run"\]/.test(heartbeatBridgeSource) &&
    /\["automation:run_event", "run_event"\]/.test(heartbeatBridgeSource) &&
    /\["automation:delivery_changed", "delivery"\]/.test(heartbeatBridgeSource) &&
    /\["automation:catalog_changed", "catalog"\]/.test(heartbeatBridgeSource) &&
    /runtime\.EventsOn\(eventName, \(\) => onChange\(kind\)\)/.test(heartbeatBridgeSource),
  "automation runtime events are coalesced invalidations that re-read guarded backend state",
);

ok(
  /pending\.has\("run_event"\)/.test(panelSource) &&
    /automationReadRun\(selectedRun\.taskID, selectedRun\.runID\)/.test(panelSource),
  "persisted run-event hints invalidate only the currently open authoritative run detail",
);

ok(
  /automationGetScheduleCatalog/.test(panelSource) &&
    /scheduleCatalog=\{scheduleCatalog\}/.test(panelSource) &&
    /scheduleCatalog\s*\.filter\(\(entry\) => entry\.kind === "calendar"/.test(panelSource),
  "schedule cycle choices are derived from the backend catalog",
);

ok(
  !/schedulePreview/.test(panelSource) &&
    !/下三次运行预览/.test(panelSource) &&
    !/automationValidateDraft/.test(panelSource) &&
    !/export function automationValidateDraft/.test(heartbeatBridgeSource) &&
    /AutomationValidateDraft\(task: unknown\): Promise<unknown>/.test(bridgeSource) &&
    /func \(a \*App\) AutomationValidateDraft\(task automation\.Task\)/.test(automationSource),
  "task editing removes preview rendering and requests while preserving the exported desktop contract",
);

ok(
  /automationListDeliveryTargets/.test(panelSource) &&
    /deliveryTargetCatalog=\{deliveryTargetCatalog\}/.test(panelSource) &&
    /<HeartbeatMultiSelect[\s\S]{0,1600}ariaLabel="通知目标"/.test(panelSource) &&
    /disabled: !target\.available && !selected/.test(panelSource) &&
    /target\.unavailableReason \|\| "当前不可用"/.test(panelSource) &&
    /label: "已保存目标"/.test(panelSource) &&
    /目标已从目录移除或授权已撤销/.test(panelSource) &&
    /`\$\{target\.label\} · \$\{target\.ref\}`/.test(panelSource) &&
    /placeholder="请选择通知目标"/.test(panelSource) &&
    /\{draft\.deliveryEnabled \? \(/.test(panelSource) &&
    /title=\{option\.label\}/.test(panelSource) &&
    /\.heartbeat-editor-select__trigger\.heartbeat-editor-multiselect__trigger \{[\s\S]{0,120}height: auto;/.test(heartbeatCss) &&
    /\.heartbeat-editor-multiselect__values \{[\s\S]{0,320}max-height: 74px;[\s\S]{0,120}overflow-y: auto;/.test(heartbeatCss) &&
    !/local_test/.test(panelSource),
  "delivery editor expands backend-catalog multi-select targets and keeps revoked opaque references removable",
);

ok(
  /interface AutomationDeliveryTargetCatalogEntry extends AutomationDeliveryTarget/.test(typesSource) &&
    /label: string;[\s\S]{0,120}provider: string;[\s\S]{0,120}available: boolean;[\s\S]{0,180}scope: string;/.test(typesSource) &&
    !/interface AutomationDeliveryTargetCatalogEntry[\s\S]{0,320}(credential|remoteId|connectionId|url):/.test(typesSource) &&
    /pending\.has\("catalog"\)[\s\S]{0,420}automationListDeliveryTargets\(\)/.test(panelSource),
  "delivery catalog refreshes on invalidation without adding secret-bearing fields",
);

ok(
  /intervalWeekdays/.test(panelSource) &&
    /schedule\?\.weekdays/.test(heartbeatBridgeSource) &&
    /可选工作日/.test(panelSource),
  "interval schedules support an optional weekday filter through the task contract",
);

ok(
  /const backOrClose = useCallback/.test(panelSource) &&
    /if \(editing \|\| viewingRun \|\| usingTemplates\)[\s\S]*returnToList\(\)/.test(panelSource) &&
    /onCancel=\{returnToList\}/.test(panelSource),
  "automation subpages and template page return to the task list instead of closing the modal",
);

ok(
  /const taskToSave = task\.createdAt \? task : \{ \.\.\.task, createdAt: Date\.now\(\) \}/.test(panelSource) &&
    !/setEditing\(\{[\s\S]{0,220}title: ""[\s\S]{0,220}createdAt: Date\.now\(\)/.test(panelSource),
  "new automation drafts are marked created only after save",
);

ok(
  /catch \(err\) \{\s*setSaveError\(`自动化任务保存失败：\$\{heartbeatErrorMessage\(err\)\}`\);\s*return;\s*\}[\s\S]{0,520}setEditing\(null\)/.test(panelSource),
  "a failed create or update keeps the editor draft mounted for retry",
);

ok(
  /const \[creating, setCreating\] = useState\(false\)/.test(panelSource) &&
    /setCreating\(true\)[\s\S]{0,240}setEditing\(\{/.test(panelSource) &&
    /const handleEdit = useCallback\(\(task: HeartbeatTask\) => \{\s*setCreating\(false\);\s*setEditing/.test(panelSource) &&
    /editing \? creating \? t\("heartbeat\.addTask"\) : t\("heartbeat\.editTask"\)/.test(panelSource),
  "new automation drafts use the add-task title while persisted tasks use the edit-task title",
);

ok(
  /\{isNew \? t\("heartbeat\.add"\) : t\("heartbeat\.save"\)\}/.test(panelSource) &&
    localeSource.includes('"heartbeat.add": "添加"') &&
    localeSource.includes('"heartbeat.save": "保存"') &&
    /\.heartbeat-btn:not\(\.heartbeat-btn--primary\):not\(\.heartbeat-btn--danger\) \{\s*background: var\(--heartbeat-ref-surface\);\s*\}/.test(alignedActionCss) &&
    !/\.heartbeat-btn \{[^}]*background:/.test(alignedActionCss),
  "automation editor keeps add and save labels readable by limiting the late surface override to default buttons",
);

ok(
  /type="time"[\s\S]{0,120}aria-label="运行时间"/.test(panelSource),
  "cycle execution time has a stable accessible name",
);

ok(
	/<AnchoredPopover[\s\S]*className="heartbeat-editor__date-picker-menu"/.test(panelSource) &&
		/\.heartbeat-editor__date-picker-trigger:focus-visible,[\s\S]{0,100}\.heartbeat-editor__date-picker-trigger\[aria-expanded="true"\] \{[\s\S]{0,140}border-color: var\(--border-focus\);/.test(heartbeatCss) &&
		!/\.heartbeat-editor__date-picker-trigger:hover,[\s\S]{0,240}var\(--warning/.test(heartbeatCss),
  "effective-range picker reuses the editor focus border instead of a warning border",
);

ok(
  /<button\s+className="heartbeat-editor__tw-add"\s+type="button"\s+onClick=\{\(\) => setDraft/.test(panelSource) &&
    !/<span\s+className="heartbeat-editor__tw-add"/.test(panelSource),
  "add-time-window control is a keyboard-operable semantic button without a clickable span",
);

ok(
  /createBusinessDiagnosticsTask[\s\S]*?enabled: true/.test(panelSource) &&
    /Open directly in add mode[\s\S]*?setEditing\(\{[\s\S]*?enabled: true/.test(panelSource) &&
    /const handleAdd = useCallback[\s\S]*?setEditing\(\{[\s\S]*?enabled: true/.test(panelSource) &&
    /automationTemplateToHeartbeatTask[\s\S]*?enabled: true/.test(heartbeatBridgeSource),
  "new manual and template automation drafts default to enabled before first save",
);

ok(
  /const \[models, setModels\] = useState<ModelInfo\[\]>\(\[\]\)/.test(panelSource) &&
    /app\.AutomationModels\(root\)\.then\(\(list\) => setModels/.test(panelSource) &&
    /const modelRouteValue = \(model: ModelInfo\) => encodeAutomationModelRoute/.test(panelSource) &&
    /const availableModels = useMemo\(\(\) => filterModelInfosForSubscriptionAccess/.test(panelSource) &&
    /const modelSelectOptions: HeartbeatSelectOption\[\] = \[[\s\S]*availableModels\.map\(\(model\)/.test(panelSource) &&
    /<HeartbeatSelect[\s\S]*value=\{selectedModelValue\}[\s\S]*options=\{modelSelectOptions\}/.test(panelSource) &&
    /const route = decodeAutomationModelRoute\(value\)/.test(panelSource) &&
    /setDraft\(\(previous\) => \(\{ \.\.\.previous, \.\.\.route \}\)\)/.test(panelSource) &&
    !/<input[\s\S]{0,160}value=\{draft\.model \|\| ""\}[\s\S]{0,160}onChange=\{\(e\) => set\("model", e\.target\.value\)\}/.test(panelSource),
  "automation model selection reuses the route-aware chat catalog through the themed picker instead of a free-text model input",
);

ok(
  /interface AutomationRunSummary \{[\s\S]*?billingSource\?: "subscription_pool" \| "newapi_byok";/.test(typesSource) &&
    /<RunFact label="计费来源" value=\{manifest\.billingSource \|\| "-"\} \/>/.test(panelSource),
  "automation run details retain and display the resolved billing source alongside the model",
);

ok(
  /const \[skills, setSkills\] = useState<SkillView\[\]>\(\[\]\)/.test(panelSource) &&
    /const \[experts, setExperts\] = useState<ExpertInfo\[\]>\(\[\]\)/.test(panelSource) &&
    /app\.SkillsSettings\(\)\.then/.test(panelSource) &&
		/app\.ListExperts\(\)\.catch/.test(panelSource) &&
		/app\.ListUserExperts\(\)\.catch/.test(panelSource) &&
    /const selectedSkill = selectedSkills\[0\] \|\| ""/.test(panelSource) &&
    /const setSkillSelection = useCallback/.test(panelSource) &&
    /function HeartbeatSelect/.test(panelSource) &&
    /<AnchoredPopover[\s\S]*className="heartbeat-editor-select__menu"[\s\S]*role="listbox"[\s\S]*role="option"/.test(panelSource) &&
    /const skillSelectOptions: HeartbeatSelectOption\[\] = \[[\s\S]*availableSkills\.map\(\(skill\)/.test(panelSource) &&
    /<HeartbeatSelect[\s\S]*value=\{selectedSkill\}[\s\S]*options=\{skillSelectOptions\}/.test(panelSource) &&
    /const expertSelectionKnown = !draft\.expertId \|\| expertOptions\.some\(\(expert\) => expert\.id === draft\.expertId\)/.test(panelSource) &&
    /const expertSelectOptions: HeartbeatSelectOption\[\] = \[[\s\S]*expertOptions\.map\(\(expert\)/.test(panelSource) &&
    /src: skill\.iconUrl\?\.trim\(\) \|\| undefined/.test(panelSource) &&
    /src: expertAvatarSrcForInfo\(expert\)/.test(panelSource) &&
    /<HeartbeatSelectVisual visual=\{selected\?\.visual\}/.test(panelSource) &&
    /<HeartbeatSelectVisual visual=\{option\.visual\}/.test(panelSource) &&
    /automationExpertRole/.test(panelSource) &&
    /<HeartbeatSelect[\s\S]*value=\{draft\.expertId \|\| ""\}[\s\S]*options=\{expertSelectOptions\}/.test(panelSource) &&
    !/<select\b/.test(panelSource) &&
    !/<option\b/.test(panelSource) &&
    !/heartbeat-editor__chip/.test(panelSource) &&
    /skills: nonEmptyStrings\(task\.skills\)/.test(heartbeatBridgeSource) &&
    /role: emptyToUndefined\(task\.role\)/.test(heartbeatBridgeSource) &&
    /expertId: emptyToUndefined\(task\.expertId\)/.test(heartbeatBridgeSource),
	"automation prompt editor merges official and custom experts while persisting the catalog ID separately from role text",
);

ok(
    /\.heartbeat-editor-select__trigger \{[\s\S]*background: color-mix\(in srgb, var\(--bg\) 82%, var\(--bg-elev\)\)[\s\S]*color: var\(--fg\)/.test(heartbeatCss) &&
    /\.heartbeat-editor-select__menu \{[\s\S]*background: var\(--bg\)/.test(heartbeatCss) &&
    /\.heartbeat-editor-select__option:hover,[\s\S]*background: var\(--accent-soft/.test(heartbeatCss) &&
    /\.heartbeat-editor-select__option--selected \{[\s\S]*background: color-mix\(in srgb, var\(--accent\) 12%, transparent\)/.test(heartbeatCss) &&
    /\.heartbeat-editor-select__visual--expert \{[\s\S]*border-radius: 50%/.test(heartbeatCss) &&
    /\.heartbeat-editor-select__visual img \{[\s\S]*object-fit: cover/.test(heartbeatCss) &&
    /\.heartbeat-editor-select--freq \{/.test(heartbeatCss) &&
    /\.heartbeat-editor-select--source-type \{/.test(heartbeatCss) &&
    !/heartbeat-editor__freq-select/.test(heartbeatCss),
  "automation pickers use fully themed custom trigger and menu colors without native select styling",
);

ok(
  /PickAutomationSourceFile\(sourceType: string\)/.test(bridgeSource) &&
    /func \(a \*App\) PickAutomationSourceFile\(sourceType string\)/.test(appSource),
  "desktop file selection is exposed to the automation frontend",
);

ok(
  /onFilesDropped/.test(panelSource) &&
    /data-business-source-kind=\{def\.kind\}/.test(panelSource) &&
    /"--wails-drop-target": "drop"/.test(panelSource) &&
    /businessSourceKindAtPoint\(point\) \|\| draggingKindRef\.current/.test(panelSource) &&
    /filePathFromDataTransfer\(e\.dataTransfer\)/.test(panelSource) &&
    /onDragLeave=\{\(e\) => \{\s*if \(!e\.currentTarget\.contains\(e\.relatedTarget as Node \| null\)\) \{\s*draggingKindRef\.current = null;\s*setDraggingKind\(null\);\s*\}\s*\}\}/.test(panelSource),
  "business diagnostics sources accept drag/drop files and clear stale fallback targets",
);

ok(
  /await heartbeatSaveTasks\(next\);\s*setTasks\(next\);/.test(panelSource) &&
    /setSaveError\(`自动化任务保存失败：\$\{heartbeatErrorMessage\(err\)\}`\);/.test(panelSource),
  "automation task save updates local state only after persisted save succeeds and surfaces save errors",
);

ok(
  /triggerPromise = heartbeatTriggerNow\(id\);[\s\S]*await triggerPromise;[\s\S]*setSaveError\(""\);[\s\S]*catch \(err\)[\s\S]*setSaveError\(`自动化任务立即运行失败：\$\{heartbeatErrorMessage\(err\)\}`\);/.test(panelSource),
  "automation run-now failures surface a visible error instead of disappearing",
);

ok(
  /heartbeat-card__open-btn--play[\s\S]{0,180}disabled=\{!task\.enabled \|\| quotaPaused \|\| !!triggering\}[\s\S]{0,180}onClick=\{onTrigger\}/.test(panelSource),
  "disabled automation tasks cannot be manually triggered into a misleading task-disabled run",
);

ok(
  /const onDayToggle = useCallback\(\(day: string\) => \{[\s\S]{0,420}setSelectedDays\(next\);\s*setDraft\("interval", buildInterval\(cycleType, next, timeVal\)\);/.test(panelSource) &&
    !/const onDayToggle = useCallback\(\(day: string\) => \{\s*setSelectedDays\(\(prev\) => \{[\s\S]{0,320}setDraft\(/.test(panelSource),
  "weekday toggles update the parent draft outside the selected-days state updater",
);

ok(
  /disabled=\{!task\.topicId\}[\s\S]{0,360}title=\{t\("heartbeat\.openTopic" as any\)\}[\s\S]{0,120}aria-label=\{t\("heartbeat\.openTopic" as any\)\}/.test(panelSource),
  "the disabled open-topic action retains a localized accessible name",
);

ok(
  !panelSource.includes('t("heartbeat.fieldConditionPrompt")') &&
    !panelSource.includes('t("heartbeat.conditionPromptPlaceholder")') &&
    !localeSource.includes('"heartbeat.fieldConditionPrompt"') &&
    !localeSource.includes('"heartbeat.conditionPromptPlaceholder"') &&
    localeSource.includes('"heartbeat.promptPlaceholder": "例如：每天早上整理今日待办，并提醒重要安排。"'),
  "automation editor omits the condition prompt field and uses a daily-life prompt example",
);

ok(
  /\.heartbeat-card \.heartbeat-card__head-actions \.heartbeat-card__open-btn \{[^}]*opacity: 1;[^}]*pointer-events: auto;/.test(heartbeatCss) &&
    /\.heartbeat-card__open-btn:disabled \{[^}]*opacity: 0\.3;/.test(heartbeatCss) &&
    !/\.heartbeat-card \.heartbeat-card__head-actions \.heartbeat-card__open-btn \{[^}]*opacity: 0;[^}]*pointer-events: none;/.test(heartbeatCss),
  "automation task cards keep every action slot visible while disabled actions remain visually muted",
);

ok(
  /heartbeat-business-file__pick/.test(panelSource) &&
    !/value=\{current\.path \|\| ""\}[\s\S]{0,120}onChange=\{\(e\) => upsertSource\(def\.kind, \{ path: e\.target\.value \}\)\}/.test(panelSource),
  "business diagnostics file sources use choose/drop UI instead of manual path entry",
);

ok(
  /heartbeatListTemplates/.test(panelSource) &&
    /TemplateGrid/.test(panelSource) &&
    /AutomationListTemplates\(\): Promise<unknown>/.test(bridgeSource) &&
    /func \(a \*App\) AutomationListTemplates\(\) \(\[\]automation\.TaskTemplate, error\)/.test(automationSource),
  "automation template page is wired through the frontend bridge to the desktop binding",
);

ok(
  /template\.kind === "business_diagnostics"[\s\S]*createBusinessDraft\(id\)[\s\S]*automationTemplateToHeartbeatTask\(template, id\)/.test(panelSource) &&
    /editing\?\.businessDiagnostics \? "经营诊断"/.test(panelSource) &&
    /<BusinessDiagnosticsEditor/.test(panelSource),
  "business diagnostics template opens the business diagnostics editor instead of a plain task editor",
);

ok(
  /const BUSINESS_DIAGNOSTICS_EXPERT_ROLE =/.test(panelSource) &&
    /createBusinessDiagnosticsTask[\s\S]*role: BUSINESS_DIAGNOSTICS_EXPERT_ROLE/.test(panelSource) &&
    /关联会话中读取数据、展开分析/.test(panelSource),
  "business diagnostics templates default to an expert role and describe conversation-backed analysis",
);

ok(
  /heartbeatSaveTaskTemplate/.test(panelSource) &&
    /同时保存为模板/.test(panelSource) &&
    /AutomationSaveTaskTemplate\(task: unknown\): Promise<unknown>/.test(bridgeSource) &&
    /func \(a \*App\) AutomationSaveTaskTemplate\(task automation\.Task\) \(automation\.TaskTemplate, error\)/.test(automationSource),
  "manual automation tasks can be saved as backend-persisted templates",
);

ok(
  /heartbeatDeleteTaskTemplate/.test(panelSource) &&
    /heartbeatRestoreTaskTemplate/.test(panelSource) &&
    /TemplateGrid templates=\{templates\} onSelect=\{handleUseTemplate\} onDelete=\{handleDeleteTemplate\}/.test(panelSource) &&
    !/window\.confirm/.test(panelSource) &&
    /confirmingTemplateDeleteID/.test(panelSource) &&
    !/template\.source !== "user"/.test(panelSource) &&
    !/template\.source === "user"/.test(panelSource) &&
    /heartbeat-template-card__delete/.test(panelSource) &&
    /heartbeat-template-card__confirm/.test(panelSource) &&
    /\.heartbeat-template-card__confirm \{[\s\S]*color-mix\(in srgb, var\(--err\)/.test(heartbeatCss) &&
    /e\.stopPropagation\(\)/.test(panelSource) &&
    /AutomationDeleteTaskTemplate\(id: string\): Promise<void>/.test(bridgeSource) &&
    /AutomationRestoreTaskTemplate\(id: string\): Promise<void>/.test(bridgeSource) &&
    /func \(a \*App\) AutomationDeleteTaskTemplate\(id string\) error/.test(automationSource) &&
    /func \(a \*App\) AutomationRestoreTaskTemplate\(id string\) error/.test(automationSource),
  "automation templates expose delete and restore actions through the desktop binding",
);

ok(
  (builtinTemplateSource.match(/\bID:\s+"/g) || []).length === 12 &&
    !/pet-wallpaper/.test(builtinTemplateSource) &&
    /Interval:\s+"24h\|daily@09:00"/.test(builtinTemplateSource) &&
    /Kind:\s+"business_diagnostics"/.test(builtinTemplateSource),
  "desktop backend exposes all built-in automation templates with default intervals",
);

ok(
  /最终结果/.test(panelSource) &&
    /技术详情/.test(panelSource) &&
    /事件时间线/.test(panelSource) &&
    /<details className="heartbeat-run-detail__technical heartbeat-run-detail__technical--result" open>[\s\S]*<summary>最终结果<\/summary>[\s\S]*<details className="heartbeat-run-detail__technical">[\s\S]*<summary>技术详情<\/summary>/.test(panelSource) &&
    !/RUN_RESULT_EXPORT_FORMATS/.test(panelSource) &&
    /availableResultFormats/.test(typesSource) &&
    /const exportFormats = \(detail\?\.availableResultFormats \|\| \[\]\)/.test(panelSource) &&
    /exportFormats\.map/.test(panelSource) &&
		/status === "running"[\s\S]{0,260}onClick=\{onCancelRun\}/.test(panelSource) &&
    /AutomationExportRunResult\(taskID: string, runID: string, format: string\): Promise<unknown>/.test(bridgeSource) &&
    /func \(a \*App\) AutomationExportRunResult\(taskID, runID, format string\)/.test(automationSource),
  "run detail keeps final result, event timeline, backend-driven exports, and running-run cancellation",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
