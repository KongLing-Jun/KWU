# 提示词评测合同去污染与 RF-003 后复验计划

## 背景与问题

系统提示词 Extended V2 已完成真实 Desktop A/B，并在质量持平时降低了 token、模型调用、
工具调用和模型耗时。但冻结评测资产记录的 RF-001、RF-002、RF-005、RF-007 会把产品支持的
配置或满足公开任务语义的产物判为失败；RF-004 已在源码中修复但文档仍未关闭。RF-003 已由
PR #321 修复，原先受完成态错误污染的数据不能直接作为修复后的稳定性结论。

本任务先修复评测合同，不改生产提示词或运行时；随后以当前生产提示词执行三场景各两次真实
Wails Desktop 中型任务，决定是否存在值得进入下一轮提示词优化的稳定行为缺陷。

## 基线与源码证据

| 项目 | 值 |
| --- | --- |
| 实现基线 | `origin/main-v3@bff9cff87fc5d5c3fcfbda9a038ffb07592663c2` |
| 实现分支 | `codex/fix-prompt-eval-contracts` |
| 实现 worktree | `D:\Reasonix-worktrees\fix-prompt-eval-contracts` |
| Codegraph 索引基线 | clean `D:\Reasonix@8206dd5b`；相关 benchmark 文件到实现基线无差异 |

Codegraph 与 worktree 源码确认：

- `benchmarks/prompt-eval/main.go` 在真实运行前把 `MaxSteps < 1` 判为非法，随后同一值进入
  临时 `config.toml` 和 `boot.Build`；产品配置把 `0` 定义为不限步数。
- `gradeProfileCardSVG` 只接受字面量 `role` 和 `focus`，而 `parseSVG` 当前不读取 `<style>`
  或可验证的焦点结构。
- `gradeDesktopCodingRetry` 的隐藏脚本只捕获 `ValueError`，公开任务只要求拒绝非法值。
- `gradeDesktopDesignDashboard` 强制 HTML 出现 SVG 文件名并要求 SVG 至少 `900×600`，公开
  任务未明确前者，也不要求固定像素画布。
- `normalizeText` 已把 `−`、`–`、`—` 统一为 `-`，且已有 Unicode minus 回归测试；RF-004
  不需要重复实现。

调用影响限定在 prompt-eval 参数校验、冻结任务和 grader。生产 Agent、Controller、Desktop、
系统提示词、provider 协议、schema、依赖和存储均不受影响。

## 目标与非目标

### 目标

- RF-001：接受 `max_steps=0`，继续拒绝负数；正数行为不变。
- RF-002：Profile Card 的姓名、职位成为公开任务内容；grader 验证内容、可聚焦交互和真实
  可见焦点样式，不依赖 `role` / `focus` 字面量。
- RF-005：非法参数抛出 `ValueError` 或 `TypeError` 均通过，其他异常仍失败。
- RF-007：公开要求 HTML 引用 `dashboard-visual.svg`；SVG 接受正尺寸 `width/height` 或
  正尺寸 `viewBox`，删除任意的 `900×600` 下限。
- 关闭已修复的 RF-003、RF-004 和非缺陷 RF-006，并保留可追溯证据。
- 修复后用当前生产提示词完成 Office、Coding、Design 各两次真实 Wails Desktop 复验。

### 非目标

- 不修改系统提示词、Scene Profile、Agent/Controller/readiness、Desktop UI 或工具权限。
- 不为历史产物硬编码无限同义词，不把 grader 放宽为“任意异常/任意文件均通过”。
- 不重复 24 次基线/候选 A/B；当前没有新提示词候选。
- 不因真实模型的单次随机偏差扩大实现范围。

## 已确认方案

### RF-001

真实运行参数校验改为 `MaxSteps < 0` 非法；错误信息区分必须为正数的 repeats/timeout 和必须
非负的 max-steps。增加 0、负数和正数表驱动测试，并确认 manifest/config 保留 0。

### RF-002

冻结任务公开指定姓名 `Jordan Lee`、职位 `Product Designer`。SVG parser 记录 `viewBox`、
样式文本及用于交互/焦点判断的必要属性。grader 要求：公开内容、可访问标题/描述、三项指标、
主操作、可聚焦交互元素，以及包含非 `none` 的可见 focus stroke/outline/filter 样式。

### RF-005

隐藏验证仅把 `ValueError` 和 `TypeError` 视为合法拒绝；没有异常或其他异常均失败。任务提示
保持语义级要求，不追加隐藏实现偏好。

### RF-007

任务明确 HTML 必须引用 `dashboard-visual.svg`。SVG 画布有效性接受以下任一条件：正数
`width` 与 `height`，或四项数值且宽高为正的 `viewBox`。不再要求固定像素下限；其他内容、
可访问性、KPI、趋势和事件结构合同保持不变。

## 验收标准

| ID | 验收场景 | 结果 |
| --- | --- | --- |
| AC-01 | prompt-eval `max_steps=0` | 参数校验、临时配置和 boot 路径接受 0 |
| AC-02 | `max_steps<0` | 在任何模型调用前拒绝 |
| AC-03 | Profile Card 无 `role` 字面量但含公开职位 | 内容检查通过 |
| AC-04 | Profile Card `<style>` 中有真实 focus 样式 | 结构检查通过；仅写 focus 文案不通过 |
| AC-05 | 非法 retry 参数抛 `ValueError` 或 `TypeError` | 两者通过；无异常/其他异常失败 |
| AC-06 | Dashboard HTML 未引用 SVG | 失败且任务已公开该要求 |
| AC-07 | Dashboard SVG 使用正尺寸 `viewBox` | 通过画布合同；无尺寸信息失败 |
| AC-08 | RF-004 Unicode minus fixture | 现有测试继续通过 |
| AC-09 | 受影响 prompt-eval 测试与 PR Gate | 全部通过 |
| AC-10 | 三场景各两次真实 Wails Desktop | 记录完成态、grader、readiness retry、调用、token、耗时与审批 |

## 提交、验证与回滚

小步提交按可独立评审的意图组织：文档合同、RF-001、RF-002、RF-005、RF-007、最终回执。
每个修复提交同时包含对应测试。实现期间运行最近的 prompt-eval 测试；交付前运行：

```powershell
go test ./benchmarks/prompt-eval/... -count=1
go vet ./benchmarks/prompt-eval/...
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

真实 Desktop 使用用户本轮确认的 OpenAI-compatible 供应商和 `gpt-5.6-luna`。API Key 仅进入
忽略的临时环境/配置，不写 Git、文档、命令输出或报告。每个提交可独立 revert；没有数据迁移。

## 风险与停止条件

- 结构 grader 仍可能对合法 SVG 产生假阴性：用正反 fixture 锁定公开合同，不追求通用 SVG
  语义理解。
- 冻结任务文本变化会改变 manifest：更新 grader 版本/契约记录，修复前后报告不得混用。
- 真实模型运行可能暴露新的运行时问题：先记录证据；除非阻断本任务基础设施，否则不在本 PR
  修复。
- 若实现需要修改生产提示词、运行时、外部 API 或安全边界，停止并重新确认范围。
