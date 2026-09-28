# 桌面专家与专家团闭环修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 2：任务分解 |
| 计划 | [`plan.md`](./plan.md) |
| 调查证据 | [`evidence.md`](./evidence.md) |
| 合并计划 | GitHub PR #212，merge commit `4ad27d632b5f117da40da2f726f64d88ae1cef7a` |
| 任务基线 | `origin/main-v3@dbf6cfcdb083c22a2d264a497b3d0dbaf016a368` |
| 工作方式 | 独立 worktree，`core.autocrlf=false`，主工作区只读 |
| 当前改动 | 仅任务合同和进度索引，无实现代码 |
| 外部 Git/PR 状态 | 以 Git 与 GitHub 为唯一事实源 |

## 任务与实现 PR 映射

| 顺序 | 任务 | 依赖 | 实现 PR | 状态 |
| --- | --- | --- | --- | --- |
| Serial-01 | [`001-建立公共系统目录缓存边界.md`](./tasks/001-建立公共系统目录缓存边界.md) | 无 | PR-1：公共客户端、缓存协调器与重置 | 待实现 |
| Serial-02 | [`002-统一目录所有权与旧会话恢复.md`](./tasks/002-统一目录所有权与旧会话恢复.md) | PR-1 已合并 | PR-2：目录 UI、Heartbeat 与恢复体验 | 待实现 |
| Serial-03 | [`003-闭合本地我的专家CRUD.md`](./tasks/003-闭合本地我的专家CRUD.md) | PR-2 已合并 | PR-3：本地 CRUD 与最终集成验收 | 待实现 |

三个切片串行执行，避免 `desktop/app.go`、`useExpertCatalog.ts` 和
`ExpertDirectory.tsx` 的重叠修改产生并行冲突。每个实现 PR 都必须从当时最新
`origin/main-v3` 建立独立 worktree，并可独立验证和回滚。

## 已确认决策

- 系统专家和专家团由 8080 公开只读；SQLite 只作本地缓存。
- 每次启动缓存先显示，后端自动刷新最多一次，不阻塞应用外壳、可解析会话和其他 tab。
- 未解析专家的旧会话历史可看、发送暂缓：刷新失败可重试；刷新后仍不存在时，只有用户
  明确选择“不使用专家继续”才清除绑定。
- 专家页常驻一个“刷新”按钮，同时刷新专家和专家团；手动操作可再次请求。
- 自动失败有缓存时静默；无缓存时只显示内容区空态。
- 手动成功显示“系统专家已刷新”；失败保留列表并只弹一次错误 Toast。
- 会话切换、本地 CRUD、标签切换不触发系统目录请求。
- “我的专家”是设备本地资产，匿名与登录用户同权 CRUD，不按云账号隔离。
- 桌面端停止调用云端普通用户专家 POST；历史云 ID 本地记录按普通本地资产处理。
- Fixture 启动提示一次、专家页常驻“测试数据”，刷新只重载本地目录。
- “重置专家”只清 SQLite/JSON 用户专家、收藏和最近状态；保留 official，零 HTTP。
- 本次不修改 `Reasonix-Cloud`；其源码与 8080 部署漂移是已接受风险。
- 不做全局文本 Toast 去重，不改旧会话提示历史，不强制迁移 SQLite 旧 user 记录。

## 调查快照

| 检查 | 结论 |
| --- | --- |
| 匿名 8080 GET | 2026-07-28 实测 HTTP 200，约 18 KB |
| 桌面匿名失败 | `cloudExpertAssetsOperation` 在 HTTP 前返回 `AUTH_REQUIRED` |
| 重复 Toast | tab、ready 状态和显式调用共同触发 `refreshExpertState` |
| 本地缓存 | `ReplaceOfficial` 已事务写入 `experts.sqlite`，但失败路径从不读取 |
| 本地 CRUD | Controller 已完整支持 `experts.json` 创建、更新、删除、复制 |
| 云模式分支 | 创建走云 POST；云 ID 更新、删除被拒绝，复制再次走云 |
| Fixture | 没有专家云客户端，当前实际使用内置专家目录 |
| 空缓存旧专家恢复 | 当前未知 ID 静默降级为 none；Draft/Locked 会用无专家 prompt 覆盖保存的 system message |
| 重置专家 | 当前 DROP 整张表后才请求云端，失败可留下空 official 缓存 |
| Stage 2 源码复核 | 关键结论已在任务基线 `dbf6cfcd` 直接复核 |

## 验证与阻塞

- 已运行聚焦既有行为测试：
  `go test ./internal/boot ./internal/control -run 'TestBuildIgnoresUnknownRestoredSessionExpert|TestResumeSceneCompatibility' -count=1`；
  两包通过。本阶段没有实现代码，因此未运行实现回归套件。
- 当前无未决产品、权限、兼容性或迁移问题；新增恢复与重置规则已获用户确认。
- 当前无任务合同未决问题；`plan.md` 保持合并后的稳定决策记录，不在 Stage 2 改写。
- 下一关口是任务分解 Draft PR 评审与合并；任务 PR 合并并获得明确实现授权前不修改代码。
