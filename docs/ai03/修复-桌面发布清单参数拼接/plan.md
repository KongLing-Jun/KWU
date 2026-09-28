# 桌面发布清单参数拼接修复计划

## 背景与问题

`desktop-v1.1.0` 的本地 `Finalize -DryRun` 已通过冻结来源、Draft 资产、Windows
Authenticode、ARM64 实机哈希和 macOS 来源验证，但稳定失败于
`generate GitHub manifest for windows-amd64`。相同的完整 `go run ./cmd/sign
manifest` 命令单独执行成功，排除资产、签名工具和 manifest 生成器本身故障。

PowerShell 实际参数绑定证明，`Invoke-DesktopGoCommand -Arguments @(...)
+ $platformArguments + @(...)` 只把第一个数组绑定给 `-Arguments`；平台参数、资产目录、
版本和发布标签没有进入命令。现有合同测试只生成空 fixture manifest，没有断言收到的
参数，因此未发现该缺口。

## 当前与预期行为

- 当前：所有目标的首个 GitHub manifest 命令缺少必需位置参数，`Finalize` 在任何
  远程发布写入前停止。
- 预期：GitHub 与 OSS 的每个 target manifest 命令都收到对应 `--platform` 参数，
  并以 incoming 目录、版本和发布标签作为最后三个位置参数。

## 目标与边界

- 为两个 target manifest 参数表达式增加明确括号，消除 PowerShell 参数绑定歧义。
- 扩展现有 `Finalize -DryRun` 合同测试，逐目标验证 GitHub/OSS manifest 的平台与
  发布身份参数。
- 不修改 manifest 格式、发布顺序、云端 API、OSS/GitHub 写入语义或更新客户端。
- 不移动 `desktop-v1.1.0` 标签，不重打包、不重新签名、不替换现有 Draft 资产。

## 实现依据与方案

- `scripts/desktop-release.ps1` 的两个 target manifest 调用是唯一缺少整体括号的动态
  数组拼接；bootstrap manifest 使用单一数组，不受影响。
- 将 `-Arguments` 的值改为
  `(@(...) + $platformArguments + @($incoming, $identity.Version, $identity.ReleaseTag))`。
- 在 `scripts/desktop-release-test.ps1` 的既有本地 Finalize fixture 中记录 manifest
  调用，断言三个 target 的 GitHub/OSS 六个命令均携带精确平台集合和末尾位置参数。

## 验收与验证

- 运行 `pwsh -NoProfile -File scripts/desktop-release-test.ps1`。
- 运行仓库完整 PR Gate；该变更应分类为发布门禁基础设施并执行对应路由合同。
- 合并后保持 v1.1.0 冻结来源与资产不变，使用修复后的工具重新执行
  `Finalize -DryRun`；必须通过完整零写入冲突预检后才能请求正式发布确认。

## 风险与回滚

- 风险局限于 PowerShell 数组绑定；测试对六个动态命令逐一锁定，防止遗漏平台或发布
  身份。
- 若出现回归，可回滚本修复；Release 保持 Draft，所有稳定指针和云端记录不变。
