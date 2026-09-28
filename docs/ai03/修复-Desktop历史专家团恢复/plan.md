# Desktop 历史专家团恢复修复计划

## 1. 文档定位

[既有 requirements](../修复-Desktop普通与专家团子代理隔离恢复/requirements.md) 是本修复唯一的产品需求权威。
本文只记录“历史专家团子专家消失”缺陷的证据、修复边界和实现方案，不改变既有产品模型。

已确认的兼容边界：本轮不迁移或重建已经缺失的旧 `.expert.json` sidecar，不扫描历史消息、普通
`task`、Task Tree 或 child 文本推断团队身份。缺失 sidecar 的旧会话继续按普通 transcript 安全降级。

## 2. 问题与证据

### 2.1 用户可见故障

在未修复基线 `9ba8b0f962607025335683065187b062eec8bb49` 的 `wails dev` 中，重新打开旧父会话
“帮我写一个贪吃蛇”后，父 transcript 的待办仍为 2/2，但“Bug 排查 / 代码重构 / 自动化测试”成员
页签消失。

该父 Session JSONL、两个 `.jobs/task-*.json`、两个 child JSONL 和对应 meta 均存在，且 job、tool call、
`parentSession`、`ParentToolCallID`所有权一致；父 Session 对应的 `.expert.json` 缺失。调查只读检查了这些
用户数据，没有把修改用户数据作为诊断或恢复手段。

### 2.2 已证明的代码路径

- `internal/control/controller.go`恢复 `team_archived`时会保留 `expertMeta`，同时把当前选择恢复为 none；
  这是“历史团队仍在、当前父 Agent 恢复普通身份”的正确后端语义。
- `desktop/subagent_history.go`的 `ExpertTeamForTab`从父 expert sidecar构造团队视图，再以父所有权过滤
  child artifact；sidecar是历史团队与成员入口的权威。
- `desktop/frontend/src/components/Transcript.tsx`多处以`activeExpert?.mode === "team"`及当前专家 ID
  相等作为团队加载、恢复和呈现门禁。`CurrentExpertForTab`在 archived 会话返回 none 后，完整 sidecar
  也无法驱动历史团队工作区。
- `internal/agent/save.go`的`SaveSessionExpert`把空`ExpertID`解释为删除 sidecar；
  `internal/control/selected_expert_refresh.go`的普通快照保存可能把“当前未选择专家”写成空 meta。
  因而一旦内存历史 meta 未恢复，后续自动保存会删除或持续维持 sidecar 缺失。
- 提交`00aa1c29`把 missing-sidecar 前端行为固定为普通 transcript fallback，并用测试明确断言不显示
  `AgentWorkspace`。该提交没有删除 sidecar，但暴露了当前“当前专家”和“历史团队”仍被耦合。

Codegraph 使用索引主工作区的 `main-v3` 基线 `9ba8b0f962607025335683065187b062eec8bb49`核对了
上述符号、调用者、数据流和消费者；当前独立工作树位于
`3d777f618a93bd224de4bada1170e0d2a1970048`，相关文件与索引基线之间没有本缺陷相关改动，并已用
直接源码、Git history/blame 和真实数据交叉验证。

### 2.3 根因边界

已证明两个可独立修复的缺陷：

1. 前端把当前 Composer 专家选择错误地当作历史团队存在性的门禁；
2. sidecar 保存 API 把空当前选择隐式解释为删除历史元数据，缺少 active/archived 不得被普通保存
   降级的持久化不变量。

现场 sidecar“第一次未写入或被删除”的具体触发操作无法从现存文件、日志和时间戳证明。运行中的应用
会原子重写父 JSONL/meta，现场也没有 sidecar 审计日志，因此本文不猜测初始丢失原因。修复目标是封闭
已证明的删除机制和状态耦合，不把未知原因包装成结论。

## 3. 目标与非目标

### 3.1 目标

- `team_active`和`team_archived`的团队标识、成员页签、成员绑定和只读 child 历史跨切换与重启保留。
- 退出专家团后，当前父 Agent 为普通身份，但历史团队工作区继续显示。
- 普通自动保存不得删除或覆盖已有 active/archived sidecar。
- active/archived 原会话不得再次选择单专家或专家团；用户必须新建会话。
- 普通 `task`不得被提升为专家团成员；所有 child 查询继续验证父所有权。
- 缺失或损坏 sidecar 时失败关闭，不从非权威数据猜测或重建团队。

### 3.2 非目标

- 恢复现场已经缺失 sidecar 的“贪吃蛇”会话或任何旧会话。
- 从 host prompt、模型文本、task label、job、child meta 或相邻会话合成 sidecar。
- 改变专家目录、task Runtime、SubagentStore、Task Tree、child 历史格式或 Provider 协议。
- 修改“子任务产物投影到主专家最终回复”的独立缺陷。
- 修改用户 AppData 中的 Session、job、child 或 sidecar 文件。

## 4. 选定方案

### 4.1 分离当前身份和历史团队

维持现有两个权威读取面，不引入新 Store：

```text
CurrentExpertForTab(tabID)
  -> 当前 Composer / 当前回复身份

ExpertTeamForTab(tabID)
  -> 当前父 Session 的历史团队工作区
  -> team_active | team_archived + member snapshot + owned child refs
```

状态合同如下：

| 会话 | CurrentExpert | sidecar TeamState | 历史团队工作区 |
| --- | --- | --- | --- |
| normal | none或单专家 | 无团队状态 | 不显示 |
| team_active | 主专家 | `team_active` | 显示 |
| team_archived | none | `team_archived` | 永久显示且只读 |

Frontend 不得用 `activeExpert.mode`或当前专家 ID 过滤`ExpertTeamForTab`返回的 session-level 团队。
Composer、当前头像和当前回复语义仍只消费`CurrentExpert`，不得把历史团队反向写成当前专家。

### 4.2 建立 sidecar 生命周期不变量

把“保存状态”和“删除状态”从 API 与调用语义上分开：

- `SaveSessionExpert`只保存有效 meta，不再以空`ExpertID`隐式删除文件；空状态由调用者按场景处理。
- 没有当前专家的普通自动保存是 no-op；若内存或磁盘已有`team_active/team_archived`，必须保留该历史
  meta，不允许写成 normal 或删除。
- `ClearExpert`对 active 团队执行显式原子转换`team_active -> team_archived`，保存成功后再清除当前
  选择；单专家清除和整个 Session 删除使用各自明确的清理入口。
- 已存在但无法解析的 sidecar 不得被普通自动保存覆盖或删除；加载/保存错误保留现有文件并向调用链
  返回或记录可诊断错误。
- 继续使用同目录临时文件加 rename 的原子写入方式，不新增数据库或双写格式。

实现前必须审计`SaveSessionExpert`和 sidecar 删除的全部调用者，确保显式删除只发生在既有单专家清除、
Session/fork/trash 生命周期所要求的路径。不得通过全局“永不删除”破坏这些既有行为。

### 4.3 以会话团队视图驱动历史 UI

调整`Transcript`中的团队恢复状态机：

- 只要有`tabId`和持久消息，就按 tab/scope 加载`ExpertTeamForTab`，不等待当前专家为 team。
- 加载成功的 active/archived 团队独立驱动成员条、`AgentWorkspace`、父工具恢复和只读成员导航。
- 当前专家变化只刷新 Composer 身份和必要的激活过渡，不清除同 tab 已加载的历史团队。
- tab、rewind、history revalidation 仍使请求按 scope 失效，保留现有竞态与虚拟化保护。
- bridge 返回 null 时沿用 `00aa1c29` 的普通 transcript fallback；返回错误时保留明确失败路径，不伪造
  工作区。

`ExpertTeamForTab`继续只从有效 sidecar读取成员快照，并只暴露通过 parent ownership 验证的 child ref。
本修复不把普通 task item 或 child artifact 本身升级为成员来源。

### 4.4 选择锁定

active/archived 的专家选择锁定继续以后端 session expert meta 为权威，不能因`CurrentExpert=none`而把
archived误判为 normal。Frontend 只展示后端裁决和“请新建会话”提示，不另建可漂移的锁定状态。

## 5. 影响范围

预计实现文件：

- `internal/agent/save.go`及直接 sidecar 测试；
- `internal/control/selected_expert_refresh.go`、`controller.go`及专家恢复/清除/自动保存测试；
- `desktop/frontend/src/components/Transcript.tsx`及专家工作区、跨层 App/Transcript 测试；
- 必要时最小调整 Desktop bridge 测试，但不改变`ExpertTeamForTab`的权威来源与所有权合同。

不修改`D:\Reasonix-worktrees\fix-expert-artifact-handoff`和
`D:\Reasonix-worktrees\validate-artifact-baseline`，所有改动只发生在本独立工作树。

## 6. 验收场景

| ID | 场景 | 必须结果 |
| --- | --- | --- |
| H-01 | 新会话启用专家团后切换并重载 | sidecar保持`team_active`，团队和成员页签恢复 |
| H-02 | active会话退出专家团 | sidecar原子转为`team_archived`，CurrentExpert为none |
| H-03 | archived会话多次自动保存并重启 | sidecar仍为archived，团队和成员页签恢复 |
| H-04 | archived成员已有owned child | 点击成员可打开连续只读历史并返回父会话 |
| H-05 | active/archived再次选择专家 | 后端拒绝并要求新建会话 |
| H-06 | archived后创建普通task | 不写入旧成员绑定，不出现新成员页签 |
| H-07 | 普通task含相似名称或child | 不显示专家团队工作区 |
| H-08 | sidecar缺失 | 普通 transcript fallback，不扫描或重建旧数据 |
| H-09 | sidecar损坏 | 不覆盖原文件、不猜测团队，错误可诊断 |
| H-10 | tab快速切换、rewind或迟到加载 | 不把其他tab/代际的团队投影到当前会话 |

## 7. 验证计划

### 7.1 自动化

- Root/Control Go：active/archived保存加载、空快照不删除、退出原子归档、单专家显式清除、损坏文件
  不覆盖、重启恢复、选择锁定。
- Desktop Go：active/archived sidecar视图、owned child可读、跨父拒绝、普通/缺失 sidecar无团队视图。
- Frontend：`CurrentExpertForTab=none`且`ExpertTeamForTab=team_archived`的 App/Transcript 重载链路；
  active重载；成员历史打开；普通task无页签；missing-sidecar fallback；tab/代际竞态。
- 运行受影响 Go package、聚焦 frontend 测试、frontend build、`go run ./scripts/code_guard`和
  `git diff --check`；最终按实际改动扩大到相关模块检查。

### 7.2 真实应用验收

最终验证使用当前分支构建的真实`wails dev`，不得用旧任务或发布版替代：

1. 新建真实专家团任务，全程由主专家交互并至少派发一个子专家；
2. 完成后切换会话再返回，随后重启应用，验证 active 团队、成员页签和只读 child 历史；
3. 退出专家团，验证父 Agent恢复普通身份且原会话不能重新选专家；
4. 再次重启，验证 archived 团队标识、成员页签和只读历史仍在；
5. 在 archived会话继续普通聊天并产生普通task时，验证其不进入旧成员页签。

主 Agent独立执行该应用验收，不以实现代理的测试报告代替。

## 8. 风险、回滚与未决调查

- sidecar API被单专家、团队、fork/rewind和删除流程共用；必须先审计调用者并用显式删除测试防止行为
  回归。
- Transcript有异步重试、虚拟化和语义恢复状态；修改只替换权威门禁，不重写滚动或历史投影架构。
- 已缺失 sidecar的会话无法区分 normal 与丢失团队；按已确认边界失败关闭，不以不可信证据恢复。
- 若实现发现现有删除调用无法在不改变产品语义的情况下分离，或发现现场初始丢失来自新的数据/安全
  边界，必须暂停实现并重新确认，而不是扩大迁移或推断范围。

回滚 frontend提交会恢复旧显示行为；回滚 backend提交会恢复旧保存风险。回滚不得删除已经正确保存的
active/archived sidecar或任何 child历史。

## 9. 交付与提交切片

1. Backend sidecar生命周期不变量及聚焦测试，一个`fix:`实现提交；通过后单独提交任务回执。
2. Frontend历史团队解耦及跨层回归，一个`fix:`实现提交；通过后单独提交任务回执。
3. 主 Agent审查、模块回归和真实应用验收；若发现缺陷，返回同一实现代理创建新的`fix:`提交，再更新
   对应任务回执。

本计划和任务合同先分别以`docs:`提交。它们不授权业务代码实现，不推送，也不创建 PR。
