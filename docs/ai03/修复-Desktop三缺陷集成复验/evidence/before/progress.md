# Desktop Agent 稳定性与运行成本对比进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 阶段 | 30 次有效矩阵运行、最终审计与对比汇总全部完成；见 `final-report.md` |
| 测试源码 | `origin/main-v3@6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad` |
| Git tree | `378e8490d5abf766f7c2d21fb907caa400abaadb` |
| worktree | `D:\Reasonix-worktrees\desktop-luna-stability-20260812-6cfe5af6`（detached、产品源码只读） |
| 证据目录 | 本目录；运行 workspace、日志、截图、观察、离线统计和缺陷分开保存 |
| Computer Use | 顶层任务亲测初始化、窗口枚举、唯一选择、激活、截图/无障碍观察和新建缓冲区文本输入均正常 |
| 真实 Desktop 样本 | 30/30；模型交付质量 28/30，正常结束 29/30，当前会话实时 final 29/30，运行失败 1/30；累计 383 次模型调用；另有 1 次不计样本的 Desktop `/models` 发现请求和 1 次因场景选择错误而排除的真实 Desktop 预运行 |
| 产品修改 | 0；本任务不修改产品代码、系统提示词、Scene Profile 或运行时实现 |
| production binary | `desktop/build/bin/kwu-desktop.exe`，SHA-256 `FC1FA59E7927B6C95723021F57F2B214E5AF4D093113DB0B40D538FD618F0A86` |
| 当前输入 | L01–L24 均已通过 fixture hash 复核和真实 Desktop 操作；6 个 Design 样本另经 Playwright 视觉验收；L18 因响应式溢出判质量失败、L24 未复现；L21 在模型请求前因 `.jsonl.meta` 文件锁失败，无 artifact/final |
| Desktop 环境 | 持久化 dev 环境 `a6defe83d02e4e84`；启动前已有 190 个历史任务、8 个 workspace，均保留且不计入样本 |
| Provider | 用户本轮确认根地址；Reasonix OpenAI-compatible 有效 base 为 `http://192.168.10.231:8080/v1`，密钥已由本轮值覆盖且不落证据 |
| 模型与运行参数 | 默认模型 `gpt-5.6-luna`；子代理 low；执行/规划轮数不限；默认 ask；auto plan off |

## 校准结果

| run | 场景 | 质量 | 正常结束 | 实时 final | tokens | 模型/工具调用 | 审批 | 模型耗时 | 用户可见 final |
| --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| L01 | 日常办公 | artifact 与端到端 grader 均通过 | 是 | 是，无切换/无重复 | 132,269 | 8 / 13 | 3 | 121.762s | 131.375s |
| L02 | 代码交付 | artifact 与端到端 grader 均通过 | 恢复后完成 | 是，无切换/无精确重复 | 273,029 | 16 / 16 | 6 | 202.233s | 203.711s |
| L03 | 设计 | artifact、端到端与视觉验收均通过 | 恢复后完成 | 是，无切换/无重复 | 285,451 | 15 / 16 | 7 | 463.281s | 488.821s |

L01 的最终 JSONL 在 `2026-08-12T15:18:23.4346556Z` 落盘，当前页首次观察到 final 为
`2026-08-12T15:18:32.854Z`，投影差约 9.420s；首次落盘后观察即已显示 final，未见持续 loading 或 stop 状态。
3 次审批的首见到点击总等待为 16.649s。`complete_step` 3/3 接受，无 ledger/readiness 拒绝、无重复工具调用、
无 ask/wait、无 child session。

L02 的最终 JSONL 在 `2026-08-12T15:27:38.3312467Z` 落盘，当前页首次观察到接受后的 final 为
`2026-08-12T15:27:39.628Z`，投影差约 1.297s。过程有 1 次 `complete_step` 证据拒绝、1 次
`todo_write` receipt 拒绝、1 次 final-readiness 拒绝；runtime 自动续跑后正常结束。`pytest` 未安装导致组合命令
一次非零退出，随后现有 `unittest discover` 5/5 通过。JSONL 有 2 条非相同 parent final（前一条被 readiness
拒绝、后一条为收口确认），不判为精确重复，但计入完成流与 final 质量开销。6 次审批首见到点击总等待 76.562s。

L03 的最终 JSONL 在 `2026-08-12T15:41:35.2197047Z` 落盘，当前页首次有时间戳的 final 观察为
`2026-08-12T15:42:00.575Z`，投影差约 25.355s；无需切换会话，stop/loading 已消失，之后稳定且无重复。
过程有 1 次 `edit_file` 匹配失败、1 次 120s bash 超时、1 次 GBK/UTF-8 解码失败和 1 次
`complete_step` receipt 拒绝，均在原会话恢复。7 次审批首见到点击总等待 7.828s。离线 grader 12/12 检查通过；
内部浏览器与 Playwright 验证 1440×900、320×800 无横向溢出、纵向操作按钮、2.4px/4px 焦点轮廓、深色模式、
reduced-motion 0.01ms。唯一控制台噪声为非任务要求的 `/favicon.ico` 404。

三次校准合计 690,749 tokens（prompt 680,085；completion 10,664）、39 次模型调用、45 次工具调用、
16 次审批、787.276s 模型耗时；质量 3/3，通过后均在当前 Desktop 会话实时显示 final。L01 无恢复完成，L02/L03
恢复后完成。相对旧 12 次发布口径的每样本均值，校准每样本 tokens 暂降 45.4%、模型调用暂降 43.7%、
工具调用暂降 45.0%，但模型耗时暂升 37.8%，主要由 L03 的 463.281s 拉高；样本仅 3 次，不能替代 30 次结论。

## repeat 2 前半检查点（L07–L09）

| run | 场景 | 质量 | 正常结束 | 实时 final | tokens | 模型/工具调用 | 审批 | 模型耗时 | 用户可见 final |
| --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| L07 | 日常办公 | artifact 与端到端 grader 均通过 | 恢复后完成 | 是，无切换/无重复 | 149,119 | 9 / 13 | 3 | 108.500s | 118.137s |
| L08 | 代码交付 | artifact、隐藏语义、本地套件与端到端 grader 均通过 | 恢复后完成 | 是，无切换/无重复 | 222,753 | 13 / 21 | 7 | 222.779s | 229.837s |
| L09 | 设计 | artifact、端到端与 Playwright 视觉验收均通过 | 是 | 是，无切换/无重复 | 141,752 | 8 / 14 | 6 | 231.699s | 239.297s |

L07–L09 合计 513,624 tokens（prompt 505,227；completion 8,397）、30 次模型调用、48 次工具调用、
16 次审批、562.978s 模型耗时、150.547s 首见到点击审批等待。质量 3/3，当前 Desktop 会话实时 final 3/3；
L07 因一次 `complete_step` 证据命令不匹配而恢复，L08 因 fixture 无 pytest 和两次 `complete_step` 拒绝而恢复，
L09 三步直接完成。L09 有一次非连续重复 `ls` 签名，不影响成品；无 ask、wait、ledger/readiness 拒绝或 child session。

相对旧 12 次发布口径的每样本均值，L07–L09 每样本 tokens 暂降 59.4%、模型调用暂降 56.7%、工具调用暂降
41.3%、审批暂降 4.5%、模型耗时暂降 1.5%。这是三场景 repeat 2 的半阶段检查点，不替代 L01–L12 严格对比。

截至 L09，9 次累计 1,894,969 tokens（prompt 1,863,731；completion 31,238）、108 次模型调用、142 次工具调用、
46 次审批、1,896.518s 模型耗时；质量 9/9，当前会话实时 final 9/9，无切换恢复或重复 final。产品源码和提示词
保持锁定 tree，证据目录密钥扫描为 0。

## 前 12 次严格可比阶段

L01–L12 已完成。质量 12/12；接受后的最终完成 12/12，其中直接完成 4 次、自动恢复后完成 8 次；
当前 Desktop 会话实时 final 12/12，0 次需要切换会话，0 次重复 final。累计 prompt 2,568,310、completion 43,629、
总 tokens 2,611,939，模型调用 148、工具调用 196、审批 59、模型耗时 2,471.687s。

相对旧发布 12 次：总 tokens -48.36%、模型调用 -46.57%、工具调用 -40.06%、审批 -11.94%，但模型耗时
+8.16%。按场景 tokens：日常办公 -50.06%、代码交付 -31.14%、设计 -59.25%。完整三维对比、审计后基线、
完成步骤开销和两次重复方差见 `strict-comparable-12-report.md`。

## low repeat 3/4 结果

L13（日常办公 / 项目报告 repeat 3）质量、冻结 grader、输入哈希和当前会话实时 final 均通过；12 次模型调用、
17 次工具调用、201,007 tokens、221.012s 模型耗时、5 次审批。3 次 `complete_step` 全部直接接受，无
ledger/readiness 拒绝、ask、wait、child session 或重复 final。模型额外执行了一次对三个源文件分别自比较的
无效完整性命令，并产生额外审批；外部 SHA-256 复核确认源文件实际未变。第 5 次审批的 22.277s 等待包含
操作员处理截图接口差异的时间，按实保留在人为审批等待中，不计入模型耗时。

L14（代码交付 / retry policy repeat 3）冻结 grader、隐藏语义、聚焦与完整 unittest 均通过；11 次模型调用、
12 次工具调用、181,700 tokens、99.221s 模型耗时、4 次审批。3 次 `complete_step` 全部直接接受，无
ledger/readiness 拒绝、重复工具调用、ask、wait、child session 或重复 final；当前会话无需切换即稳定显示 final。
README 哈希与冻结 fixture 相同。测试产生两个正常的 `__pycache__/*.pyc` 副作用，已按额外文件记录。

L15（设计 / 空状态 repeat 3）冻结 grader、精确文案、语义、响应式、焦点、深色、reduced-motion、无渐变、
源文件保留和本地 HTTP 200 均通过；13 次模型调用、14 次工具调用、340,636 tokens、180.138s 模型耗时、
5 次审批。Windows GBK 默认读取 UTF-8 的问题再次复现（DEF-003），另有一次 `complete_step` 精确 receipt
不匹配；均在原会话自动恢复。Playwright CLI 成功生成 1440×900 浅/深色、320×800 移动图和真实 Tab 焦点图；
补充交互脚本在焦点图后两次 `page.evaluate` 超时，记为 E 类 DEF-006，不影响 Wails 完成态或 artifact 判定。

L16（日常办公 / 预算分析 repeat 3）冻结 grader、两份 Markdown、算术和输入哈希均通过；16 次模型调用、
17 次工具调用、283,074 tokens、292.276s 模型耗时、8 次审批。`openpyxl` 不可用后改用 XLSX ZIP/XML，
两次 `complete_step` 因收据时序或命令不精确被拒绝，Windows 默认 GBK 读取 UTF-8 的问题第三次复现
（DEF-003）；均在原会话恢复。final 落盘后约 10.263s 在当前 Wails 会话可见，无切换、无重复。

L17（代码交付 / nested config repeat 3）冻结 grader 的隐藏语义、6 项本地测试、最少新增测试数和两份保护文件
哈希均通过；17 次模型调用、25 次工具调用、304,112 tokens、152.709s 模型耗时、6 次审批。两次
`complete_step` 因收据命令缩写被拒绝后自动恢复；final 落盘后约 7.342s 在当前 Wails 会话可见，无切换、
无重复。测试产生三个正常的 `__pycache__/*.pyc`。一次测试后证据复制误落到 artifact workspace，已完整移出并
保留在 `output/operator-accidental-L17-copy`，明确归为 E 类测试基础设施处理，不计模型副作用。
L18（设计 / operations dashboard repeat 3）14 项结构 grader 全部通过，SVG 可加载，Tab 焦点、主操作、
reduced-motion 和控制台检查通过；但 Playwright 在 320px 测得页面 `scrollWidth=954`（`innerWidth=320`），
内容网格仍为 936px，且 1440px 下固定宽度 SVG image 与右侧事件面板重叠。最终回复仍声称支持窄屏，故模型
交付质量和 end-to-end 判失败，记录 `DEF-007`；Wails 运行则正常结束，当前会话实时 final 通过，无切换、
无重复。运行包含 1 个 child：父 259,676 tokens / 14 次调用，child 26,342 tokens / 6 次调用；合计
286,018 tokens、20 次模型调用、26 次工具调用、4 次审批、211.641s 模型耗时。一次 child `complete_step`
收据拒绝和一次父校验条件过严均自动恢复。

L19（日常办公 / project report repeat 4）冻结 grader 的报告、3 条明确行动项和三份源文件哈希全部通过；
9 次模型调用、13 次工具调用、148,538 tokens、86.495s 模型耗时、2 次写入审批。3 次 `complete_step`
全部直接接受，无 ledger/readiness 拒绝、重复工具调用、ask、wait、child session 或重复 final；当前会话无需
切换即稳定显示 final。

L20（代码交付 / retry policy repeat 4）冻结 grader 的隐藏语义、5 项本地测试、最少新增测试数和 README
哈希全部通过；10 次模型调用、13 次工具调用、163,062 tokens、101.751s 模型耗时、3 次审批。3 次
`complete_step` 全部直接接受，无 ledger/readiness 拒绝、重复工具调用、ask、wait、child session 或重复 final；
当前会话无需切换即稳定显示 final。测试产生两个正常的 `__pycache__/*.pyc`。

L21（设计 / empty state repeat 4）在第一轮 draft scene commit、任何模型请求之前失败。Desktop 在发送后
18.942s 显示 `.jsonl.meta` 被其他进程占用的 Windows 文件锁错误，但外层误报为模型流重试后断开；随后回到
空闲 composer，没有 HTML/CSS、assistant final、审批或恢复。JSONL 仅含 system record，telemetry 和 extractor
均为 0 tokens / 0 模型调用 / 0 工具调用 / 0 模型耗时；输入哈希保持不变。失败后只读检查仅发现一个
`kwu-desktop.exe` 进程，不能据此确定实际锁持有者。该样本不重试，质量、正常结束、实时 final 均判失败，
记为 B 类 `DEF-008`。

截至 L21，累计 prompt 4,449,968、completion 70,118、总 tokens 4,520,086，模型调用 256、工具调用 333、
审批 96、模型耗时 3,816.930s；模型交付质量 19/21，当前会话实时 final 20/21，运行时失败 1/21。

L22（日常办公 / budget analysis repeat 4）冻结 grader、两份 Markdown、算术、原因区分、CNY 405,000
上限和两个输入哈希全部通过；13 次模型调用、18 次工具调用、228,732 tokens、254.150s 模型耗时、6 次审批。
一次 `complete_step` 因先于其引用的算术命令被拒绝，模型随后运行命令并接受；第一版最终内容断言错误地要求
两份格式不同的文档同时包含所有货币字符串，模型识别过严后改为逐文档校验并通过。最终回复落盘后约 12.771s
在当前 Wails 会话可见，无切换、无重复；记为恢复后完成，不另立产品缺陷。

截至 L22，累计 prompt 4,674,616、completion 74,202、总 tokens 4,748,818，模型调用 269、工具调用 351、
审批 102、模型耗时 4,071.080s；模型交付质量 20/22，当前会话实时 final 21/22，运行时失败 1/22。

L23（代码交付 / nested config repeat 4）冻结 grader 的隐藏语义、6 项本地测试、最少新增测试数以及
README/settings 哈希全部通过；15 次模型调用、23 次工具调用、260,675 tokens、169.948s 模型耗时、5 次审批。
一次 `complete_step` 将说明文字误作 verification command 而被拒绝，随后用匹配的读文件收据通过；模型还运行
了两个近似的 status/diff 命令并调用一次 wait。Python LSP 因缺少 `pyright-langserver` 无法执行，但完整
unittest 6/6 通过，final 如实披露。final 落盘后约 7.342s 在当前 Wails 会话可见，无切换、无重复。

截至 L23，累计 prompt 4,932,660、completion 76,833、总 tokens 5,009,493，模型调用 284、工具调用 374、
审批 107、模型耗时 4,241.028s；模型交付质量 21/23，当前会话实时 final 22/23，运行时失败 1/23。

L24（设计 / operations dashboard repeat 4）冻结结构 grader 14/14、端到端与 Playwright 视觉验收均通过；
10 次模型调用、15 次工具调用、202,140 tokens、222.028s 模型耗时、5 次审批。一次 `complete_step` 因
Python heredoc receipt 未精确匹配被拒绝，模型以准确收据恢复；无 ledger/readiness 拒绝、ask、wait、child 或
重复工具签名。1440×900 无面板重叠，320×800 的页面 `scrollWidth=320`，SVG、焦点、reduced-motion 与
控制台检查通过。final 在当前 Wails 会话稳定显示，无切换、无重复；L18 的 DEF-007 未复现。

固定 low 24 次累计 prompt 5,128,605、completion 83,028、总 tokens 5,211,633，模型调用 294、工具调用 389、
审批 112、模型耗时 4,463.056s；模型交付质量 22/24，正常结束 23/24，当前会话实时 final 23/24。完整按场景、
四次重复方差、步骤和缺陷收口见 `low-24-report.md`。

## 默认/自动推理强度进度

A01（日常办公 / budget analysis auto repeat 1）冻结 grader、两份 Markdown、算术与输入哈希均通过；默认/自动
模式在 Desktop 显示为“自动”，session meta 的产品表示为空 effort。17 次模型调用、19 次工具调用、303,702
tokens、261.310s 模型耗时、7 次审批。openpyxl 缺失后切换 XLSX ZIP/XML；一次 `complete_step` 收据时序
拒绝；Windows 默认 GBK 读取 UTF-8 再次复现 DEF-003，指定 UTF-8 后通过。final 落盘后约 1.518s 在当前
Wails 会话可见，无切换、无重复。

A02（代码交付 / retry policy auto repeat 1）在发送前于 Wails 首页明确选中“代码开发”，父 session meta 记录
`scene=coding`，默认/自动 effort 仍为空值；代码技能卡可见 `effort low`，并委派 1 个 child。冻结隐藏语义、
本地 5 项 unittest、编译检查、新增测试数和 README 哈希均通过。19 次模型调用（父 15、child 4）、29 次
工具调用、264,046 tokens（父 255,533、child 8,513）、238.478s 模型耗时、8 次审批。一次
`complete_step` 因当轮文件收据不匹配被拒绝后自动恢复；3 个非连续重复工具签名，无 ask/wait/readiness 拒绝。
final 落盘后约 1.354s 在当前 Wails 会话可见，无切换、无重复。

A02 之前有 1 次不计入矩阵的真实 Desktop 预运行：发送时仍在日常办公场景，session meta 为 `office`。虽成品
和 final 均通过，但系统提示词场景错误，不能作为代码交付样本。完整保留在
`output/test-infrastructure/A02-office-scene-invalid/`，归为 E 类测试操作错误，不算产品缺陷；有效 A02 使用全新
fixture workspace `A02R` 重跑。此后每次发送前强制截图核对场景、模型和 effort。

截至有效 A02，26 次累计 prompt 5,687,780、completion 91,601、总 tokens 5,779,381，模型调用 330、工具调用
437、审批 127、模型耗时 4,962.844s；模型交付质量 24/26，正常结束 25/26，当前会话实时 final 25/26。

A03（设计 / empty state auto repeat 1）冻结 grader 12/12 与端到端通过；Playwright 实测 1440×900 浅/深色、
320×800 移动、两次真实 Tab 后主操作焦点、reduced-motion、精确文案、语义与零渐变均通过。16 次模型调用、
21 次工具调用、303,578 tokens、284.102s 模型耗时、7 次审批。模型在自检中发现初稿装饰线误用了渐变并主动
改为纯色；一次 HTTP verification 的 `complete_step` 收据文本不精确被拒绝后恢复。final 落盘后约 11.370s
在当前 Wails 会话可见，无切换、无重复。独立 Playwright 长脚本再次卡住并被 30s 边界终止，归入 E 类
DEF-006；拆分后的 CLI 截图、焦点脚本和指标脚本全部成功，不影响 artifact 判定。

默认/自动前三场景检查点 A01–A03 合计 prompt 856,330、completion 14,996、总 tokens 871,326，模型调用 52、
工具调用 69、审批 22、模型耗时 783.890s；质量、正常结束、当前会话实时 final 均为 3/3，三次都含可恢复事件。
截至 A03 的 27 次累计 prompt 5,984,935、completion 98,024、总 tokens 6,082,959，模型调用 346、工具调用
458、审批 134、模型耗时 5,246.946s；模型交付质量 25/27，正常结束 26/27，当前会话实时 final 26/27。

A04（日常办公 / budget analysis auto repeat 2）冻结 grader、两份 Markdown、预算算术、解释状态、CNY
405,000 上限和两个输入哈希均通过；13 次模型调用、16 次工具调用、223,044 tokens、144.585s 模型耗时、
4 次审批。4 次 `complete_step` 全部直接接受，无重复工具签名、ledger/readiness 拒绝、ask、wait、child 或
恢复事件。final 落盘后约 6.691s 在当前 Wails 会话可见并稳定，无切换、无重复。

截至 A04 的 28 次累计 prompt 6,204,244、completion 101,759、总 tokens 6,306,003，模型调用 359、工具调用
474、审批 138、模型耗时 5,391.531s；模型交付质量 26/28，正常结束 27/28，当前会话实时 final 27/28。

A05（代码交付 / retry policy auto repeat 2）冻结 grader 的隐藏语义、本地 5 项测试、聚焦测试数量和 README
哈希均通过；14 次模型调用、18 次工具调用、237,213 tokens、200.918s 模型耗时、6 次审批。一次
`complete_step` 因当轮文件收据不匹配被拒绝后自动恢复；调用 1 次 wait，无重复工具签名、readiness 拒绝或 ask。
本次 `run_skill` 没有产生持久化 child session，与 A02 的一个 child 存在运行形态方差，tokens 全计在父会话。
final 落盘后约 22.160s 在当前 Wails 会话可见并稳定，无切换、无重复。

截至 A05 的 29 次累计 prompt 6,439,090、completion 104,126、总 tokens 6,543,216，模型调用 373、工具调用
492、审批 144、模型耗时 5,592.449s；模型交付质量 27/29，正常结束 28/29，当前会话实时 final 28/29。

A06（设计 / empty state auto repeat 2）冻结 grader 12/12 与端到端通过；Playwright 实测 1440×900 浅/深色、
320×800 移动、真实 Tab 主操作焦点、reduced-motion、精确文案、语义、零渐变和零控制台错误均通过。10 次
模型调用、13 次工具调用、188,279 tokens、213.734s 模型耗时、4 次审批。3 次 `complete_step` 全部直接接受，
无重复工具签名、ledger/readiness 拒绝、ask、wait 或 child。final 落盘后约 7.821s 在当前会话可见，但 stop/loading
状态又持续约 26.672s 后才清除，记为 C 类 DEF-009；无需切换且 final 已实时出现，故按锁定规则仍判实时通过。

默认/自动 6 次合计 prompt 1,493,516、completion 26,346、总 tokens 1,519,862，模型调用 89、工具调用 116、
审批 36、模型耗时 1,343.127s；质量、正常结束、当前会话实时 final 均为 6/6，其中直接完成 2 次、恢复后完成
4 次。30 次总计 prompt 6,622,121、completion 109,374、总 tokens 6,731,495，模型调用 383、工具调用 505、
审批 148、模型耗时 5,806.183s；模型交付质量 28/30，正常结束 29/30，当前会话实时 final 29/30。

## 已锁定方法

- 三个业务场景只使用日常办公、代码交付、设计；普通聊天、专家团、项目不作为场景分类。
- 30 次均由真实 Wails Desktop 发起和验收；Computer Use 由本顶层任务亲自操作。
- 固定 low 矩阵 24 次：六个任务各 4 次。前三个 run 是三场景校准，同时计入 repeat 1。
- 前 12 次（repeat 1/2）采用候选评测时期的精确用户任务文本，确保与旧 12 次候选样本严格可比。
- 当前主线与旧候选的系统提示词候选文件及三个 Scene Profile 文件内容哈希一致。
- 旧候选之后，冻结任务中只有 `desktop-design-operations-dashboard` 的用户任务文本被补充了
  “HTML must reference dashboard-visual.svg”；严格可比矩阵仍使用旧文本，当前 grader 另行记录。
- repeat 3/4 继续使用同一旧文本，避免把任务文本变化混入方差；auto 矩阵选择旧 smoke 同款且文本未变的
  Office 预算、Coding 重试、Design 空状态，各 2 次。
- 质量、Agent 执行效率、Desktop 实时完成稳定性分开报告；不合并成一个总分。
- final 已落盘但当前页仍 loading，只有切换后才显示时：质量可通过，实时完成态必须失败。
- API Key 不写入本目录、Git、报告、日志或命令行；只允许在用户本轮确认后通过 Desktop UI 写入持久化 dev credentials。

## dev 环境切换记录

- 一次性隔离 Profile 只完成启动与设置入口预检，未输入凭据、未发起请求；用户要求改用 dev 后已终止，
  原目录保留为弃用证据且不计入矩阵。
- dev 环境由未设置 `KWU_DEV_PROFILE_ROOT` 的同一测试二进制启动，历史任务和 workspace 正常可见。
- 本轮提供的 API Key 已覆盖保存；没有读取、导出或复用旧 credentials、配置、日志或历史消息中的密钥。
- 连接发现成功，`gpt-5.6-luna` 已选中并保持默认；Provider 卡片显示已设密钥与有效 `/v1` base。
- 旧候选 Design child meta 明确记录 `effort: low`；因此将 dev 子代理从当时可见的非 low 状态固定为 Low。
- Computer Use 一次因前台切换触发 `user input detected` 保护；重新观察并激活同一 Reasonix 窗口后继续，
  没有把动作发送到 Codex 或其他应用。

## 阶段门禁

1. 完成无网络模型请求的 benchmark/统计器自检和 Wails 构建身份记录。
2. 用户重新确认 Provider URL、API Key、`gpt-5.6-luna`。
3. 三场景各 1 次校准；核对 JSONL、telemetry、grader、Computer Use 完成态和截图链路。
4. 向用户汇报校准结果后，才继续其余 low 矩阵。
5. low repeat 1/2、low repeat 3/4、auto 矩阵分别汇报。
6. 最终离线汇总并逐项检查 30 个记录，无缺项后才关闭目标。

## 已发现的可比性风险

- 已确认 `DEF-001`（E 类）：旧 Office 项目报告 repeat 2 目录含两个父会话，历史发布 tokens 与
  calls/tools/duration/observation 混用了不同父会话。保留发布值，同时用 observation 对应父会话重算：
  旧总 tokens 为 5,139,314，而不是发布的 5,057,932；差异 81,382。详见 `baseline-audit.md`。
- 当前 Desktop grader 已修复旧 RF-004、RF-005、RF-007 的部分冻结契约缺陷，raw grader 不能直接与旧 raw
  grader 比较；主比较沿用旧报告的“缺陷归因后任务质量”，同时保留当前 raw grader 结果。
- 旧候选 observation 没有记录 todo、complete_step 拒绝、readiness 拒绝、wait、final 实时显示等全部字段；
  新矩阵会完整记录，旧侧缺失项标为 `not_instrumented`，不得当作 0。
- Provider 和模型服务可能波动；每次保留模型耗时、cache usage、调用数、结束状态和 D 类归因证据。
- `DEF-003`：L03 本地校验在后台服务退出和 Windows 默认编码上发生两次恢复，最终通过；继续观察是否复现。
- `DEF-004`：Computer Use 对 Windows Edge 的 URL-confidence 检查阻止原生 Edge artifact 观察；真实 Wails 验收不受影响，
  artifact 视觉证据改由内置 Browser/Playwright 补充，分类为 E。
- `DEF-005`：L03 页面产生一个不影响功能的 favicon 404，分类为 D，任务未要求 favicon。
