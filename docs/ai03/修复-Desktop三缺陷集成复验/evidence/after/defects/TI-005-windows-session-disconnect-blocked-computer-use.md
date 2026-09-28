# TI-005：Windows 会话断连阻塞 Computer Use 审批输入

## 分类与影响

- 分类：E（测试基础设施问题），不是 Reasonix 产品缺陷。
- 发生点：L18 第一次 bash 审批。
- 产品完成态影响：无；恢复连接后，L18 在同一会话实时显示 final，无需切换且没有重复回复。
- 用户可见耗时影响：有；第一次审批观察等待 116.161 秒，L18 的原始 final 耗时保留该等待。

## 现象与证据

1. L18 审批框最晚于 `2026-08-13T12:49:31.932Z` 可见。
2. Computer Use 截图先返回 `IGraphicsCaptureItemInterop.CreateForMonitor failed (0x80070057)`，随后点击和键盘都返回 `GetCursorPos failed: 拒绝访问 (0x80070005)`。
3. accessibility-only 观察仍能读取同一 Wails 窗口和审批框，证明 Reasonix 进程与会话未崩溃。
4. `qwinsta` 显示承载 Reasonix 与 Codex 的用户会话 1 为 `Disc`；两者进程也都属于 SessionId 1。
5. 会话恢复为 `console ai03 1 Active` 后，重新绑定并激活精确 Wails 窗口，截图、树和目标一致，审批于 `2026-08-13T12:51:28.093Z` 成功提交。

## 判定与口径

- 该等待计入 L18 的原始人工审批等待和用户可见 final 耗时，因为它是真实运行中发生的端到端阻塞。
- 同时单列 E 类污染；不把它判为 DEF-008/009/010 复发，也不把等待解释为模型或 Provider 延迟。
- 后续高成本样本启动前先检查目标用户会话为 `Active`；若运行中断连，保留原始样本并明确标注基础设施等待。
