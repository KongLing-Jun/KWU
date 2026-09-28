# 任务 004：执行 30 次真实 Desktop 复验

## 状态与归属

- 状态：已完成；L01–L24 固定 `low` 与 A01–A06 默认/自动矩阵共 30/30 完成。
- 执行组：Serial-04，分校准、严格 12、low 24、auto 6 四个汇报点。
- 指派：本顶层任务 `/root` 亲自执行全部 Computer Use；原生 Desktop 操作禁止委派。
- 依赖：任务 003 已全绿；锁定产品 commit 为
  `6a93ac2c64287b0e103bb81ae3abc07b692caaba`。用户已在本任务尚无真实模型请求时完成 Provider URL、
  API Key、模型确认。
- 交付切片：30 个有效矩阵位置及 after 证据；不得添加真实模型样本凑数。

## 固定环境与验收手段

- 使用任务 003 锁定 commit 构建的真实 Wails Desktop，直接复用持久化 dev 环境，不用隔离 profile。
- 模型固定 `gpt-5.6-luna`；low 矩阵固定 low，auto 矩阵使用当时产品默认/自动 effort。
- 已确认 Provider URL 为 `http://192.168.10.231:8080/v1`、模型为 `gpt-5.6-luna`；本次确认覆盖整个
  固定矩阵，不再重复索取。API Key 不回显、不写入证据/命令/日志，也不从配置或日志反向读取；仅当
  用户主动变更参数时暂停确认。
- 每次用全新 workspace/project/session，但保留 dev profile 的真实跨会话状态并记录环境 ID。
- Computer Use 负责观察、激活、输入、审批和用户可见终态；内置浏览器/Playwright 可补充 DOM、时间线、
  截图和 WebView 验收，但不能代替原生 Desktop 结果。
- CLI、JSONL、sidecar、脚本只用于准备、只读取证、统计和离线评分。

## 固定矩阵与顺序

| Run | 场景 / task / repeat / effort |
| --- | --- |
| L01–L03 | 三场景校准：project-report、retry-policy、empty-state；repeat 1；low |
| L04–L06 | repeat 1 的其余三个任务：budget-analysis、nested-config、operations-dashboard；low |
| L07–L12 | 同六任务 repeat 2；low；与 L01–L06 合成严格可比 12 |
| L13–L18 | 同六任务 repeat 3；low |
| L19–L24 | 同六任务 repeat 4；low |
| A01/A04 | 日常办公 `desktop-office-budget-analysis` repeat 1/2；auto |
| A02/A05 | 代码交付 `desktop-coding-retry-policy` repeat 1/2；auto |
| A03/A06 | 设计 `desktop-design-empty-state` repeat 1/2；auto |

每组六任务内严格沿用修复前 `matrix.csv` 顺序。冻结 prompt、seed 规则、workspace fixture、grader 和
统计口径从任务 001 已验证的 run-spec 派生并 hash 对账，不在本任务重新设计。

## 每次必须记录

- 场景、task、repeat、run ID、commit、binary hash、模型、effort、session/workspace/project；
- 成品正确性、artifact、运行校验、视觉验收、额外文件和副作用；
- prompt/completion/cache/total tokens，父/child tokens，模型调用与耗时；
- 工具调用、重复工具、todo 步骤与 todo_write、complete_step 及拒绝原因、ledger/readiness 拒绝；
- ask/审批、wait、人为审批等待、用户可见 final 耗时；
- final 是否实时显示、是否切换会话才显示、重复回复；
- 正常完成、失败、中断、卡住或恢复后完成，并保存 observation/stats/grade/run-spec。

## 判定与暂停合同

- final 已落盘但 Desktop 仍 loading，切换后才显示：质量可通过，Desktop 实时完成态必须失败。
- 恢复后完成不改判为原地正常完成；替代会话只作旁证，不替换原 run。
- 确定性严重缺陷：先冻结当前窗口、截图、会话/meta/telemetry、日志、进程、workspace、commit、binary
  hash；当次按失败入矩阵；完成脱敏和完整性检查后暂停启动后续样本并汇报。
- Provider 波动单列 D 类；测试设施问题单列 E 类；两者也不得静默重跑覆盖。
- 校准三场景后、严格 12 后、low 24 后、全部 30 后分别汇报，未经检查不连续高成本运行。

## 验收标准

1. 恰好 30 个矩阵位置各有唯一权威 run，无矩阵外真实模型请求。
2. L01/L02/L03 作为三场景校准直接计入 30，统计链路可靠后才继续。
3. 全部记录字段、原生 final/idle 时间线和 artifact/视觉证据齐全，可从原始日志重算。
4. 运行期间 commit、binary、prompt、effort 和 Provider 账户/URL 未漂移。
5. 严重缺陷按证据优先合同暂停；不存在因切换会话或重跑而改判。

## 完成回执

- 状态与已完成矩阵位置：已完成，30/30；L01–L24 与 A01–A06 均交付正确成品、最终正常完成并在
  当前会话实时显示接受 final，切换会话 0、接受后重复回复 0。
- Provider 再确认（不含密钥）：用户在首个真实模型请求前确认
  http://192.168.10.231:8080/v1、gpt-5.6-luna；API Key 已在本任务确认但未写入证据。
- 阶段统计与原生验收：候选期冻结合同语义质量 30/30；真实 Desktop 最终正常完成 30/30、当前会话
  实时 final 30/30、切换会话 0、接受后重复回复 0。按权威 matrix/run-spec 状态，18 次直接、12 次
  恢复后完成。合计 6,800,607 tokens、370 次 telemetry 模型调用、481 次工具调用、86 次审批、
  5,547.273s 模型耗时；详见 `evidence/after/final-report.md`。当前 grader 原始通过 24/30；6 次口径
  差异均保留原始失败并由独立语义/运行/视觉验收解释，不篡改 grader 结果。
- 缺陷/暂停/偏差：没有复现 DEF-002/008/009/010；A03/A06 扩大了 FIND-L12-001 的后台 HTTP 服务
  生命周期证据，A06 一次 readiness 拒绝后由 wait/kill_shell 恢复；该问题是确定的高成本剩余风险，但未
  造成 Desktop 完成失败。FIND-L15-001 与 TI-001 至 TI-006 均已分维度判定，无严重产品缺陷要求暂停。
- 执行期证据：已生成并最终核验 30 个 run-spec/observation/stats/grade、Desktop 截图和设计样本
  Playwright 视觉证据；最终 PR 按用户要求只保留缺陷记录、阶段报告和最终报告 Markdown，关键指标与判定
  已完整转录。
