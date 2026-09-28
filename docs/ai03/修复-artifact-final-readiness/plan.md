# Artifact final-readiness 误判修复计划

## 1. 问题与证据

真实 Ask、Auto、Yolo 会话都出现同一失败：`write_file` 成功创建目标文件，磁盘内容正确，
assistant 也给出完成文本，但宿主把 final 标记为 `readiness_rejected_candidate`。成功调用
`verify_artifact` 后仍缺少匿名 `artifact file`，最终重复阻断。

根因位于确定性 artifact intent parser：中文动作正则会在“写文件工具”中命中单字“写”，
其后“文件”生成一个匿名 target；同一句“创建 marker.txt”又生成命名 target。evidence
matcher 正确坚持每个 target 必须由不同 canonical path 满足，因此一个真实 marker 文件无法
同时满足两个错误合同目标。

这不是模型、写权限、文件副作用、验证工具、路径规范化或 final 文本失败。Auto、Yolo 原始
提示没有“不要调用其他工具”仍然失败，也证明 Ask 的工具限制冲突不是共同根因。

## 2. 基线与调用链

| 项目 | 值 |
| --- | --- |
| 远端基线 | `origin/main-v3@70d919f4c50ca81f18952bc90c3817e9d9d78af8` |
| 实现 worktree | `D:\Reasonix-worktrees\fix-artifact-final-readiness` |
| 分支 | `codex/fix-artifact-final-readiness` |
| Codegraph 工作区 | `D:\Reasonix` |
| Codegraph 索引 | clean 主工作区；1960 files、46984 nodes、175716 edges、pending 0 |

Codegraph 数据库不提供 Git commit 字段；索引时主工作区 clean，HEAD 与 `origin/main-v3`
均为上述 commit，故该 commit 是可证明的导航基线。独立 worktree 不初始化 Codegraph。

主要链路：

`Agent.Run` → `artifactRequirementFromInput` → `artifactTargetsFromActions` →
`ArtifactRequirement.Targets` → 写工具 receipt → 宿主 artifact verifier →
`FreshSuccessfulArtifactVerificationMatch` → `artifactReadinessFailure` → final。

## 3. 已确认产品合同

- 用户要求一次写入时，Agent 只发起一次用户可见的写工具调用。
- `verify_artifact` 下沉为宿主内部完整性校验，不是 Agent 主动工具调用，不计入用户限定的
  工具次数，也不需要模型为了 readiness 再发起第二次工具调用。
- 宿主验证仍必须晚于最新可能 workspace mutation；后台任务未结束时不得放行。
- 只有高置信度 artifact requirement 可以成为拒绝 final 的硬合同。
- 不为某一句提示增加白名单，不通过不断扩张正则穷举不可预测表达。

## 4. 目标与非目标

### 目标

- “调用一次写文件工具，创建 marker.txt”只产生 `marker.txt` 一个 target。
- 工具、命令、API、函数等元语言中的动作词不产生匿名 artifact。
- 明确的路径、格式、数量、文件列表继续成为硬合同。
- 真正要求两个文件时继续要求两个 distinct paths。
- final 边界由宿主验证可确定的候选路径，不要求模型调用 verifier。
- 验证失败仍给出可诊断的 missing targets/candidates，并保持 bounded retry。

### 非目标

- 不取消 artifact readiness 或 distinct-path matcher。
- 不让一个物理文件满足两个真实目标。
- 不把所有自然语言语义交给另一个 LLM 作为硬门禁。
- 不特殊兼容“不要调用任何其他工具”这一个字符串。
- 不改变文件写工具权限、workspace confinement、存储 schema 或外部 API。

### 同 PR 基线卫生边界

主线已在 `08e91321` 明确删除历史文档 `docs/todos/02/任务树/验收.md`，但仍残留一个只读取
该文档并检查文本片段的测试，导致 clean `origin/main-v3` 的 Go 全量测试失败。用户授权在当前
worktree、同一 PR 中增加独立修复切片：删除这项失去权威输入的文档门禁测试，不恢复过时
文档，不修改 tasktree 运行时代码，也不改变本主题的产品合同与实际应用验收结论。

## 5. 选定方案

### 5.1 高置信度动作资格

保留确定性 parser，但收缩其硬断言范围。动作候选在生成 target 前必须通过局部语境资格判断：

- `调用/使用/通过 + 写文件/生成图片 + 工具/命令/API/接口/函数` 是元语言；
- `写/创建/生成 + 一个文件/报告/图片` 是交付动作；
- `创建 marker.txt` 等明确路径始终是高置信度命名目标；
- 无法可靠判定的模糊表达不得凭空生成匿名硬目标。

判断基于动作所在局部分句和有限结构，而不是完整 prompt 的任意关键词。不能在事后无条件
删除“匿名 target + 命名 target”，因为用户可能确实要求两个交付物。

### 5.2 宿主内部验证

Agent 到达无工具调用的 final candidate 后，宿主在 readiness 判定前执行验证：

1. 若有未结束后台工作，保持阻断，不验证。
2. 收集当前 requirement 的明确路径，以及本回合成功 writer receipts 中的实际路径。
3. 对候选逐个执行 workspace-confined regular-file 验证；不存在的备选路径只使该候选失败，
   不阻断其他候选被记录。
4. 成功结果写入 host evidence ledger，但不写入 assistant ToolCalls，不产生模型工具回合。
5. 复用现有 freshness、source exclusion 和 distinct matcher 决定是否 ready。

模型工具 registry 不再默认暴露 `verify_artifact`；boot 为 Agent 单独注入 workspace-bound
host verifier。内部验证绕过用户工具审批和模型 hooks，因为它只读、由宿主安全合同触发；
文件 confinement 和 canonical path 检查保持不变。

### 5.3 兼容回退

- 对明确命名目标，即使用 bash 写入，宿主也能根据 requirement 路径验证。
- 对匿名目标，优先使用结构化 writer receipt 路径。
- 若匿名输出只通过无法提取路径的 shell 副作用产生，宿主不得猜路径；readiness 继续阻断并
  说明缺少可验证候选。后续可独立设计结构化 artifact handoff，不在本修复中扫描整个工作区。
- `verify_artifact` builtin 保留为内部实现单元，避免复制路径安全逻辑。

## 6. 验收标准

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | `调用一次写文件工具，创建 marker.txt` | 只有命名 target，无匿名 target |
| AC-02 | `使用图片生成 API 创建 cover.png` | 只有 `cover.png` |
| AC-03 | `写一个文件` | 保留一个匿名 target |
| AC-04 | `写一个说明文件，再创建 marker.txt` | 两个 targets，要求两个 paths |
| AC-05 | Agent 写入 marker 后回复 `done` | 宿主内部验证后一次通过，无 verifier 工具回合 |
| AC-06 | 文件缺失、目录或越界路径 | 内部验证失败，final 继续阻断 |
| AC-07 | 验证后又发生 mutation | 旧 receipt 失效，宿主重新验证 |
| AC-08 | 后台工作未结束 | 不提前验证或放行；settle 后重新验证 |
| AC-09 | 普通/Token Economy 工具面 | 模型工具列表不含 `verify_artifact` |
| AC-10 | Ask/Auto/Yolo 真实最小提示 | 文件正确、final 正常，无 readiness retry |

## 7. 自动化与实际应用验收

迭代期先运行 parser、readiness、boot 和 verifier 聚焦测试。实现稳定后至少运行：

```powershell
go test ./internal/agent ./internal/evidence ./internal/tool/builtin ./internal/boot -count=1
go run ./scripts/code_guard
git diff --check
```

实际应用验收使用最新构建和真实 gpt-5.6-luna 会话，分别执行 Ask、Auto、Yolo：

```text
请只调用一次写文件工具，在当前工作区创建 marker.txt，内容严格为 ok；
不要调用其他工具。完成后仅回复 done。
```

每种姿态使用独立 marker 名，观察：

- 会话只有一次模型写工具调用，没有模型 `verify_artifact` 调用；
- 文件存在且内容严格为 `ok`；
- ledger 有最新的宿主验证 receipt；
- assistant `done` 不是 `readiness_rejected_candidate`；
- UI 不出现“最终回复尚未满足完成条件”。

运行前可关闭已有 Reasonix 应用进程解除单例锁。只关闭可确认属于本应用的进程；构建、启动、
日志位置和恢复方式在验收回执中记录。真实行为稳定后同步最新基线，重跑关键场景和最终 PR Gate。

## 8. 风险、兼容性与回滚

| 风险 | 控制 |
| --- | --- |
| 元语言过滤过宽导致真实交付漏检 | 正负成对测试，要求工具名结构而非孤立关键词 |
| 删除 verifier 工具暴露影响提示和专家流程 | boot、token profile、专家 handoff 测试同步迁移到 host verifier |
| 内部验证发生得过早 | 仅在无模型工具调用的 final 边界执行，并保留 background/freshness 门禁 |
| 批量备选中一个不存在拖累全部 | 每个候选独立验证和记录 |
| 匿名 shell 输出不可定位 | fail closed，不扫描或猜测 workspace 文件 |
| host receipt 缺少用户可见审计 | readiness audit 记录结果；测试断言 session 不伪造 ToolCalls |

没有数据迁移。代码可按小步提交反向撤销：先回滚 host verifier，再回滚 parser；旧 session
数据不受影响。

## 9. 交付方式

用户明确授权计划、任务、代码和验收回执共用一个 PR，并要求小步提交。建议提交序列：

1. `docs:制定artifact final-readiness修复计划`
2. `fix:避免工具元语言生成伪产物目标`
3. `fix:将产物验证下沉至宿主readiness`
4. `fix:移除过时任务树文档门禁`
5. 仅在后续验收发现缺陷时增加独立 `fix:` 提交

创建 PR 前同步最新项目基线、运行仓库要求的完整 Gate，并把真实应用验收结果写入 PR 描述。
