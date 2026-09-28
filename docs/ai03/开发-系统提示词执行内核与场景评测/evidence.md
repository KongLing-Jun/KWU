# 系统提示词执行内核与场景评测证据

## 1. 基线身份

- 第一轮源码与 Codegraph 导航基线：`origin/main-v3@f0f5a90f3aefcc0a8208b3f2bf97cd71d662943d`。
- 第二轮开始前已抓取并合并 `origin/main-v3@a4029fd0`；31 个基线提交未修改
  `DefaultSystemPrompt`、Scene Profile 或 `boot.Build` 的提示词组装路径，独立 worktree 保持干净，
  主工作区保持只读。
- WorkBuddy 本机安装包：`genieVersion=5.3.8`，
  `commit=e9991e2be9dcafcce0fad23fc065dd91a7f3efed`；来源为
  `D:/workbuddy/resources/app.asar.unpacked/cli/product.json` 等产品清单。
- WorkBuddy 证据只用于观察模块化 Agent、Skills、子代理、运行时提醒、Output Style 和工作流组织，
  不足以证明其内部根提示词全文或与本应用相同的三 Scene 设计。
- 对本轮最有价值的参考不是提示词长度，而是职责分离：稳定的通用 Agent 行为、按需 Skills、
  专业角色/子代理和独立 Output Style。对应方案已落到“通用执行内核 + 短 Scene Profile +
  现有 Skills/Expert/Language Policy”，不引入 WorkBuddy 产品专属流程。

## 2. 当前静态提示词规模

下表“常用估算”按 UTF-8 字节约除以 4，用于接近英文 tokenizer 的量级比较；“仓库保守估算”
使用 `internal/agent/compact.go` 的跨语言上下文预算算法，不能解释为供应商实际计费。

| 组成 | 常用估算 | 仓库保守估算 |
| --- | ---: | ---: |
| `DefaultSystemPrompt` | 约 186 | 706 |
| 固定公共部分合计（含示例模型身份） | 约 841 | 3,302 |
| Office Scene | 约 36 | 143 |
| Coding Scene | 约 35 | 138 |
| Design Scene | 约 69 | 275 |
| 单场景固定静态总量 | 约 876～910 | 约 3,440～3,577 |

Memory、Skill Index、Expert、Turn Context、工具 schema 和历史消息可变，不计入上述静态数。

## 3. 真实模型基线方法

- 端点：`http://192.168.10.231:8080/v1`；模型：`gpt-5.6-luna`；协议：OpenAI-compatible `/v1/chat/completions`。
- 最小协议探测成功：模型精确返回 `PROBE_OK`，usage 为输入 39、输出 7、总计 46。
- 任务：Office、Coding、Design 各 3 个，共 9 个；每个重复 2 次，共 18 次。
- 每次使用生产 `boot.Build`、真实 Scene、生产工具注册、Controller 和 usage 事件；工作区隔离。
- Profile 固定为 low effort、temperature 0、max steps 18、auto plan off、context window 200000；
  单次上限 4 分钟，approval timeout 3 秒，Shell 优先 PowerShell，权限模式为 allow。
- Artifact grader 与 Controller run status 分开记录。
- 任务/prompt/seed manifest SHA-256：
  `1f1a3309b961b5fef30191652ff8c3868fb5d1eba444aafbb8a0ff769ce4528c`。
- 凭据使用一次性状态目录；运行结束目录已删除，结果 secret 扫描无匹配，Git worktree 干净。
- 去敏逐次结果、审计修正和运行参数见 [`baseline-results.md`](./baseline-results.md)；原始模型输出
  和绝对本地路径仅保留在被 Git 忽略的隔离目录。

## 4. 基线结果

| 指标 | 结果 |
| --- | ---: |
| 原始冻结 grader | 8/18（44.4%） |
| 证据审计后的产物语义正确率 | 18/18（100%） |
| 包含正常结束的端到端成功率 | **16/18（88.9%）** |
| Office | 6/6 |
| Coding | 6/6 |
| Design | 4/6；产物 6/6 |
| 总耗时 | 1,937.919 秒 |

两次 `design-dashboard-wireframe` 均生成满足 1440×900、可访问 title/desc、导航、侧栏、
四个 KPI 卡、趋势区和三行以上事故表格的 SVG，但 Controller 在约 238 秒返回
`context deadline exceeded`。按预定规则，正确产物不能覆盖运行级超时。

## 5. Grader 缺陷

- 数字格式：正确的 `49,000` 未匹配 `49000`。
- 回复语言：中文 `主题` 未匹配英文 `subject`。
- 否定语义：`避免 neon gradients` 被裸禁用子串判错。
- 等价表达：`14 天免费试用` 未匹配 `14-day`。
- 中英文标签：`对比度`、`订单摘要` 未匹配 `contrast`、`order summary`。
- SVG 结构：第二个 SVG 有四张独立指标卡，却因字面 `KPI` 次数不足被判错。
- manifest 只覆盖 task/prompt/seed，未覆盖 grader 实现；候选比较前必须修复。

原始 8/18 保留为机器报告事实，但不能作为当前模型能力分数；人工审计不覆盖两个超时失败。

## 6. 完整运行成本

| Scene | 运行 | 总 tokens | 平均 tokens/次 | 平均耗时 | 模型调用 | 工具调用 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Office | 6 | 1,663,169 | 277,195 | 74.3s | 94 | 104 |
| Coding | 6 | 1,244,802 | 207,467 | 64.8s | 74 | 92 |
| Design | 6 | 1,878,005 | 313,001 | 167.1s | 113 | 121 |
| **总计/平均** | **18** | **4,785,976** | **265,888** | **102.1s** | **281** | **317** |

Provider 报告 4,728,594 prompt tokens 与 57,382 completion tokens；平均每个任务 15.6 次
模型调用和 17.6 次工具调用。它们是所有回合累计 usage，不是 system prompt 的单次长度。

按当前平均模型调用数，静态 prompt 每增加 1,000 tokens，若回合不变，每任务约增加
15,600 prompt tokens；但若更强内核减少无效回合，长提示词仍可能降低完整运行成本。因此
候选选择同时看端到端成功、回合、总 usage、延迟和静态长度，不用“短”或“长”代理效果。

## 7. 调查结论

1. 当前提示词已经能在本小样本稳定生成正确 Office、Coding 和 Design 产物。
2. 可重复失败集中在 Design SVG 的完成后继续运行，首个内核改动应强化最小充分验证和停止条件。
3. 当前三个 Scene 使用不同任务，不能据此推断 Scene 因果效果；后续必须做相同输入的 profile on/off 配对。
4. WorkBuddy 值得参考的是模块化、按需能力与运行时纠偏方式，不是静态 prompt 体积或原文。
5. 本轮 Windows 环境提示 Bash sandbox 不可用；writer root 只约束受控文件工具，不能约束
   通用 PowerShell/Bash，因此本轮是能力基线而不是安全验证。后续候选保持临时工作区、一次性
   Profile 和不注入其他凭据的边界，不为提示词优化另建沙箱平台。

## 8. Extended V2 与真实 Desktop 结论

- Extended V2 内核门禁、构建身份、24 次 low-effort Desktop A/B、3 次 provider-auto smoke、
  六任务矩阵和 RF-001～007 见 [`desktop-ab-results.md`](./desktop-ab-results.md)。
- 缺陷归因后的任务语义/视觉质量：baseline 12/12，candidate 12/12；候选总 tokens
  6,122,252→5,057,932（-17.4%），模型调用 363→277（-23.7%），工具调用
  390→327（-16.2%），模型耗时 -9.3%。
- Office、Coding、Design 的 provider-auto 兼容 smoke 3/3 通过；所有 Design 产物均独立在
  1440×900 与 390×844 打开，并完成键盘焦点检查。
- 候选审批数增加，Desktop 正常结束数受 RF-003 污染且没有改善。最终接受候选的依据是同等
  任务质量下整体执行效率提高，不是声称 Controller 完成态或权限体验已由提示词修复。
