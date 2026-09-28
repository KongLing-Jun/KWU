# 用户反馈功能进度

## 当前阶段

- 阶段：Stage 3，用户反馈桌面端统一 Draft PR。
- 状态：方案 PR #265、任务 PR #266 已合并；任务 001/002/003 已进入统一
  Draft PR #267。真实验收发现旧 QA 包被注入 `0.0.0`，误用生产配置与
  `https://www.suiyuan-ai.com`，此前“开发云提交通过”结论已撤回；任务 004
  已增加提交回读确认，并使用明确的 dev 构建重新完成开发云落盘验收。
- 工作分支：`codex/feat-user-feedback-foundation`。
- 基线：`origin/main-v3@1c9a8682`。
- 工作树：`D:\Reasonix-worktrees\user-feedback-foundation`，`core.autocrlf=false`。
- 稳定方案与验收：[plan.md](./plan.md)。

## 任务与 PR 映射

| 顺序 | 任务 | 实现 PR | 依赖 | 状态 |
| --- | --- | --- | --- | --- |
| S1 | [001 云端反馈提交客户端](tasks/001-实现云端反馈提交客户端.md) | 统一 Draft PR | 任务 PR 合并及实施授权 | 已完成 |
| S2 | [002 桌面反馈安全桥](tasks/002-实现桌面反馈安全桥.md) | 统一 Draft PR | 任务 001 审查通过 | 已完成 |
| S3 | [003 反馈设置界面](tasks/003-实现反馈设置界面.md) | 统一 Draft PR | 任务 001/002 提交及验证完成 | 已完成 |
| S4 | [004 确认反馈目标云落盘](tasks/004-确认反馈目标云落盘.md) | 统一 Draft PR | 真实云验收发现环境串线 | 已完成 |

任务 001 与 002 共同构成可测试、可回滚的桌面基础能力，单独任一项都不可被用户
使用，因此映射到同一 PR；任务 003 是独立用户界面和最终应用验收切片。

## 实现与边界快照

- `internal/cloud` 已实现严格的 `POST /api/v1/error-reports` multipart 客户端；
  当前增加单报告 GET 回读，不提供历史列表、附件下载或服务端改动。
- 桌面桥已实现原生多选后的 PNG/JPEG 重编码、缩略图、有界内存令牌、单图直传、
  多图稳定 ZIP、scope-bound 幂等键、120 秒超时和中文结构化错误。
- 成功提交、放弃、删除、退出、换号及 shutdown 会释放对应字节；失败保留。
- 真实 runtime 组装云客户端；Fixture/disabled/init error 均 fail-closed。
- 创作型账户菜单和设置中心已增加“帮助与反馈”；页面只包含中文文字、最多 4 张
  PNG/JPEG、提交状态和结果，不受套餐权限控制。
- 前端只持有安全缩略图、文件名、大小和内存令牌；失败同内容手动重试复用 attempt
  ID，编辑后换键，账号 scope 变化或关闭确认后释放草稿。

## 验证状态

- `go test ./internal/cloud -count=1`：任务 001 已通过。
- `cd desktop; go test . -run 'TestFeedback|TestSubmitFeedback' -count=1`：通过。
- `cd desktop; go test . -count=1`：通过。
- 图片净化后可重新解码；JPEG 测试元数据被移除；多图 ZIP 字节稳定且可解压。
- `go run ./scripts/code_guard`：通过，未修改 `scripts/code_guard_baseline.json`。
- `-race` 因当前 Go 环境未启用 CGO 而无法运行；它不是当前项目 PR Gate 要求。
- `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`：通过；分类为
  Root Go `./internal/cloud`、Desktop Go `.` 和 frontend production build。
  最终同步状态的命令结果记录在 Draft PR #267 描述，避免通过修改本文件追记
  Git/PR 状态。
- `feedback-settings.test.tsx` 29 项、反馈导航契约及三组既有 SettingsPanel
  回归测试通过。
- `pnpm build`：通过。
- 内置浏览器宽屏与 860×700 窄窗口、独立 Wails 桌面程序的入口和未登录/
  不可用态验证通过，无相关 console error/warn。
- 旧 QA 程序曾分别提交纯文字、单图和双图并返回以下报告编号：
  - 纯文字：`errpt_01KYV202QEAN1J90T85AJ5HY4R`。
  - 单图：`errpt_01KYV23PMRVS7C3BC21QMW1QFG`；客户端净化后图片为 16 KiB。
  - 双图：`errpt_01KYV27SJR90PW7F4H2494WSPJ`；客户端净化后图片分别为
    16 KiB、175 KiB，多图按稳定 ZIP 上传。
- 上述程序实际使用生产 `Clouvia` 配置与线上云平台，不能作为
  `192.168.10.243:8080` 开发云验收证据。
- 修复后的 `main.version=dev` 构建仅活动于 `kwu-dev` 环境。桌面端于
  2026-07-31 12:21（UTC+8）提交纯文字反馈，POST 返回报告编号
  `errpt_01KYV6GMFP8W06XNNA13EZBZ92` 和请求编号
  `req-e62fb4c7f5411c5adf64c5f99cb20a49`；应用随后用同一用户登录态 GET
  到相同编号及 `submitted` 状态后才显示成功并清空草稿。
- 管理员列表接口随后返回 HTTP 200，请求编号
  `req_787795b0549b87b89e923e6df86db707`，在三条记录中找到相同报告编号，
  状态为 `submitted`、创建时间为 `2026-07-31T04:21:57.878392Z`。因此本次
  桌面提交、用户回读和管理员落盘查询三层证据一致。

## 风险与停止条件

- 当前无本地技术阻塞；任务 004 的真实应用验收已通过。
- 若实施需要流式上传、临时文件、新依赖、反馈历史、其他布局、日志或新隐私协议，
  必须停止并更新方案，不得在任务内扩范围。
- 云平台已接受单图和多图 ZIP，但用户端接口不返回附件正文或下载地址，当前也
  没有已登录的云平台管理页；因此平台人员打开附件后的目测清晰度不冒充已验证。
  清晰度由原分辨率重编码、可重新解码及 ZIP 可解压自动测试覆盖，仍可由平台侧
  使用上述报告编号做最终抽查。

## 下一道门

1. 同步最新基线并在最终状态运行 PR Gate。
2. 更新 Draft PR #267 的实现与验收说明；不得自动合并。
