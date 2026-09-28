# 模型身份回答约束修复计划

## 1. 背景与问题

用户在多个可选模型中发送“你好，你是什么模型”时，模型可能统一自称 Claude。
当前内置 system prompt 只声明产品助手身份“云智企办”，没有把当前会话绑定的模型
名称提供给模型，也没有禁止模型根据训练背景、回答风格或兼容协议猜测身份。

本修复采用最小方案：将启动时已经解析出的裸模型名写入每个会话的固定 system
prompt，并要求涉及模型身份的问题只能按该绑定名称回答。

## 2. 当前行为与期望行为

### 当前行为

- `boot.Build` 将 `Options.Model` 或默认模型解析为 `ProviderEntry`。
- system prompt 的组装不使用解析后的 `entry.Model`。
- 默认提示词声明“你是云智企办”，但没有区分产品身份和模型身份。
- 桌面切换模型时会创建新 Controller，以新 system prompt 替换旧 system prompt，
  再保留其余对话历史。
- 本地绑定模型只表示本次会话请求使用的模型选择，不证明远程网关实际路由结果。

### 期望行为

- system prompt 明确给出当前会话绑定的裸模型名，例如 `deepseek-v4-flash`。
- 用户询问“你是什么模型”“底层模型是什么”“当前模型”等身份问题时，助手只按
  该裸模型名回答，不输出 `kwu/` 等 Provider 前缀。
- 助手不得根据自身训练认知、回答风格、Provider 类型或 OpenAI/Anthropic 兼容协议
  猜测成 Claude、GPT 或其他模型。
- “云智企办”继续作为产品助手身份，但不得当作底层模型名称。
- 不把绑定名称描述为已经验证的远程实际路由型号。

## 3. 基线与源码证据

- 计划基线：`origin/main-v3@2a49def2a5f066215b890de6d4505c76c4c31c8a`。
- Codegraph 在该主工作区基线上用于定位活动调用路径、数据流和影响面；独立 worktree
  使用实际源码搜索与 Git diff 复核。
- `internal/boot/boot.go:192-204` 从 `Options.Model`/默认模型解析 `entry`，并同时得到
  `entry.Model` 与包含 Provider 前缀的 `modelRef`。
- `internal/config/config.go:1284-1315` 证明 `ResolveModel` 支持完整引用、Provider 名和
  裸模型名，并将最终模型写入返回副本的 `Model` 字段。
- `internal/boot/boot.go:264-270` 读取基础 system prompt；活动组装闭包位于
  `internal/boot/boot.go:330-369`，当前没有注入模型身份。
- `internal/agent/session.go:41-48` 将组装结果保存为会话首条 system 消息。
- `desktop/app.go:7203-7240` 将桌面目标模型规范化为 `provider/model`；
  `desktop/app.go:7307-7352` 使用目标模型重建 Controller，并通过
  `resumeWithFreshSystemPrompt` 保留用户对话但刷新 system prompt。
- `desktop/app_test.go:2238-2309` 已覆盖切换模型时 system prompt 被刷新，可扩展为新旧
  模型身份断言。
- `internal/boot/build_config.go` 的 `buildAssembly` 当前是未接入活动 `Build` 的重构
  候选；实现不得只修改该路径而遗漏 `internal/boot/boot.go`。

## 4. 目标与非目标

### 目标

- 所有通过活动 `boot.Build` 创建的会话获得当前裸模型名身份约束。
- 内置和用户自定义基础 system prompt 都追加同一约束。
- 新会话、恢复会话、模型切换后的会话均使用对应绑定模型名。
- 模型名称作为数据安全编码，避免换行或引号破坏提示词结构。
- 增加聚焦测试证明内容、顺序、Provider 前缀隔离和切换刷新。

### 非目标

- 不改变模型选择器、Provider 目录、计费路由或请求中的 `model` 字段。
- 不声称能够证明订阅网关最终调用了哪个远程模型。
- 不新增模型身份查询工具、网络探测、前端提示或服务端路由元数据。
- 不修改历史 assistant 回答，也不扫描或重写既有会话文件。
- 不改变 Expert、Memory、Skills、场景、语言及其他现有 prompt 策略。
- 不把 Planner、Subagent 等派生调用的模型身份混入主会话 system prompt。

## 5. 用户场景与验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 新会话绑定 `kwu/deepseek-v4-flash` | system prompt 只声明裸模型名 `deepseek-v4-flash`，身份策略中不出现 `kwu/deepseek-v4-flash` |
| AC-02 | 用户询问当前模型 | system prompt 要求按绑定裸模型名回答，不得猜测 Claude、GPT 等其他身份 |
| AC-03 | 自定义基础 system prompt | 自定义内容保留，并追加同一模型身份约束 |
| AC-04 | 模型由完整引用、Provider 名或裸名称解析 | 均使用 `ResolveModel` 后的 `entry.Model`，不使用未经解析的输入 |
| AC-05 | 从 `old-model` 切换到 `new-model` | 新 Controller 的 system prompt 包含 `new-model`、不再包含旧身份；其余用户历史保持不变 |
| AC-06 | 会话恢复或场景/Expert 导致 prompt 重建 | 仍使用创建该 Controller 时绑定的同一裸模型名 |
| AC-07 | 模型 ID 含引号、反斜杠或换行 | 注入值被确定性编码，不能改变策略段落结构 |
| AC-08 | 既有 prompt 行为 | 决策、文件交付、时间、语言、Memory、Skills 和场景策略保持原顺序与内容 |
| AC-09 | Provider 请求 | 请求模型、路由、计费来源及工具 schema 不变 |

## 6. 选定方案

### 6.1 独立的动态模型身份策略

- 在配置/prompt 策略层提供接收裸模型名的构造函数，而不是把动态值写进
  `DefaultSystemPrompt` 常量。
- 将模型名编码为明确的数据字面量；策略文字说明它是“当前会话绑定名称”，不是远程
  路由证明。
- 策略要求模型身份问题使用该值回答，禁止从自身认知、风格、Provider 或兼容协议
  推断其他名称。
- 策略明确区分“云智企办”产品身份与模型身份。

### 6.2 在活动 system prompt 组装点注入

- `boot.Build` 成功执行 `ResolveModel` 后使用 `entry.Model` 构造策略。
- 在活动 `buildSystemPrompt` 闭包中将该策略作为固定、缓存稳定的 system 内容追加；
  不放进每轮 user 消息。
- 同一闭包负责场景和 Expert 导致的 prompt 重建，因此无需在 Controller 再保存一份
  可变模型状态。
- 同步维护重构候选 `buildAssembly` 的等价契约，或在任务阶段先确认其仍为测试约束；
  任何情况下活动 `boot.Build` 是接受标准的来源。

### 6.3 测试策略

- 在 `internal/boot` 使用自定义 Provider 名与不同裸模型名，断言只注入解析后的裸名。
- 复用自定义 system prompt 测试证明固定策略不会被用户 prompt 绕过。
- 扩展桌面模型切换测试，断言新 system prompt 刷新模型身份且用户消息保持不变。
- 对模型名编码函数增加特殊字符表驱动测试。

## 7. 影响范围、兼容性与迁移

预计影响：

- `internal/config/` 中的模型身份 prompt 构造与单元测试。
- `internal/boot/boot.go` 活动 system prompt 组装及聚焦测试。
- `internal/boot/build_config.go` 仅在保持重构候选契约确有必要时同步。
- `desktop/app_test.go` 的模型切换 system prompt 回归断言。
- 后续 Stage 2 任务合同与完成回执。

不新增配置项、持久化字段、依赖或数据迁移。所有现有模型引用解析和 Provider 请求保持
兼容。既有会话在恢复、切换模型或其他正常 Controller 重建后获得新约束；不原地修改
旧 JSONL 文件。

## 8. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 把 Provider 前缀暴露给用户 | 只接受 `ResolveModel` 后的 `entry.Model`，测试明确拒绝完整引用 |
| 自定义模型 ID 破坏提示词结构 | 使用确定性数据编码并覆盖特殊字符测试 |
| 将绑定模型误称为上游实际路由 | 策略和用户文案均限定为“当前会话绑定名称” |
| 只改到未接入的 `buildAssembly` | 活动 `boot.Build` 端到端测试是完成门槛 |
| 模型切换仍携带旧身份 | 扩展现有 `TestSetModelForTabRefreshesCarriedSystemPrompt` |
| 固定 prompt 改动降低缓存命中 | 每个 Controller 内模型名稳定；仅模型切换时创建新的稳定前缀 |
| 误影响 Planner/Subagent 身份 | 仅描述主会话绑定模型；派生调用不扩入本次范围 |

## 9. 回滚

变更不涉及数据迁移。回滚模型身份策略构造、system prompt 追加点和对应测试即可恢复
现状；模型选择、会话历史、Provider 路由和持久化格式不需要恢复操作。

## 10. 验证计划

### 聚焦自动化

- `go test ./internal/config -run 'TestModelIdentityPolicy' -count=1`
- `go test ./internal/boot -run 'TestBuild.*ModelIdentity' -count=1`
- `go test ./desktop -run 'TestSetModelForTabRefreshesCarriedSystemPrompt' -count=1`
- 对受影响 Go package 至少运行完整 package 级 `go test`。
- 检查 `git ls-files --eol`、实际 diff 和 `git diff --check`。

### 应用验收

- 在 Desktop 新建会话，依次选择至少两个裸模型名不同的模型。
- 每次发送“你好，你是什么模型”，确认回答与当前模型选择器显示的裸模型名一致。
- 切换模型并保留当前对话，再次询问，确认回答更新为新模型名且不继续复述旧身份。
- 覆盖订阅池与 BYOK 各一个模型；只验证绑定名称，不把结果当作上游路由证明。
- 确认普通办公请求、Expert、场景和工具调用行为未发生可见变化。

### PR Gate

获得 PR 授权后，先同步最新 `origin/main-v3`，再执行仓库规定的 common checks 与：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

任何必需检查失败时不得创建或更新 PR。

## 11. 后续关口

计划获批并合并后，从最新 `main-v3` 创建 Stage 2 独立 worktree，将实现拆为一个可独立
评审、测试、合并和回滚的任务合同。任务合同合并并获得实现授权后才进入运行时代码
修改。当前无未决产品、兼容性、迁移或外部 API 决策。
