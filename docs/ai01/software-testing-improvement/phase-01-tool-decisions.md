# Phase 1 测试工具采用决定

> 状态：Accepted for current architecture
>
> 决定日期：2026-07-23
>
> Owner：`@SivanCola`

本文对应 T1.5、T1.6。结论是当前不新增测试框架或容器依赖；这是采用门评估
后的明确拒绝，不是遗漏 PoC。

| 工具 | 决定 | 仓库证据 | 重新评估触发条件 |
|---|---|---|---|
| `testify` | 不迁移现有测试 | 当前约 566 个 Root/Desktop Go 测试文件以 `testing`、table test、小 fake 为主；本次 HTTP 缺口无需新依赖 | 重复断言已显著妨碍可读性，且单包 PoC 能减少代码而不隐藏失败 |
| `mockery` | 不引入生成 mock | 现有 seam 多为调用方最小接口或手写 fake；无需治理生成物和版本 | 大量稳定接口的手写 mock 已产生可测量维护成本 |
| Pact Go | 不 PoC | 客户端和当前云契约在同一仓库治理范围内；未发现可稳定运行的独立 provider verification CI | 服务端独立发布、schema 漂移已有真实缺陷、provider CI 与 broker owner 均落实 |
| go-vcr | 不 PoC | `httptest` 与合成 fixture 已覆盖主要 HTTP 契约，仓库不需要保存真实流量 | 第三方协议无法用手写 fixture可靠复现，且专用账号、强脱敏、过期策略齐备 |
| Testcontainers | 不 PoC | 生产持久化以嵌入式 SQLite 和文件为主，真实数据库测试可在进程内运行 | 新增 PostgreSQL、Redis、队列或对象存储且协议/事务无法由本地 fixture 证明 |

## 约束

- 不把 `go.sum` 中由传递依赖带入的 `testify` 视为项目已采用。
- 不生成只有 consumer pact、没有 provider verification 的伪闭环。
- 不把客户凭据、手机号、设备标识、支付信息或模型内容写入 cassette。
- 政企离线发布验收不得依赖 Docker；即使未来 CI 使用 Testcontainers，也要
  保留无 Docker 的发布复核路径。
