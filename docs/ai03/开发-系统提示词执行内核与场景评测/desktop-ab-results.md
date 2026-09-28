# Extended V2 真实 Desktop A/B 结果

## 结论

Extended V2 通过本轮提示词验收，可以作为生产默认提示词候选保留。24 次 low-effort 真实
Desktop A/B 中，基线与候选经缺陷归因后的任务语义/视觉质量均为 12/12；候选总 tokens
降低 17.4%，模型调用降低 23.7%，工具调用降低 16.2%，模型耗时降低 9.3%。候选没有以更长
提示词换来更高完整运行成本。

这不是“所有问题已修复”的结论。Controller 的 artifact readiness 缺陷 RF-003 同时污染两组，
候选正常结束 6/12、基线 7/12，不能据此证明完成态修复已经落地。候选的明确收益是：在不降低
Office、Coding、Design 产物质量和真实视觉质量的前提下，整体执行回合与 token 成本下降。

## 构建与固定条件

| 构建 | 源码提交 | 测试二进制 SHA-256 |
| --- | --- | --- |
| baseline | `ee4c1dc6faaf8d9fa8bdfa23cfbb2c4dbef515bb` | `9730ed093ffd8066441181f9e6176583ddcd417d3283d0a75c158b7b982a18d3` |
| candidate | `adb40277b9b2e1f159963e74d7665711c6e1e216` | `22b2a38eb527023e5c3a2cef68afd2697077ba5580b57c00d38c2c0c8022bbe9` |

- 两个程序均由 Wails v2.12.0 production build 生成；仅在输出目录的 Wails 副本上启用
  `testdiag` WebView2 CDP，patch SHA-256 相同，生产源码未因此修改。
- 两组运行使用用户确认的同一自定义供应商与 `gpt-5.6-luna`，temperature 0、provider 默认
  low effort、auto plan off、产品原生 max steps 0、默认 ask 权限。
- 六个冻结中型任务各运行 baseline/candidate 两次，共 24 次；每次使用独立 workspace，输入
  保留、Desktop 可见状态、grader、模型/工具调用、耗时和一次性审批分开记录。
- Design 产物由独立 Playwright 在 1440×900 与 390×844 打开，并用键盘实际聚焦主要操作。
- 候选通过后临时移除 provider `default_effort`，Office/Coding/Design 各追加一次真正省略
  provider effort 字段的兼容 smoke。
- 四次不满足隔离条件的尝试未计入矩阵：一次候选 Office 使用了错误 Profile、两次候选预算
  选错 workspace、一次 baseline Coding 创建了空会话。保留原始忽略目录，不用其结果补分。

## 24 次 low-effort 矩阵

“质量”是剔除已证实 grader/Controller 缺陷后的任务语义与 Design 视觉结果；“正常结束”严格
使用 Desktop 最终状态，不用正确产物覆盖运行异常。

| 任务 | baseline 质量 / 正常结束 | candidate 质量 / 正常结束 | 模型调用 B→C | 工具调用 B→C | 主要归因 |
| --- | --- | --- | ---: | ---: | --- |
| Office 项目报告 | 2/2 · 0/2 | 2/2 · 0/2 | 71→60 | 79→68 | 两组均受 RF-003 污染 |
| Office 预算分析 | 2/2 · 2/2 | 2/2 · 2/2 | 60→38 | 59→37 | 各一轮受 RF-004 词法误判 |
| Coding 重试策略 | 2/2 · 2/2 | 2/2 · 2/2 | 31→24 | 26→34 | baseline 2/2 受 RF-005 隐藏异常类型污染 |
| Coding 深层合并 | 2/2 · 0/2 | 2/2 · 0/2 | 58→55 | 74→67 | 真实测试全通过；两组均受 RF-003 污染 |
| Design 空状态 | 2/2 · 2/2 | 2/2 · 2/2 | 72→36 | 62→43 | 两组桌面/窄屏/焦点均通过 |
| Design 仪表盘 | 2/2 · 1/2 | 2/2 · 0/2 | 71→64 | 90→78 | 四个页面视觉通过；RF-003 与 RF-007 污染 |

没有追加第三轮：混合 raw grader 结果均已由源码/产物证据归因到冻结契约缺陷；RF-003 是跨构建
稳定复现的运行时缺陷；其余任务两轮在任务质量上没有冲突。继续采样不能解决归因，只会增加成本。

## 汇总成本

| 指标 | baseline | candidate | 候选变化 |
| --- | ---: | ---: | ---: |
| 原始 artifact grader | 7/12 | 9/12 | +2 |
| 原始端到端 | 3/12 | 5/12 | +2 |
| 缺陷归因后任务质量 | 12/12 | 12/12 | 持平 |
| Desktop 正常结束 | 7/12 | 6/12 | -1；RF-003 污染 |
| 总 tokens | 6,122,252 | 5,057,932 | **-17.4%** |
| prompt / completion tokens | 6,052,855 / 69,397 | 4,997,654 / 60,278 | -17.4% / -13.1% |
| 模型调用 | 363 | 277 | **-23.7%** |
| 工具调用 | 390 | 327 | **-16.2%** |
| 模型耗时 | 2,519.8s | 2,285.2s | **-9.3%** |
| 已记录一次性审批 | 62 | 67 | +8.1% |

| Scene | baseline tokens | candidate tokens | 变化 | 模型调用 B→C | 工具调用 B→C |
| --- | ---: | ---: | ---: | ---: | ---: |
| Office | 2,217,265 | 1,667,249 | -24.8% | 131→98 | 138→105 |
| Coding | 1,380,241 | 1,414,279 | +2.5% | 89→79 | 100→101 |
| Design | 2,524,746 | 1,976,404 | -21.7% | 143→100 | 152→121 |

候选审批次数没有改善，Office 反而由 19 增至 26；因此“更少审批”不是本轮收益。Coding tokens
小幅增加但模型调用下降，说明单次上下文变大；后续若要继续优化，优先检查 Coding 上下文携带，
不能继续盲目增加静态提示词。

## provider-auto 兼容 smoke

| Scene / 任务 | Desktop | 质量 | tokens | 模型 / 工具调用 | 归因 |
| --- | --- | --- | ---: | ---: | --- |
| Office / 预算分析 | 正常 | 通过 | 296,166 | 17 / 22 | raw grader 受 RF-004 污染 |
| Coding / 重试策略 | 正常；独立 6 tests 通过 | 通过 | 305,114 | 14 / 19 | hidden grader 受 RF-005 污染 |
| Design / 空状态 | 正常；双 viewport 与焦点通过 | 通过 | 424,834 | 22 / 26 | grader 全通过 |

三个新会话的 branch meta 均保存空 effort override；同时候选验收 Profile 已移除
`default_effort`，因此请求不会再回落到 low，而是省略供应商特定 effort 字段。三场景兼容性通过。

## 缺陷与边界

- RF-001：评测 runner 拒绝产品定义为“不限步数”的 `max_steps=0`；内核门禁使用 10000 绕行，
  Desktop 仍使用产品原生 0。
- RF-002：Profile Card grader 把角色/焦点语义限定为英文词面。
- RF-003：最新写入后调用 `verify_artifact`，readiness 仍报告未验证。A/B 中 baseline 5 次、
  candidate 6 次复现；不属于提示词可掩盖的问题。
- RF-004：预算 grader 不接受 Unicode 负号或中文等价事实，人工核对的数值和语义正确。
- RF-005：重试任务只要求拒绝非法值，hidden grader 却只接受 `ValueError`；`TypeError` 用于非法
  类型是合理实现，真实本地测试均通过。
- RF-006：验收清理观察（非产品缺陷）。测试 Profile 明确设置 `close_behavior = "background"`，
  因此关窗后进程继续存活符合配置；最终按已确认 PID 清理，不据此创建运行时修复任务。
- RF-007：Design dashboard grader 隐含要求 HTML 字面引用 SVG、SVG 显式宽高至少 900×600、
  卡片矩形高度至少 70；冻结 prompt/brief 未声明这些几何阈值。四个实际页面均成功渲染并通过
  双 viewport、无横向溢出与键盘焦点验收。
- CDP 轮询偶发超时通过重新附着同一 Wails 页面恢复，未重启或替换当轮 Desktop；这属于验收
  控制链路限制，不计为产品缺陷。受影响轮次的审批数只记录已确认点击数。

## 决策

1. 保留并接受 Extended V2；不再为冻结 grader 的词法/隐藏几何阈值修改提示词。
2. 本轮不修改 Controller、Desktop、grader 或权限运行时；RF-001～007 作为后续独立任务输入。
3. 不声称完成态问题已由提示词修好。候选价值是同等任务质量下的整体效率收益，以及 Coding、
   Design 验证约束在真实 Desktop 中没有造成回归。
4. Extended V2 比上一轮 Extended 仅增加约 3.3% 静态 runes，但完整 A/B tokens 下降 17.4%；
   该结果支持“效果与完整运行成本优先”，不支持“提示词越多越强”。
