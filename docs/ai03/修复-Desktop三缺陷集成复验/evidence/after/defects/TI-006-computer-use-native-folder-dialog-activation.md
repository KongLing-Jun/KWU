# TI-006：Computer Use 原生文件夹对话框目标缓存/激活异常

## 分类与影响

- 分类：E（测试基础设施问题），目前没有证据支持 Reasonix 产品崩溃。
- 发生点：L19 准备阶段；尚未发送 L19 模型请求，因此不计入 30 次样本。
- 影响：前两次尝试在原生 `Choose working folder` 对话框中输入路径后，Computer Use 对失效元素或坐标执行点击失败；随后 `kwu-desktop.exe` 进程不可见。第三次用纯键盘路径成功选择同一目录，应用继续正常运行。

## 复现与边界证据

1. 从 L18 当前会话点击“打开本地工作空间”，原生文件夹对话框正常出现。
2. 输入完整 L19 路径后，第一次按已过期元素索引点击返回 `element ... is not available`；坐标点击随后返回 `failed to activate captured window`，检查时进程已不存在。
3. 重新启动锁定二进制后重复相同准备流程，失效元素/坐标激活路径再次导致进程不可见。
4. 第三次重新启动同一二进制，保持同一 dev 环境和同一 L19 目录，改用：输入完整路径 → `Return` 导航到目录 → `Tab` 聚焦“选择文件夹” → `Return`。目录成功注册，Desktop 显示 L19 空白工作空间且进程保持响应。
5. 两次异常发生时间窗内，Windows Application 日志无 `Application Error`、`Windows Error Reporting` 或 `.NET Runtime` 事件；WER archive 无新增报告；L19 未创建会话或发送模型请求。

## 固定证据

- 首次对话框：`screenshots/L19/00-preflight-folder-dialog.jpg`、`01-preflight-folder-dialog-crop.jpg`。
- 第二次对话框：`screenshots/L19/02-retry-folder-dialog.jpg`。
- 成功键盘恢复：`screenshots/L19/03-recovery-select-folder-focus.jpg`、`04-ready-after-keyboard-recovery.jpg`。
- 成功恢复后的 Wails 进程：PID `34248`，启动时间 `2026-08-13 21:15:00 +08:00`，窗口标题“云智企办”，`Responding=True`。

## 当前处置

- 对原生文件夹选择器不再使用 Wails 主窗口缓存中的对话框元素索引或坐标点击。
- 后续工作空间选择统一采用可观察的键盘流程，每一步重新观察后再输入。
- 本问题单列为测试基础设施缺陷；除非脱离 Computer Use 后能证明 Reasonix 自身退出，否则不计为 Desktop 产品稳定性失败。
