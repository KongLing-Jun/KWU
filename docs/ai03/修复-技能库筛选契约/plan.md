# 技能库筛选契约修复计划

## 背景与问题

桌面端“技能”页加载公共技能库时显示“加载技能库失败，请稍后重试”。该页面仍可显示本地已安装技能，但公共目录无法加载，重试结果不变。

实际请求证明服务端可达，但客户端请求的筛选参数已不被服务端接受：

- `GET /api/v1/skills?limit=1` 返回 `200`；
- `GET /api/v1/skills?type=user&limit=1` 返回 `400 VALIDATION_FAILED`，消息为“请求参数不正确”。

根因是桌面端 `fetchPublicSkills` 无条件附加 `type=user`，而当前服务端的公开目录契约不再接收该参数。前端将后端错误统一为通用加载失败提示，故用户无法从页面分辨该确定性契约错误。

## 当前与期望行为

当前行为：进入技能库时，前端调用 `App.PublicSkills`；后端拼接 `type=user`、`limit` 和可选 cursor；服务端返回 400；前端显示失败横幅与空态。

期望行为：目录和分页请求只传服务端当前支持的 `limit` 与可选 cursor，成功加载服务端默认公开目录。搜索保持现有的 `keyword`、`limit`、可选 cursor 契约和原有错误处理。

## 目标、非目标与边界

目标：

- 恢复公共技能目录首次加载与分页。
- 以回归测试锁定请求不得携带已废弃的 `type` 参数。
- 保持 Wails bridge、目录响应模型、安装/卸载和前端交互不变。

非目标：

- 不修改服务端、用户登录、权限、网络代理或本地技能目录。
- 不为目录增加缓存、重试、降级来源或新的筛选 UI。
- 不改变搜索端点和搜索关键词校验。

## 场景与验收标准

1. 首次进入技能库时，目录请求为 `/api/v1/skills?limit=20`，且不含 `type`；服务端 2xx 响应能显示目录数据。
2. 有 cursor 时，目录请求保留原值并仍不含 `type`。
3. 服务端 4xx/5xx、无效响应和网络失败继续走现有前端错误提示；本修复不掩盖真实失败。
4. 搜索请求继续携带规范化后的 `keyword`、`limit`、可选 cursor，不带认证头。
5. 实际桌面界面进入“技能”页后不再出现此次 `VALIDATION_FAILED` 加载失败；若服务器返回空目录，则显示正常空态而不是错误横幅。

## 实施方案与影响面

在 `desktop/skills_marketplace.go` 的 `fetchPublicSkills` 删除 `query.Set("type", "user")`，不改变 endpoint、超时、HTTP client 或响应解码。更新 `desktop/skills_marketplace_test.go`：将现有目录测试改为断言查询参数中没有 `type`，同时继续断言 limit 与 cursor。

预计改动：

- `desktop/skills_marketplace.go`
- `desktop/skills_marketplace_test.go`
- 本主题计划、进度和任务收据

外部 bridge 方法签名、公共技能 JSON 模型、本地安装元数据和前端页面不变，因此无数据迁移或兼容层。

### 补充决策：本地“我安装的”不依赖远端目录

交互验收发现，`SkillsSettingsPage` 在远端目录完整加载后会用远端 slug 集合二次过滤本地
非内置技能。移除 `type=user` 后，远端默认目录的覆盖范围不再等同于用户本地已安装技能，
从而隐藏了未出现在该页目录中的本地用户、私有或离线技能。

“我安装的”是本地管理入口，权威来源应为 `SkillsSettings()` 返回的本地技能快照。补充修复
移除该远端目录过滤；公共目录仍只用于浏览、搜索与安装。更新范围增加
`desktop/frontend/src/components/CapabilitiesPanel.tsx` 和其直接交互测试。

## 依据、风险与回滚

基线：`origin/main-v3@6c5282e0`（2026-08-06 获取）。当前工作树源码确认：

- `desktop/skills_marketplace.go:381` 构造目录请求，`:393` 追加失效参数；
- `desktop/skills_marketplace_test.go` 将该参数硬编码为旧契约；
- `desktop/frontend/src/components/CapabilitiesPanel.tsx:3093-3100` 调用 bridge 并把错误呈现为截图中的横幅。

风险是服务端未来恢复或要求类型筛选。控制方式是仅移除已经由真实 400 证明无效的参数，保留所有其他请求边界；若服务端明确发布新筛选语义，应另行以显式产品筛选模型实现。回滚只需恢复一行与相应测试，但会复现当前故障。

## 验证计划

- 先运行 `go test ./desktop -run 'Test(FetchPublicSkills|SearchPublicSkills)' -count=1`。
- 再运行受影响 Desktop 包 `go test ./desktop -count=1`。
- 启动实际 Wails 桌面应用，打开技能页，确认请求成功、无加载失败横幅，并验证一次分页或刷新路径。
- 检查 `git diff --check`、EOL 与 `go run ./scripts/code_guard`。本次修改 Root Go，按仓库规则不运行前端构建；未授权 PR，不运行 PR Gate。
