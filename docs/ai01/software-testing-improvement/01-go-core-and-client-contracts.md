# Phase 1：Go 核心与客户端契约

## Goal

先保护政企桌面端最关键的 Go 行为：状态机、持久化、文件系统、权限边界、外部 HTTP、离线恢复和错误脱敏。默认使用 Go 标准库和项目现有 seam，不为了统一风格重写已有测试。

## Task 1.1：按风险审计核心包

**Goal**：识别“失败会导致数据损坏、越权、不可恢复或现场不可用”的 Go 路径。

P0 候选：

- Session JSONL/meta 写入、崩溃恢复、迁移和删除。
- SQLite 事务、锁、重复请求、幂等和 schema 兼容。
- 账号、订阅、模型选择、审批和 actor/profile 隔离。
- 文件导入导出、路径规范化、符号链接、中文路径和超大输入。
- jobs、事件、取消、关闭、重启和并发状态迁移。
- 更新清单解析、签名校验、下载失败和旧版本兼容。

每个边界记录：正常路径、错误路径、历史缺陷、恢复动作、审计字段和 owner。

## Task 1.2：标准库优先的测试规范

**Goal**：降低依赖和生成物成本，同时保持失败信息清楚。

- 默认使用 `testing`、table-driven tests、`t.TempDir()`、`httptest` 和小型手写 fake。
- 优先测试可观察行为和不变量，不绑定私有实现。
- 时间、随机、文件系统、网络和外部进程通过现有接口或最小 seam 控制。
- 不为了 mock 而新增宽泛接口；接口属于使用方并保持最小。
- 只有重复断言或 mock 维护成本被证明后，才 PoC `testify`/`mockery`。
- 生成 mock 必须固定版本、可重现、可审查，并进入 code generation 检查。

## Task 1.3：HTTP 与云服务错误契约

**Goal**：在不访问生产服务的情况下证明桌面客户端对协议和失败模式的处理。

优先使用 `httptest.Server` 覆盖：

- 成功、空响应、错误 envelope、未知字段和 schema 兼容。
- 401/403、429、5xx、超时、取消、断连、慢响应和不完整 body。
- token 过期、refresh 失败、设备不匹配、重放和并发刷新。
- 离线进入、离线恢复、重试退避和用户可见错误脱敏。
- 代理、自定义 CA 和证书错误的明确分类。

测试应验证内部错误码、retryable、用户提示和日志字段，不只比较 HTTP 状态码。

## Task 1.4：契约测试采用门

**Goal**：只在跨仓库、跨团队真实需要时引入 Pact Go。

进入 PoC 的必要条件：

- 客户端和服务端由不同团队独立发布。
- schema/错误码漂移已产生真实缺陷。
- provider verification 可以在服务端 CI 稳定运行。
- 契约 broker、权限、保留和敏感字段治理有 owner。

否则继续使用共享 schema、服务端 OpenAPI/fixture 与 `httptest`。只生成 consumer pact 而没有 provider verification 不算闭环。

## Task 1.5：录制回放采用门

**Goal**：判断 go-vcr 是否能补充复杂第三方协议，同时避免客户数据进入仓库。

仅允许：

- 专用测试账号和合成输入。
- 请求前脱敏 Authorization、Cookie、手机号、设备标识、支付和模型内容。
- 固定、审阅、可重放的 cassette。
- 失效时间、重新录制流程和 schema 漂移检查明确。

若无法证明脱敏完整性，拒绝采用，改用手写 fixture。

## Task 1.6：Testcontainers 采用门

当前本地持久化以嵌入式 SQLite 为主，默认不引入 Docker。只有生产路径新增 PostgreSQL、Redis、消息队列、对象存储等真实依赖，且 in-process fixture 无法证明事务/协议行为时才 PoC。

政企离线现场不能运行 Docker，不影响开发 CI 使用，但必须另有无需 Docker 的发布验收路径。

## 验证

```powershell
go test ./path/to/affected/package
go test ./...

Push-Location desktop
go test ./...
Pop-Location
```

## 阶段退出门

- [x] P0 Go 风险边界有 owner 与测试缺口清单。
- [ ] 关键 HTTP/离线/错误脱敏路径由本地确定性测试覆盖。
- [x] 没有因工具偏好强制迁移现有测试。
- [x] Pact、go-vcr、Testcontainers 分别有采用门和接受/拒绝记录。
- [x] 测试不访问生产服务且不包含真实客户数据。
