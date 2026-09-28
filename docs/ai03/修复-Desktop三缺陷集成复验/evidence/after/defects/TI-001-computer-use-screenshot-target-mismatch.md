# TI-001：Computer Use 截图与目标窗口/可访问性树不一致

## 分类与影响

- 分类：E（测试基础设施问题），不是 Reasonix 产品缺陷。
- 状态：累计可复现 5 次；通过重新枚举或激活精确窗口并重新取证可恢复。
- 产品执行影响：无；两次都未在错窗状态执行输入动作。
- 取证影响：若只看截图、不交叉核对 `Window.app` 与 accessibility tree，可能误把 Codex 前台窗口当作
  Reasonix 证据，甚至发生误点击。

## 现象

调用 `get_window_state` 时，返回的 `window.app` 仍为：

`process:D:\Reasonix-worktrees\integration-def002-def008-def009\desktop\build\bin\kwu-desktop.exe`

accessibility tree 也仍是“云智企办”及其 composer/final 控件，但 JPEG 截图显示调用工具时重新获得前台焦点的
Codex 窗口。

## 五次发生点

1. L01 发送前配置阶段：发生在 L01 topic 创建 `2026-08-13T09:33:52.957Z` 与模型/effort 元数据写入
   `2026-08-13T09:35:38.204Z` 之间。未点击；重新枚举并激活精确进程窗口后截图恢复正常。
2. L02 final 稳定性观察：`2026-08-13T09:49:05.306Z`，返回树显示 L02 final、2 个 copy 控件、无 stop，
   但图片显示 Codex。未保存错窗图片、未点击；重新枚举并激活后，于 `2026-08-13T09:49:37.297Z`
   获得路径、树和截图一致的 L02 final 证据。
3. L18 第一次审批的 Windows 会话恢复后：返回树显示 L18 bash 审批框，但 JPEG 显示 Codex。未保存错窗图片、
   未点击；重新 `get_window` 并激活精确 Wails 进程后，获得树、审批框截图和进程路径一致的状态，再执行审批。
4. L19/L20 阶段发生一次同类错配，逐 run observation 已引用本记录；未在错配状态动作，重新激活后继续。
5. A06 final 候选后稳定性观察：返回树显示 A06 的“最终回复尚未满足完成条件，正在继续处理”，JPEG 却显示
   Codex。错配截图保存为 `screenshots/A06/11-ti001-tree-screenshot-mismatch.jpg`，随后只执行窗口激活与重新观察；
   新状态正确显示 Wails 的 `kill_shell` 审批后才继续点击。

## 复现流程

1. 通过 `list_apps` 选择精确 Reasonix process path，并 `get_window`。
2. `activate_window` 后调用 `get_window_state(include_screenshot=true, include_text=true)`，可得到正确画面。
3. 在后续工具调用让 Codex 恢复前台后，再对保存的 Reasonix `Window` 调用 `get_window_state`。
4. 偶发得到 Reasonix `window`/tree + Codex JPEG 的混合结果。

## 当前防误操作合同

- 每次输入前重新核对精确 `Window.app`、关键 accessibility 控件和截图三者一致。
- 一旦不一致，不保存截图、不执行点击或键盘输入。
- 重新 `list_apps`，要求精确进程路径仅有 1 个窗口，重新 `get_window`、`activate_window`、再观察。
- 本问题不改变任何 Reasonix Desktop 样本的产品质量或实时完成态判定，只单列 E 类取证风险。

前四次错窗截图按最小化原则未写入测试证据目录；A06 为证明 final/retracted-candidate 与审批时序，保留了一张
受控错配截图。它可能包含无关 Codex 任务内容，仅用于本地审计，不影响任何 Reasonix 产品判定。
