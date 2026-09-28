# Phase 0 D-TEST-01～07 工程评审记录

> 评审日期：2026-07-23
> 评审角色：测试/研发工程审查。
> 边界：本记录完成技术方案评审，不代表产品、交付、安全、法务或 Release
> owner 的正式批准，也不把未实现能力写成已上线。

## 评审结论

| Decision | 工程结论 | 当前证据 | 尚未完成 |
|---|---|---|---|
| D-TEST-01 标准库与现有 seam 优先 | 接受 | Root/Desktop 现有 table test、临时目录、SQLite、手写 fake 可持续扩展；本轮未引入 mock 框架 | 团队 ADR ratification |
| D-TEST-02 客户端逐级升级 | 接受 | `httptest` 已覆盖多条 HTTP 边界；Pact/go-vcr/Testcontainers 采用门均未触发 | 自定义 CA seam 和已发现 HTTP 生产缺陷 |
| D-TEST-03 四层 UI 自动化 | 接受 | Vitest 11 项、browser 5 项、真实 WebView2 启动、270 节点 UIA tree 分层归档 | WebView2 CDP seam、系统对话框/托盘场景 |
| D-TEST-04 安装/升级/回滚为发布一级能力 | 接受为目标架构 | 架构、更新器、安装器静态契约和真实原生启动已有部分证据 | 真机 amd64/arm64、N-2/N-1、失败中断、卸载与回滚 |
| D-TEST-05 双层信任 | 接受为目标架构 | Minisign/SHA 契约与发布顺序已审计；开发 exe `NotSigned` 被正确阻断 | Authenticode、时间戳、最终 SBOM、漏洞/许可证审批 |
| D-TEST-06 Frontend 渐进迁移 | 接受 | Vitest/Testing Library 独立 PoC 与旧 runner 并存，Playwright 只覆盖 5 条 smoke | 迁移比例需基于后续维护成本调整 |
| D-TEST-07 比例门禁和分层执行 | 接受，实施部分完成 | `pr-gate` 分类及基础设施映射测试通过；首次失败/重试格式已固定 | Nightly full race/8h soak、release 24/72h 和制品信任门 |

## 逐项理由

### D-TEST-01

当前 Go 测试资产已经能够表达网络断连、SQLite、生命周期、fuzz、benchmark 和
goleak。没有证据证明引入 `testify/mockery` 能抵消生成物、依赖和审阅成本，
因此维持标准库优先。

### D-TEST-02

Phase 1 已证明本地 `httptest` 能暴露响应大小、URL secret、取消分类和 5xx
契约问题。当前没有跨团队 schema broker、合法脱敏录制源或真实容器化依赖，
所以高级工具的采用门未触发，而不是“忘记执行”。

### D-TEST-03

四层证据确实不同：普通浏览器通过不能证明 WebView2 CDP，原生窗口/UIA tree
也不能证明 DOM binding。分层策略已防止用较低层成功覆盖较高层阻塞。

### D-TEST-04

Phase 4 静态契约能发现架构和脚本问题，但不能证明目标客户环境的安装/升级。
因此该能力必须保留为 release 一级目标；本轮不因环境缺失降低要求。

### D-TEST-05

开发 exe 未签名、最终 SBOM 缺失以及固定版本 govulncheck 的可达漏洞说明
Minisign 不能替代操作系统信任、依赖治理和最终制品证据。

### D-TEST-06

独立 Vitest/Testing Library PoC 没有删除旧 runner，满足可回滚迁移；现阶段
不需要为统一框架重写全部旧测试。

### D-TEST-07

PR 分类测试可以快速反馈，但当前 nightly/release 责任仍有真实缺口。接受的是
目标分层和 fail-closed 原则，不是宣称完整门禁已经实现。

## 决策状态

七项均完成工程层“接受/拒绝/待 PoC”评审：本轮全部为工程接受，其中
D-TEST-04、05、07 的落地仍显著不完整。它们继续保持 Proposed/engineering
baseline，正式 ADR 需要对应 owner 评审。

T0.3 和 Checkpoint M0 不因本记录完成：政企支持矩阵仍缺产品、交付、安全和
研发共同签字。
