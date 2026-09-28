# 资源说明

| 路径 | 内容 | 数量 |
|---|---|---:|
| `runtime/connectors.full.json` | WorkBuddy 5.3.5 运行时合并定义 | 73 |
| `runtime/connectors.index.csv` | 运行时扁平索引 | 73 行 |
| `runtime/marketplace.version-gated.json` | 被版本过滤的 EMR 及未索引兼容包 | 1 + 4 |
| `marketplace/connectors.json` | 市场中央索引 | 74 |
| `marketplace/configs` | 每个连接器根配置 | 144 文件 |
| `marketplace/skills-entry` | 连接器主 Skill 与飞书一级子 Skill | 78 文件 |
| `assets/ui` | 用户提供的界面参考 | 3 张 |
| `workbuddy-marketplace-5.3.5.zip` | 修复后的完整市场缓存 | 1,687 文件 |

完整 ZIP 验证结果：

```text
ZIP entries:        2218
Files:              1687
Uncompressed bytes: 30514261
Leading ./ paths:   0
Duplicate names:    0
Stream read errors: 0
Explorer roots:     .codebuddy-connector, connectors, dist, icons
SHA-256:            5302C2FC0D08253ED77FA7882ACEDE7BB6E82D57750458C2E883B83386D1D618
```

本目录不包含用户连接状态、账号标识、Token、密钥、日志、Cookie 或浏览器存储。
