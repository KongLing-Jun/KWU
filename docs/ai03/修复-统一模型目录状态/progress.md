# 统一模型目录状态进度

## 当前状态

- 阶段：Stage 3 顺序一致性修正与独立应用复验完成，等待 PR 授权
- 基线：`origin/main-v3` / `06307e38bc8775cc4c28188d31a8bf5682d243db`
- 分支：`codex/share-model-catalog`
- 工作树：`D:\Reasonix-worktrees\shared-model-catalog`
- PR：未授权创建

## 已确认决策

- 会话页模型选择框与模型广场共用一份 App 级模型目录状态。
- 页面切换不刷新目录；标签页、账户/订阅、配置、当前模型和显式刷新触发统一重读。
- 刷新时清空旧列表；失败时显示不可用，不显示旧 scope 数据。
- 模型广场现有刷新按钮同时触发价格目录刷新和共享模型目录重读。
- 两处可切换模型沿用 ModelSwitcher 原有 provider 分组顺序；模型广场只消费该分组的扁平结果，不增加视觉分组。
- 价格缓存、远端价格协议、Go 后端和其他目录消费者不在范围内。
- 用户授权计划、任务和实现连续执行并按小步提交；未授权 PR。

## 任务与验证

- 任务分解：`tasks/001-统一模型目录状态.md`，串行组 A，单一实现切片。
- 实现：共享状态类型、App 投影与 props 连线完成；两个子组件的独立目录读取已删除；ModelSwitcher 原有 provider 分组/排序已提取复用，Marketplace 按共享分组扁平顺序渲染。
- 聚焦测试：任务要求的 4 个测试全部通过；ModelSwitcher 共享状态测试为 21 个行为断言，Marketplace 为 44 个断言并覆盖两处顺序一致；App stale sequence guard 有静态契约，相关 Composer、订阅修复和账户工作区契约亦通过。
- 测试类型检查：`pnpm exec tsc --noEmit -p tsconfig.test.json` 通过，直接构造 `Composer` 的 fixture 已满足必填共享目录 prop。
- 前端构建：`pnpm build` 通过。
- 应用验收：主会话在当前 Wails dev 应用及其隔离浏览器页面中逐项读取两处列表；两处均为 15 个模型，模型名称与顺序完全一致，均按 5 个平台积分模型后接 10 个 GPT 自备密钥模型展示；原页面往返不新增 loading/unavailable 的验收继续有效。
- 验收边界：为避免改写另一个开发会话正在使用的共享价格缓存，未在真实页面点击显式刷新；价格刷新后触发共享目录失效及同步 loading/恢复由组件与 App 契约测试覆盖。

## 阻塞项

- 无。

## 下一门禁

等待用户决定是否授权同步最新 `main-v3`、运行 PR Gate 并创建 Draft PR。
