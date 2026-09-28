# 任务 002：下沉宿主 artifact 验证

## 目标与产物

把 artifact regular-file 验证从模型主动 `verify_artifact` 调用迁移到 final 边界的宿主内部
步骤。模型工具面不再默认暴露 verifier；成功验证只写入 host evidence ledger。

## 允许范围

- `internal/agent/agent.go`、artifact readiness 和 receipt helper
- `internal/boot` 的 verifier 构造、注入和工具面
- `internal/config` 的 artifact delivery policy
- 必要的 Agent、boot、expert-team 和 token-profile 测试
- 本任务回执和 `../progress.md`

保留 `internal/tool/builtin/verifyartifact.go` 的 confinement/canonicalization 合同。不得让内部
验证伪装成 assistant ToolCalls，也不得绕过后台任务、最新 mutation 或 distinct matching。

## 实施与验收

- final candidate 前收集命名 requirement 路径和 writer receipt 路径；
- 候选逐个内部验证，成功结果记录为 verifier receipt；
- 未结束后台任务时不验证；settle 后 verifier 必须新于 settlement boundary；
- 普通和 Token Economy registry 都不向模型暴露 verifier；
- 文件不存在、目录、越界路径、未知匿名 shell 输出保持 fail closed；
- 兼容直接构造 Agent 的现有测试/调用方。

聚焦验证：

```powershell
go test ./internal/agent ./internal/boot ./internal/tool/builtin -run 'Artifact|Verifier|FinalReadiness|TokenProfile|ExpertTeam' -count=1
```

## 状态与回执

| 项目 | 值 |
| --- | --- |
| 状态 | 已完成 |
| 修改文件 | `internal/agent`、`internal/boot`、artifact delivery policy、A2A 协议及测试 |
| 测试结果 | 受影响四包全量测试及 config 测试通过 |
| 偏差与风险 | 不扫描 workspace；无结构化路径的匿名 shell 输出仍保持 fail-closed |

实现结果：

- `Agent.Options` 接收独立的 host-owned read-only verifier；
- 普通和 Token Economy model registry 均过滤 `verify_artifact`，即使显式配置也不暴露；
- 无工具 final candidate 先处理 background settlement，再逐个验证命名 targets 和成功 writer
  receipts，成功结果只进入 evidence ledger；
- session 和 provider 请求中没有伪造 verifier ToolCall/ToolResult；
- expert-team lead 由宿主验证 child handoff 路径，子 Agent 继续只负责写入和返回路径；
- 验证失败、缺少候选和验证后 mutation 均保持 hard block/freshness 语义。

验证：

```text
go test ./internal/agent ./internal/evidence ./internal/tool/builtin ./internal/boot -count=1
go test ./internal/config -count=1
```

均通过。
