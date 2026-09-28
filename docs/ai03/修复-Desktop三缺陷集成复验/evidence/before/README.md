# 修复前三场景 Desktop 稳定性证据

本目录记录 `origin/main-v3@6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad` 上 30 次真实 Desktop
测试的文档化结论，用于与三缺陷集成后的同任务、同 effort、同统计口径复验比较。权威矩阵为
L01–L24、A01–A06，共 30 个唯一 run；失败样本没有被后来恢复结果覆盖。

## 仓库保留范围

- 方法、预检、基线审计、阶段报告和最终报告；
- DEF-001～DEF-009 的文档化现象、复现流程、判断和源码线索；
- commit、tree、Desktop binary hash、归档身份和敏感信息边界。

逐次 JSON、CSV、截图、日志、run-spec、grade、统计脚本和浏览器输出曾在测试与最终审计时完成解析、
交叉核对和评分，但按 PR 整理要求不再纳入 Git。历史文档中的原始文件名仅用于标识当时取证来源。

## 完整原始归档记录

- 测试时位置：`D:\Reasonix-evidence\desktop-stability-20260812-6cfe5af6-raw.zip`
- 文件：1,202
- 解压后大小：42,009,118 bytes
- ZIP 大小：22,662,239 bytes
- ZIP SHA-256：`8326C698BCAE3F959FA503AA1D8003229CF9F4644D607B176CB0A4F6E0B15021`
- 外部清单 SHA-256：`0A40AE897E6EEB925B2B40446D5D10B50867D353554341793B610C62AABE1EA7`
- 测试时逐项重算文件数、大小和 SHA-256：零差异。

该本机归档不是本 PR 的版本化依赖；以上信息只记录测试时已经完成的完整性核验。

## 敏感信息边界

原始目录扫描没有发现 key-like 密钥；唯一 auth-like 字段位于未进入 Git 的 runtime profile。仓库保留的
Markdown 文档不包含 API Key、runtime profile 或生成 workspace。
