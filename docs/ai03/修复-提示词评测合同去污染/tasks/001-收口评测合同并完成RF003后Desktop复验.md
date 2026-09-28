# 001 收口评测合同并完成 RF-003 后 Desktop 复验

## 目标与产物

修复 RF-001、RF-002、RF-005、RF-007 的 prompt-eval 合同偏差，关闭已解决/非缺陷记录，
并在最终实现上完成 Office、Coding、Design 各两次真实 Wails Desktop 复验。

预期产物：

- `benchmarks/prompt-eval` 参数、任务、grader 与回归测试；
- 更新后的缺陷状态和本任务完成回执；
- Git 忽略目录中的去敏真实 Desktop 结果；
- 一个可独立评审、合并和回滚的 Draft PR。

## 依赖与证据

- 依赖 RF-003 修复 PR #321，已进入基线 `bff9cff8`。
- Codegraph 索引基线为 clean `8206dd5b`；相关 benchmark 文件到实现基线无差异。
- 具体根因、方案和验收标准见 [`../plan.md`](../plan.md)。
- RF-004 已由 `normalizeText` 的 Unicode minus 归一化和现有测试覆盖。

## 允许与禁止范围

允许：

- `benchmarks/prompt-eval/**`；
- `docs/ai03/修复-提示词评测合同去污染/**`；
- 为实际 Desktop 验收创建 Git 忽略的临时配置、工作区与结果。

禁止：

- 生产系统提示词、Scene Profile、Agent、Controller、Desktop UI、工具权限和 provider；
- schema、依赖、存储和迁移；
- 把凭据、内部 URL、完整原始对话或敏感日志写入版本库；
- 为通过 grader 而降低公开任务的真实质量要求。

## 执行切片与提交

1. 文档合同：计划、任务和进度。
2. RF-001：非负 max-steps 参数合同及测试。
3. RF-002：公开 Profile Card 内容、SVG 语义/focus grader 及正反测试。
4. RF-005：Python 非法参数异常类型合同及测试。
5. RF-007：公开 Dashboard 集成要求、响应式 SVG 画布合同及正反测试。
6. 最终回执：自动化、Desktop 结果、偏差和剩余风险。

各切片串行；前一切片测试通过后再提交下一切片。测试与实现必须在同一意图提交中。

## 保持行为

- positive `max_steps`、repeats、timeout 和 worker 参数行为不变。
- Profile Card 固定 `640×360`、可访问标题/描述、指标、操作和分组要求不变。
- retry 正常值、封顶值、布尔非法和本地测试要求不变。
- Dashboard KPI、事件、趋势、响应式、reduced motion、focus 和禁用渐变要求不变。
- manifest 继续包含任务、提示词和运行参数；报告仍区分 artifact/run/E2E。

## 自动化验收

每个切片运行精确测试，最终至少运行：

```powershell
go test ./benchmarks/prompt-eval/... -count=1
go vet ./benchmarks/prompt-eval/...
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

新增测试必须包含正反例，不能只证明已知成功 fixture。

## 实际 Desktop 验收

使用当前生产提示词，不创建新候选。通过真实 Wails Desktop 执行：

- Office：一个中型任务，两次；
- Coding：一个中型任务，两次；
- Design：一个中型任务，两次。

用户已在本轮确认 OpenAI-compatible 供应商、API Key 和模型 `gpt-5.6-luna`。凭据只进入忽略
的临时环境/配置。每轮记录完成态、grader、readiness retry、模型/工具调用、token、耗时、
审批和真实产物观察；异常先记录，不擅自扩展为运行时或提示词修复。

### Desktop QA 清单

| 场景/状态 | 功能检查 | 可见与产物检查 | 证据 |
| --- | --- | --- | --- |
| 启动与项目切换 | Wails 启动、打开六个独立工作区、新建独立会话 | 初始界面无阻断性遮挡，场景和工作区可辨识 | 运行日志、窗口截图 |
| Office 完成态 | 读取输入、生成两份 Markdown、核算总计与团队差异 | 两份文件结构清晰，事实/未知原因和预算上限可辨识 | 文件、grader、会话状态 |
| Coding 完成态 | 修改实现、补测、执行测试、保留 README | 正常/封顶/非法输入均有可核验结果 | diff、pytest、grader、会话状态 |
| Design 完成态 | 生成 HTML/CSS/SVG 并互相引用 | 宽屏和窄屏无关键裁切，主操作可键盘聚焦且 focus 可见 | 宽/窄屏截图、grader、会话状态 |
| 权限与收尾 | 仅批准任务内操作，Agent 正常结束 | 无重复 readiness、无完成态循环、无越界改动 | 审批数、retry 数、工具/模型调用和耗时 |

探索性检查：对 Coding 非法输入和未修改 `README.md` 做反向确认；对 Design 的窄屏、reduced-motion
与键盘 Tab 焦点做独立检查。若发现 Controller/运行时错误，记录证据并继续其余轮次，不用 grader 或提示词掩盖。

## 分配与状态

| 项目 | 值 |
| --- | --- |
| 执行者 | 主会话 Codex；不继续委派 |
| 状态 | 已完成；等待 Draft PR 授权 |
| PR 映射 | 计划、任务、实现、测试和回执共用一个 Draft PR |

## 完成回执

### 状态

评测合同修复完成。正式 Wails Desktop 验收为 5/6 端到端通过：Coding 2/2、Design 2/2，
Office 1/2。失败轮次不是 grader 假阴性：`decision-summary.md` 遗漏已确认事实 `contractor`，
而 Agent 最终答复错误声称两份文档均已完成并验证，属于生产提示词后续需要定向解决的语义复核问题。

### 修改文件与测试

- `main.go`：`max_steps=0` 表示不限步数，负数继续拒绝；grader 版本提升到 v6。
- `tasks.go` / `graders.go`：对齐 Profile Card 的公开固定内容，并以 SVG 语义和真实可聚焦性判定。
- `desktop_tasks.go` / `desktop_graders.go`：公开 Dashboard 集成合同；retry 接受 `ValueError` /
  `TypeError`；响应式 SVG 接受正尺寸或正尺寸 `viewBox`。
- 对上述合同补充正反回归测试。

已通过：

```powershell
go test ./benchmarks/prompt-eval/... -count=1
go vet ./benchmarks/prompt-eval/...
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

正式 Desktop 结果：

| 场景/轮次 | 产物 / 端到端 | active 耗时 | tokens | 模型 / 工具调用 | 审批等待 | readiness retry |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Office R1 | 通过 / 通过 | 149.914 s | 493,961 | 17 / 16 | 845 ms | 1 |
| Office R2 | 失败 / 失败 | 131.659 s | 275,674 | 12 / 15 | 253 ms | 0 |
| Coding R1 | 通过 / 通过 | 89.146 s | 280,409 | 13 / 12 | 545 ms | 0 |
| Coding R2 | 通过 / 通过 | 67.243 s | 256,800 | 12 / 15 | 511 ms | 0 |
| Design R1 | 通过 / 通过 | 150.626 s | 304,305 | 12 / 14 | 435 ms | 0 |
| Design R2 | 通过 / 通过 | 199.974 s | 361,078 | 14 / 17 | 541 ms | 0 |

六轮 active 平均 131.427 s、平均 328,705 tokens；审批等待共 3.130 s。Office R1 的一次
readiness retry 来自 Agent 仍有任务项处于 `in_progress`，没有复现 RF-003 的产物状态陈旧路径。
Design 两轮均通过 1440×900、390×844、键盘 focus 和 reduced-motion 独立检查。

### 偏差、风险与交接

- 首次 Office 探索运行有一次约 67.7 s 的人工审批等待，计时已判无效并排除；正式统计同时
  保留 raw wall time 和可测审批等待，以 active 耗时比较，避免把操作等待归因给 Agent。
- RF-001、RF-002、RF-005、RF-007 已在评测层收口；RF-004 由既有实现覆盖，RF-006 为非缺陷。
- RF-003 修复后六轮没有出现产物验证状态陈旧；一次合法 readiness 拒绝不应再归为 RF-003。
- 当前最明确的提示词问题是 Office 委派后主 Agent 未逐项核对已确认事实，却作出完成声明。
  下一轮应针对“交付前从用户要求到产物逐条语义核验”做最小改动和对照复验，不应盲目扩写内核。
- Coding 两轮稳定；Design 能自纠错并通过视觉检查，但 tokens 和时长最高，后续可在不降低
  视觉验收的前提下减少重复检查与工具往返。
