# 自动化模型路由一致性修复计划

## 1. 状态与基线

- 工作树：`D:\Reasonix-worktrees\fix-automation-model-snapshot`
- 分支：`codex/fix-automation-model-snapshot`
- 基线：`origin/main-v3@18a793797e9e5d9dc6146f6393be1aa67b199015`
- 工作流：模型与计费路由会影响持久化任务和积分归属，按高风险修复处理。
- 兼容前提：产品尚无用户；不为历史空模型任务增加迁移或兼容分支。
- 最新基线与事故发生时的 `498c6ae4` 相比，相关自动化、路由、tab 构建和模型选择文件均未变化；中间提交只涉及项目/工作空间表面。

## 2. 已记录的故障与基线证据

### 2.1 环境和前提

- Windows 桌面 `vdev`，时区 `Asia/Shanghai`。
- 数据根：`C:\Users\ai03\AppData\Roaming\kwu-dev\environments\a6defe83d02e4e84`。
- 自动化任务：`task-224d3b2af51860c5`，名称“图表读数练习”，全局 scope，复用同一 topic/session。
- 任务持久化的 `Prompt.Model` 和 `Prompt.BillingSource` 均为空。
- 8 月 18 日首次手动运行时，运行清单解析为 `gpt/gpt-5.6-luna + newapi_byok`，并创建 Luna 会话。
- 8 月 20 日定时运行时，设置页默认路由已经变为 `kwu/deepseek-v4-flash + subscription_pool`。

### 2.2 复现步骤

1. 创建选择“留空使用默认模型”的复用对话自动化任务。
2. 在默认路由 A 下运行一次，使任务绑定并持久化会话。
3. 将设置页默认路由切换为不同模型和不同计费来源的路由 B。
4. 正常退出并重启 Desktop，使旧会话由异步 tab Controller 恢复。
5. 再次触发同一自动化任务。
6. 从自动化列表进入该任务会话，检查 Composer 模型选择器并发送一条手动续聊消息。
7. 对照任务 JSON、运行 manifest、session `.meta`/turn metadata 和 `model-billing.sqlite`。

### 2.3 实际行为

- 8 月 20 日 manifest 记录 `kwu/deepseek-v4-flash + subscription_pool`，输入/输出 token 为 `33718/711`。
- 同时间计费账本中 token 完全一致的真实调用却是 `gpt-5.6-luna + subscription_pool`，并结算 `554.5664` 免费积分。
- session 最终保存了目录中不存在的交叉组合 `gpt/gpt-5.6-luna + subscription_pool`。
- 前端按 model ref 和 billing source 精确匹配目录，无法找到该组合，因此显示“请选择模型”；“自动”只是推理强度。
- 8 月 24 日发送“你好”时，turn metadata 和账本再次证明调用为 `gpt-5.6-luna + subscription_pool`，结算 `506.1528` 免费积分。

### 2.4 期望行为

本计划采用已确认的新语义：

- “使用当前默认模型”只是在创建或编辑表单中选择当前默认路由；保存后任务固定为具体 `model + billingSource`。
- 修改设置页默认模型不影响已有任务。
- 自动化任务拥有执行路由；session 只提供历史，不得覆盖任务路由。
- 模型不可用或构建后的有效路由与任务不一致时，本次运行在模型调用前 blocked，不自动回退或静默换模。
- 手动续聊切换模型不修改任务；编辑任务模型在下一次自动化执行时生效。

## 3. 根因与现有合同

### 3.1 空任务模型在运行时动态解析

`desktop/automation_route.go` 的 `automationTaskRouteInput` 在 `Prompt.Model` 为空时读取任务 scope/actor 当时的设置页默认模型与默认计费来源。`desktop/automation.go` 的 `resolveAutomationTask` 只把结果写入本次运行的内存 task，不回写持久化任务。

`AutomationSaveTasks`、`AutomationCreateTask` 和 `AutomationUpdateTask` 当前直接把 task 交给 core service，因此持久化记录可以长期为空。旧 Wails 入口 `HeartbeatSaveTasks` 还会经 `heartbeatCompatSaveTasks` 直接调用同一个 service，并在兼容转换时明确构造空 route。core `automation.NormalizeTask` 没有 Desktop profile/catalog 上下文，不能在内部正确物化默认路由。

保存路由时还必须复用运行期 `automationTopicScope` 的规范化规则：非 project 一律是 global/空 root，project root 转为绝对路径。否则公开 API 可提交 `global + stale project root` 或 project 相对路径，保存时按一个 profile 选模型、运行时按另一个 profile 执行。

### 3.2 异步 Controller 构建让旧 session 模型覆盖显式路由

`bindAutomationTaskTopic` 把本次解析路由交给 tab opener。新 tab 初始持有正确模型和计费来源，runner 的构建前检查因此认为路由一致。

随后 `desktop/tabs.go` 的 `buildTabControllerWithContext` 从 session `.meta` 读取旧模型；只要该模型能被当前 config 解析，就覆盖 `tab.model`，但保留本次任务的 `tab.billingSource`。`availableSubscriptionModelFromConfig` 又允许可解析且受订阅目录支持的同 model ID，最终形成跨 provider model ref 与 subscription source 的组合。

### 3.3 UI 与后端对异常组合采取不同解释

- `ModelSwitcher.resolveCurrentModel` 要求目录中的 `ref/model + billingSource` 精确匹配，异常组合显示“请选择模型”。
- `desktopBillingRouteResolver` 以 model ID 验证订阅支持并改用订阅 gateway，异常组合仍可执行和结算。

因此 UI 占位不能证明没有实际模型，manifest 也不能证明 Controller 最终使用了同一路由。

## 4. 目标与非目标

### 4.1 目标

- Desktop 保存的每个实际自动化任务都持久化具体、可选且规范的模型路由。
- 模板继续保持可移植；由模板创建实际任务时才快照当前默认路由。
- 同一任务的持久化定义、run manifest、tab/session metadata、实际 gateway model ID 和计费来源一致。
- 显式任务路由在异步 Controller 构建期间保持权威，不受旧 session 模型影响。
- 在 `RunCapabilityTurn` 前增加最终精确路由守卫，残余不一致不得产生模型请求或积分结算。
- 普通手动打开 session 时继续恢复其保存的模型与计费来源。
- UI 清楚表达“当前默认模型只在保存时固定”。

### 4.2 非目标

- 不在启动、读取或运行时主动迁移/回填现有空模型任务、异常 `.meta` 或 `desktop-tabs.json`；任务参与用户发起的 create/update/bulk save 时按新的保存合同正常物化。
- 不增加“持续跟随默认模型”模式、`modelPolicy` 字段或新 schema。
- 不让手动 session 模型切换反向修改任务。
- 不在保存任务时立即重建其已有会话 Controller。
- 不修改模板 schema；现有 `TaskTemplate` 本就不包含模型字段。
- 不按 billing source 强制重写 model ref，不放宽或改变模型目录和订阅计费规则。
- 不新增通用 tab 构建框架、持久化事务、依赖或 PR Gate 例外。

## 5. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 通过任一 Desktop 入口保存选择“使用当前默认模型（保存后固定）”的实际任务 | 返回值（适用时）和任务存储均包含当时具体 `Prompt.Model`、`Prompt.BillingSource` 和一致的 `Capabilities.ModelRef` |
| AC-02 | 创建后修改设置页默认模型 | 已保存任务不变；下一次运行继续使用任务固定路由 |
| AC-03 | 编辑任务并再次选择当前默认模型 | 保存时快照编辑时的默认路由；不立即重建已有 session，下一次自动化运行应用新路由 |
| AC-04 | 从内置或用户模板创建任务 | 模板保持无模型种子；实际任务保存时物化创建者当前默认路由 |
| AC-05 | 默认或显式路由不可用 | create/update/native bulk/heartbeat compat bulk save 失败；bulk save 不产生部分写入；运行不静默回退 |
| AC-06 | 固定任务路由与复用 session 保存路由不同 | 自动化保留历史，但 Controller、turn、session `.meta`、manifest 和账本都使用任务路由 |
| AC-07 | 普通手动打开已有 session | 未提供显式任务路由时，继续恢复 session 保存的模型和计费来源 |
| AC-08 | Controller 构建后有效路由与任务不一致 | 在 `RunCapabilityTurn` 和计费任务创建前 blocked，不进行二次自动重建 |
| AC-09 | 自动化任务编辑器 | 空草稿选项文案为“使用当前默认模型（保存后固定）”；已保存任务重新打开时显示具体模型 |
| AC-10 | 在任务会话中手动切换模型 | 只影响手动续聊；持久化任务不变，下一次自动化恢复任务路由 |
| AC-11 | 范围控制 | `Task`/`TaskTemplate` schema、依赖、`scripts/code_guard_baseline.json` 和无关 session 行为不变 |

## 6. 选定方案

### 6.1 在 Desktop 持久化边界物化任务路由

复用现有 `resolveAutomationTask`，或提取只表达该行为的窄 helper：

1. 先按 `automationTopicScope` 的既有规则规范化并写回 task scope/root：非 project 为 global/空 root，project root 为绝对路径。
2. 按规范后的 scope/workspace 和 actor 读取路由目录。
3. 空草稿使用该作用域完整的当前默认模型与默认计费来源；若空 model 意外携带残留 source，也以完整默认 route 覆盖，不能拼接成混合 route。
4. 显式草稿按目录精确验证，不可用即返回错误。
5. 写入 `Prompt.Model`、`Prompt.BillingSource` 和 `Capabilities.ModelRef`。

在调用 core service 前覆盖全部四个 Desktop 持久化入口：

- `AutomationCreateTask`
- `AutomationUpdateTask`
- `AutomationSaveTasks`，并先物化完整列表再进行一次 service 调用，保持全有或全无。
- `HeartbeatSaveTasks`/`heartbeatCompatSaveTasks`：兼容转换并合并 native task 后，复用同一个完整列表物化 helper，再进行一次 service 调用；保持该入口既有事件行为。

不把 Desktop 路由解析下沉到 `internal/automation`，避免 core persistence 依赖 profile、凭据和动态订阅目录。`AutomationSaveTaskTemplate` 不物化；当前模板只保存标题、描述、prompt、interval、icon、kind 和 source，模板转成草稿后仍通过 `AutomationCreateTask` 快照。

### 6.2 让显式路由成为一次 Controller build 的不可变输入

把完整 `modelbilling.BillingRouteSelection` 作为 build-local 输入，从 route-aware topic opener 和 controller-less 显式模型切换传入 tab build。该输入与 build generation 绑定，不从稍后可能变化的 `tab.model` 猜测。

- 有显式路由：使用其 model/source 加载 config 和构建 Controller，跳过 session model 覆盖；仍加载同一 session 的历史、专家、权限、场景和路径。
- 无显式路由：保持当前普通 session 恢复逻辑，继续使用 `.meta` 模型。
- 显式路由在构建时必须再次精确命中；不得经过 `availableSubscriptionModelFromConfig` 或 `ResolveModelWithFallback` 换成其他模型。解析后目录若发生变化，构建失败并由 runner 记为 blocked。

不新增“最终守卫发现不一致后再重建”的路径，避免无意义启动、continuation session 创建和用户并发窗口。若应用启动时普通 session 恢复已经先于自动化绑定开始，沿用现有 wait-then-repair；修复后的 repair build 必须携带显式路由且不能再被 session 覆盖。

现有 pending build 不能只依据等待前的 tab 字段决定是否 repair。即使字段当时已经等于任务 route，普通 restore 仍可能随后从旧 `.meta` 覆盖 model。因此 runner 对所有 controller-missing pending build 先等待当前 generation，完成后重新精确比较；仅当最终 route 不同时调用既有原子 route repair，再等待显式 repair build。这样可达窗口得到成功修复，而不是只被最终守卫 blocked。

若 pending 普通 restore 因旧 session route 启动失败，等待返回错误后也必须锁下重读 tab：`StartupErr` 非空或 `Ready` 但无 Controller 时，对已在本次 run 开头验证为可用的任务 route 执行一次显式 repair，并等待该 build。只有 context cancel/deadline 和 tab 仍处于纯 pending 的等待超时直接返回，不用 task route 掩盖取消或无界等待问题。

### 6.3 在模型调用前增加最终路由守卫

`waitAutomationTopicController` 成功后、任何 Controller 状态变更和 `RunCapabilityTurn` 之前，重新读取 tab 的实际 `model + billingSource` 并与本次 route 精确比较。

- 完全一致：继续执行。
- 不一致：返回 blocked runner error；不重建、不调用模型、不创建计费任务。

既有构建前 repair 继续负责恢复已有/失败 tab，但不再被视为最终正确性证明。

### 6.4 更新表单语义而不扩展持久化格式

- 保留 value 为空的临时草稿选项，统一显示“使用当前默认模型（保存后固定）”。
- 空值只存在于未保存的编辑草稿；保存成功返回的 task 带具体路由，列表和再次编辑均展示具体模型。
- scope/workspace 在保存前变化时，前端继续提交其 canonical 值；后端仍按运行期 `automationTopicScope` 规则规范化并写回，避免前端异步目录状态或其他公开调用方成为错误事实源。

## 7. 方案挑战与取舍

| 候选方案 | 结论 | 原因 |
| --- | --- | --- |
| 每次运行动态读取设置页默认模型 | 不采用 | 已确认任务创建/编辑时固定；动态变化使成本和复现不可预测 |
| 只在 session model 与 billing source 前缀不匹配时拒绝覆盖 | 不采用 | 同一计费来源下两个不同模型仍会发生 manifest/实际调用漂移 |
| 把异常 `gpt/... + subscription_pool` 改写成 `kwu/...` | 不采用 | 只能修复选择器显示，不能保证使用任务选择的模型 |
| Controller 构建后发现不同就自动重建 | 不采用 | 会先构建错误路由，可能创建 continuation session，并扩大竞态和失败面 |
| 只由前端预选具体默认模型 | 不采用 | bulk/API/模板草稿路径仍可保存空值；前端异步状态不应成为持久化事实源 |
| 在 core `NormalizeTask` 中读取默认模型 | 不采用 | core 无 Desktop actor/profile/catalog 上下文，产生反向依赖 |
| 增加旧任务迁移或 `modelPolicy` | 不采用 | 明确无用户和老数据；没有验收标准支撑额外兼容概念 |

本方案比单点条件判断多一个保存边界和一个执行守卫，但每一处分别关闭“持久化不确定性”“异步覆盖”和“调用前残余漂移”，没有未受控的静默路径。

## 8. 影响文件与任务拆分

### 任务 001：固定自动化任务模型路由

预计范围：

- `desktop/automation.go`
- `desktop/automation_route.go`（仅在窄 helper/错误合同确有需要时）
- `desktop/heartbeat_compat.go`
- `desktop/automation_app_test.go`、`desktop/automation_route_test.go` 及邻近测试
- `desktop/heartbeat_compat_test.go`
- `desktop/frontend/src/features/automation/heartbeat/HeartbeatPanel.tsx`
- `desktop/frontend/src/locales/zh.ts`
- 必要的现有自动化表单合同测试源文件，但按仓库规则只运行 `pnpm build`

交付：create/update/bulk save 物化并持久化具体路由；模板实例化由 create 路径快照；UI 文案对齐。

### 任务 002：保证任务路由主导自动化执行

依赖任务 001，预计范围：

- `desktop/tabs.go`
- `desktop/app.go`
- `desktop/automation_runner.go`
- `desktop/automation_topic.go`（只有 route build 传递确有需要时）
- `desktop/automation_topic_test.go`、`desktop/tabs_topic_test.go`、`desktop/model_billing_test.go` 及邻近测试

交付：build-local 显式路由优先、普通 session 恢复不变、调用前精确守卫和端到端回归。

两个任务串行实施。任务 001 建立持久化定义合同；任务 002 修复运行期根因，并同时保护显式任务路由和 controller-less 模型切换。

## 9. 兼容性、迁移与回滚

### 9.1 兼容与迁移

- 不提供现有空模型任务的专门迁移、启动回填或读取回填；开发测试数据在验收前重建。若旧记录参与用户发起的 update/bulk save，则与该次保存中的其他任务一样物化，不属于后台迁移。
- `automation.Task` JSON 字段和 Wails 方法签名不变；具体值写入现有字段。
- `TaskTemplate` 不含模型字段并保持不变。
- 已有显式模型任务继续按原目录验证。
- 普通 session 打开、手动模型切换和历史格式保持不变。

### 9.2 回滚

- 代码以两个独立行为提交，可按相反顺序回滚。
- 没有 schema 或批量迁移，因此不需要数据降级脚本。
- 新版本保存的具体模型字段已被旧代码支持；回滚后任务仍可运行，但旧代码会重新暴露 session 覆盖风险。

## 10. 风险与控制

| 风险 | 可达条件 | 控制与验证 |
| --- | --- | --- |
| 保存时使用错误作用域默认模型 | 编辑期间切换 global/project workspace | 后端按最终 task scope/workspace/actor 解析；覆盖 global/project 测试 |
| 旧 Wails 保存入口绕过物化 | legacy heartbeat binding 仍被调用 | compat 合并列表后复用同一批量 helper；覆盖成功快照和失败零写入 |
| bulk save 部分物化后部分落盘 | 中间任务路由不可用 | service 调用前完成全列表物化；失败时断言存储完全不变 |
| 任务保存后目录变化 | 订阅失效、模型下架或 provider 移除 | 下一次运行精确验证并 blocked，不自动回退 |
| 旧 session 再次覆盖显式路由 | 异步 build 读取 `.meta` | build-local route 与 generation 绑定；普通无 route 恢复单独回归 |
| pending restore 在预检查后覆盖 route | tab 字段已匹配但 Ctrl 尚未就绪 | 所有 pending build 先等待并重新比较；必要时走一次现有 repair |
| 旧 session route 导致 pending restore 启动失败 | 旧模型下架、旧 source 无凭据或旧 config 不可构建 | 等待错误后区分 tab startup failure 与 cancel/timeout；前者用有效 task route repair 一次 |
| 构建 fallback 与 manifest 漂移 | 解析后目录在 build 中变化 | `RunCapabilityTurn` 前精确守卫；断言无 gateway/计费调用 |
| 手动模型切换误改任务 | 用户进入任务会话续聊 | 不建立 tab-to-task 反向更新；任务存储回归保持不变 |
| 保存任务触发隐藏运行时副作用 | 编辑已有任务模型 | 保存路径不重建 Controller；下一次自动化执行才应用 |
| 过度清理 legacy 代码扩大范围 | core 仍允许通用空模型 fixture | 只收紧 Desktop 产品持久化边界，不重构 core schema/迁移代码 |

若实现需要新增持久化字段、全局迁移、修改计费规则、让 manual session 同步任务、保存时立即重建 Controller 或改变模板结构，必须停止并重新审批计划。

## 11. 验证计划

### 11.1 聚焦自动化验证

Desktop Go 变更位于单一 nested module/package，迭代可先运行精确测试，最终至少运行：

```powershell
cd desktop
go test . -count=1
```

Frontend 变更按仓库规则只运行生产构建：

```powershell
cd desktop/frontend
pnpm build
```

重点自动化证据：

- create/update/bulk save 返回与落盘的具体 route。
- heartbeat compat bulk save 与 native bulk 使用相同物化/原子拒绝合同。
- global stale root 和 project relative root 保存后与运行期 scope/root 一致；空 model 的残留 source 不会污染完整默认 route。
- 模板草稿创建时物化 route。
- 不可用 route 原子拒绝。
- 旧 session route 与任务 route 不同时，Controller、tab、`.meta` 和调用 identity 一致。
- 旧 session route 无法构建时，pending wait 的 startup failure 仍可由有效 task route repair 后成功；context cancel/deadline 和纯 timeout 不触发 repair。
- 普通 session reopen 仍恢复保存 route。
- 最终 mismatch 在 `RunCapabilityTurn` 前 blocked，gateway/计费调用数为零。

### 11.2 真实应用 A/B

按照仓库要求，A 与 B 串行使用同一 `kwu-dev` dev 配置、fixture 服务、账号/profile 和数据位置；只隔离源码/Git。使用最新 `origin/main-v3` 的独立 baseline worktree 运行 A，patched worktree 运行 B，均从各自 `desktop/` 执行 `wails dev`。

1. 在 A 前备份受控 dev profile 的任务、session、tab、config 和 billing 数据，并记录有效 `KWU_CLOUD_FAKE`、cloud/profile、默认 route、进程、端口和 singleton 状态。
2. 将默认 route 设为 A（例如 Luna + BYOK），创建选择“当前默认”的专用测试任务并手动触发一次。
3. 正常关闭应用，确认进程、端口和 singleton lock 已释放。
4. 将设置页默认 route 改为 B（例如 DeepSeek + subscription），重启并再次触发同一任务，然后进入会话手动续聊。
5. A 必须复现：持久化任务仍为空，且在重启复用 session 后 manifest/实际 route 或选择器身份发生漂移。记录任务 JSON、manifest、turn/session metadata、账本和截图。
6. 正常关闭 A，确认资源释放；从备份恢复完全相同的初始 profile 状态。
7. 在 B 执行同样步骤。任务创建后必须保存 route A；切换设置默认值后任务与两次运行仍使用 route A；选择器显示具体模型，手动续聊沿用该 session route，manifest/turn/账本一致。
8. B 通过后保留 patched `wails dev` 进程供用户手工验收。

如果 baseline 因外部目录或 fixture 状态无法稳定复现，必须报告“分离环境对比”限制，不能声称真实应用 A/B 通过。不得手工删除未证明陈旧的 lock。

### 11.3 仓库门禁

最终同步最新 `main-v3` 后，按完整 NUL 路径列表检查分类，并运行：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

预计分类为 Desktop Go + frontend；不修改 root Go、依赖或 gate infrastructure。任一必需检查失败时不得发布 PR。

## 12. 就绪判断

- 用户已确认创建/编辑保存时固定模型路由、模板实例化时快照、不可用即阻止、手动续聊不改任务、编辑后下一次运行生效。
- 根因、扩展点、消费者、计费风险、失败行为、非目标和 A/B 验收均有实际源码或运行证据。
- 高风险只读评审发现的 legacy 保存入口、pending build 窗口和 scope/root 漂移均已按现有合同收敛到任务中。
- 没有待定产品、API、数据、迁移、依赖或回滚决策。
- 计划提交后再创建两个任务合同，并在高风险只读评审完成后一起请求一次实施批准。
