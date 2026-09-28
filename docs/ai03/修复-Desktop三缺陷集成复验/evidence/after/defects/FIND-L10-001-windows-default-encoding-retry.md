# FIND-L10-001：Windows 默认文本编码导致可避免的验证重试

## 分类与结论

- 分类：D（模型生成的跨平台命令不稳健），没有证据指向 Reasonix Agent 运行时或 Desktop 投影。
- 严重度：低；成品内容、最终质量和完成态均通过，但增加了 1 次失败工具调用、1 次模型续跑和 1 次审批。
- 现象在 L03 的静态校验中已经出现过一次，L10 再次复现，说明模型在 Windows 上使用 `Path.read_text()` 时没有稳定显式指定 UTF-8。
- 当前任务只记录，不修改产品代码、提示词或运行时配置。

## L10 现象与恢复

模型先运行 `Path('budget-analysis.md').read_text()` 与
`Path('decision-summary.md').read_text()`。Windows Python 使用 GBK 默认编码，读取 UTF-8 Markdown 时抛出
`UnicodeDecodeError: 'gbk' codec can't decode byte 0x92 ...`。模型随后将两处调用改成
`read_text(encoding='utf-8')`，第二次验证通过并正常结束。

## 固定证据

- 会话：`20260813-112458.402889500-gpt-gpt-5.6-luna.jsonl`。
- 第一次失败与 UTF-8 重试分别形成两次 bash 调用和两次人工审批；L10 总计 3 次 bash 审批。
- 冻结 grader 对两份成品与两个输入保护检查全部通过；Desktop 当前会话实时显示 final，待办 `4/4`，无需切换会话。
- L03 的完成记录也明确记载第一次静态 Python 检查因 Windows GBK 默认编码失败，显式 UTF-8 后通过。

## 判定边界

- 模型交付质量：通过。
- Agent 执行效率：轻微失败，多消耗一次失败命令、一次审批与后续模型处理。
- Desktop 实时完成稳定性：通过，与本问题无关。
- 建议在本轮结束后的提示词优化建议中加入 Windows 文本读写显式 UTF-8 的通用约束；本轮不改变冻结提示词。
