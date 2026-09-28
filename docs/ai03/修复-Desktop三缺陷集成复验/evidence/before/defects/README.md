# 缺陷记录索引

本目录只记录证据，不实施修复。编号格式为 `DEF-001`。

每个缺陷必须包含：

- 分类 A/B/C/D/E；
- 首次 run 与复现 run；
- 用户可见现象和严格判定；
- 精确复现步骤；
- workspace、父/child session、截图和去敏日志路径；
- tokens、模型/工具/审批/计划步骤和完成态影响；
- 源码只读调查线索与置信度；
- 建议对策、风险和独立后续任务边界。

final 落盘但当前页持续 loading、切换后恢复的缺陷必须标为 C 类实时完成失败，不得因恢复改判通过。

## 本轮索引

| 编号 | 分类 | 摘要 | 影响 |
| --- | --- | --- | --- |
| DEF-001 | E | 旧基线一个 project 混用两个父会话 | 历史 tokens 口径审计 |
| DEF-002 | D/B/E | L02 receipt/readiness 多重恢复 | 质量通过，增加步骤与成本 |
| DEF-003 | B/D/E | Windows 默认 GBK 与本地校验恢复 | 4 次样本引用，均恢复 |
| DEF-004 | E | Computer Use 无法可靠识别 Edge URL | 改由 Playwright 补充 artifact 验收 |
| DEF-005 | D | L03 缺少 favicon | 非阻断 404 |
| DEF-006 | E | 自定义 Playwright 长脚本卡住 | 拆分 CLI/短脚本后验收成功 |
| DEF-007 | D/E | L18 dashboard 响应式溢出 | 质量失败；L24 未复现 |
| DEF-008 | B | L21 首轮 `.jsonl.meta` 文件锁 | 模型前硬失败，无成品/final |
| DEF-009 | C | A06 final 已显示但 stop/loading 延迟清除 | 实时 final 通过，完成尾延迟 26.672s |
