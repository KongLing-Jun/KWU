# Phase 5：安全、供应链与发布门禁

## Goal

在现有不可变发布来源、Minisign、架构校验和人工 Publish 审批基础上，补齐操作系统信任、SBOM、漏洞、许可证和离线可审计证据。

## Task 5.1：系统签名与更新签名分工

**Goal**：明确两类签名不能互相替代。

| 能力 | 目的 | 当前状态 | 目标 |
|---|---|---|---|
| Windows Authenticode | Windows/SmartScreen/企业终端信任发布者 | 发布 workflow 注释明确尚未启用 | 生产证书、时间戳、签名后校验 |
| Minisign | 应用更新器验证下载制品 | 已实现并有测试 | 保留，密钥轮换与灾备演练 |
| SHA-256 | 传输/归档完整性和人工核验 | 发布链路已有相关能力 | 与制品、SBOM、来源 SHA 一起归档 |

签名顺序必须固定：构建 → 系统签名 → 生成哈希/SBOM → Minisign/manifest → 发布。任何会改变二进制字节的步骤必须发生在最终哈希和更新签名前。

## Task 5.2：Go 与代码安全扫描

- 保留 `govulncheck`，固定版本，不再使用无界 `@latest`。
- 先建立发现分级、例外 owner、到期日和修复 SLA，再评估由 `continue-on-error` 转为阻断。
- `gosec` 先以报告模式 PoC，审阅误报和与现有 lint/CodeQL 的重复，再决定门禁范围。
- `go mod verify`、依赖下载来源、内部 Go/npm 代理和 checksum 策略纳入构建说明。
- 安全扫描失败不得通过跳过 package、忽略目录或永久 suppress 解决。

## Task 5.3：SBOM 与制品扫描

**Goal**：让客户和安全团队能够回答“这个安装包由什么组成”。

- 对最终 Windows 安装器及解包后的应用目录生成 CycloneDX 或 SPDX SBOM。
- 同时覆盖 Go module、前端 npm 依赖、Wails/runtime、打包工具和随附二进制。
- 可用 Syft 做 PoC、Grype 做扫描候选，但必须固定版本和漏洞数据库快照。
- 离线发布环境需要内部镜像或经过审批的数据库更新流程。
- 归档 SBOM、扫描报告、工具版本、数据库时间、例外和签字人。

扫描器结果存在误报/漏报可能，严重项必须人工核对上游公告和可达性，不把“0 findings”写成绝对安全。

## Task 5.4：许可证与来源

- 生成 direct/transitive dependency 清单和许可证分类。
- 建立 allow/deny/review 列表；未知许可证默认阻断发布候选。
- GitHub Actions、Go 工具、Node 包、NSIS 和 Wails CLI 固定精确版本或 commit SHA。
- 外部二进制记录来源 URL、校验和、内部镜像和审批人。
- 依赖升级必须重新生成 SBOM、扫描并审查 release notes。

## Task 5.5：分层 CI

```text
PR
├─ code_guard / baseline
├─ targeted Go + desktop + frontend
├─ affected race/fuzz smoke
└─ release contract checks

Nightly
├─ full root race + desktop risk slice
├─ bounded fuzz
├─ browser/WebView2/native smoke
├─ govulncheck/gosec diagnostic
└─ 8h soak

Release candidate
├─ Windows install/upgrade/uninstall matrix
├─ 24h/72h soak as required
├─ Authenticode + Minisign + SHA-256
├─ SBOM + vulnerability + license review
└─ immutable source and human approval
```

扩展 `scripts/pr-gate.mjs` 时必须同步 `scripts/pr-gate.test.mjs`，未知路径继续 fail closed。昂贵测试只在相关变更、nightly 或 release 执行，不能因为成本高而完全缺席。

## Task 5.6：证据包

每个发布候选归档：

- source commit/tag、workflow run、构建环境和工具锁定版本。
- Windows/架构/WebView2/权限矩阵结果。
- 安装器、SBOM、manifest 和报告的 SHA-256。
- Authenticode、Minisign 和时间戳验证结果。
- 漏洞/许可证发现、例外、owner 和到期日。
- 测试命令、耗时、失败/跳过/超时和人工批准记录。

日志、dump、trace 和截图上传前执行敏感信息检查并设置合理保留期限。

## 回滚

- 新扫描误阻断：降为诊断，不删除规则；修正分类后再提升。
- Authenticode 失败：不得发布未签名生产安装包；修复证书链或签名服务。
- SBOM 不完整：阻断发布候选，修复扫描范围，不用手工删减组件。
- CI runner 不稳定：保留人工可重复的隔离环境，不用 browser smoke 冒充 native/release 验证。

## 阶段退出门

- [x] Authenticode、Minisign、SHA-256 的职责和顺序明确。
- [ ] `govulncheck` 版本固定，例外与阻断策略获批。
- [ ] 最终安装器有可复现 SBOM、漏洞和许可证报告。
- [x] PR/nightly/release 的职责、耗时和失败处理清晰。
- [ ] 发布证据包可在无外网环境中复核。
