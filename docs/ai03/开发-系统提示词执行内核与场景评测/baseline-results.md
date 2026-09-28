# 当前提示词真实模型去敏基线

本文件保留计划阶段的人工证据审计。实现后的可复现 evaluator 使用修复后的
`prompt-eval-v5` 重新运行；统一候选与回归对照以
[`evaluation-results.md`](./evaluation-results.md) 为准，不把两轮不同 runner 的 usage 直接混算。

## 运行身份与固定参数

- 源码：`origin/main-v3@f0f5a90f3aefcc0a8208b3f2bf97cd71d662943d`
- 运行时间：2026-08-04；模型：`gpt-5.6-luna`；协议：OpenAI-compatible `/v1/chat/completions`
- 任务：9 个诊断任务 × 2 次重复；manifest SHA-256：
  `1f1a3309b961b5fef30191652ff8c3868fb5d1eba444aafbb8a0ff769ce4528c`
- Profile：low effort、temperature 0、max steps 18、auto plan off、context window 200000
- 控制：单次 4 分钟、approval timeout 3 秒、Shell 优先 PowerShell、权限模式 allow
- 环境：Windows 无可用 OS shell sandbox，默认允许网络；writer root 只约束受控文件工具。
  权限模式 allow，显式拒绝 `rm -rf*`、`git push*`、`git commit*`。本结果是能力基线，不是安全验收。

本版本化摘要未包含 key、完整模型回复、任务绝对路径或凭据目录；原始运行结果只在 Git 忽略
目录中保留。`Raw` 是原冻结 grader 结果；`Artifact`
是对产物要求逐项审计后的结论；`Run` 还要求 Controller 正常结束。

## 逐次结果

| Task | Scene | Repeat | Raw | Artifact | Run | Tokens | Model calls | Tool calls | Duration | 审计说明 |
| --- | --- | ---: | --- | --- | --- | ---: | ---: | ---: | ---: | --- |
| meeting-summary | Office | 1 | pass | pass | pass | 261,053 | 15 | 17 | 68s | 原 grader 正确 |
| meeting-summary | Office | 2 | pass | pass | pass | 308,963 | 12 | 14 | 65s | 原 grader 正确 |
| sales-analysis | Office | 1 | fail | pass | pass | 269,925 | 21 | 23 | 101s | `49,000` 与 `49000` 等价 |
| sales-analysis | Office | 2 | fail | pass | pass | 313,902 | 16 | 18 | 逗号格式假阴性 |
| customer-email | Office | 1 | fail | pass | pass | 221,630 | 15 | 16 | 中文“主题”满足 subject |
| customer-email | Office | 2 | fail | pass | pass | 287,696 | 15 | 16 | 中文标签假阴性 |
| fix-add | Coding | 1 | pass | pass | pass | 74,399 | 4 | 3 | 16s | 聚焦验证通过 |
| fix-add | Coding | 2 | pass | pass | pass | 74,406 | 4 | 3 | 21s | 聚焦验证通过 |
| duration-parser | Coding | 1 | pass | pass | pass | 337,032 | 22 | 30 | 115s | 聚焦验证通过 |
| duration-parser | Coding | 2 | pass | pass | pass | 237,340 | 12 | 16 | 61s | 聚焦验证通过 |
| nested-config | Coding | 1 | pass | pass | pass | 303,512 | 18 | 22 | 90s | 聚焦验证通过 |
| nested-config | Coding | 2 | pass | pass | pass | 218,113 | 14 | 18 | 86s | 聚焦验证通过 |
| landing-spec | Design | 1 | fail | pass | pass | 388,770 | 20 | 20 | 147s | “避免 neon gradient”被反向判错 |
| landing-spec | Design | 2 | fail | pass | pass | 299,975 | 20 | 21 | 149s | 中文“14 天”等价表达 |
| dashboard-wireframe | Design | 1 | fail | pass | fail | 301,930 | 19 | 20 | 238s | 正确 SVG；Controller 超时 |
| dashboard-wireframe | Design | 2 | fail | pass | fail | 364,696 | 21 | 20 | 238s | 四张独立指标卡；Controller 超时 |
| checkout-audit | Design | 1 | fail | pass | pass | 262,016 | 15 | 22 | 116s | 中文“对比度”等价标签 |
| checkout-audit | Design | 2 | fail | pass | pass | 260,618 | 18 | 18 | 114s | 中文“订单摘要”等价标签 |

## 汇总

| 指标 | 结果 |
| --- | ---: |
| 原冻结 grader | 8/18（44.4%） |
| 产物语义正确 | 18/18（100%） |
| 端到端成功 | 16/18（88.9%） |
| Prompt / completion tokens | 4,728,594 / 57,382 |
| 总 tokens | 4,785,976 |
| 模型调用 / 工具调用 | 281 / 317 |
| 总耗时 | 1,937.919 秒 |

原始 grader 分数作为历史机器事实保留，但候选比较前必须修复数字格式、双语标签、否定语义
和 SVG 结构评分，并把 grader 与完整运行参数纳入 manifest。两个正确 SVG 的超时仍是端到端失败。
