# 系统提示词执行内核与场景评测进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Extended V2 实现与真实 Desktop A/B 已完成，进入本地最终门禁 |
| 稳定计划 | [`plan.md`](./plan.md) |
| 完整证据 | [`evidence.md`](./evidence.md)、[`desktop-ab-results.md`](./desktop-ab-results.md) |
| 当前基线 | `origin/main-v3@a4029fd0` |
| 主候选 | Extended V2；候选与生产默认提示词一致 |
| 产品提示词 | 通用执行内核与三个 Scene Profile 已定向修改并通过真实模型验收 |
| 内核门禁 | 3/3 smoke；确认运行 5/6 raw 通过，1/6 由冻结 grader/Controller 缺陷污染 |
| Desktop A/B | 24/24 完成；候选质量 12/12，tokens -17.4%，模型调用 -23.7% |
| provider-auto | Office、Coding、Design 3/3 兼容 smoke 通过 |
| PR | 未授权推送或创建 PR |

## 决策摘要

- 接受 Extended V2：相对 Extended 静态 runes 仅增加约 3.3%，完整 Desktop A/B 的任务质量
  与基线持平，完整运行 tokens、模型调用、工具调用和模型耗时均下降。
- 不把提示词长度当作能力代理；结论来自六个冻结中型任务、两轮交替 A/B 和三场景真实视觉/
  行为证据。
- 候选审批数 62→67，Desktop 正常结束 7/12→6/12；这两项没有改善，不作为候选收益。
- RF-001～007 已记录。Controller readiness 与 grader 隐藏/词法契约不在本任务修复；测试关窗
  后后台运行已确认符合 Profile 配置，不作为产品缺陷，也没有用提示词掩盖问题。
- 混合 raw 结果已有源码与产物证据完成归因，两轮任务质量一致，因此不追加无效第三轮。

## 任务

| 任务 | 状态 |
| --- | --- |
| [`001`](./tasks/001-建立真实提示词评测器.md) | completed：上一轮真实 runner、grader、隔离与脱敏 |
| [`002`](./tasks/002-优化通用执行内核.md) | completed/no-go：上一轮多候选未通过硬门槛 |
| [`003`](./tasks/003-优化三场景Profile.md) | completed/no-go：上一轮增强 Profile 退化并回滚 |
| [`004`](./tasks/004-真实Desktop应用验收.md) | completed：上一轮基线兼容 smoke |
| [`005`](./tasks/005-建立Desktop中型任务评测资产.md) | completed：六个确定性 fixture、grader 与观察契约 |
| [`006`](./tasks/006-定向优化系统提示词与Profile.md) | completed：实现、本地契约与真实模型内核门禁 |
| [`007`](./tasks/007-完成真实Desktop自适应AB验收.md) | completed：24 次 A/B、3 次 provider-auto smoke 与独立视觉验收 |

## 下一关口

完成本地代码/文档门禁、清理一次性凭据和测试进程后交付；如需 PR，须另行获得授权并先同步
最新 `main-v3`。
