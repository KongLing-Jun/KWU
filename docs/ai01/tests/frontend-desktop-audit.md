# 前端与桌面端测试

## 测试范围

### Chrome 真实页面

通过集成分支 Vite 服务 `http://127.0.0.1:5284/` 验证：

- 首页导航、创作模式、业务场景、输入框、模型/技能/连接器入口。
- 消息通道：QQ、飞书、Lark、微信四个页签。
- QQ App ID、App Secret 的必填与启用约束。
- 项目页、专家/专家团/分类页、自动化页、模型广场。
- 账户菜单、套餐与支付、设置、更新、外观入口的可达性。
- 页面切换后控制台无新增错误。

浏览器直连 Vite 使用项目 mock bridge，能证明 React 页面、组件状态和前端契约，但不能代替 Wails、文件系统、SQLite 或真实云端调用。

### Wails 真实桌面窗口

目标进程：

`D:\project\Reasonix-worktrees\integration-main-v3-20260724\desktop\build\bin\kwu-desktop-dev.exe`

真实窗口标题为“云智企办”。可访问性树确认以下元素来自该进程：

- 主导航：历史、新建对话、消息通道、项目、专家、自动化、模型广场。
- 任务、工作空间和账户区域。
- 登录/切换模型状态条。
- 日常办公、代码开发、设计创意三个创作模式。
- 文档处理、金融服务、数据分析及可视化、更多业务场景。
- 输入框、协作方式、技能、连接器、工具批准模式和发送按钮。

桌面控制工具在后续截图阶段把图像返回到另一个前台窗口。为避免误操作，本轮停止了基于坐标的桌面点击；这属于测试工具定位限制，不是 Reasonix 产品缺陷。

## 自动化结果

| 命令 | 结果 | 证据摘要 |
| --- | --- | --- |
| `pnpm test:all` | PASS | 129 个前端测试文件全部退出码 0 |
| `pnpm build` | PASS | CSS、z-index、TypeScript、Vite 生产构建通过 |
| `pnpm test:browser` | PASS | 5 个浏览器场景全部通过 |

构建仍存在非阻塞体积警告：主 CSS 约 761 kB，主 JS 约 703 kB，Vite 提示部分 chunk 超过 600 kB。开发 Mock 已从生产主包移除；语言资源只保留简体中文，约 116 kB（gzip 约 36 kB）。本轮没有运行加载性能基准，不能据此判定用户可见性能变化。

## 问题 FE-QA-001

- 严重度：Medium
- 类型：浏览器测试基础设施契约漂移
- 影响：`pnpm test:browser` 稳定失败，浏览器回归门禁无法作为可靠绿灯。

### 复现

```powershell
cd D:\project\Reasonix-worktrees\integration-main-v3-20260724\desktop\frontend
pnpm test:browser
```

结果：

- `exposes login recovery, retry, and model switch actions` 等待 `Retry` 超时。
- `keeps sending disabled while the model service is offline` 等待 `Retry` 超时。
- 其余 3 项通过。

### 根因证据

1. `e2e/browser-smoke.spec.ts:24` 在登录恢复场景点击 `Retry`。
2. 产品组件 `src/components/SendAvailabilityBanner.tsx:63` 只有当 `retryable === true` 且 `action === "retry"` 时才渲染 `Retry`。
3. `src/vitest/component-send-availability.vitest.test.tsx:42` 明确断言登录恢复场景不应出现 `Retry`，说明 Playwright 断言已落后于当前产品契约。
4. `e2e/fixtures/browser-smoke.tsx:23-27` 的 offline 夹具设置了 `retryable: true`，但没有设置 `action: "retry"`，所以组件按契约不渲染按钮。

### 修复与回归

- 登录恢复 Playwright 场景删除 `Retry` 点击，只验证 `Sign in` 与 `Switch model`。
- offline 夹具补充 `action: "retry"`，继续验证 retry 回调。
- 修复后 `pnpm test:browser` 5/5 通过。

## 问题 FE-QA-002

- 严重度：Medium
- 类型：自动化任务卡片交互一致性
- 影响：不同状态的条目展示不同数量的操作按钮，部分按钮仅在悬停时出现，造成截图所示的布局和可发现性不一致。
- 根因：样式层按悬停状态隐藏操作区，而不同任务状态又分别决定运行、打开和删除按钮是否可用。
- 修复：操作槽位始终可见；不可执行的动作保留禁用态和弱化视觉，避免卡片间跳动。
- 回归：新增自动化面板契约测试，覆盖按钮槽位与禁用态。

## 问题 FE-QA-003

- 严重度：Medium
- 类型：生产资源边界
- 影响：浏览器开发 Mock 静态进入生产主包；英文、简体中文、繁体中文三个语言包同步进入前端资源。
- 根因：bridge 静态导入 Mock，i18n 静态导入全部语言资源。
- 修复：Mock 仅在 Vite 开发模式按需导入；删除英文和繁体中文语言包，界面语言固定为简体中文，同时兼容读取历史语言偏好。
- 回归：生产构建中不存在 Mock 标记及英文/繁中语言 chunk；类型检查、组件测试和浏览器测试通过。

## 未执行的桌面真实副作用

- 未向聊天后端发送消息，因为当前集成配置没有可用模型。
- 未选择真实目录、上传文件或执行 shell。
- 未登录、注册、支付、扫码或发送 IM 消息。
- 未删除任务、会话、专家、自动化或本地文件。

这些项目不应标记为通过；对应前端状态与 bridge 调用由现有自动化测试覆盖。
