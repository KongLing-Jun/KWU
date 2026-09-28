# 模型选择器与账号资产同步证据

## 证据范围

- 真实诊断基线：`origin/main-v3@772172e3`；本次文档修正已同步到 `origin/main-v3@ecfebd2d`。中间包含 App、Composer、前端类型和模型路由刷新事件等其他功能改动，因此以下运行时结果只作为诊断基线证据；模型身份字段、label 覆盖和异步目录链路已在当前源码逐项重新确认。新增 `model-route:changed` 只会再次刷新 facts 与 Meta，没有提供规范 `modelRef`。
- 环境：Windows 原生 Wails Dev、未启用 fake、真实登录账号。
- 操作：只切换已有会话，不打开模型列表、不发送消息。
- 采集：前端开发态锚点记录每次有效渲染的相对时间、目标 tab、模型输入、已解析 route、目录请求、订阅 facts、Meta、Settings 中的原始 default/planner ref、发送可用性和触发器文案。
- 隐私：模型切换锚点不记录账号 ID、令牌、凭证、消息内容或完整会话路径。后续账号资产验收经用户明确同意记录余额原值和版本，只记录 UI 安全资产字段，不记录完整账号标识或凭证。

诊断锚点只存在于独立诊断工作树，没有进入产品分支；它用于证明瞬时状态，不能替代最终无锚点应用验收。

## 运行时结果

### 应用恢复

恢复当前 Flash 会话时，约 `755ms` 内记录 11 个模型展示相关状态：

- 第 3 个状态已经收到 `kwu/deepseek-v4-flash`。
- 前 10 个状态仍显示“暂无可选模型”。
- 第 11 个状态目录得到 5 个候选后才显示最终模型。

这证明空态不等于会话没有模型，而是选择器等待候选目录后才允许解析当前项。

### 已有会话切换

| 场景 | 关键相对时间 | 目标 tab 可见链路 |
| --- | --- | --- |
| Flash → Pro | 目标 tab 后 `0ms` 暂无；`+209ms` Pro；`+1066ms` 请选择；`+1152ms` Pro | 暂无可选模型 → Pro → 请选择模型 → Pro |
| Pro → Qwen | 目标 tab 后 `0ms` 暂无；`+129ms` Qwen；`+1009ms` 请选择；`+1094ms` Qwen | 暂无可选模型 → Qwen → 请选择模型 → Qwen |
| Qwen → 另一 Qwen | 目标 tab 后 `0ms` 暂无；`+139ms` Qwen；`+1022ms` 请选择；`+1120ms` Qwen | 暂无可选模型 → Qwen → 请选择模型 → Qwen |
| Qwen → Flash（前轮） | `+301ms` 目标 tab；`+427ms` Flash；`+1420ms` settled | 暂无可选模型 → Flash |

Pro/Qwen 目标 tab 激活后记录 5 个状态快照，其中“暂无可选模型”内部重复渲染两次；按用户可见文案合并后是 4 个阶段。其额外空选阶段稳定出现在 controller Meta 就绪后；Flash 没有该阶段，因为执行模型与原始 planner 都是 Flash，controller label 不产生组合后缀。

### 原始 planner 配置

- 应用恢复和上述每次切换中，Settings 锚点稳定记录 `default=kwu/deepseek-v4-flash`、`planner=kwu/deepseek-v4-flash`；planner 不是空值。
- `ModelsSettingsSection` 以 `plannerRef === defaultRef ? "" : plannerRef` 计算选择值，因此把“显式 Flash 恰好等于当前默认 Flash”显示成“使用当前模型（单模型）”。
- `buildBootPlanner` 只把原始 `planner_model == ""` 视为单模型继承；非空且与执行模型不同会同步生成 `executor + planner ...`，不存在“异步把继承 planner 从 Flash 切回 Qwen”的链路。
- 默认模型切换只清除计费来源不兼容的派生模型；同一计费来源的显式 planner 按既有契约保留。因此切换到 Pro/Qwen 会话后，原始 planner 仍是 Flash。

## 源码对应

### 目录快照空态

- `ModelSwitcher` 使用 `tabId + accountScopeKey` 作为目录 key。
- tab 改变时，旧 snapshot 与新 key 不匹配，候选暂时归零并异步调用 `ModelsForTab`、`ModelMarketplace`。
- 当前模型必须从已过滤候选中解析；候选为零时触发器固定显示“暂无可选模型”。
- 运行时锚点同时证明：本次样本中目标 tab 的 `input` 和 `tabLabel` 已经是保存模型，但 `models=0`、`current=none`；该提前出现时序不是现有 DTO 的稳定保证。

### planner 展示标签误作模型身份

- controller 构建时，非空且与执行模型不同的显式 planner 生成 `executor + planner plannerModel` 的人类 label。
- Desktop controller 就绪后执行 `tab.Label = ctrl.Label()`。
- App 的 subscription facts key 和活动模型路由回退读取 `state.meta.label ?? activeTab.label`。
- 组合 label 不是目录 ref，路由解析得到 `none`，facts 进入 loading，选择器短暂显示“请选择模型”。
- facts 完成后 `ModelsForTab` 的唯一 current route 恢复规范 ref，选择器再次显示绑定模型。
- Desktop 的 `WorkspaceTab.model` 和持久化 tab entry 已保存规范 ref，但现有 `Meta`/`TabMeta` 只暴露可变 `label`；因此前端无法在所有首次访问时同步取得规范身份。

### planner 继承显示失真

- `ModelsSettingsSection` 以“planner ref 是否等于 default ref”判断继承，掩盖了原始非空配置。
- `SetPlannerModel("")` 已能持久化真正的继承；缺陷不在保存接口，而在设置页把显式同值折叠为空选项。
- 修复必须以原始 planner 是否为空判断继承。已有非空同值配置不得自动迁移或清空，而应如实显示为显式模型；用户主动选择“使用当前模型（单模型）”时才写入空值。

## 根因结论

1. 会话保存模型真实存在，问题不是后端丢失绑定。
2. 第一类闪烁由“目录候选是否加载完成”错误决定“已知绑定模型是否可以展示”。
3. 第二类闪烁由“人类 controller label”与“规范模型身份”复用同一字段造成。
4. 设置页同时存在可达的语义缺陷：显式 planner 与默认模型相同时被错误显示为继承，导致用户误判配置，并使后续执行模型变化暴露遗留 planner。
5. “正在验证模型权限”属于发送门禁；它应继续失败关闭，但不应清除已知模型展示。
6. Task 002 的资产事件与缓存 API 不参与上述链路；Task 003 的套餐重筛会触及相邻前端状态，因此需要回归该边界。

## 真实结算余额通知缺口

### 环境与操作

- 后端基线：已合并原 Task 002 的 `origin/main-v3@c89b3a60`。
- 前端：Task 003 独立工作树的完成实现，加未提交临时锚点；锚点不进入 PR。
- 环境：Windows 原生 Wails Dev、未启用 fake、复用真实登录状态。
- 操作：创建新会话，选择 Flash，经用户明确授权发送一次“仅回复 1”；请求完成后 30 秒内不进入账户页、不点击刷新。

### 运行时证据

1. 冷启动恢复时，账号 Store 在 `+1541.0ms` 得到余额 `94063.7412`、版本 `15`。
2. 新会话从 Pro 切到 Flash：
   - `model.pick` 后持久化 pending 期间继续显示 Pro。
   - 约 `1.45s` 后直接显示 Flash。
   - 期间没有“请选择模型”或“暂无可选模型”。
3. `turn.send` 记录模型为 Flash，余额仍为 `94063.7412`、版本 `15`。
4. `turn.done` 在约 `2.81s` 后成功，余额仍为 `94063.7412`、版本 `15`。
5. 随后 30 秒内没有出现 `assets.event`、`cache-read-start`、`cache-read-resolved`、`store-apply` 或新版本 `account.sidebar-render`。
6. 进入账户页触发其挂载 `CloudBootstrap` 后，左侧账号菜单和账户页卡片同时显示 `93841.0328`。

### 源码对应

- `desktop/model_billing.go`
  - `desktopRemoteSettlementObserver.CreditSettlementObserved` 在终态结算观察时读取远端当前账号、复核账号和套餐 scope，再调用 `runtime.assets.StoreBalance`。
  - 成功写入后直接返回，没有调用 `runtime.assetNotifier.notify`。
- `desktop/cloud_runtime.go`
  - real runtime 虽持有生命周期安全的 `assetNotifier`，但 `Realtime=false` 且不构造 `EventSyncApplication`；因此不会有真实事件 worker 补发通知。
- `desktop/frontend/src/features/account/controller/useAccountSidebar.ts`
  - Task 003 只在收到 `cloud-assets:changed` 后读取 `CloudCachedAssets`；没有通知就不会启动缓存读取。
- `desktop/frontend/src/features/account/ui/AccountCenter.tsx`
  - 账户页与左侧菜单共用 `accountStore.balance`。
  - 账户页挂载会调用 `CloudBootstrap`，所以进入页面后两个显示同时更新。

### 结论

1. 两个余额组件不需要分别增加刷新逻辑；共享 Store 已能让它们同时重渲染。
2. Task 003 前端事件消费链没有机会运行，因为真实结算生产者缺少唤醒。
3. 最小修复点是：完整 scope 复核与 `StoreBalance` 成功后，真实结算观察器向现有通知器提交 `balanceChanged`。
4. 该通知只覆盖当前 Desktop 自己发起的真实模型任务；其他设备消费、充值和后台调整仍由账户页或手动刷新校准，不引入轮询或真实事件 transport。

## 修复证明要求

- 从目标 tab 首次显示起，触发器时间线只有其保存绑定模型，pending 期间保持禁用。
- controller label 加入 planner 说明时，规范模型 ref、目录请求次数和触发器文案不改变。
- 设置页只在原始 planner 为空时显示“使用当前模型（单模型）”；显式同值显示实际模型，选择单模型后重新读取 Settings 得到空 planner。
- 目录与套餐事实稳定后，真实不可用模型仍进入“请选择模型”或“暂无可选模型”。
- 连续切换多个已有会话、快速切回和套餐通知交错时，不出现跨 tab 身份污染。
- 模型会话切换最终验收使用无诊断覆盖层的原生 `wails dev`，该场景不发送消息。
- Task 002 补修后，重新发送一次经授权的最小真实请求；不得进入账户页或点击刷新，必须观察到 `turn_done` 后的 `balanceChanged`、缓存读取、Store 版本前进以及两个余额消费面共享同一新值。
