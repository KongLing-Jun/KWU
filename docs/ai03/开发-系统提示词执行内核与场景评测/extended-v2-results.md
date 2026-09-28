# Extended V2 内核门禁结果

## 结论

Extended V2 可以进入真实 Desktop A/B。三个场景 smoke 全部通过；六个确认任务中五个产物与
Controller 均通过，Profile Card 的两次 SVG 产物经源码复核均满足姓名、角色、指标、操作与
焦点样式要求，但被 grader 词法规则误判，同时复现 Controller readiness 状态异常。因此该项
运行态记为不可判定，不归因于提示词，也不阻断其余验收。

与上一轮 Extended 的同组回归相比，V2 使预算、重试和空状态三个不稳定项本次通过，Profile
Card 的产物语义也正确；完成态缺陷仍需由 Desktop 判断。当前没有证据支持继续增加提示词，
也没有证据支持为迎合 grader 修改文案。

## 固定条件与偏差

- 供应商、模型、temperature、effort、auto plan 和超时均按已确认配置；凭据未进入报告或 Git。
- 候选 SHA-256：`b1bd7666069b368866e94efaa2ba55544ab8315e84357be793ec842d5f12fcc6`；
  3542 runes。上一轮 Extended 为 3428，V2 静态长度增加约 3.3%。
- 因 RF-001，Controller runner 使用 `max_steps=10000` 代替产品支持但 runner 拒绝的 `0`；
  对本组任务实际等同不限步数。Desktop 仍使用产品原生 `0`。
- 所有原始报告和工作区位于 Git 忽略的 `.codex-run/prompt-eval/results/v2-*`。

## 结果

| 阶段 | 任务 | Artifact | Run | Tokens | 模型调用 | 工具调用 | 秒 | 结论 |
| --- | --- | --- | --- | ---: | ---: | ---: | ---: | --- |
| smoke | office-meeting-summary | pass | pass | 293,021 | 11 | 13 | 45.3 | 通过 |
| smoke | coding-fix-add | pass | pass | 98,508 | 5 | 4 | 18.6 | 通过 |
| smoke | design-landing-spec | pass | pass | 309,010 | 13 | 15 | 138.4 | 通过 |
| confirm | office-project-status | pass | pass | 212,268 | 13 | 15 | 102.7 | 通过 |
| confirm | office-budget-variance | pass | pass | 263,724 | 10 | 13 | 56.0 | 通过 |
| confirm | coding-normalize-tags | pass | pass | 98,873 | 5 | 4 | 45.1 | 通过 |
| confirm | coding-retry-delay | pass | pass | 267,669 | 23 | 33 | 108.8 | 通过；核心语义回归通过 |
| confirm | design-empty-state-spec | pass | pass | 265,836 | 15 | 16 | 137.1 | 通过 |
| confirm | design-profile-card-svg | false negative | fail | 430,984 | 29 | 26 | 204.4 | RF-002/RF-003，不可判定 |
| replacement | design-profile-card-svg | false negative | fail | 469,475 | 27 | 24 | 162.9 | RF-002/RF-003 稳定复现 |

## 需要 Desktop 验证的风险

- Office 和文档型 Design 虽正常结束，仍常用 10–15 次模型调用；效果合格但效率不理想。
- Coding 重试题语义正确并正常结束，但用了 23 次模型调用；需观察中型任务是否因工具链而放大。
- Profile Card 两次都在最后一次 `verify_artifact` 后被 readiness 拒绝；真实 Desktop 必须单独记录
  是否完成、是否等待用户、是否继续调用，以及实际预览中的焦点可见性。
