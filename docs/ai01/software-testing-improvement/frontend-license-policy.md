# Frontend 生产依赖许可证策略

> 状态：Engineering gate baseline，法务对 `review` 项的决定仍是发布前置条件。
> 执行日期：2026-07-23
> 数据源：提交锁文件对应的
> `pnpm --dir desktop/frontend licenses list --prod --json`。

## 1. 分类规则

### Allow

当前工程门禁允许以下精确 SPDX 标识：

- `MIT`
- `ISC`
- `BSD-2-Clause`
- `BSD-3-Clause`

Allow 只表示当前工程政策允许进入下一步扫描，不表示依赖来源、完整性、漏洞或
商标已经自动获批。依赖升级后必须重新生成清单。

### Deny

以下标识或文本默认阻断发布候选：

- `GPL-*`
- `AGPL-*`
- `SSPL-*`
- `BUSL-*`
- `Commons Clause`
- `UNLICENSED`

Deny 不允许由 AI、重跑或脚本参数自动改成 allow。确需使用时必须先由法务和
安全批准架构/依赖变更，本策略随后以代码评审方式更新。

### Review

除精确 allow 和 deny 以外的全部值默认进入 review，包括：

- 自定义许可证和 `SEE LICENSE ...`；
- 多许可证表达式；
- 未识别、缺失或非标准许可证字段；
- 当前的 GSAP standard/no-charge license。

Release 模式只要存在 review 或 deny 就返回非零退出码；报告模式仍输出完整
分类，供人工处理。

## 2. 当前锁文件结果

| 决策 | License | 包数 | 代表包 |
|---|---|---:|---|
| allow | MIT | 126 | React、zustand、unified 生态 |
| allow | ISC | 4 | `lucide-react`、`qrcode.react` |
| allow | BSD-2-Clause | 1 | `entities` |
| allow | BSD-3-Clause | 1 | `highlight.js` |
| review | GSAP standard license 文本 | 2 | `gsap`、`@gsap/react` |
| deny | 无 | 0 | — |

当前结果不是发布通过：两项 GSAP 依赖仍需确认政企商业分发、离线部署、客户
再分发和许可文本随附要求。

## 3. 确定性命令

```text
node scripts/testing-frontend-license-policy.test.mjs
node scripts/testing-frontend-license-policy.mjs
node scripts/testing-frontend-license-policy.mjs --release
```

- 单元测试验证 allow/deny/review 分类；
- 报告模式对当前锁文件返回分类 JSON；
- release 模式当前按预期返回退出码 1，并输出
  `release blocked by review/deny entries`。

首次真实清单执行在 Windows Node 26 下使用 `spawnSync pnpm.cmd`，返回
`EINVAL`。测试脚本随后改为通过固定的 `ComSpec /d /s /c` 静态命令调用
`pnpm`，没有拼接用户输入；复核通过。首次失败没有被改写为许可证失败或通过。

## 4. 范围与未完成项

本策略只覆盖 frontend production 依赖，尚不覆盖：

- Root/Desktop Go module；
- Wails runtime/CLI；
- NSIS、WebView2 loader 和随附二进制；
- GitHub Actions 和发布工具；
- 最终安装器及解包目录。

因此 T5.7 的“建立 allow/deny/review 清单”完成，但 Phase 5 发布门仍未通过。
完整发布结论必须结合最终 SBOM、来源、漏洞扫描、许可证文本、法务决定和人工
审批。
