# 技能库筛选契约修复进度

## 当前状态

- 阶段：任务 001 已验收并已创建 Draft PR；交互验收发现任务 002，正在修复本地“我安装的”筛选回归。
- 基线：`origin/main-v3@6c5282e0`。
- 分支：`codex/docs-fix-skill-catalog-contract`。
- PR：未授权；不创建或更新 PR。

## 已确认决策

- 删除已被服务端拒绝的目录 `type=user` 参数，采用服务端默认公开目录。
- 搜索、安装、本地技能管理、UI 文案和 bridge 签名不在本次范围。
- 用户授权计划、任务、代码连续完成并按阶段小步提交；要求真实应用验收。
- “我安装的”只展示本地 `SkillsSettings()` 中的非内置技能，不以远端目录作为白名单。

## 实现与验证

- `GET /api/v1/skills?limit=1`：服务端返回 200。
- 同端点加 `type=user`：服务端返回 400 `VALIDATION_FAILED`。
- DNS、TLS 与服务可用；不是本机网络、代理或本地技能数据故障。
- 实现删除了目录请求的 `type=user`，回归测试断言该键完全不存在，同时保留 limit 和 cursor。
- `go test . -run 'Test(FetchPublicSkills|SearchPublicSkills)' -count=1`（在 `desktop`）：通过。
- `go test . -count=1`（在 `desktop`）：通过，`ok kwu/desktop 93.373s`。
- 真实开发环境 `GET http://192.168.10.243:8080/api/v1/skills?limit=20`：200，返回 20 条目录记录和 `has_more=true`；旧参数同端点为 400。
- `wails build`：通过，生成 `desktop/build/bin/kwu-desktop.exe`。
- `go run ./scripts/code_guard`、baseline 不变检查、`git diff --check` 与受影响文件 EOL 检查：通过。

## 下一门禁

完成任务 002 的前端回归测试、生产构建和实际页面验收；随后更新 Draft PR 并运行最终 Gate。
