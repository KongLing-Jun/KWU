# 产品身份基础信息修复计划

## 1. 基线与失败证据

- 实现基线：`origin/main-v3@768d8fa2711c7fa0b48460677aa78037806fd053`。
- 环境：Windows 开发配置，`wails dev`；模型使用已配置的 `gpt` BYOK provider。
- 全新会话选择 `gpt/gpt-5.6-luna`，询问中英文官方品牌名，实际回答
  `云智企办 / YunZhiQiBan`；仓库权威常量是 `云智企办 / Clouvia`
  （`internal/brand/brand.go`）。失败路径已在未修改基线上复现。
- 另一全新 Luna 会话询问品牌、版本和供应商入口，实际知道中文名，但回答版本
  “本次会话未提供，无法确认”，供应商入口仅猜测为“设置或关于/模型信息”。
- 同一原始分支保持 `gpt` provider 和 `newapi_byok` 不变，完成两组真实矩阵：全新 Luna
  会话按 Luna→Terra→Luna 切换，全新 Terra 会话按 Terra→Luna 切换；每次均回答当前
  模型，后端 `modelRef` 与 Composer 最终值一致。现有自动化
  `TestSetModelForTabRefreshesCarriedSystemPrompt` 及相邻 route 测试重复 5 轮通过。因此正常
  模型切换不是本次缺陷，也不是从官方 provider 切到 GPT 造成的假象。
- 上述全新 Terra→Luna 会话的 executor 两次请求合计输入 `64,796` tokens，其中
  `64,512` cache hit、`284` miss；持久化历史只有一个最终 Luna system prompt，user
  message 中没有 runtime identity。该证据说明当前稳定系统前缀可命中缓存，不能解释为
  不同模型共享同一 cache entry。

开发环境登录失效是独立现象：服务端鉴权返回 `AUTH_REQUIRED` 后，本地 durable session
被持久化为 `expired`。开发与正式 profile 的目录、环境 identity、会话文件和 device ID
相互隔离；没有证据表明正式版并行运行导致开发版失效。当前客户端证据无法继续区分
refresh 被拒、会话被撤销或 access token 被拒，需要服务端鉴权日志。GPT BYOK 不依赖该
云登录，本次基线调用已实际成功。

## 2. 根因与边界

- `DefaultSystemPrompt` 只有中文 `brand.DisplayName`，没有 `brand.EnglishName`，也没有禁止
  自创、翻译或音译官方品牌名的约束，因此模型会补出 `YunZhiQiBan` 等名称。
- Desktop 构建版本由 `desktop/main.go` 的 `version` 提供并通过 ldflags 注入，但
  `boot.Options` 和系统 prompt 未接收该值，因此模型无法知道当前软件版本。
- `ProviderEntry`、`ModelRef` 和 `BillingSource` 表示接入配置与计费路由，不能作为模型
  真实供应商的权威事实。设置中心已有“模型”页，应引导用户自行查看，
  而不是让模型猜测、暴露内部 provider 前缀或生硬声明“不知道”。

## 3. 目标与非目标

### 目标

- Desktop 主会话能稳定回答官方中文名 `云智企办`、英文名 `Clouvia`，不再创造别名。
- 回答当前软件版本时只使用 Desktop 运行时 `version`（开发构建为 `dev`，发布构建为
  ldflags 注入值）。
- 用户询问模型供应商、服务商或来源时，不猜测具体供应商，也不以“不知道”结束；引导
  用户在应用的“设置”→“模型”查看当前模型服务信息。
- 上述内容只进入 controller 创建时组装的系统 prompt；正常切换 Luna/Terra 后仍回答
  当前模型，不回退为旧模型。

### 非目标

- 不改模型选择、provider 配置、`ModelRef`、`BillingSource`、凭据或云登录逻辑。
- 不在每轮 user message 注入身份块，不新增会话持久化字段，不累计上下文 token。
- 不修改 `resumeWithFreshSystemPrompt`、Legacy scene 兼容语义或历史消息。
- 不给内部 task、skill、planner、parallel task 等子代理新增 Desktop 版本传播。它们的输出
  由主会话整合，不直接承担用户可见的产品身份回答。
- 不修复本次观察到的开发云登录失效；该问题需要服务端日志才能继续定因。

## 4. 方案与影响范围

1. 在 config 层增加一个只接受 Desktop 版本字符串的产品身份 policy。名称和英文名直接
   引用 `internal/brand` 权威常量；版本按数据引用；供应商回答只规定用户查看入口。
2. 给 `boot.Options` 增加可选 Desktop 版本字段。仅字段非空时，在 memory、skill index
   等现有装配完成后，把上述 policy 追加到最终 system prompt 尾部。这样既不影响 CLI
   和其他未提供版本的 host，也为旧缓存保留最长公共前缀；自定义 system prompt 不能
   意外删除该产品事实。
3. 在 `buildDesktopController` 这一唯一 Desktop controller 构建边界写入 `main.version`，
   覆盖首次启动、设置重建、模型切换和 bot runtime 等现有 Desktop 构建入口。
4. 增加 config/boot/Desktop 聚焦测试，并复跑现有模型切换系统 prompt 刷新测试。

预计生产代码只涉及：

- `internal/config/render_agent_header.go`
- `internal/boot/boot.go`
- `internal/boot/build_config.go`
- `desktop/model_billing.go`

测试仅放在对应现有测试文件；不新增依赖或通用身份框架。

## 5. 验收标准

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | 全新 Desktop 会话询问中英文官方品牌 | 回答 `云智企办 / Clouvia`，不出现 `CloudWise`、`YunZhiQiBan` 等自创名称 |
| AC-02 | 询问当前软件版本 | 开发构建回答 `dev`；发布构建使用同一 `main.version` 权威值 |
| AC-03 | 询问具体模型供应商/服务商 | 不猜测具体公司或从 provider/protocol 推断；自然引导到“设置”→“模型”查看 |
| AC-04 | Luna 会话切换到 Terra 后询问当前模型 | 依次回答 `gpt-5.6-luna`、`gpt-5.6-terra`，不回答旧模型 |
| AC-05 | token 与持久化 | 没有每轮身份块、user message 改写或新增持久化字段；只有固定系统 prompt 增量 |
| AC-06 | 范围 | provider/billing、登录、切换历史、Legacy scene、子代理和 code guard baseline 均不变 |

## 6. 风险、替代与回滚

- 固定 system prompt 尾部预计增加约 `80-120` 个输入 token；它会计入每次请求的 prompt
  tokens，但稳定后可作为 cache hit，且不会按轮次在历史中累计。首次使用新 policy 或
  版本变化时新增尾部可能 miss，之后保持稳定。拒绝旧分支的每轮 runtime identity 注入。
- 让 Desktop 截获自然语言身份问题并直接渲染答案可以做到零模型 token，但会新增意图
  识别和对话一致性分支；相对约百个固定可缓存 token 风险更大，本次拒绝。
- 直接把 provider 名称告诉模型会把配置别名误当成真实供应商，也可能暴露内部路由；
  因此只提供查看入口。
- 修改会话恢复 helper 能覆盖 Legacy session，但会改变已存在的兼容语义，并触及已经在
  正常路径通过的模型切换；没有真实失败证据，本次拒绝该扩张。
- 若产品回答出现回归，可整体回退产品 identity policy、boot 字段和 Desktop 边界赋值；
  无数据迁移或清理。

## 7. 验证

聚焦检查：

```powershell
go test ./internal/config ./internal/boot
go test . -run 'TestDesktop.*ProductIdentity|TestSetModelForTabRefreshesCarriedSystemPrompt' -count=1
```

应用 A/B 串行使用同一开发 profile、同一 GPT BYOK 配置和等价新会话：A 为上述未修改基线
证据；B 从 patched worktree 的 `desktop/` 运行 `wails dev`，用 Luna 验证品牌、版本和
供应商口径，再在同一会话切到 Terra 验证当前模型。B 通过后保留 patched 应用运行供用户
手工验收。

最终按实际 diff 运行项目 common checks 和：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

未获得 PR 授权前不推送、不创建或更新 PR。

## 8. 阶段门

根因、外部行为、范围、兼容性和回滚已由源码、自动化与真实基线验证收敛。用户已批准
固定、尾部、可缓存且不随轮次累计的系统身份方案；实施中只有证据表明上述边界错误或
不足时才重新确认。
