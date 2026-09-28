# low repeat 2 前半检查点（L07–L09）

## 锁定身份

- `origin/main-v3`: `6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad`
- Git tree: `378e8490d5abf766f7c2d21fb907caa400abaadb`
- Desktop binary SHA-256: `FC1FA59E7927B6C95723021F57F2B214E5AF4D093113DB0B40D538FD618F0A86`
- 模型：`gpt-5.6-luna`
- effort：fixed `low`
- Desktop：真实 Wails Desktop；持久化 dev 环境；Computer Use 由顶层任务执行
- 产物视觉：Playwright + 系统 Microsoft Edge；不替代 Desktop 完成态验收

## 结果

| run | 场景 / 任务 | 质量 | 执行结束 | Desktop 实时 final | tokens | 模型调用 | 工具调用 | 审批 | 模型耗时 | 用户可见 final |
| --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| L07 | 日常办公 / project-report r2 | 通过 | 恢复后完成 | 通过；无切换、无重复 | 149,119 | 9 | 13 | 3 | 108.500s | 118.137s |
| L08 | 代码交付 / retry-policy r2 | 通过 | 恢复后完成 | 通过；无切换、无重复 | 222,753 | 13 | 21 | 7 | 222.779s | 229.837s |
| L09 | 设计 / empty-state r2 | 通过 | 直接完成 | 通过；无切换、无重复 | 141,752 | 8 | 14 | 6 | 231.699s | 239.297s |

三次合计：prompt 505,227、completion 8,397、total 513,624 tokens；模型调用 30、工具调用 48、审批 16，
模型耗时 562.978s。审批首见到点击累计 150.547s。todo 9 步、`todo_write` 4 次、`complete_step` 12 次，
其中拒绝 3 次；ledger/readiness 拒绝 0、ask 0、wait 0、child session 0。工具重复仅 L09 一次非连续 `ls` 签名。

## 三条独立结论

1. 模型交付质量：3/3。L07、L08 的 artifact/端到端 grader 通过；L09 artifact 与端到端 grader 12/12，
   Playwright 另验精确文案、语义结构、1440×1000、390×844、键盘焦点、深色模式、reduced-motion、无渐变、
   无控制台或页面错误。
2. Agent 执行效率：相对旧 12 次发布口径的每样本均值，tokens -59.4%、模型调用 -56.7%、工具调用 -41.3%、
   审批 -4.5%、模型耗时 -1.5%。这是三样本检查点，不是 L01–L12 完整严格对比。
3. Desktop 实时完成稳定性：3/3 final 在当前会话出现并保持稳定；均未切换会话、未出现重复 final。
   JSONL final 落盘到首次有时间戳的画面观察差分别为 L07 9.285s、L08 6.782s、L09 7.299s；三次首次
   观察均已显示 final，未见“持续加载直到切换”失效模式。

## 恢复与问题

- L07：一次 `complete_step` 使用了不匹配的验证命令证据，被拒后改引已有成功回执并完成。
- L08：fixture 未安装 pytest，首次 pytest 非零；切换到标准库 unittest 后 5/5 通过。两次 `complete_step`
  证据拒绝后完成。
- L09：生成阶段自行修正一个 CSS 值；三步均首次签收，无拒绝。一次非连续重复 `ls` 不影响结果。
- 本检查点没有新增确定性产品缺陷；保留既有缺陷记录继续观察。以上恢复现象记录为运行效率数据，不在本任务修复。

## 累计状态

L01–L09 累计质量 9/9、当前会话实时 final 9/9；直接完成 3 次、恢复后完成 6 次。累计 prompt 1,863,731、
completion 31,238、total 1,894,969 tokens；模型调用 108、工具调用 142、审批 46、模型耗时 1,896.518s。
产品 Git worktree 保持 clean，锁定 tree 与 binary hash 不变，证据目录 API key 模式扫描 0 命中。

下一阶段执行 repeat 2 后半 L10–L12，完成前 12 次严格可比矩阵后再给出完整修复前/修复后对比。
