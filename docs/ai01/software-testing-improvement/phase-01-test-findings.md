# Phase 1 测试发现

> 状态：Open
>
> 发现日期：2026-07-23
>
> 范围：测试审计；未修改生产代码

本文记录测试过程中稳定复现、但按当前任务边界不修改生产实现的问题。临时
断言先确认旧行为会失败，随后撤销生产修复和期望失败断言；默认测试门禁保持
绿色。示例只使用 `httptest`、合成 URL 和合成 secret，不访问生产服务。

## F1：远端 install source 未拒绝超大响应

- 严重度：High
- 位置：`internal/installsource/http.go`
- 复现：本地 server 返回 `defaultFetchLimit + 1` 字节。
- 期望：返回 `ErrSourceUnreadable`，不向解析器交付任何截断内容。
- 实际：`io.LimitReader` 只读取前 2 MiB，`fetchText` 返回成功。
- 影响：上限被实现成静默截断而不是拒绝；精心构造的前缀可能被当成完整
  manifest 继续处理，且调用方无法区分完整响应与截断响应。
- 建议：读取 `limit + 1` 并显式拒绝超限；补默认门禁回归测试。

## F2：远端 URL 的凭据和查询参数进入错误文本

- 严重度：High
- 位置：`internal/installsource/http.go`
- 复现：向本地 500 fixture 请求
  `http://user:password@127.0.0.1/.../manifest?token=query-secret`。
- 期望：错误保留 host/path/HTTP 状态，移除 userinfo、query 和 fragment。
- 实际：完整 URL 被拼接到 `ErrSourceUnreadable`，合成 password/token 可见。
- 影响：临时下载令牌或 Basic Auth 可能进入 UI、日志和支持包。
- 建议：统一生成 display-safe URL；补 401、5xx、transport error 三条脱敏
  回归测试。

## F3：cloud registration 丢失 context 取消/超时分类

- 严重度：Medium
- 位置：`internal/cloudregistration/http_client.go`
- 复现：取消 request context，或在 response body 尚未结束时触发 deadline。
- 期望：调用方可以用 `errors.Is(err, context.Canceled/DeadlineExceeded)` 分类；
  用户取消不应被当作可重试网络故障。
- 实际：错误只剩 `NETWORK_UNAVAILABLE`；取消仍带 `Retryable=true`。
- 影响：上层可能错误重试用户主动取消的请求，也无法区分 deadline 与断网。
- 建议：保留 context cause，并让取消为 non-retryable；继续保留安全 request-id。

## F4：无结构化 error 的 5xx 被归为协议错误

- 严重度：Medium
- 位置：`internal/cloudregistration/http_client.go`
- 复现：本地 server 返回 HTTP 503 和合法 JSON envelope，但 `error=null`。
- 期望：映射到可重试服务/网络错误。
- 实际：返回 non-retryable `CLOUD_PROTOCOL_ERROR`。
- 影响：短暂服务故障可能不进入正确的退避/恢复路径。
- 建议：保留服务端结构化 error 优先级；无结构化 5xx 使用稳定、可重试的
  内部分类，并补 500/502/503 table test。

## 已保留的测试增强

- `TestFetchTextAppliesTimeoutAndUA` 现在真实检查 `kwu-install/1.0` User-Agent，
  同时保留取消映射断言。
- `TestHTTPClientMapsDisconnectWithoutLeakingRegistrationSecrets` 使用自定义
  transport 模拟断连，验证 `NETWORK_UNAVAILABLE`、retryable、request-id 和
  注册密文不泄漏。
- `TestHTTPClientMapsRegistrationNotReadyAndCloudErrors` 已覆盖结构化 429/503，
  验证服务端错误码、retryable 和 request-id；脱敏断言覆盖密码密文、验证码、
  实际 captcha ticket 与手机号。无结构化 `error=null` 的 5xx 仍保留为 F4，
  未用测试掩盖生产缺陷。
- `TestHTTPClientSupportsExplicitCustomCA` 使用本地 TLS server：默认信任链
  稳定失败并映射为网络错误，显式注入只信任该 fixture 证书的
  `http.Client` 后成功。它只证明客户端注入契约可测，不代表产品已有 CA
  配置、存储或轮换入口。

验证：

```powershell
go test ./internal/cloudregistration ./internal/netclient ./internal/installsource -count=1
```

结果：通过。
