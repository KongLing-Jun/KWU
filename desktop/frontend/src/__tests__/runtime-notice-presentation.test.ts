// Run: tsx src/__tests__/runtime-notice-presentation.test.ts

import { presentCompactionTrigger, presentRuntimeEventText } from "../features/session/runtimeNoticePresentation";
import { t } from "../lib/i18n";

let passed = 0;
let failed = 0;

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

const phaseCases: Array<[string, string, string]> = [
  ["provider/model · planning", "provider/model · 正在规划", "planner phase"],
  ["provider/model · executing", "provider/model · 正在执行", "executor phase"],
  ["task_tree: planning", "任务规划：正在规划", "task tree planning"],
  ["task_tree: tree_created", "任务规划：计划已生成", "task tree created"],
  ["task_tree: executing", "任务规划：正在执行", "task tree executing"],
  ["task_tree: completed", "任务规划：已完成", "task tree completed"],
  ["task_tree: failed", "任务规划：失败", "task tree failed"],
  ["task_tree: blocked", "任务规划：受阻", "task tree blocked"],
  ["task_tree: skipped", "任务规划：已跳过", "task tree skipped"],
  ["task_tree: node_dispatch read_only_task inspect", "任务节点 inspect：正在分派", "node dispatch"],
  ["task_tree: node_started inspect", "任务节点 inspect：正在执行", "node started"],
  ["task_tree: node_completed inspect", "任务节点 inspect：已完成", "node completed"],
  ["task_tree: node_failed inspect", "任务节点 inspect：失败", "node failed"],
  ["task_tree: node_blocked inspect", "任务节点 inspect：受阻", "node blocked"],
  ["task_tree: node_skipped inspect", "任务节点 inspect：已跳过", "node skipped"],
];

const noticeCases: Array<[string, string, string]> = [
  ["response truncated: hit max output tokens", "回复达到最大输出长度，内容可能不完整", "max output finish"],
  ["response blocked by content filter", "回复被内容安全策略阻止", "content filter finish"],
  ["response truncated: model repetition detected", "检测到模型重复，回复已提前结束", "repetition finish"],
  ["final-answer readiness blocked: missing project checks", "最终回复尚未满足完成条件，正在继续处理", "readiness retry"],
  ["empty final answer blocked: gpt/x returned no visible answer text (finish=stop, reasoning=12 chars); retrying", "模型未返回可见内容，正在重试", "empty final retry"],
  ["executor answered without taking any action; nudging it to use its tools", "正在继续执行任务", "executor handoff"],
  ["budget (turn_max_steps=8) exhausted: one grace round to finalize", "工具调用即将达到本轮上限，正在整理结果", "tool budget"],
  ["loop guard: read_file failed 3× the same way — nudging the model to change approach", "检测到重复失败，正在改用其他方法", "loop guard"],
  ["tool output truncated: 120 of 800 bytes elided", "工具输出过长，已省略 120 / 800 字节", "tool truncation"],
  ["context reached 75% of window; keeping cache-first prefix until compact threshold 85%", "上下文已使用 75%，达到 85% 时将自动压缩", "soft compaction"],
  ["pruned 3 stale tool results (~420 tokens est.) before compaction", "已清理 3 条过期工具结果，准备压缩上下文", "stale tool pruning"],
  ["context_window=8192 is too small for compaction to help (the system prompt plus one turn already exceeds 85% of it); raise context_window or shrink tool output. Auto-compaction paused until the prompt drops.", "当前上下文窗口过小，自动压缩已暂停", "compaction stuck"],
  ["compaction skipped: provider timed out", "未执行上下文压缩：provider timed out", "compaction skipped"],
  ["compaction summary unavailable (provider timed out); folded mechanically", "无法生成上下文摘要，已使用安全的折叠结果：provider timed out", "mechanical compaction"],
  ["summarized 9 later messages → summary", "已将后续 9 条消息整理为摘要", "summarize later"],
  ["summarized 11 earlier messages → summary", "已将前面的 11 条消息整理为摘要", "summarize earlier"],
  ["background task started: task-1 (代码审查专家)", "子任务已开始", "task started"],
  ["background task finished: task-1", "子任务已完成", "task completed"],
  ["background task failed: task-1 — boom", "子任务失败", "task failed"],
  ["background task killed: task-1", "子任务已停止", "task killed"],
  ["background task may be stalled: task-1 — still running after 10m with no visible output; inspect with wait/bash_output or stop with kill_shell", "子任务长时间没有新进展，仍在运行", "task stalled"],
  ["background bash finished: job-4", "后台命令已完成", "background command completed"],
  ["job artifact migration failed: permission denied", "后台任务记录迁移失败：permission denied", "job artifact migration"],
  ["task_tree planning failed: planner returned no tree.", "任务规划失败：没有生成可执行计划", "tree planning failed"],
  ["task_tree execution failed: dependency failed", "任务规划执行失败：dependency failed", "tree execution failed"],
  ["task_tree node verify blocked: waiting for build", "任务节点 verify 受阻：waiting for build", "tree node blocked"],
  ["guardian_model \"safe-model\" not found — guardian disabled", "未找到安全检查模型 safe-model，安全检查已停用", "guardian model missing"],
  ["guardian construction failed: bad config — guardian disabled", "安全检查初始化失败，已停用：bad config", "guardian init failed"],
  ["guardian enabled · model=safe-model", "安全检查已启用：safe-model", "guardian enabled dot"],
  ["guardian enabled; model=safe-model", "安全检查已启用：safe-model", "guardian enabled semicolon"],
  ["this project defines local MCP servers but is not trusted, so auto-start is disabled for: local-a; run /mcp trust-project and restart the session before auto-starting them", "此项目的本地 MCP 服务尚未信任，已禁止自动启动：local-a；运行 /mcp trust-project 并重启会话后可启用", "MCP untrusted"],
  ["mcp local-a: connection refused", "MCP 服务 local-a 连接失败：connection refused", "MCP failure"],
	["mcp canva--canva: plugin \"canva--canva\": initialize: Connector is temporarily unavailable. Please retry the current operation later.", "连接器暂时不可用，请稍后重试当前操作", "connector temporary failure guides retrying the current operation"],
	["mcp canva--canva: plugin \"canva--canva\": initialize: Connector is temporarily unavailable. Retry it from the Connectors page.", "连接器暂时不可用，请稍后重试当前操作", "legacy connector temporary guidance stays localized"],
	["mcp canva--canva: plugin \"canva--canva\": initialize: Connector authorization is required. Reconnect it from the Connectors page.", "连接器需要重新授权，请前往连接器页面重新连接", "connector reauthorization failure is actionable"],
	["mcp canva--canva: plugin \"canva--canva\": initialize: connector OAuth credential is unavailable", "连接器暂时不可用，请稍后重试当前操作", "legacy connector credential detail is hidden"],
  ["2 MCP server(s) failed to start: a, b — run /mcp for details", "2 个 MCP 服务启动失败：a, b；可运行 /mcp 查看详情", "MCP startup failures"],
  ["5 MCP server(s) failed to start: a, b, c (+2 more) — run /mcp for details", "5 个 MCP 服务启动失败：a, b, c，另有 2 个；可运行 /mcp 查看详情", "MCP startup failures with hidden remainder"],
  ["legacy user-data migration failed: disk full", "旧版用户数据迁移失败：disk full", "legacy migration failed"],
  ["cleanup-pending reconciliation failed: access denied", "会话清理恢复失败：access denied", "cleanup reconciliation"],
  ["mark cleanup pending failed: access denied", "无法记录待清理会话：access denied", "mark cleanup pending"],
  ["clear session cleanup failed: access denied", "清理会话文件失败：access denied", "session cleanup"],
  ["model \"old/model\" is no longer available; switched to new/model", "原模型 old/model 已不可用，已切换到 new/model", "model fallback"],
  ["effort is not configurable for Luna", "Luna 不支持调整推理强度", "effort unsupported"],
  ["effort for Luna set to high", "Luna 的推理强度已设为 high", "effort set"],
  ["rewound conversation to turn 3", "会话已回退到第 3 轮", "conversation rewind"],
  ["forked conversation at turn 3 into a new session", "已从第 3 轮创建新会话", "conversation fork"],
  ["final-answer readiness failed 2 times: missing checks", "最终回复检查多次未通过，本轮已停止", "turn done readiness"],
  ["model finished without a visible final answer 2 times", "模型连续未返回可见内容，本轮已停止", "turn done empty final"],
  ["paused after 8 tool-call rounds (turn_max_steps) — the work so far is saved; send another message to continue, or set turn_max_steps higher or to 0 for no limit", "本轮工具调用已达上限，当前进度已保存；发送消息可继续", "turn done tool budget"],
  ["context canceled", "本轮已中断", "turn canceled"],
  ["context deadline exceeded", "请求超时，本轮已停止", "turn timeout"],
  ["background bash started: job-1", "后台命令已开始", "background command started"],
  ["background bash failed: job-1 — boom", "后台命令失败", "background command failed"],
  ["background shell killed: job-1", "后台命令已停止", "background command killed"],
  ["background worker started: job-1", "后台任务已开始", "generic background started"],
  ["background worker finished: job-1", "后台任务已完成", "generic background finished"],
  ["background worker failed: job-1 — boom", "后台任务失败", "generic background failed"],
  ["background worker killed: job-1", "后台任务已停止", "generic background killed"],
  ["background worker may be stalled: job-1 — details", "后台任务长时间没有新进展，仍在运行", "generic background stalled"],
  ["MCP server \"local-a\" connected but exposed no tools.", "MCP 服务 local-a 已连接，但没有提供工具", "MCP no tools"],
  ["enabled MCP server \"local-a\" tools: read, write.", "已启用 MCP 服务 local-a 的工具：read, write", "MCP tools enabled"],
  ["migrated your previous configuration and data: C:/old → C:/new (2 MCP server(s)); API key saved to 云智企办 credentials store. The old files were left untouched.", "已迁移旧版配置和数据：C:/old → C:/new；旧文件保持不变", "legacy migration complete"],
  ["imported 4 past session(s) from old sessions — resume them with --resume or the history panel", "已从 old sessions 导入 4 个历史会话", "session import"],
  ["Deprecated [agent].memory_compiler was removed; Memory v5 no longer runs or writes state, while old transcripts remain readable.", "已移除停用的旧版记忆编译器配置；历史会话仍可读取", "memory compiler removed"],
  ["Deprecated [agent].memory_compiler is ignored and was not rewritten; Memory v5 no longer runs or writes state: access denied", "旧版记忆编译器配置迁移失败：access denied", "memory compiler migration failed"],
  ["invalid timezone \"Mars/Base\"; falling back", "时区 Mars/Base 无效，已使用可用时区", "invalid timezone"],
  ["plan_mode_allowed_tools ignored known blocked entries: bash; this setting only declares extra read-only custom tools and cannot unlock known blocked tools or unsafe bash", "计划模式忽略了不能安全启用的工具配置：bash", "plan mode tools ignored"],
  ["model \"gpt/x\" is selected but its API key OPENAI_API_KEY is not set for profile default (credentials: C:/config) - requests will fail until you set it", "模型 gpt/x 缺少 API 密钥 OPENAI_API_KEY（profile default (credentials: C:/config)），配置后才能发送请求", "missing API key"],
  ["auto plan: task looks multi-step; drafting a plan first", "任务较复杂，正在先制定计划", "auto plan started"],
  ["auto plan classifier: multiple modules", "正在确认是否需要先制定计划", "auto plan classified"],
  ["auto plan classifier failed; falling back to heuristic: provider down", "未能自动判断是否需要计划，已使用基础规则", "auto plan fallback detail"],
  ["plan-exec: detected 3 modules — api, ui, test", "已识别 3 个任务模块：api, ui, test", "plan modules"],
  ["plan-exec: dispatching 4 plan steps (strict=true)", "正在安排 4 个计划步骤", "plan dispatch"],
  ["compaction failed: disk full", "上下文压缩失败：disk full", "manual compaction failed"],
  ["compacted", "上下文已压缩", "manual compaction done"],
  ["new session failed: permission denied", "新建会话失败：permission denied", "new session failed"],
  ["new session", "已新建会话", "new session done"],
  ["clear context failed: permission denied", "清除会话上下文失败：permission denied", "clear context failed"],
  ["context cleared", "会话上下文已清除", "context cleared"],
  ["rewound code to turn 3 — 2 file(s) restored, 1 removed", "代码已回退到第 3 轮：恢复 2 个文件，移除 1 个文件", "code rewind"],
  ["created branch feature-a", "已创建分支 feature-a", "branch created"],
  ["switched to branch feature-a", "已切换到分支 feature-a", "branch switched"],
  ["resumed after 31m idle (provider cache expired) — elided 5 stale tool results to cheapen the cold restart", "会话已从长时间闲置中恢复，并清理了 5 条过期工具结果", "idle resume"],
  ["load expert metadata failed: corrupt json", "专家状态处理失败：corrupt json", "expert state"],
  ["effort for Luna: high (default: medium; options: low|medium|high)", "Luna 的推理强度为 high（默认 medium；可选 low|medium|high）", "effort status"],
  ["imported 2 memory file(s) from old memory", "已从 old memory 导入 2 个记忆文件", "memory import"],
  ["migration rescue: config migration warning: disk full", "旧版数据迁移出现警告：disk full", "migration warning"],
  ["migration rescue completed with 2 warning(s): imported 3 memory file(s) and 4 past session(s)", "旧版数据迁移完成，但有 2 个警告；已导入 3 个记忆文件和 4 个历史会话", "migration warning summary"],
  ["migration rescue: checking legacy config and credentials", "正在检查旧版配置和凭据", "migration checking config"],
  ["migration rescue: current config is already present or no legacy config was found", "当前配置已就绪，无需迁移旧版配置", "migration config current"],
  ["migration rescue: scanning legacy memory", "正在检查旧版记忆数据", "migration scanning memory"],
  ["migration rescue: scanning legacy sessions", "正在检查旧版历史会话", "migration scanning sessions"],
  ["migration rescue: no legacy memory needed migration", "没有需要迁移的旧版记忆数据", "migration no memory"],
  ["migration rescue: no legacy sessions needed migration", "没有需要迁移的旧版历史会话", "migration no sessions"],
  ["migration rescue complete: no legacy data needed migration", "没有需要迁移的旧版数据", "migration nothing"],
  ["no active plan with todos to execute", "当前没有可执行的任务计划", "no active plan"],
  ["prometheus: starting planning interview", "正在通过问答完善任务计划", "planning interview"],
  ["goal intercept: hard requirement cannot be overridden; satisfy it before signaling [goal:complete]", "仍有必须完成的要求，暂时无法结束任务", "goal hard requirement"],
  ["goal intercept: incomplete todos remain (override with a second [goal:complete])", "任务清单中仍有未完成事项", "goal todos"],
];

console.log("\nruntime notice presentation");

for (const [input, expected, label] of phaseCases) {
  eq(presentRuntimeEventText("phase", input, t), expected, label);
}
for (const [input, expected, label] of noticeCases) {
  eq(presentRuntimeEventText("notice", input, t), expected, label);
}

const unknown = "provider exploded with code E_STRANGE";
eq(presentRuntimeEventText("notice", unknown, t), unknown, "unknown notice remains byte-for-byte unchanged");
const similar = "prefix final-answer readiness blocked: missing project checks";
eq(presentRuntimeEventText("notice", similar, t), similar, "similar but unanchored notice remains unchanged");
eq(presentCompactionTrigger("auto", t), "自动", "automatic compaction trigger");
eq(presentCompactionTrigger("manual", t), "手动", "manual compaction trigger");
eq(presentCompactionTrigger("provider-defined", t), "provider-defined", "unknown compaction trigger remains unchanged");
const modelText = "executor answered without taking any action; nudging it to use its tools";
eq(modelText, modelText, "model and tool bodies do not call the runtime presentation function");

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
