# Desktop 三缺陷集成与稳定性复验计划

## 1. 问题与证据

`origin/main-v3@6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad` 上的 30 次真实
Desktop 三场景测试已经完成，并留下三个需要独立修复的运行时问题：

- DEF-002：完成步骤路径证据、todo 原子拒绝反馈和 readiness 重试后的最终答复存在合同摩擦；
- DEF-008：首次模型请求前，场景提交与 Desktop 元数据写入竞争 `.jsonl.meta`，并把本地文件错误误报为模型断流；
- DEF-009：final 已投影后，丢失或延迟的终态事件可使 Composer 暂时保留 Stop/loading。

三个修复分支已经分别完成实现和聚焦验证，但尚未在同一个构建中接受联合门禁和同口径 30 次
真实 Desktop 复验。原测试证据全部位于 Git 忽略目录，若删除 detached 测试 worktree，会失去
唯一的完整原始副本。

## 2. 基线与调查结论

| 对象 | 精确状态 |
| --- | --- |
| 修复前测试基线 | `6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad` |
| 集成基线 | `origin/main-v3@32f006922a5ae570b6d9fdcb2eb96c46282bf127` |
| 集成分支 | `codex/integration-def002-def008-def009` |
| DEF-002 | `codex/fix-def-002-completion-contract@bc7302f7500b43726f4230c41d924d93112adf32` |
| DEF-008 | `codex/fix-def008-scene-meta-race@bf731618b848f1730cec6058db9ec15964530778` |
| DEF-009 | `codex/fix-desktop-terminal-reconcile@bd0b515de21f1f3ebcf595e7f1dc85c0b8e4047d` |

Git 直接核验显示三个修复分支都以当前 `origin/main-v3` 为祖先，分别领先 11、4、3 个提交；
三者的相对基线改动文件没有重叠。没有文本重叠不等于没有语义耦合：它们依次覆盖模型请求前的
场景/元数据提交、Agent 执行中的 evidence/todo/readiness，以及 final 后的 Desktop 终态投影。

Codegraph 只索引主工作区 `D:\Reasonix`，调查时该工作区为 `70d919f4`，落后集成基线，因此只作
调用链导航。分支基线、改动文件、提交拓扑、核心源码和测试均已在各实际 worktree 中直接复核。
主要共享合同如下：

1. 场景提交成功后才允许 runner 发起模型请求，失败不得提前发布内存 Locked 状态；
2. 成功工具 receipt 进入 evidence ledger，`complete_step`、canonical todo 和 final readiness 顺序收口；
3. Controller 先把权威 RuntimeStatus 置为 idle，再发出 `TurnDone`；Desktop 可用 `ListTabs` 对账；
4. 三层共同决定用户看到的发送、审批、自动恢复、final 和 Composer idle 时间线。

## 3. 当前行为与期望行为

| 阶段 | 修复前证据 | 集成后期望 |
| --- | --- | --- |
| 场景锁定 | 1/30 在模型请求前因 `.jsonl.meta` 锁硬失败 | 同进程元数据更新串行化；失败不污染内存状态；本地 I/O 错误不冒充模型断流 |
| 完成合同 | 23/30 出现某种完成合同摩擦；L02 发生完整 readiness/弱 final 级联 | 等价路径 receipt 可匹配；拒绝明确保持旧状态；重试后的 accepted final 重述实质交付 |
| Desktop 终态 | A06 final 已显示后 26.672 秒才 idle | final 后以 backend 权威状态快速对账；丢事件时不等 30 秒 watchdog；新回合不被旧结果解锁 |
| 整体稳定性 | 质量 28/30，正常结束与实时 final 29/30 | 不预改判任何样本；严格按同一矩阵重新测量并单独报告质量、Agent 效率和 Desktop 稳定性 |

## 4. 目标与非目标

### 目标

- 在最新主线建立一个保留三分支完整提交历史和分支边界的集成分支；
- 将修复前 30 次测试的脱敏、可审计结论以文档形式纳入 Git；
- 将完整原始证据制作不可变归档并记录文件清单和 SHA-256，不让测试 worktree 成为唯一副本；
- 对合并结果运行联合自动化门禁和原 30 次真实 Desktop 矩阵；
- 使用同一冻结任务、统计脚本、持久化 dev 环境和判定规则，形成修复前/集成后的可比报告；
- 发现严重缺陷时先冻结现场和当次失败结果，再暂停启动后续样本。

### 非目标

- 不修改系统提示词、Scene Profile、Provider 配置或冻结任务来改善分数；
- 不在本任务新增或修改产品代码；集成内容只来自三个既有修复分支，新增缺陷另建记录并暂停讨论；
- 不重写、squash、amend 或 rebase 三个既有修复分支；
- 不把逐次 JSON/CSV、原始 JSONL、runtime profile、截图集、生成 workspace、统计脚本、浏览器临时输出
  或二进制直接提交到普通 Git；
- 不以切换会话、人工重跑或后续成功样本覆盖原始失败；
- PR 只在用户授权后创建或更新；未获另行授权不标记 Ready 或合并。

## 5. 证据保存方案

### 5.1 Git 中的长期文档化证据

在本计划目录下建立 `evidence/`，长期保留方法、阶段/最终报告、基线审计、缺陷记录、复现流程、
commit/tree/Desktop binary 身份、归档 SHA-256 和关键验证结果。

执行期逐次 JSON/CSV、observation、stats、grade、run-spec、截图与浏览器输出必须从矩阵 ID 派生并完成
解析、交叉核对和最终审计，禁止手工挑选成功样本。PR 整理后只保留其文档化结论；历史文档中的原始文件名
作为取证来源标识，不表示对应工件仍在仓库中。

### 5.2 Git 外的完整原始证据

完整保留当前约 1,202 个文件、42 MB 的 `.codex-run/desktop-stability-20260812-6cfe5af6`：

1. 原 detached 测试 worktree 在归档和校验完成前保持只读；
2. 在 `D:\Reasonix-evidence\` 创建完整压缩归档；
3. 归档前只做读取和敏感模式计数，不在输出中显示任何密钥或匹配内容；
4. 归档后重新解包或逐项校验文件清单，并把归档 SHA-256 写入长期 Markdown 记录；
5. 原 worktree 与归档构成两份副本；删除任一副本必须另行确认。

集成后的原始运行证据使用新的目录和身份记录，绝不覆盖 `before` 证据。

## 6. 集成方案与提交历史

批准实施后按以下顺序执行：

1. `docs:` 提交修复前证据记录和归档身份；
2. 使用 `--no-ff` 合并 DEF-008，保留其原始提交；
3. 使用 `--no-ff` 合并 DEF-002，保留其原始提交；
4. 使用 `--no-ff` 合并 DEF-009，保留其原始提交；
5. 审查每个 merge commit、联合 diff、EOL 和任务回执；
6. 若只在集成态暴露缺陷，先建立独立缺陷记录、冻结证据并暂停，不在本任务修复或回写既有分支。

导入分支形成于本次更新后的提交纪律之前；为保留用户要求的真实历史，不追溯重写其已有提交。

Draft PR 整理阶段，用户进一步要求证据只保留文档。为避免“先提交原始工件、再删除”仍把二进制和 JSON
带入仓库历史，PR 集成分支从最新 `main-v3` 重建：三个来源修复 head 及其原始提交哈希保持不变，只重新
生成集成 merge 与文档证据提交。重建产品代码必须与同步后真实 Desktop 受测提交逐路径、逐 blob 一致。

## 7. 自动化验证

合并后先执行无 Provider 验证，失败则不启动真实 30 次：

- 三分支各自列出的聚焦测试；
- `go test ./...`：联合改动超过 4 个 Root Go package，满足扩大条件；
- 在 `desktop/` 执行 `go test .`；
- DEF-009 前端终态对账、meta、cancel reconcile 聚焦测试；
- 在 `desktop/frontend/` 执行 `pnpm build`；
- `go run ./scripts/code_guard`、baseline 保护命令和 `git diff --check`；
- `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。

最终 PR gate 在任何未来 PR 授权前，还需同步当时最新 `main-v3` 后重跑；本阶段通过不构成 PR 授权。

## 8. 真实 Desktop 复验

### 8.1 固定条件

- 使用集成提交构建的真实 Wails Desktop 和顶层任务亲自执行的 Computer Use；
- 使用原持久化 dev 环境，同一 Provider URL、账号和 `gpt-5.6-luna`；
- 每个 run 使用全新 workspace/project/session；
- 用户已在 2026-08-13、本任务尚未发起任何真实模型请求时确认 Provider URL
  `http://192.168.10.231:8080/v1`、API Key 和模型 `gpt-5.6-luna`；该确认覆盖本次矩阵，除非用户
  主动变更参数，否则不再重复索取；密钥不写入文档、命令或日志，也不从配置或日志反向读取；
- low 24 次与 auto 6 次完全复用原冻结任务和顺序，不增加额外真实模型样本。

### 8.2 阶段

1. 三场景各 1 次校准，直接计入 30 次；
2. 完成前两次重复形成严格可比 12 次，提交阶段汇报；
3. 通过统计链路检查后完成 low 第 3、4 次和 auto 6 次；
4. 生成全部 30 次的质量、成本、方差和完成态报告。

### 8.3 严重缺陷处置

发现确定性严重缺陷时：

1. 当次样本按原位置记为失败，不重跑覆盖；
2. 保持当前 Desktop、会话和窗口现场；
3. 固定截图、时间线、session/meta/telemetry、日志、进程、workspace、commit 和 binary hash；
4. 完成脱敏和证据完整性检查；
5. 暂停启动后续样本并向用户汇报；
6. 另行复现只能记录为矩阵外样本。

## 9. 验收标准

- 三修复分支的所有原始提交可从集成分支追溯，三次 merge 边界明确；
- 修复前原始证据在执行期与完整归档清单核对一致，长期 Markdown 记录不含 API Key；
- 联合自动化门禁全部通过；
- 30 个有效样本在执行期均有唯一 run ID、完整 observation/stats/grade/run-spec 和 Desktop 终态证据，
  聚合结果与关键异常已转录到长期文档；
- 严格 12 次继续分别报告质量、正常结束、实时 final 和成本，不用总分掩盖差异；
- DEF-008 不再出现同进程元数据竞争导致的模型前硬失败或错误分类；
- DEF-002 的等价路径匹配、原子拒绝反馈和完整 final 恢复符合确定性测试；真实拒绝仍按事实记录；
- DEF-009 在 backend 已 idle 时不保留长尾 Stop/loading，记录 final 落盘、投影和 idle 的实际分布；
- 没有切换会话后改判实时通过、重复回复、额外文件或未记录副作用；
- 最终报告同时给出修复前、单分支验证和三修复集成后的边界，不把一次成功宣称为概率归零。

## 10. 风险与回滚

| 风险 | 控制 |
| --- | --- |
| 三个独立修复语义耦合但文件不冲突 | 按生命周期顺序合并，联合测试后才发真实请求 |
| 持久化 dev 环境带入跨会话状态 | 这是稳定性目标的一部分；每次隔离 workspace/session，并记录环境 ID |
| Provider/模型波动污染归因 | 固定账号、URL、模型、effort 和任务；单列 D 类波动，不用重跑覆盖 |
| 30 次期间主线继续前进 | 测试开始后锁定集成 commit；不在矩阵中途变基线 |
| 原始证据含敏感或机器特定数据 | 原始归档留在 Git 外；Git 只纳入脱敏 Markdown 记录 |
| 高成本运行中途发现严重问题 | 先固定现场，暂停后续样本，保留已花费样本 |

集成分支已发布为 Draft PR；合并前可关闭 PR 并删除远端集成分支，三个来源分支不改写。合并后按
merge commit 单独 revert。Git 外证据归档不属于代码回滚范围。
