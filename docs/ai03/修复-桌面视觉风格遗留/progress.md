# 进度：桌面视觉风格遗留配置

## 当前阶段

Stage 3（堆叠执行）— 实现、聚焦自动化验证、代码审阅、真实 Wails 配置迁移和人工视觉验收均完成。

## 已确认决策

- 废弃桌面视觉风格配置，统一默认 `carbon`。
- 启动时忽略并清除历史 `desktop.theme_style`。
- 覆盖正式、开发隔离和 profile 变体；只在对应 profile 实际启动时迁移。
- 保留 `SetDesktopAppearance(theme, style)` 的 `style` 参数以兼容旧调用，但不再写入。
- CLI `ui.theme_style` 保持不变。
- 验收必须包含历史 `graphite` 控制组与迁移后实验组的文件和真实 UI 对照。

## 任务与交付

- `tasks/001-迁移桌面主题风格遗留.md`：实现历史 `desktop.theme_style` 的兼容清理和覆盖测试；真实进程重启迁移与三页目视验收均通过。
- 提交策略：任务合同、实现、发现问题后的修复各自独立提交；不创建或更新 PR，除非获得额外授权。

## 验证与证据

- 基线：`origin/main-v3` @ `6c5282e0`。
- 直接源码证据记录在 `plan.md`；Codegraph 服务在本会话不可用。
- 根 `internal/config` 包测试通过；桌面迁移/兼容/指标聚焦测试通过；前端主题契约 30/30 与生产构建通过。
- 自动化控制组保留活动 `theme_style = "graphite"`，实验组启动读取后清除字段且保留其它偏好；重复读取不写文件；正式和隔离 resolver 均只使用临时根。
- `desktop` 完整 `go test .` 在 112 秒无输出后按时限终止，未计为通过。
- 内置浏览器按约定优先尝试访问候选 Vite 服务，但 Browser Use 的本机 URL 策略拦截了该访问。用户关闭既有实例后，候选 Wails 以隔离 profile 成功启动；主会话手动注入 `desktop.theme_style = "graphite"`，重启真实进程后确认该字段已移除，`language`、`layout_style`、`theme`、`status_bar_style` 均保留。Computer Use 的窗口截图/点击接口在本会话返回 `node_repl exec context not found`，故无法自动操作三页页面。
- 用户已人工确认候选窗口的登录、注册和找回密码页面均无旧金色遗留。

## 下一关

已完成交付，等待是否授权执行 PR Gate 并创建 Draft PR。
