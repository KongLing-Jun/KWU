# Phase 1 P0 Go 风险登记

> 状态：Active
>
> 审计基线：`15a98177251bd024b499206e9d90b979fbc48436`
>
> Owner：`@SivanCola`

本登记对应 T1.1。P0 指失败可能造成数据损坏、跨 profile 越权、不可恢复，
或让政企现场核心流程不可用。测试文件存在只证明已有资产；只有本次实际运行
的命令才列为“已验证”。

## 风险与证据

| P0 边界 | 主要代码 | 正常路径证据 | 错误/隔离证据 | 恢复证据 | 当前结论 |
|---|---|---|---|---|---|
| Session JSONL 与 sidecar | `internal/agent/save.go`、`internal/store/session.go`、`desktop/sessions.go` | `TestSaveLoadSessionRoundTrip`、标题与预览 round-trip | malformed JSONL、路径逃逸、symlink、恢复冲突测试 | 原子临时文件替换、cleanup-pending、删除/恢复/跨卷 copy fallback 测试 | 现有覆盖较强；仍缺可注入的磁盘满/替换失败故障测试 |
| Model billing SQLite | `internal/modelbilling` | bootstrap、freeze、call lifecycle、稳定顺序 | scope/binding mismatch、重复请求、非法状态、并发 rows 释放 | V1～V6 migration、事务 rollback、recovery-required 测试 | 事务和 schema 风险已有高密度测试；仍缺损坏数据库处置策略 |
| Automation/Expert SQLite 与状态机 | `internal/automation`、`internal/expert` | CRUD、lease、run transition、builtin seed | 重复 task、非法 transition、锁竞争、无效 JSON | stale lease takeover、heartbeat migration、失败回滚 | 已有确定性本地测试，不依赖 Docker |
| Account、订阅与 profile 隔离 | `internal/cloud`、`desktop/cloud_*` | auth/binding/session/payment 主流程 | scope mismatch、expired/offline、重复支付/冻结、取消 | account switch rollback、event replay、task/payment recovery | 需继续保持 actor/profile/binding generation 三元边界 |
| Cloud registration HTTP | `internal/cloudregistration/http_client.go` | 注册 wire contract、request-id、响应校验 | 429、deadline、断连、超大/畸形响应、secret 脱敏 | 当前 cancel/deadline 不保留 `errors.Is` 分类；无结构化 5xx 被归为协议错误 | 生产问题仅记录，见 `phase-01-test-findings.md` |
| Jobs、事件、取消与关闭 | `internal/jobs`、`internal/cloud/event_*` | start/wait/output、event apply | panic、并发访问、非协作 job、取消 | artifact restore/migrate、reconnect/replay、destroy cleanup | `goleak` 已覆盖 jobs；长稳与资源趋势留给 Phase 3 |
| Desktop 更新器 | `desktop/updater.go`、`desktop/internal/update` | manifest、平台 asset、缓存、下载 | invalid manifest、超时、来源失败、版本不一致、hash/signature 错误 | 多源 fallback、断流 Range resume、IPv4 fallback、取消停止 | 本地 fixture 覆盖较强；真机文件锁/安装中断属于 Phase 4 |
| 远端 install source | `internal/installsource` | 本地 `httptest` 下载、UA、认证映射 | SSRF、401/403、超时/取消、非 2xx | 失败不产生 apply；配置 apply 自带 rollback 测试 | 2 MiB 截断误接受和 URL 凭据泄漏仍未修复，见 findings |

## 本次新增并已验证

```powershell
go test ./internal/installsource `
  -run 'TestFetchText(AppliesTimeoutAndUA|AuthMapsToErrAuthRequired|RefusesInternalAddress)$' `
  -count=1
go test ./internal/installsource ./internal/cloudregistration -count=1
```

两条命令均通过。审计期间用临时回归断言稳定复现了 2 MiB 截断、URL secret
回显、context 分类丢失和无结构化 5xx 误分类；按用户要求未修改生产实现，
也未把期望失败的临时断言留在默认测试门禁。完整证据见
[Phase 1 测试发现](phase-01-test-findings.md)。

Phase 1 相关包复核：

```powershell
go test ./internal/agent ./internal/modelbilling ./internal/automation `
  ./internal/jobs ./internal/cloud ./internal/netclient `
  ./internal/cloudregistration ./internal/installsource -count=1

Push-Location desktop
go test . -run 'Test(NormalizeVersion|Evaluate|UpdatePolicyFor|FetchManifest|DownloadUpdate|SaveCachedUpdate|CachedUpdate|CheckSHA256|ExtractBinary|DownloadRecovers|DownloadGivesUp|RetryTransient|DownloadResumes|DownloadFallsBack)' -count=1
go test ./internal/update -count=1
Pop-Location
```

2026-07-23 全部通过；最长的 `internal/modelbilling` 为 111.408 秒。这里是
Phase 1 定向复核，不替代最终比例门禁。

## 未关闭项

- 自定义 CA 的底层注入契约已由本地 TLS fixture 验证，但产品仍无配置入口、
  存储或轮换 seam；不能把测试注入写成产品已支持。需要网络 owner 先确认
  配置入口、存储边界和证书轮换行为。
- 远端清单上限、URL secret 脱敏、cloud context/5xx 分类为已复现的生产缺陷；
  本任务只提交测试和 Markdown，不修生产实现。
- Session/SQLite 的磁盘满、权限突变、数据库损坏恢复需要可注入故障或明确的
  产品处置策略，不能仅靠构造任意 OS 错误宣称可恢复。
- Windows 安装器文件锁、更新中断和真实升级数据保留属于 Phase 4 真机矩阵。
