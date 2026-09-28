# 任务 001：注入 Desktop 产品身份信息

## 任务状态

| 项目 | 内容 |
| --- | --- |
| 状态 | 已完成 |
| 串行组 | Serial-01 |
| 依赖 | 已就绪的 [`../plan.md`](../plan.md) |
| 分配 | 新鲜实现子代理，`gpt-5.6-luna`，max reasoning，context fork none |
| PR | 未授权推送或创建 PR |

## 目标与交付物

让 Desktop 主会话从一次性系统 prompt 获得权威产品身份：中文名 `云智企办`、英文名
`Clouvia`、当前 Desktop `version`，以及“具体模型服务到设置→模型查看”的回答口径。

交付物：

- 一个数据安全、确定性的 Desktop 产品身份 policy；
- Desktop 版本从唯一 controller 构建边界传入 boot prompt 的最小链路；
- config、boot 和 Desktop 边界的聚焦测试；
- 现有模型切换回归测试与真实 Luna/Terra B 侧验收；
- 本文件更新为完成回执并单独提交。

## 依据

- 基线全新 Luna 会话把英文品牌猜成 `YunZhiQiBan`，版本回答无法确认。
- `internal/brand/brand.go` 是产品中英文名权威来源。
- `desktop/main.go:version` 是 Desktop 版本权威来源；`App.Version()` 已直接返回该值。
- `internal/boot/build_config.go:buildSystemPrompt` 是 main session 固定 policy 的装配点。
- `desktop/model_billing.go:buildDesktopController` 是唯一 Desktop controller 构建边界。
- 正常 Luna→Terra 路径和 `TestSetModelForTabRefreshesCarriedSystemPrompt` 已在基线通过。
- 保持 `gpt/newapi_byok` 的真实矩阵 Luna→Terra→Luna、Terra→Luna 均通过；对应两请求
  executor cache 统计为 `64,512` hit / `284` miss，user history 无身份块。

## 允许范围

生产代码：

- `internal/config/render_agent_header.go`
- `internal/boot/boot.go`
- `internal/boot/build_config.go`
- `desktop/model_billing.go`

测试与回执：

- `internal/config/system_prompt_test.go`
- `internal/boot/boot_test.go`
- `desktop/model_billing_test.go` 或现有更贴近唯一构建边界的 Desktop Go 测试文件
- 本任务合同/完成回执

## 禁止范围与保留行为

- 不修改 `internal/control/input.go` 或任何 user message compose、preview、save 路径。
- 不修改 `desktop/session_prompt.go`、`Controller.Resume`、scene 或历史消息替换规则。
- 不修改模型选择、provider、billing source、凭据、登录、云运行时或前端。
- 不修改 task/skill/planner/parallel task 等内部子代理 prompt 或版本传播。
- 不新增持久化字段、配置项、依赖、抽象框架或兼容分支。
- 不修改 `scripts/code_guard_baseline.json`，不推送、不创建或更新 PR。
- 保持自定义 system prompt、CLI 和未提供 Desktop 版本的其他 host 现有行为。

## 实施约束

1. 产品中英文名必须引用 `brand.DisplayName` 和 `brand.EnglishName`，不得复制硬编码常量。
2. Desktop 版本必须按不可信数据引用/转义；不得允许其形成 prompt 结构。
3. policy 仅在 boot options 提供非空 Desktop 版本时追加，且每个 controller 只在最终
   system prompt 尾部装配一次；不得插入 memory 或 skill index 之前。
4. 供应商规则不得把 provider 名、协议类型或 billing source 宣称为真实供应商；回答应自然
   引导用户去“设置”→“模型”查看，不以“我不知道”作为用户可见结论。
5. `buildDesktopController` 必须统一写入当前 `main.version`，不得在各调用点重复赋值。
6. 不触碰已经通过的模型切换实现；通过现有回归测试和真实应用 B 侧证明未回归。
7. 每个改动行必须能追溯到 AC-01 至 AC-06；提交前审计实际 diff 和禁止范围。

## 验收标准

- AC-01：policy 使用 `云智企办 / Clouvia`，明确禁止自创、翻译或音译官方品牌名。
- AC-02：Desktop 运行时版本进入一次性系统 prompt，`dev` 与发布 ldflags 值同源。
- AC-03：供应商问题不猜测、不暴露内部路由，指向“设置”→“模型”查看。
- AC-04：Luna→Terra 后当前模型回答更新为 Terra，不出现旧模型。
- AC-05：diff 中没有每轮身份注入、user message 改写或持久化 schema 变化。
- AC-06：CLI、其他 host、provider/billing、登录、Legacy scene 和子代理保持原行为。

## 聚焦验证

```powershell
go test ./internal/config ./internal/boot
go test . -run 'TestDesktop.*ProductIdentity|TestSetModelForTabRefreshesCarriedSystemPrompt' -count=1
```

实现子代理运行聚焦检查并审计 diff；主代理独立复核提交、测试和任务回执。主代理随后按
`plan.md` 执行 patched Wails dev 的 B 侧：全新 Luna 会话验证品牌、版本、供应商口径，
同会话切 Terra 验证当前模型，并完成项目 common checks 与 PR gate。B 通过后保持 patched
应用运行供用户手工验收。

## 提交要求

- 代码与测试：一个小型 `fix:` 提交；必要修正用新的 `fix:` 提交，不 amend。
- 验证完成后更新本文件的状态、实际变化、命令结果、偏差和剩余风险，使用独立 `docs:`
  提交。

## 完成回执

用户已批准固定、尾部、可缓存且不随轮次累计的 system prompt 方案。代码实现、独立复核、
仓库门禁和入口修正后的真实应用 B 侧均已完成，AC-01 至 AC-06 通过。

### 实际变更

- 在 `internal/config/render_agent_header.go` 增加 `DesktopProductIdentityPolicy`：直接引用
  `brand.DisplayName` 与 `brand.EnglishName`，明确禁止自创、翻译、音译官方品牌名；Desktop
  版本使用 `strconv.Quote` 作为数据，并将模型服务问题引导至“设置” > “模型”。
- 在 `boot.Options` 增加可选 `DesktopVersion`；主 `Build` 与 `buildAssembly` 均在 memory、skill
  index 装配完成后，仅在版本非空时把 policy 追加到最终 system prompt 尾部。
- 在 `buildDesktopController` 唯一边界统一写入当前 `main.version`，覆盖调用方传入值；没有
  修改各 Desktop controller 调用点。
- 增加 config policy 转义测试、boot 主/保留装配路径的尾部与空版本测试、Desktop 边界版本
  同源测试。实际改动文件仅为本合同允许的 7 个文件。

### 提交与验证结果

- 代码与测试提交：`326797db` (`fix:注入Desktop产品身份信息`)。
- 预算修正提交：`cb704a8f` (`fix:压缩Desktop产品身份策略`)，未 amend 原提交。
- 门禁复杂度修正提交：`bfba41dd` (`fix:消除Desktop身份策略复杂度分支`)，未 amend 原提交。
- UI 入口修正提交：`120933b0` (`fix:纠正模型供应商查看入口`)，未 amend 原提交。
- `go test ./internal/config ./internal/boot`：通过；`internal/config` 通过，`internal/boot`
  通过（包级运行约 123 秒）。
- `go test ./internal/boot -run 'TestBuildAppendsDesktopProductIdentityPolicy|TestBuildAssemblyAppendsDesktopProductIdentityPolicy' -count=1`：通过。
- 在 `desktop/` 执行
  `go test . -run 'TestDesktop.*ProductIdentity|TestSetModelForTabRefreshesCarriedSystemPrompt' -count=1`：通过。
- `DesktopProductIdentityPolicy("dev")` 实测为 373 UTF-8 bytes，`len(policy)/4` 估算为 93；
  新增预算测试直接断言 bytes `<= 480` 且估算 `<= 120`，通过。
- 本次入口修正后 `DesktopProductIdentityPolicy("dev")` 实测为 352 UTF-8 bytes，
  `len(policy)/4` 估算为 88；测试要求包含 `"设置" > "模型"` 并排除“模型广场”与
  `Model Marketplace`，通过。
- `go run ./scripts/code_guard`：通过；未修改 `scripts/code_guard_baseline.json`。
- 本次入口修正后 `go test ./internal/config` 全量通过，`go test ./internal/boot -count=1 -v`
  全量通过（72.286 秒），Desktop 聚焦测试通过。实现子代理的一次精确组合命令曾无 Go
  失败日志退出；主代理随后通过 PR gate 重新执行带 `-count=1` 的相同包组合，正常通过。
- `gofmt` 与 `git diff --check`：通过；diff 审计确认没有修改 user message、历史恢复、模型
  路由、provider/billing、登录、子代理、前端、依赖或 code guard baseline。
- `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`：通过；分类为 Root Go
  `./internal/boot ./internal/config` targeted、Desktop Go `.` targeted，无 frontend、release
  contract 或 gate contract 检查。
- `120933b0` 之前的 patched `desktop/` B 侧曾以共享 dev profile 运行 `wails dev`。全新
  Global 会话选择 `gpt/gpt-5.6-luna` 后，产品身份回答包含 `云智企办 / Clouvia`、版本
  `dev`，但供应商信息指向“模型广场”；这是本次入口修正前的失败证据，不是 AC-03 通过
  结果。同会话等待空闲并确认选择器切至 `gpt/gpt-5.6-terra` 后，当前模型回答为
  `gpt-5.6-terra`。
- B 侧后端 `ListTabs` / `MetaForTab` 确认最终 `modelRef=gpt/gpt-5.6-terra`、
  `billingSource=newapi_byok`、`running=false`。持久化 session 的 system 消息包含产品身份
  policy 一次且仅含 Terra 模型身份；两条 user 消息均不含产品身份或 `runtime-identity` 块。
- 重新编译 `120933b0` 后的 patched Desktop，并在全新 Global Luna 会话询问“当前模型供应商
  信息在哪里查看？请只回答查看路径。”，实际回答为 `设置 > 模型`。该 session 的 system
  消息包含 `"设置" > "模型"` 且不含“模型广场”或 `Model Marketplace`；user 消息不含
  产品身份块，AC-03 通过。

### 偏差与剩余风险

- 初始 policy 文案超出已批准的 80-120 token 预算；审查后通过 `cb704a8f` 压缩措辞，保留
  官方双名、版本数据转义、四类供应商推断禁止和“设置” > “模型”引导，当前预算满足计划边界。
  门禁复核又发现 `build_config.go:227 buildSystemPrompt` 复杂度从 10 超至 11；通过
  `bfba41dd` 提取复用的空版本判断 helper 消除新增分支，保持两条 boot 路径的最终尾部行为
  不变。除此之外无实现偏差，改动仍严格限于任务合同允许范围。
- 未新增持久化字段或每轮身份块；系统 prompt 仅在 Desktop controller 创建时追加一次，版本
  变化会产生预期的尾部 cache miss。
- B 侧 GPT provider 的 usage 返回 `cacheHitTokens=0`、`cacheMissTokens=0`，因此该次请求没有
  可用的实际 cache hit 比例，不能据此宣称命中或未命中。源码装配、持久化 session 和回归
  测试仍证明 policy 位于稳定 system 尾部且不随轮次累计；此前基线同 provider 会话曾正常
  上报 `64,512` hit / `284` miss。
