# 隐藏内置供应商地址修复计划

## 决策状态

- 实现基线：`origin/main-v3@768d8fa2`。
- 工作树：`D:\Reasonix-worktrees\hide-official-provider-url`。
- 用户已确认：Desktop 暂时继续直连 AirClaude；不强制退出旧登录；升级只清理官方内置 provider，不重置其他配置；根默认模型改为 `kwu/deepseek-v4-flash`；登录目录临时失败时继续保留现有四模型静态兜底；无活动会话时 effort 保持默认 `auto`，不为历史全局 effort 增加新的持久化字段。
- 本文档只冻结方案和验收，不授权产品代码实现；任务合同完成并通过高风险只读审查后，再请求一次明确实施批准。

## 问题与证据

### 当前可见行为

1. `internal/config/kwu_official.go` 把官方 API 地址、`gpt-5.4` 默认模型和官方 provider 作为通用配置默认值。
2. `internal/config/config.go` 的根 `default_model` 由上述 provider 默认值生成，因此新配置当前写出 `kwu/gpt-5.4`。
3. `internal/config/render.go` 在 User/Full scope 无条件遍历 `Config.Providers` 并写出 `base_url`、模型目录和 provider 默认值。内置官方 provider 因此进入用户 `config.toml`。
4. `desktop/settings_app.go` 将 `ProviderEntry.BaseURL` 原样投影到 `ProviderView`。内置卡片虽已标记 `BuiltIn`、`Locked`，但没有清空 `BaseURL`。
5. `desktop/frontend/src/features/settings/ui/ModelsSettingsSection.tsx` 直接渲染 `group.baseUrl`；`desktop/frontend/src/lib/brand.ts` 又包含官方现网和历史地址，用地址识别内置 provider，所以生产前端源码和 bundle 也包含该地址。
6. `desktop/model_billing.go` 的订阅目录注入只会更新已经存在的 `kwu` provider；如果 TOML 不再包含 provider 且通用加载因文件 provider 替换默认列表而丢失内置项，该函数会直接返回，订阅路由可能无法解析。
7. `internal/config/load.go` 已有精确的官方条目判定：名称为 `kwu`，且 `base_url` 与官方现网或历史地址等价。相同名称但不同地址不会被判为官方。
8. `internal/config/pricing.go` 已有按 `config_version` 在 Desktop 启动时只执行一次的用户配置升级入口；`desktop/app.go` 在首次配置加载前调用它，且使用不加载凭据的 edit loader。

### 真实基线复现记录

- 环境：Windows，源码基线 `D:\Reasonix-worktrees\fix-product-identity-v2@768d8fa2`，该工作树干净并与 `origin/main-v3` 一致；运行进程命令为 `wails dev`，目标窗口进程来自该工作树的 `desktop\build\bin\kwu-desktop-dev.exe`。
- 有效 dev 云环境：`http://192.168.10.243:8080/`；由现有 dev profile marker 确认状态根为 `C:\Users\ai03\AppData\Roaming\kwu-dev\environments\a6defe83d02e4e84`。当前检查 shell 没有设置 `KWU_DEV_PROFILE_ROOT` 覆盖；无法从已退出的应用子进程反查其环境块，因此 A/B 启动时必须显式记录有效 profile。
- 初始 `config.toml`：大小 `10658` 字节，SHA-256 为 `1043F055D9B7965BCF8B3A6CFFE856C4126065A7106BEFCBA0A6ECCEE83C464C`；第 6 行是 `config_version = 3`，第 7 行是 profile 已保存的 `default_model = "kwu/deepseek-v4-flash"`，第 66 行开始的官方 `[[providers]]` 在第 69 行直接写出官方 API 基址，并在第 71 行写出 provider `default = "gpt-5.4"`。实际结果与“地址不应进入配置”相反。
- UI 步骤：激活上述基线窗口，打开账户菜单，进入“设置 > 模型”。“使用”页显示该 profile 已保存的 `kwu/deepseek-v4-flash`；用户本次请求所附的同版“接入”页截图直接显示内置云智企办卡片和官方 API 基址，构成 UI 暴露失败证据。
- 登录前置状态：该 profile 当前 UI 为“登录已失效”，因此本轮基线不能证明有效登录跨升级保持，只能证明启动没有主动删除登录文件。`cloud\v1\binding.json` 的 SHA-256 为 `F3C960DC25BB10A34219B6E4745E4B8AD0F4C04499547CDD5F6A8BFC4DC86441`，`cloud\v1\sessions.json` 的 SHA-256 为 `828349D08BA0CC2A3BF29A472D9D79A2936C32C90277AEEB6855ABE0AC64521C`；内容未写入文档。
- 基线窗口在切换“接入”页时退出，遗留的 `wails dev` watcher 没有应用子进程；不把该次退出归因于本缺陷，也不将未完成的点击结果计入证据。补丁 A/B 必须从正常启动、等价初态重新执行。
- 期望：配置和 UI 不出现基址；根默认为 Flash；登录文件不变；订阅 provider 仍可由运行时构造。当前实际只满足 profile 已显式保存 Flash，其他三项未满足或尚未验证。

### 当前四模型静态兜底

`desktop/model_billing.go` 的 `desktopAirClaudeSubscriptionModels` 当前精确包含：

1. `gpt-5.4`
2. `gpt-5.4-mini`
3. `deepseek-v4-flash`
4. `deepseek-v4-pro`

它只在 Desktop 订阅目录没有可用远端结果或测试模式需要目录时注入。用户确认本次不改变这四个值、不删除该兜底、不改目录失败后的现有行为。

### 根因

当前实现把三种不同职责合并在同一个 `ProviderEntry` 上：

- 运行时请求必须知道的官方地址；
- 用户可编辑并持久化的自定义 provider 配置；
- 设置页展示的 provider 元数据。

直接从默认配置删除官方 provider 会让订阅模型解析失去运行时条目；只在前端隐藏则仍会从 TOML 和 Wails DTO 暴露；只在渲染器跳过则旧配置升级后和文件 provider 替换默认列表的运行时路径仍需处理。因此修复必须同时收口持久化、Desktop 订阅路由和设置视图三个边界。

## 安全边界

### 本次保证

- 新安装、升级迁移后以及后续设置保存产生的用户 `config.toml` 不包含官方现网/历史 API 地址，也不包含内置官方 `[[providers]]` 块。
- 设置后端 DTO 不向前端返回内置 provider 地址，设置卡片不显示地址。
- 生产前端源码和 bundle 不包含用于识别内置 provider 的官方现网/历史 API 地址常量。
- 相同名称但不同地址的用户自定义 provider 不被删除、不被隐藏，也不被当作订阅内置 provider 持久化迁移。
- 用户登录文件、会话令牌和绑定状态不被迁移逻辑读取、清除或重写。

### 明确不保证

- Desktop 仍然直连 AirClaude，官方地址必须存在于后端运行时代码或其构建产物中，也会出现在 DNS/代理/网络连接信息里。本次不是密钥保护，也不能对有本机调试或逆向能力的用户隐藏服务地址。
- 不把 API 地址硬编码当作认证手段。实际防滥用边界继续由登录、短期 MaaS 凭据、服务端账号/套餐授权、额度和风控承担。
- 不隐藏模型商城公开价格来源 URL；本次敏感对象仅为 OpenAI 兼容请求基址及其历史地址。
- 不改变 CLI 的分发策略。共享配置代码变化会自然影响 CLI 的默认配置渲染，但产品只交付 Desktop；仍需保持通用配置加载和自定义 provider 可用。
- 不新增 `default_effort` 或等价配置字段。无活动会话时不再通过即将移除的官方 provider 块持久化全局 effort；实际前端始终使用带 `tabId` 的会话级接口，已有会话 effort 保存机制保持不变。

## 目标

1. 将内置官方 provider 从“用户持久化配置”变成“订阅路由运行时设施”。
2. 保持登录后的 Desktop 订阅模型调用、刷新和动态目录正常工作。
3. 把根默认模型从不应成为默认的 `kwu/gpt-5.4` 改为 `kwu/deepseek-v4-flash`。
4. 对旧版用户配置做一次精确、可重复执行且不影响登录状态的迁移。
5. 保持自定义 BYOK provider、既有会话模型、planner/subagent 模型和四模型静态兜底不变。

## 非目标

- 不改为服务端代理，不隐藏后端二进制或网络层可观察的地址。
- 不轮换或重构 MaaS 凭据格式，不修改登录、退出登录、账号绑定和套餐协议。
- 不改变远端模型目录成功、失败、授权失败或刷新状态机。
- 不移除、不增删、不重排四模型静态兜底。
- 不批量迁移已有 tab、会话历史、planner、subagent、automation 或 bot 中显式保存的 `gpt-5.4` 引用。
- 不重置整份配置，不删除同名但不同地址的自定义 provider。
- 不借机清理其他前端测试夹具、公开商城 URL、品牌文字或 provider 架构。

## 期望行为

### 新安装与正常保存

- `Config.Default()` 的内存默认模型为 `kwu/deepseek-v4-flash`，且该引用在内存官方 provider 目录中可解析。
- User、Full 和 Project TOML 渲染遇到精确官方 `kwu` provider 时跳过该条目；其他 provider 保持原顺序和字段。
- 根 `default_model = "kwu/deepseek-v4-flash"` 继续正常持久化。这一行不是地址，也不能依赖前端筛选来修正。
- 保存设置后重新加载，订阅路由仍能在自己的配置副本中获得内置 `kwu` provider；显式 BYOK 路由仍使用用户配置。
- 通用加载器在 TOML 因包含其他 provider 而替换默认 provider 列表后，只要配置仍引用内置 `kwu` 且不存在同名条目，就重新物化非持久化官方 provider。因此包含自定义 provider 的 CLI/共享配置重新加载后，根默认仍可解析；该运行时物化仍不会进入下次 TOML 输出。
- 没有活动会话时 effort 为 `auto`（运行时空字符串），不会为了保存非默认值创建官方 provider 块或新增配置字段；有活动会话时继续由现有会话状态保存具体 effort。

### 升级

- `config_version` 从 `3` 提升到 `4`。
- 当旧文件包含名称为 `kwu` 且地址精确匹配官方现网或历史地址的条目时，升级移除该官方条目；同一文件内其他 provider 原样保留。
- 只有在确认旧根默认引用属于被移除的官方条目，且文件中不存在同名不同地址的自定义 `kwu` 时，精确的 `default_model = "kwu/gpt-5.4"` 才迁移为 `kwu/deepseek-v4-flash`。同名不同地址的自定义 `kwu` 及其默认模型不迁移。
- 升级沿用现有一次性配置维护入口，继续执行其既有官方价格重置职责，然后写入版本 `4`。第二次启动不再重写。
- 升级只操作当前 profile 的用户配置文件；不删除登录状态、不重写凭据存储、不清理 tab 或会话。

### Desktop 订阅运行时

- `subscription_pool` 配置副本在应用远端或静态目录前，确保存在运行时官方 provider；如果配置中没有该条目则补入，如果存在同名自定义条目则只在该订阅路由的临时副本中替换为官方条目。
- 该替换不得写回 TOML，也不得影响 `new_api_byok` 路由读取的同名自定义 provider。
- 动态目录成功时沿用账号目录；目录不可用时仍使用现有四模型静态兜底。
- 当目录包含 `deepseek-v4-flash` 时优先保留它作为 provider 默认模型；目录不包含时沿用现有“选择首个可用模型”的回退。
- 登录、刷新模型、创建新会话和实际发送均继续通过既有订阅鉴权和 MaaS 凭据链路。

### 设置界面

- 后端对内置 `kwu` 的 `ProviderView.BaseURL` 返回空字符串；自定义 provider 的地址继续返回。
- 设置模型详情不再把订阅目录写入可编辑 provider 列表：动态账号目录只装饰独立的 `OfficialProviders` 卡片；配置中的精确官方条目从普通列表跳过，同名不同地址的自定义项仍进入普通列表。
- 后端只有在 provider ID 为 `kwu` 且条目已由精确地址规则标记为 `BuiltIn` 时才施加锁定和脱敏；不得再仅凭名称把自定义 `kwu` 变成内置卡片。
- 前端只使用后端 `builtIn`/`locked`/provider ID 等非地址元数据识别内置卡片，删除官方现网和历史地址常量。
- 内置卡片不渲染地址行；刷新模型按钮不能因为地址为空而被禁用，仍调用专用官方刷新接口。
- 自定义卡片继续显示地址并保持配置、刷新、删除行为。
- 前端设置状态显式合并 `officialProviders` 与 `providers`：卡片按 `builtIn` 分组，模型选项按 `ref + billingSource` 建路由。官方与同名自定义 `kwu` 可以同时存在，不按名称去重，也不让一个来源的权限覆盖另一个来源。

## 方案

### 1. 收口官方 provider 判定与 TOML 渲染

- 复用 `officialProviderKind`/`sameEndpoint` 的精确判定，不新增仅按 provider 名称删除的宽泛规则。
- 提供包内可复用的小型谓词或等价现有 helper，使升级、渲染和 Desktop 运行时使用同一官方定义。
- `RenderTOMLForScope` 和 Project renderer 在输出 provider 前过滤精确官方条目。过滤只作用于输出，不原地修改 `Config.Providers`。
- 添加 User、Full、Project 三种 scope 的回归测试：官方现网/历史条目不输出；同名自定义地址和其他 provider 完整输出。

### 2. 调整默认模型和官方内存目录

- 将 `kwuOfficialDefaultModel` 改为 `deepseek-v4-flash`。
- 将通用内存官方目录精确调整为 `deepseek-v4-flash`、`gpt-5.4`、`gpt-5.4-mini`，使新默认可解析并兼容既有 GPT 引用；这不是 Desktop 四模型失败兜底。后者仍由 `desktopAirClaudeSubscriptionModels` 独立拥有，四个值和顺序保持不变。
- 收窄 `isDeprecatedKWUFallbackModel` 及相邻规范化规则，避免把新的官方默认 `deepseek-v4-flash` 再迁回 GPT；不借此改变 `deepseek-v4-pro` 的历史通用配置迁移语义。
- 更新根默认、ResolveModel 和官方配置规范化测试，明确新默认可解析。
- 在两条 provider 文件替换默认列表的既有加载边界调用同一个窄物化 helper：`Load`/`LoadForRoot` 在 `mergeTOMLProviders` 替换后调用，edit loader 在 `replaceProvidersFromExistingEditFile` 后调用。当根/agent 等当前配置引用 `kwu`、文件 provider 列表不存在任何同名项时，补入官方运行时模板。该规则不写文件、不覆盖同名自定义项，也不引入新的 provider 注册框架。
- 分别增加 `Load`/`LoadForRoot` 和 `LoadForEditWithoutCredentials` 往返测试：“保存一个其他名称的自定义 provider 后重新加载，根 Flash 默认仍可解析且再次保存仍不输出官方块”；这同时冻结 CLI/共享 loader 与无凭据 edit loader 合同。

### 3. 扩展一次性升级迁移

- 保留现有启动时机和无凭据 edit loader，把 `ResetOfficialProviderPricingOnUpgrade` 收口为能同时执行既有价格维护与版本 4 provider/default 迁移的升级函数；调用点和测试按最终名称同步。
- 在加载默认值前先从旧文件 header 记录是否存在精确官方条目、是否同时存在同名自定义条目及根默认值，防止加载期补全掩盖原始意图。
- 保存前从配置中删除精确官方条目；只有旧 header 证明根 `kwu/gpt-5.4` 指向该官方条目且不存在同名自定义条目时迁移根默认。
- 通过原子 Save 路径落盘。损坏 TOML 返回错误且不覆盖原文件；不存在的文件不创建。
- 测试覆盖现网地址、历史地址、尾斜杠等价、混合 provider、同名自定义 provider、二次运行、损坏文件和登录/凭据路径零接触边界。

### 4. 在订阅路由配置副本中注入运行时 provider

- 修改 `applyDesktopAirClaudeSubscriptionCatalogModels` 或其紧邻 helper，使其先在传入配置副本内 upsert 官方 provider，再应用归一化模型目录。
- 该 helper 只由 `subscription_pool` 路径调用；BYOK loader 不调用。用测试证明同一个磁盘配置在订阅副本中得到官方 `kwu`，在 BYOK 副本中仍得到自定义 `kwu`。
- 当授权目录为空而现有逻辑要求失败关闭时，继续清空临时官方目录；当调用显式静态兜底入口时仍得到原四模型。不得把静态兜底扩大到新的失败分支。
- 更新新会话默认、模型解析、任务运行时和刷新相邻测试，证明不依赖 TOML 中存在官方 provider。
- real 与 fake/fixture 的 `subscription_pool` 构造都必须得到运行时官方 provider。fake 可以继续由 fixture 提供目录和网关，但不得偶然依赖 TOML 中残留官方块。

### 5. 在后端 DTO 和前端展示边界脱敏

- `decorateDesktopProviderView` 对内置 provider 清空 `BaseURL`；`officialProviderTemplate` 可继续在后端构造运行时条目，但投影到前端前必须经过同一脱敏。
- `settingsModelDetailsFromConfig` 不再为展示而修改传入的可编辑 provider 目录；官方动态模型通过既有订阅状态直接装饰 `OfficialProviders`，普通 `Providers` 过滤精确官方条目并保留同名自定义条目。
- `decorateDesktopProviderView` 必须同时检查 `view.BuiltIn` 和官方 provider ID；同名自定义条目保持 BYOK、可编辑和可见地址。
- `providerDisplay` 不再读取或导出官方 URL 常量，改用现有的内置标记和 provider ID。
- `SettingsPanel` 构造一份仅用于卡片和模型选择的合并 provider 视图，保留原始 `providers` 作为可编辑自定义目录；模型候选直接从合并视图产生 `ref + billingSource`，订阅权限过滤按该路由身份选择对应 provider，不能再用 provider 名称唯一匹配。
- 内置卡片在空地址时仍可刷新；自定义 provider 的空地址校验和按钮禁用规则保持不变。
- 更新设置快照、渐进加载、订阅模型访问等前端夹具，确保前端目录中不再出现两个敏感地址字面量。

## 受影响合同与主要文件

- 配置默认与规范化：`internal/config/config.go`、`internal/config/kwu_official.go`、`internal/config/load.go`、`internal/config/load_runtime.go`。
- TOML 持久化：`internal/config/render.go`、相邻 render/edit/backfill 测试。
- 一次性升级：`internal/config/pricing.go`、`desktop/app.go`、相邻升级测试。
- Desktop 订阅路由：`desktop/model_billing.go`、`desktop/model_billing_test.go` 及必要的路由测试。
- 设置 DTO：`desktop/settings_app.go`、`desktop/settings_app_test.go`。
- 前端识别与卡片：`desktop/frontend/src/lib/brand.ts`、`desktop/frontend/src/features/settings/ui/SettingsPanel.tsx`、`desktop/frontend/src/features/settings/ui/ModelsSettingsSection.tsx` 及相邻测试。

不新增 Wails 公共方法，不改变 `ProviderView` JSON 字段形状；`baseUrl` 对内置卡片从真实值变为空字符串，是本次唯一展示合同变化。

## 风险与控制

### 订阅 provider 消失导致不可用

风险路径是 TOML provider 列表替换内存默认列表后，目录注入找不到 `kwu`。控制措施是订阅路由 helper 必须 upsert 运行时官方条目，并以“磁盘无官方 provider”的配置测试实际解析和任务初始化。

通用 Load 还有一条独立风险：文件只包含其他自定义 provider 时会替换默认列表。控制措施是在既有替换边界按真实 `kwu` 引用且无同名 provider 的条件物化官方模板，并增加保存/加载/再次保存往返测试；Desktop 注入不能替代该合同。

### 同名自定义 provider 被覆盖

持久化迁移和渲染只匹配名称加精确官方地址；同名不同地址保留。只有 `subscription_pool` 的临时配置副本可覆盖同名条目，显式 BYOK 副本和磁盘文件不得改变。设置模型详情不得复用该临时订阅副本覆盖可编辑列表。

### 新默认被旧规范化逻辑回退

`deepseek-v4-flash` 当前被通用逻辑标为历史 fallback。实现必须同步收窄该规则，并通过 Default、加载、保存、升级、订阅目录五条路径验证新默认保持不变。

### 隐藏地址造成刷新按钮失效

当前前端用 `baseUrl` 参与刷新按钮可用性判断。内置卡片改为基于专用官方刷新能力判断，自定义 provider 仍要求有效地址。

### 官方与同名自定义路由歧义

当前前端按 provider 名称查唯一项。实现必须把设置候选提升为既有 `ref + billingSource` 路由身份，并让卡片通过 `builtIn` 分组；不得按名称去重或猜测来源。

### 安全预期过高

验收和发布说明必须明确这是减少无意暴露，不是客户端保密。只要 Desktop 直连，地址可被高级用户获得；任何服务端安全判断不得依赖地址不可见。

## 兼容与回滚

- 配置版本 4 只删除可由程序重新生成的官方内置 provider，并精确迁移对应根默认；没有数据库迁移。
- 登录状态、账号绑定、凭据、会话和工作区数据格式不变，无需退出登录或重新登录。
- 回滚代码后，已迁移配置不会自动恢复官方 provider 块；旧版 `Config.Default()` 在无文件 provider 时可补默认，但若旧版依赖持久化块，回滚前应恢复到版本 3 应用并用其设置保存。发布回滚说明必须记录这一限制。
- 服务端若需要紧急阻断，优先撤销 MaaS 凭据/账号授权或停用接口，不依赖客户端回滚。

## 验收标准

1. 新 profile 首次写出的 `config.toml` 包含 `default_model = "kwu/deepseek-v4-flash"`，不含官方现网/历史 API 地址，也不含内置 `[[providers]] name = "kwu"` 块。
2. 版本 3 官方配置升级后满足同一结果，登录状态文件内容和登录 UI 状态保持不变。
3. 混合配置升级只删除精确官方条目；同名不同地址、自定义模型、其他 provider 和非根模型引用保持不变；官方与同名自定义条目同时存在时不迁移根默认。
4. 升级运行第二次文件不再变化；损坏配置不被覆盖。
5. 未登录时内置卡片存在、无地址、显示既有未登录状态；自定义 provider 卡片仍显示地址，同名不同地址的 `kwu` 也保持普通 BYOK 卡片。
6. 已登录且目录成功时内置卡片显示账号动态模型，刷新按钮可用；新会话默认选中 `deepseek-v4-flash`（该模型存在时）。
7. 模型目录临时失败时沿用现有四模型兜底，四个模型值和顺序精确不变。
8. 从不含官方 provider 的磁盘配置启动，`subscription_pool` 仍能解析模型、完成鉴权并建立运行时；同一配置的 BYOK 路由不被订阅注入污染。
9. 前端生产源码和构建产物不包含官方现网/历史 API 基址；Wails 设置 DTO 的内置 `baseUrl` 为空。
10. 退出登录行为、会话恢复、planner/subagent/automation/bot 显式模型、自定义 provider 增删改保持既有契约。
11. TOML 只包含其他名称自定义 provider 时，通用 Load 后根 `kwu/deepseek-v4-flash` 仍可解析；再次保存仍不输出官方 provider。
12. `officialProviders` 含内置 `kwu`、`providers` 含同名自定义 `kwu` 时，两张卡片和两条 `ref + billingSource` 路由同时存在，订阅权限只筛选订阅路由。
13. 无活动会话时 effort 保持 `auto` 且保存配置不产生官方 provider、官方地址或新的 effort 字段；带 `tabId` 的会话级 effort 选择和恢复测试保持通过。

## 验证计划

### 聚焦自动化

- Root Go：从仓库根运行 `go test ./internal/config`，覆盖默认、渲染、加载规范化和版本 4 升级。
- Desktop Go：从 `desktop` 运行 `go test .`，覆盖启动迁移调用、设置 DTO、订阅配置注入、默认路由和四模型兜底。
- 前端聚焦测试：运行受影响的设置测试脚本，覆盖内置脱敏、刷新可用、自定义地址保留和非地址识别。
- 前端生产构建：从 `desktop/frontend` 运行 `pnpm build`。
- 字面量审计：对 `desktop/frontend/src` 和实际 `dist` 使用精确搜索，两个敏感 API 基址必须为零命中；后端源码命中属于已接受边界。

### 真实应用 A/B

- A 使用固定基线 `768d8fa2` 的独立 baseline worktree，从其 `desktop/` 以当前 dev 云地址和 profile `a6defe83d02e4e84` 串行运行 `wails dev`。启动前记录 profile 根和对应 cache/webview 根的文件清单；至少保存 `config.toml`、`cloud\v1\binding.json`、`cloud\v1\sessions.json`、`desktop-tabs.json`、`desktop-window.json`、`desktop-projects.json`、模型计费 SQLite 及其 sidecar 的大小、mtime 和 SHA-256，并在仓库外临时目录保留可恢复副本。
- A 重复基线步骤：账户菜单 > 设置 > 模型 > 接入；记录卡片地址、登录状态、根默认和 config provider 块。正常关闭 A，确认应用和 watcher 子进程退出、监听端口与单例锁释放；不手工删除未证明为 stale 的锁。
- 在应用完全停止后，把上述受影响状态文件从初始副本恢复；重新计算哈希并要求 `config.toml`、binding、sessions 及数据库与初始值一致。cache/webview 仅在观察到影响登录或设置状态时纳入恢复；不能证明等价则标记为 separate-environment comparison。
- B 从补丁 worktree 的 `desktop/` 使用相同有效配置和初始状态运行 `wails dev`：记录升级后登录仍有效、配置已精确清理、卡片无地址、模型刷新和订阅模型选择可用、默认 Flash。
- 目录失败兜底用受控测试或现有 fake/fixture 验证，避免人为破坏真实账号或线上服务。
- 如真实 AirClaude 或专用登录账号不可用，明确记录外部依赖限制，不以仅自动化结果宣称真实订阅调用已经验收。
- 当前基线 profile 的会话已失效。最终“有效登录不被清除”必须由用户重新登录该 dev profile 或提供专用测试账号后验证；在此之前只能证明 binding/sessions 文件未被迁移修改，不能宣称有效登录应用验收通过。
- B 通过后保留补丁应用和 `wails dev` 进程供用户手动验收，除非安全需要或用户要求关闭。

### 最终仓库门禁

- 按完整 NUL 分隔变更列表分类并运行受影响 Root Go、Desktop Go 和 frontend 检查。
- 运行仓库 `AGENTS.md` 规定的 code guard、baseline 不变检查、`git diff --check`。
- 运行 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`，检查分类摘要。
- 未获明确授权不创建或更新 PR。
