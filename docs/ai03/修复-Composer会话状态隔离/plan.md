# Composer 会话状态隔离修复计划

## 背景与问题

已有历史会话 A 使用 DeepSeek，新会话 B 改用 GPT 后，重新打开 A 会显示 GPT。
这不是正常的“新会话默认值继承”，而是历史会话的 Composer 模型路由被当前
全局默认值覆盖。模型与计费来源共同决定实际调用通道，错误恢复还可能使
订阅积分与 BYOK 资产串线。

实现后的实际应用压力验收又发现一个合入阻断问题：在短会话与工具密集的
长会话之间反复切换时，目标正文会先于标题和 Composer 身份出现，且 WebView2
渲染进程可报 `STATUS_BREAKPOINT`。短会话 12 轮切换稳定，长会话约 8 轮即可
复现；单次长会话往返后渲染进程工作集增长约 28 MB。

## 已确认的产品契约

1. Composer 的模型、计费来源和推理强度属于会话。
2. 会话内切换只修改当前会话，不修改其他会话或全局默认值。
3. 全局默认值只由设置页管理，仅初始化之后创建的新会话。
4. 历史执行事实不被后续 Composer 选择改写。
5. 分支/Fork 创建时复制父会话当时的模型、计费来源和推理强度，之后彼此独立。
6. 旧会话缺少计费来源时，仅在规范化模型引用能唯一确定通道时推断并回填。
7. 旧会话缺少推理强度时使用 `auto`，不继承当前活动会话。
8. 模型下架、订阅失效、BYOK 凭据缺失及服务不可用继续复用现有 fallback、
   登录、配置与重试交互，本修复不新增异常 UX。

## 当前行为与根因证据

- `desktop/app.go` 的 `SetModelRouteForTab -> setModelRouteForTab ->
  setModelRouteForTabWithPersistence(..., true)` 会在会话切换成功后调用
  `persistUserDefaultModelSelection`，把 Composer 选择写入用户全局配置。
- `desktop/tabs.go` 的 `desktopTabEntry` 已持久化 `Model`、`BillingSource`和
  `Effort`，所以未关闭的标签页具备正确数据基础。
- `internal/agent/branch.go` 的 `BranchMeta` 只保存 `Model`，已关闭并从历史树
  重新打开的会话丢失 `BillingSource` 与 `Effort`。
- `desktop/tabs.go` 打开历史 Topic 时先取当前全局默认计费来源，只在已存
  模型与该全局来源相容时恢复模型，因此会话 A 会被会话 B 改写的全局
  来源间接覆盖。
- `desktop/tabs.go` 的新空白会话虽已使用设置页模型默认值，但推理强度
  仍从活动会话复制。
- Desktop `Fork` 已复制模型与强度，但漏掉 `BillingSource`，新分支 sidecar
  也没有完整 Composer 状态。
- `desktop/frontend/src/features/session/useController.ts` 在 `ActivateTopic` 返回前
  直接把目标快照作为活动渲染状态，而 `App.tsx` 的标题和模型路由仍读取旧
  `activeTab`，形成新正文、旧标题/Composer 的非原子中间帧。
- `desktop/frontend/src/components/ToolCard.tsx` 虽将已完成工具卡折叠为零高度，
  仍会解析完整参数/输出，并挂载隐藏的 `CodeViewer`、`DiffView` 和嵌套工具 DOM。
  `Transcript` 的热区只按用户轮数计数，少轮次但工具输出很多的会话会一次性
  重建全部隐藏详情。这与真实应用中只有工具密集长会话稳定复现崩溃相符。
- `SessionViewSnapshotCache` 最多保留 8 个浅拷贝快照，数据缓存本身有界；本次
  不把尚未被堆快照证明的内存泄漏当作根因。

### 引入历史

- `b7786ce6` 建立了“会话内 `/model` 同时写用户默认配置”的旧语义。
- `8f59fe11` 引入新会话继承活动会话设置的历史行为。
- `8e5c6ce6` 在 2026-07-16 将当前默认计费来源接入历史 Topic 打开路径，
  并在存在默认来源时跳过已存模型，是本次历史会话覆盖的主要回归点。
- `395fb274` 在 2026-07-18 引入路由感知和 `topicModelMatchesBillingSource`，但
  没有为历史会话持久化完整 `(model, billingSource, effort)`，因此只部分缓解。

## 基线与调查方法

- 实施基线：`origin/main-v3@5c9a4ffcf3060c67c83b86454e35696431b116c3`。
- Codegraph 仅在主工作区 `D:\Reasonix` 的该基线上用于追踪调用路径与影响面。
- 独立工作树使用实际源码、Git diff、编译和测试复核所有重要结论。
- 基线 `go run ./scripts/code_guard` 通过；目标 Go 文件均为 LF。

## 目标

- 建立可持久化的会话 Composer 状态：模型、计费来源、推理强度。
- 打开、切换、关闭后重开及应用重启都恢复该会话自身状态。
- Composer 切换不再修改设置页管理的全局新会话默认值。
- 旧会话按已批准的唯一推断/`auto` 策略兼容，并在下次正常持久化时回填。
- Fork/分支创建时继承完整 Composer 状态，后续独立。
- 会话切换不出现可操作的旧 Composer 与目标正文混合状态。
- 已折叠工具卡不挂载不可见的重型详情，长会话反复切换不再触发 WebView2
  渲染进程崩溃，并控制稳定状态下的 DOM/工作集增长。

## 非目标

- 不修改草稿、附件、Token 模式、工具审批、MCP 或场景的归属。
- 不改变已经执行的历史消息、云端账本、任务计费记录或模型回复。
- 不新增模型不可用 UI，不改变现有同路线 fallback 和错误处理。
- 不改变设置页 `SetDefaultModelRoute` 以及自动化任务的显式路由契约。
- 不引入新依赖、网络请求或破坏性数据迁移。
- 不删除、截断或不可逆折叠历史内容，不改变工具卡的展开结果。
- 不以本次修复重写 Transcript 为完整虚拟列表；若惰性挂载仍不能稳定通过
  实际压力验收，再单独评估虚拟化及其滚动锚点风险。

## 实现方案

1. 在 `agent.BranchMeta` 增加可选 `billing_source` 和 `effort`，与已有 `model`
   共同成为会话 sidecar 的 Composer 快照。JSON 新字段对旧版读取向后兼容。
2. Desktop 保存 session path/快照时原子更新三个字段；打开历史会话和
   恢复持久化 Tab 时优先使用 sidecar，再使用旧 Topic 模型推断，最后只为真正
   新会话使用全局默认。
3. 对缺少 `billing_source` 的规范 `provider/model` 使用当前已有路由规则唯一
   推断；缺少 `effort` 的旧历史会话显式设为 `auto`。
4. 调整 Tab/Composer 模型切换为仅持久化当前会话；保留设置页全局默认
   接口。
5. 新空白会话不再复制活动会话 `effort`；使用该模型的全局配置/`auto`
   初始值。Token/MCP 等既有连续性不在本次范围。
6. Desktop Fork 在新 Tab 和新 sidecar 中复制完整状态；Controller 分支保留父
   sidecar 中的同一快照，避免其他分支入口丢字段。
7. 工具卡保留轻量标题与摘要；完成态折叠时不创建参数、输出、diff、代码查看器
   或嵌套工具详情，首次展开时再挂载，关闭动画完成后释放详情 DOM。
8. 会话 surface transition 期间禁止 Composer 交互，并使标题/正文/Composer
   使用一致的呈现阶段；目标会话准备好后只提交一次重型 Transcript 挂载。
9. 为折叠详情惰性挂载和切换阶段一致性补充前端聚焦回归测试，再以真实
   WebView2 重复执行长/短会话压力场景。

## 影响范围

预计修改：

- `internal/agent/branch.go` 及相邻测试：sidecar schema 与向后兼容。
- `internal/control/controller.go` 或相邻分支帮助代码：分支快照继承。
- `desktop/app.go`、`desktop/tabs.go`：Tab 切换语义、恢复优先级、新会话与 Fork。
- Desktop/agent/control 聚焦测试。
- 本目录的计划、任务和进度文档。

前端已将模型与强度请求按 `activeTabId` 发往 Tab API。原计划“不需要前端
交互改动”的判断已被压力验收推翻：为修复渲染崩溃与过渡混显，本次追加
`ToolCard`、Transcript/surface transition 及相邻前端测试的受控修改。

## 验收标准

| ID | 场景 | 期望 |
| --- | --- | --- |
| AC-01 | A 会话 DeepSeek，B 会话 GPT | 来回切换时各自显示自己的模型 |
| AC-02 | 在 B 的 Composer 切换模型 | A 的模型、计费来源和强度均不变 |
| AC-03 | 关闭 A 后从历史树重开 | 恢复 A 的完整 Composer 状态 |
| AC-04 | 应用重启 | 所有持久化 Tab 保持各自状态 |
| AC-05 | A=`high`，B=`low/auto` | 切换、关闭重开、重启后强度不串 |
| AC-06 | Composer 切模型/强度 | 设置页全局默认配置不变 |
| AC-07 | 设置页修改默认路由后新建会话 | 新会话使用新默认，已有会话不变 |
| AC-08 | 旧 sidecar 只有规范模型 | 唯一推断通道，强度为 `auto`，不采用当前全局路由 |
| AC-09 | 从 A Fork/创建分支 | 新会话初始继承 A；任一方后续切换不影响另一方 |
| AC-10 | 模型/凭据/订阅不可用 | 复用现有 fallback 和 SendAvailability 行为 |
| AC-11 | 订阅会话与 BYOK 会话并存 | 恢复、发送、计费和凭据加载不串线 |
| AC-12 | 正在运行的任务 | 仍禁止中途切换模型路由或强度 |
| AC-13 | 切换尚未完成 | 不可操作旧会话 Composer，不出现目标正文配旧标题的混合帧 |
| AC-14 | 已完成工具卡保持折叠 | 不挂载代码/diff/嵌套详情；展开后内容完整，关闭后释放详情 DOM |
| AC-15 | 短会话 A 与工具密集长会话 B 往返 20 轮 | 无 `STATUS_BREAKPOINT`、白屏或应用崩溃，最终身份和滚动状态正确 |
| AC-16 | 压力切换后空闲并强制 GC | 存活 JS 堆不随轮次线性增长；区分 V8/WebView2 保留工作集与真实存活对象泄漏 |

## 兼容、迁移与回滚

- 新 JSON 字段使用 `omitempty`；旧版会忽略未知字段，新版可读取旧 sidecar。
- 不扫描或批量改写历史会话；在打开/快照的正常路径中惰性回填。
- 旧数据推断只使用现有规范路由规则，不读取其他会话或当前全局来源作为会话事实。
- 回滚时可撤回恢复/切换逻辑和测试；已写入的附加 JSON 字段对旧代码无害，
  不需要数据清理。

## 风险与控制

| 风险 | 控制 |
| --- | --- |
| 历史会话误走订阅/BYOK | 将模型与来源成对持久化，覆盖交叉路线恢复测试 |
| 旧会话缺字段 | 唯一推断 + `auto`，不批量迁移 |
| 破坏现有不可用处理 | 不新增状态，保留 `ResolveModelWithFallback` 和 `tabSendAvailability` |
| 新会话行为改变 | 只取消已批准的活动会话强度继承，其他 Token/MCP 行为保留 |
| Fork 入口不一致 | Desktop Tab 与 Controller sidecar 两层都验证完整快照 |
| 切换中有运行任务 | 保留现有 `controllerHasActiveRuntimeWork` 门禁 |
| 惰性挂载改变工具详情 | 保留标题/摘要和展开入口；展开、关闭、归档加载、媒体及运行态分别测试 |
| 过渡门禁降低即时感 | 点击反馈立即可见；只阻止身份未绑定时的危险交互，目标就绪后一次提交 |
| WebView2 崩溃难以单测 | 自动化验证 DOM 生命周期，最终必须用真实长会话压力切换验收 |

## 验证计划

### 聚焦自动化

- `go test ./internal/agent -run 'TestBranchMeta.*Composer|TestBranchMetaRoundTrip' -count=1`
- `go test ./internal/control -run 'TestBranch.*Composer|TestFork.*Composer' -count=1`
- 在 `desktop/` 运行会话路由、Topic 重开、Tab 重启、强度和 Fork 聚焦测试。
- 直接受影响 Go package 至少运行 package 级 `go test`；Desktop 从其独立 module 运行。
- `go run ./scripts/code_guard`、`git diff --check`、EOL 与实际 diff 检查。
- 前端聚焦运行工具卡和会话切换测试，并按仓库规则运行 `pnpm build`。

### 应用验收

1. 准备两个有真实对话记录的会话，分别选 DeepSeek 与 GPT，推理强度设为不同值。
2. 来回切换，关闭其中一个后从历史树重开，再重启应用，每次观察模型、
   计费来源与强度。
3. 在其中一个会话切换模型，确认另一会话和设置页默认值不变。
4. 新建会话确认使用设置页默认；从旧会话 Fork 后确认初始继承且后续独立。
5. 覆盖一个订阅模型与一个 BYOK 模型，验证发送成功且账号/凭据提示与现状一致。
6. 使用一个短会话和一个包含大量工具记录的长会话往返至少 20 轮；切换中
   观察标题、Composer 与正文不得串帧，结束后记录 WebView2 渲染工作集，
   并在开发构建可用时强制 GC 对比存活 JS 堆，确认无 `STATUS_BREAKPOINT`
   或随轮次线性增长的存活对象。

### PR Gate

用户已授权本次按“计划 → 任务 → 实现”连续执行并小步提交，但未授权
创建或更新 PR。获得 PR 授权后，必须先同步最新 `origin/main-v3`，完成应用验收，
再运行仓库 common checks 与：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

必需检查失败时不得创建或更新 PR。
