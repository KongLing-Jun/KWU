# 同步最新 main-v3 后的三场景 Desktop 冒烟报告

## 结论

同步 `origin/main-v3@d68363ff67af3b3735eca4bcb58a2375d70fdf36` 后，集成提交
`db39ae76fa364b5a09bb31e6560e2b55164fbd0a` 通过权威 PR gate、Wails 重建和三场景各一次真实
Desktop 验收：模型成品 3/3、Agent 正常结束 3/3、当前会话实时 final 3/3；切换会话 0、重复回复 0。
`DEF-002/008/009/010` 均未复现。

本轮不再重跑第二套 30 次矩阵。原 30 次是锁定在 `6a93ac2c` 的完整统计来源；同步后的上游变更没有修改
核心 Agent/提示词包，但涉及 Composer、Transcript、项目、连接器和自动化界面，因此用办公、代码、设计三个
新鲜 fixture 的真实 Desktop 链路做针对性验收更合适。三个样本不会覆盖或偷换原 30 次的成本口径。

## 产品与门禁身份

| 项目 | 结果 |
| --- | --- |
| 同步基线 | `origin/main-v3@d68363ff67af3b3735eca4bcb58a2375d70fdf36` |
| 集成提交 / tree | `db39ae76fa364b5a09bb31e6560e2b55164fbd0a` / `1122c75ccfe8ff43d6aaae492062385e39cd0ce2` |
| 模型 / effort / profile | `gpt/gpt-5.6-luna` / low / 持久 dev |
| PR gate | `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`，通过，195.6 秒 |
| Wails build | `wails build`，通过，82.8 秒 |
| 二进制 | 63,681,536 bytes；SHA-256 `01F8D2A0DF647A1C8B83147C425043608580CBCFBEBE463519FCACAE3222855D` |

同步前双方改动没有路径冲突；merge 后工作树先保持干净，再构建和运行。测试期间没有修改产品代码、系统提示词、
Provider 或运行时配置，也没有把 API Key 写入证据。

## 三次有效运行

| Run | 场景 / 任务 | prompt / completion / total tokens | 模型 / 工具 / 重复 | 审批 | complete / 拒绝 | 模型耗时 / 可见 final | 质量 / 实时完成 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| S01 | 办公 / `desktop-office-project-report` | 145,739 / 1,618 / 147,357 | 9 / 12 / 0 | 1 | 3 / 0 | 64.101s / 121.443s | 通过 / 通过 |
| S02R | 代码 / `desktop-coding-retry-policy` | 251,541 / 2,179 / 253,720 | 15 / 19 / 1 | 3 | 3 / 0 | 117.032s / 124.990s | 通过 / 通过 |
| S03R | 设计 / `desktop-design-empty-state` | 367,550 / 4,599 / 372,149 | 14 / 15 / 0 | 5 | 4 / 1 | 363.395s / 374.974s | 通过 / 通过 |
| 合计 | 3 个场景 | 764,830 / 8,396 / 773,226 | 38 / 46 / 1 | 9 | 10 / 1 | 544.528s / 621.407s | 3/3 / 3/3 |

三次均有 3 个 todo 步骤和 1 次 `todo_write`；`ask`、`wait`、ledger/readiness 拒绝均为 0。全部 tokens
来自父 Agent，child tokens 为 0。S02R/S03R 界面显示了一次 `run_skill` 调用，但没有物化 child JSONL，
telemetry 也没有 subagent 计费来源，不能把它误写成子代理运行。

审批等待时间没有在这轮短冒烟中逐次打点，因此保留为未知，不伪造为 0；审批次数来自 Desktop 可见事件。

## Desktop 完成态

| Run | 发送时间 UTC | JSONL 最后写入 | 首次确认 final + idle | 写入到观测上界 | 切会话 / 重复 |
| --- | --- | --- | --- | ---: | --- |
| S01 | 15:35:28.747 | 15:36:33.059 | 15:37:30.190 | 57.131s | 0 / 0 |
| S02R | 15:43:01.114 | 15:44:58.296 | 15:45:06.104 | 7.808s | 0 / 0 |
| S03R | 15:46:17.401 | 15:52:20.972 | 15:52:32.375 | 11.403s | 0 / 0 |

这些差值是 JSONL 文件时间到下一次 Computer Use 观测的上界，不等于精确投影延迟。S01 的轮询较疏，不能据
57.131 秒推断 Desktop 持续 loading；当次在未切换会话的活动窗口直接看见 final 和 idle composer。
三次都不符合“final 已落盘、Desktop 一直加载直到切换才显示”的失败定义。

## 成品、运行校验与视觉验收

- S01：`leadership-status.md` 与 `action-items.csv` 通过冻结 grader；三个输入 hash 均保持不变，无额外业务文件。
- S02R：修复 `retry_policy.py` 并补充 `test_retry_policy.py`；`python -m unittest -v` 的 4 项测试通过，
  冻结 grader 全通过，README 未改。测试生成两个 `__pycache__` 文件，按副作用记录，不计成品失败。
- S03R：HTML/CSS 冻结 grader 全通过，源文案文件不变；Playwright 在 1440×900 和 320×800 均通过，
  Tab 首焦点为主操作，计算焦点轮廓为 `rgb(244, 185, 66) solid 2.4px`。HTML/CSS 为 HTTP 200；
  唯一控制台错误是非必需 `/favicon.ico` 的 404。
- 每个持久 dev workspace 都有正常的 `.clouvia/desktop` topic metadata，不算越界副作用。

## 已知高成本尾部，没有新缺陷编号

S03R 首次静态检查因 Windows 默认 GBK 读取 UTF-8 Markdown 而重试，对应既有 `FIND-L10-001`。第一次
后台 HTTP 验证因 shell 后台进程回收机制运行 133.801 秒后超时；Agent 随后改用显式创建、请求、终止的
单进程方案，HTML/CSS 均返回 200，对应既有 `FIND-L12-001`。其间有一次 `complete_step` 因引用的验证
receipt 与实际命令不完全匹配而拒绝，重选真实 receipt 后通过。验收结束核对 8765、8766、8767 均无监听，
没有残留 scratch 文件。

Computer Use 的观察 JavaScript 会话曾因单次 30 秒等待超时重置；重新绑定的是同一个 Wails 窗口，应用没有
关闭，也没有切换 Reasonix 会话。Playwright 的 `file:` 导航被工具策略拒绝后改用临时 localhost，属于 E 类
测试设施事件，不影响产品判定。

## 与完整矩阵首组三个匹配样本的方向性比较

| 指标 | `6a93ac2c` L01/L02/L03 | `db39ae76` S01/S02R/S03R | 变化 |
| --- | ---: | ---: | ---: |
| 质量 / 正常 / 实时 final | 3/3 / 3/3 / 3/3 | 3/3 / 3/3 / 3/3 | 持平 |
| 总 tokens | 505,916 | 773,226 | +52.84% |
| 模型调用 | 33 | 38 | +15.15% |
| 工具调用 | 52 | 46 | -11.54% |
| 重复工具调用 | 3 | 1 | -66.67% |
| 审批 | 11 | 9 | -18.18% |
| 模型耗时 | 621.207s | 544.528s | -12.34% |
| 用户可见 final 总时长 | 656.744s | 621.407s | -5.38% |

这是每任务 N=1 的配对观察，不做显著性或趋势外推。tokens 上升主要集中在 S03R（相对 L03 +103.14%），
其运行明确命中了既有后台 HTTP 生命周期尾部；与此同时总模型耗时、用户可见耗时、工具、审批和重复调用都下降。
因此该样本必须作为成本方差警报保留，但不足以推翻锁定 30 次的统计，也不足以把变化归因于同步后的 UI/项目代码。

## 是否再跑 30 次

决定：不自动再烧第二轮 30 次。

理由是完整矩阵已经在包含 DEF-002/008/009/010 的锁定产品上完成；同步后权威门禁全绿，核心 Agent/提示词
包未变化，而三场景的新鲜 fixture 已覆盖新的 Wails 二进制、Composer/Transcript 投影、审批、成品、运行校验
和 final 完成态。重复 30 次的主要新增信息会是模型/Provider 方差采样，而不是对这批上游代码的针对性验证。

若后续要专门确认 tokens +52.84% 是否可重复，应另立“成本方差复验”任务，先修或隔离
`FIND-L12-001`，再按相同三个任务至少每项 4 次；不能把当前 3 次冒烟伪装成新的 full-30 成本结论。
