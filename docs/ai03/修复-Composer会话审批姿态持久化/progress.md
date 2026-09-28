# Composer 会话审批姿态持久化进度

## 当前状态

- 阶段：Stage 3 已完成，等待后续 PR 授权
- 基线：`origin/main-v3` `0f64078be0a02828e1155719405c751c4b311a26`
- 分支：`codex/fix-composer-session-approval-posture`
- worktree：独立 worktree，`core.autocrlf=false`

## 已确认

- 产品合同、边界、legacy 与失败语义已由委托输入确认，无待决产品问题。
- 使用现有 `BranchMeta`，不建立平行存储。
- sidecar 字段采用可判缺失的 additive 表示，显式 Ask 与 legacy missing 分离。
- Composer 修改不再写全局默认；设置页行为不变。
- bot/IM/automation 排除。

## 任务与验证

- 计划：已落地，见 `plan.md`。
- 验收矩阵：已定义，见 `acceptance.md`。
- 任务拆分：3 个串行任务，见 `tasks/001-*.md` 至 `tasks/003-*.md`。
- 实现：任务 001 后端、任务 002 前端、任务 003 集成验收均已完成。
- 自动化：`go test ./internal/agent -count=1 -timeout=180s`、desktop module 的 `go test . -count=1 -timeout=240s`、相关前端定向测试与 `pnpm build` 通过。
- 最终 Gate：`node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 通过；分类为 Root Go `./internal/agent`、Desktop Go `.`、frontend production build。
- 真实应用验收：隔离 Wails dev 实例通过全局/项目、A/B、历史/重启、legacy missing、失败事务回滚、快速切换和全局默认隔离；详见 `acceptance.md`。

## 阻塞

无产品、实现或验证阻塞。本环境没有 Codegraph，已按规则使用最新 worktree 源码、历史、测试、真实应用与最终 diff 复核。

## 下一门禁

工作树保持 clean；仅在获得后续授权后推送或创建 Draft PR，并在同步最新 `origin/main-v3` 后重跑 PR Gate。
