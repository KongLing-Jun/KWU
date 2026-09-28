# 任务 003：闭合本地“我的专家”CRUD 并完成整体验收

## 1. 执行合同

| 项目 | 内容 |
| --- | --- |
| 状态 | 待实现；等待任务 002 的实现 PR 合并 |
| 输入 | [`../plan.md`](../plan.md) 的 AC-08、AC-09、AC-12 及 AC-01～AC-15 集成复验 |
| 依赖 | 任务 001/002 已合并，目录 owner 和刷新行为稳定 |
| 拓扑 | Serial-03；映射实现 PR-3，基于已合并 PR-2 |
| 指派 | 与前两任务相同的单个执行者，`gpt-5.6-sol` / medium reasoning |
| 产物 | 设备本地 CRUD、历史云 ID 解限、聚焦测试、整体验收和任务回执 |

本任务只改变 `source=user` 的本地路由和权限，不修改任务 001/002 的系统目录、缓存或
旧会话恢复模型。PR-3 可独立回滚，不影响 public official 读取。

## 2. 实现依据

| 位置 | 当前事实 | 必需结果 |
| --- | --- | --- |
| `desktop/app.go:1428-1523` | real cloud 创建先 POST，再写 JSON 镜像 | 创建无条件走 Controller JSON |
| `desktop/app.go:1526-1578` | `exp_`/`exg_` 在更新、删除中被拒绝 | ID 前缀不再决定权限 |
| `desktop/app.go:1583-1631` | 云 ID 复制再次调用云创建 | 复制只生成本地 user |
| `controller.go:1583-1688` | 已有 JSON 创建、删除、复制和库状态 | 复用，不新建持久层 |
| `selected_expert_refresh.go:11-40` | 已有 JSON 更新和 running 保护 | 保留多 tab/运行中保护 |
| `ExpertDirectory.tsx:555,941,1315-1718` | 云 ID 隐藏编辑、删除 | source=user 一律显示本地动作 |
| `ExpertDirectory.tsx:1113-1252` | JSON user 与 SQLite catalog user 已有不同保存/删除路由 | 保留按存储事实分流 |
| `definition_catalog.go:9-36` | SQLite 先于 JSON 合并并按 ID 去重 | 不迁移、不复制旧 SQLite user |

## 3. 行为与实现要求

- `CreateUserExpert` 无条件调用当前 tab Controller 的 `CreateUserExpert` JSON 路径；
  匿名、authenticated 和 offline 登录状态行为一致。
- 删除 real cloud POST、镜像回写和账户 scope 获取分支；不删除
  `ExpertAssetsClient.CreateUserExpert` 兼容接口。
- Update/Delete/Duplicate 不再调用 `isCloudUserExpertAssetID`；`exp_`/`exg_` 只是普通
  本地 ID。
- JSON user 继续通过 Controller CRUD；SQLite `source=user` 继续通过
  UpdateExpert/DeleteExpert；前端以实际 `userExpertIds`/catalog 来源选择路由。
- 新建、导入、复制统一写 `experts.json`；不创建新的 SQLite user，不强制迁移旧行。
- official 的编辑、删除入口和后端拒绝必须继续由 `source=official`/store 约束保护；
  不能仅信任前端 source。
- 保留运行中专家不可更新/删除、清除选择、运行时目录重载、收藏/最近和多 tab 同步。
- 登录、退出、切换账号不得隐藏、清空或改变本机用户专家权限；不得触发云专家 POST。
- 历史云端资产本身不下载、不清理；只处理已经存在于本地 JSON 的镜像。

## 4. 范围

允许修改：

- `desktop/app.go` 的 Create/Update/Delete/Duplicate user expert 路由
- 与上述 Wails 方法直接相关的 Desktop 专家测试
- `desktop/frontend/src/features/experts/ui/ExpertDirectory.tsx`
- `useExpertCatalog.ts` 仅限 CRUD 后局部刷新和路由调用
- `expertBridge.ts`、专家 CRUD/目录聚焦测试
- 仅在按钮恢复需要时最小修改相关样式
- 本任务回执和集成验证记录

禁止修改：

- `internal/cloud` 客户端接口、Reasonix-Cloud 和云端历史数据
- SQLite/JSON schema、路径、账号隔离或迁移
- 任务 001 的刷新协调器、重置、official 缓存语义
- 任务 002 的旧会话恢复状态、发送可用性和 Heartbeat owner
- “我的专家”文案、登录引导、页面跳转、全局 Toast 去重
- `scripts/code_guard_baseline.json`、PR Gate 映射和 `plan.md`

## 5. 验收与验证

聚焦覆盖：

- 三种账户状态下创建均只写 JSON、云 POST 调用数为零；
- 普通本地 ID 与 `exp_`/`exg_` 的 expert/team 创建、编辑、复制、删除；
- SQLite legacy user 仍走 catalog CRUD，JSON user 不误路由；
- official 前后端仍只读；
- running selected expert、非运行 selected expert和多 tab 同步保持；
- 登录、退出、账号变化前后列表、按钮和数据不变；
- CRUD 后不触发系统目录 GET。

迭代期：

```powershell
cd desktop
go test . -run 'Expert|CloudExpert|Multitab' -count=1
go vet .

cd frontend
pnpm exec tsx src/__tests__/expert-directory-backend.test.tsx
pnpm exec tsx src/__tests__/expert-directory-contract.test.ts
pnpm exec tsx src/__tests__/expert-contract.test.ts
pnpm test:typecheck
```

任务自身稳定后复验任务 001/002 聚焦测试，运行 desktop 受影响包完整测试、前端
`pnpm test:all` 和 `pnpm build`。主会话执行计划 AC-01～AC-15 的最终应用验收：

1. 匿名真实 8080、空缓存启动和多个旧会话请求计数；
2. 有缓存断网、无缓存断网、手动刷新和旧专家恢复/明确放弃；
3. 匿名与登录状态下两种模式及历史云 ID 的完整本地 CRUD；
4. 断网重置保留 official、清空本地 user，fixture 刷新零 HTTP；
5. 活跃 prompt 不被目录刷新改写，official 始终只读。

工具无法可靠覆盖凭据或人工体感时，再给用户明确步骤、数据、预期与观察点，并标记等待
应用验收；不得用实现者自测替代。

## 6. 风险、回滚与回执

主要风险是 JSON/SQLite 路由混淆、ID 前缀解限误放开 official、云 POST 残留、账号事件
清空本地数据、CRUD 重新触发系统 GET，以及回滚 PR-3 后新建本地数据变得不可管理。
回滚不删除本任务期间产生的数据；PR 描述必须说明旧版本可能再次限制 `exp_`/`exg_`。

完成时填写：

| 项目 | 结果 |
| --- | --- |
| 最终状态 | 待实现 |
| 实际修改文件 | 待填写 |
| 聚焦/完整检查 | 待填写 |
| 实现者应用自测 | 待填写，不替代主会话验收 |
| 主会话应用验收 | 待填写 AC-01～AC-15 结果 |
| 偏差与剩余风险 | 待填写；材料变化必须停止并重新审批 |

执行者不得推送、操作 PR、扩面或委派。主会话复核提交、diff、EOL、测试和回执；返修
必须新增 `fix:` 提交，不 amend。
