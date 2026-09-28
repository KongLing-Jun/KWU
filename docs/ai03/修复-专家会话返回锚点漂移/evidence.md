# 专家会话返回锚点漂移证据

## 权威输入

- 问题记录：
  `docs/ai03/修复-Desktop普通与专家团子代理隔离恢复/验收问题-专家会话返回双恢复与锚点漂移.md`
- clean 单 wheel：
  `C:\Users\ai03\AppData\Local\Temp\pr344-f160-single-wheel-20260810-182131`
- 早期诊断：
  `C:\Users\ai03\AppData\Local\Temp\pr344-02ec-anchor-diagnostic-20260810-164115`

临时目录不随仓库分发；本页只保留实施所需的最小结构化事实，原日志和截图仍是原始证据。

## clean 单 wheel 时序

离开前缓存：opaque anchor `ab14cf0b8`、question index 5、offset `-606.2000122070312`。

返回时 `team-legacy` 使用 row 33：

| frame | DOM | beforeTop | afterTop |
|---:|---|---:|---:|
| 1 | missing | 0 | 4580 |
| 2 | missing | 4580 | 4580 |
| 3 | missing | 4204 | 4204 |
| 4 | mounted | 4204 | 3640 |
| 5 | mounted | 5115.2 | 4551.2 |

随后 `team-persisted` 对同一 opaque anchor/offset 使用 row 39：

| frame | DOM | beforeTop | afterTop |
|---:|---|---:|---:|
| 1 | missing | 0 | 5880 |
| 2 | missing | 5880 | 5880 |
| 3 | missing | 5504 | 5504 |
| 4 | mounted | 5504 | 4940 |
| 5 | mounted | 6415.2 | 5851.2 |

persisted 第五帧在时间 `1786357581300` 完成并清除 pending；随后：

| 相对时间 | 事件 | identity | offset |
|---:|---|---|---:|
| +4ms | programmatic scroll + DOM capture/cache write | 同一 q5 | `-579.7999877929688` |
| +19ms | programmatic scroll + DOM capture/cache write | 同一 q5 | `-42.20000457763672` |
| +40ms | virtual range | 同一 persisted list | cache 已错误 |
| +178ms | virtual range | 同一 persisted list | cache 保持错误 |

结论只到这里：固定五帧且任一成功即完成不充分；pending 清除后晚到程序化 scroll 可重写 canonical
offset。证据没有唯一标注晚到 scroll 的物理发起者，也没有证明 StrictMode 或双 owner 是必要条件。

## 视觉结果

- `02-after-wheel-stable.png`：离开前显示回复清单第 7–9 项附近。
- `03-after-return-stable.png`：返回后显示下一轮用户问题和回复开头。
- 可访问树同时证明正文仍存在，不是数据丢失或错误会话。

## 反事实边界

- 若在 active restore 期间保持写回冻结，则上述两次 programmatic scroll 不能覆盖 q5/`-606.2`。
- 若完成条件要求 DOM identity、offset 和布局连续稳定，则 persisted 第五帧不能立即完成。
- 仅增加固定帧数无法证明跨设备稳定；默认回到底部会改变已确认的产品行为；完整 owner 重构缺少必要性证据。

## 精确回归与修复边界

- 未改生产代码时，精确恢复测试为 `10 passed, 3 failed`：晚到程序化 scroll 后视口停在 `957.8`，
  即相对目标 `-42.2`，而 canonical 目标应为 `-606.2`。
- 首轮修复把 pending anchor 作为 canonical target，以 DOM identity、`≤1px` offset 与只读 layout revision
  连续稳定 30 帧作为完成条件，并以 180 帧为硬上限；DOM 缺失时才 virtual seek。
- 首轮原生复核目录
  `C:\Users\ai03\AppData\Local\Temp\p1-anchor-fix-native-20260810-210500` 又证明一个同范围边界：overview
  在稳定后被 child presentation 替换，virtual handle unmount 后的程序化 clamp/measure 仍可写入 bottom。
- 最终修正只在 expert overview handle replacement 期间保留 canonical 标记并复用既有 bounded restore；错误
  semantic 与 bottom 事件都不能消费该标记，真实用户输入仍立即取消。没有引入新的 owner/generation 状态机。

## 最终原生 Desktop 验收

证据目录：
`C:\Users\ai03\AppData\Local\Temp\p1-anchor-fix-native-head2-20260810-214116`。
验收开始时工作树 clean，run-meta 记录生产实现精确 HEAD
`5fff54342692948ad2ff08112ec3d6eecebc38fe`；其后的最终提交只修正测试静态类型与回执，生产树未变化。

- 明确 bottom 后，seq `113319` 只有一次 wheel begin，`restart=0`、`expire=1`，实际 deltaY
  `-1083.333203125`。
- 单 wheel 后 seq `113322` 保存 opaque anchor `ab14cf0b8`、question index 5、offset
  `204.80029296875`；离开前 seq `113438` 仍为同一值，误差 `0.199707px`、`stableFrames=30`。
- seq `113439` 有 Transcript unmount；模型广场截图证明中间页面。返回后 legacy 与 persisted 均执行恢复，
  seq `113701` 最终仍为同一 identity/offset，误差 `0.199707px`、`stableFrames=30`，`beforeTop=afterTop=6636`。
- 验收窗口只有 seq `113322`、`113467` 两次 cache write，内容完全一致；返回完成后没有晚写、bottom
  fallback 或 incomplete。最终距 bottom `174px`，保持中部而非吸底。
- `01-after-single-wheel-stable.png` 与 `03-after-return-stable.png` 显示同一视觉段落；
  `02-model-marketplace.png` 对应 Transcript 卸载期间。

文件 SHA-256：

| 文件 | SHA-256 |
|---|---|
| `01-after-single-wheel-stable.png` | `D44AD84142CA2A1A9864AE9262FC9DD012650104C67059D39CA164C2DD3E8C3B` |
| `02-model-marketplace.png` | `7EFFDC2E24AF8CEEC8B866AB7B35C236AD04661523B70250CAAC743F159DC457` |
| `03-after-return-stable.png` | `F192796DDF131A379CEB1D83CAB3D403BD7161CC63C9D4AF9706233B0267563E` |
| `run-meta.txt` | `BFD81F2D62C39F7EA517053CA470301914C370EBF7AE9BA47C7407178A49C644` |
| `wails.stderr.log` | `B24BF144D121F1E90CCA6380D19DE49F2FF4C5908B997406388B78D4AE8B7F5C` |
| `wails.stdout.log` | `1A6C1276EA20162BEB9CA7D57CD3EAFB4C029E6FDD6C540EAE272E5AC7639C8A` |

证据偏差：最终交互后使用 screenshot-only 采集，避免 accessibility text capture 自身触发额外 wheel/presentation
副作用，因此临时目录没有独立可访问树文件；Transcript 卸载由原生 mount/unmount trace 证明，identity/offset
由结构化 trace 证明，视觉一致性由三张截图证明。结构化摘要和哈希保存在本文，原始日志不复制入仓库。

## 独立复审与比例化验证

- 未参与实现的代理增量审查最终测试修正并结合生产 diff 复审，P0/P1/P2 findings 均为零。
- restore `16/16`、anchor `32/32`、expert virtualization `133/133`、diagnostics `1/1` 通过。
- `pnpm typecheck`、`pnpm build`、`go run ./scripts/code_guard`、Code Guard baseline 文件三项检查、
  OpenSpec 1.8.0 strict validation、`git diff --check` 和受影响文件 LF EOL 均通过。
- `pnpm test:typecheck` 的本任务文件已无错误；命令仍因七个未改测试文件中的既有 fixture/contract 类型错误
  失败，涉及 capabilities、installer、model marketplace、project gateway、readiness candidate、runtime notice
  和 workbuddy navigation。生产配置和这些源文件均未被本修复修改，记录为已接受的基线 warning。

## 2026-08-10 可见跳动复验推翻旧完成结论

证据目录：
`C:\Users\ai03\AppData\Local\Temp\reasonix-visible-jump-20260810-231140`。

在包含 `viewport.visible-jump` DEV 埋点的 clean HEAD 上，原生执行一次 bottom→一次向上 wheel→稳定中部→
模型广场卸载 Transcript→返回同一专家会话：

- 离开前 view `vce71b186`、opaque anchor `ab14cf0b8`、question index 5、offset
  `496.60009765625`，证明 semantic identity 与初始位置保存正常。
- 返回时 `team-legacy` 产生 4 次可见变化、累计绝对位移约 `10236.8px`，随后因 persisted 到达而取消。
- 同一返回中的 `team-persisted` 产生 179 次可见变化、累计绝对位移约 `164721.6px`；180 帧耗尽后完成
  原因为 `incomplete`，不是 `stable`。
- persisted 首帧 virtual seek 把 `scrollTop` 从 0 写到 6248；offset correction 与后续测量把位置推到
  7296 左右。目标 row 39 随即离开 virtual range（约 first 43 到 last 46）并正常卸载；下一帧代码又因 DOM
  缺失执行 seek，再被 correction 推回。该循环在第二次返回中近似逐帧复现。
- 首个返回边界同时证明 App surface transition 期间 `transcriptFileTabId` 暂为 undefined；现有
  `expertTeamResolutionPending` 以 tabId 为前提，因此没有 gate legacy。tabId 后续就绪才查询并提交
  persisted。

结论：旧计划中“DOM identity 连续稳定”和“允许 legacy/persisted 各自恢复”的完成条件实质不足。正确边界是
pending 期间不暴露 legacy、最终 presentation 稳定前不可见，并在首次取得 row geometry 后允许 DOM 离开
overscan；DOM 缺失不得再次触发 virtual seek。旧原生验收和独立复审只证明最终 identity/offset，不能继续
作为“无可见跳动”的完成证据，因此任务 001/002 与 progress 已重新打开。

## 2026-08-11 clean HEAD 最终权威验收

本节取代此前被复验推翻的完成结论。两条路径均在 clean HEAD
`ce010d9b8ef9aae882b3e272750ac05bd24a288d` 上以 Computer Use 操作真实 Wails 应用，并以 DEV trace
作结构化 oracle。验收期间无 HMR，源码和文档未变化，验收结束后应用保持运行。

### 专家会话返回

- 证据目录：
  `C:\Users\ai03\AppData\Local\Temp\reasonix-anchor-final-20260811-025231\single-wheel-20260811-110128`。
- Wails PID `16608`、app PID `37552`，运行 SHA-256 前缀 `D4B103`；事件 baseline `117302`。
- `trace-events-seq117303-117691.jsonl` 在同一真实 return、instance 7、view `vce71b186` 中记录：
  seq `117472`/`117483` `expert-cache.lookup hit=false`，seq `117493` write `hasTeam=true`，随后 seq
  `117503` lookup `hit=true`、`loadedScopeMatch=true`、`hasTeam=true`。因此一次专家返回同时覆盖 cache
  miss、权威 sidecar 写入与实例内 cache hit，而非依赖两个独立返回。
- 唯一 wheel batch `166`：`begin=1`、`restart=0`、`expire=1`，delta `-1083.3332`。
- 离开前 opaque anchor `ab14cf0b8`、question index 5、offset `499.399902`。
- 返回 presentation 顺序只有 `team-pending → team-persisted`，无 `team-legacy`；virtual seek `1`，
  restore attempt `40`，最终 `stableFrames=30`、error `0.40039px`。
- `viewport.visible-jump=0`、summary `0`、incomplete `0`；稳定后没有 cache write、seek 或 retry。
- 返回前后截图 SHA-256 前缀分别为 `2699E7A8`、`C8CE6604`。

### 普通长会话返回

- 证据目录：
  `C:\Users\ai03\AppData\Local\Temp\reasonix-anchor-final-20260811-025231\normal-single-wheel-20260811031530`。
- 使用既有会话 `20260808-130641.387050600-gpt-gpt-5.6-luna`；path `normal`、item count `104`、
  `virtualized=true`、view `v80ccb533`，事件 baseline `118065`。
- 唯一 wheel batch `167`：`begin=1`、`restart=0`、`expire=1`；opaque anchor `a4e3d9d47`、
  question index 5、offset `55.2`。
- 返回仍为 normal path；virtual seek `1`，restore attempt `38`，最终 `stableFrames=30`、error `0`。
- `viewport.visible-jump=0`、summary `0`、incomplete `0`；稳定后继续观察 1.4 秒，目标 view 无新事件。

两条原生路径共同证明：用户看到的是稳定后一次揭示，而不是隐藏恢复过程；专家与普通长会话都保留同一
semantic identity/offset，且没有重复 seek、可见 correction 或 hard-limit incomplete。

### 最终独立复审与比例化 Gate

- 独立复审在上述精确 clean HEAD 上完成，未发现 P0/P1/P2。
- Expert virtualization `207/207`；受影响的 control/boot/serve Go tests、Code Guard 与 baseline 检查、
  `git diff --check`、LF EOL 均通过。
- Frontend restore/anchor/diagnostics、typecheck/build 和 OpenSpec strict 的比例化结果见任务 001/002 回执。
- 非 required 的完整 `go test ./...` 后续两次在 Windows 并行 boot 阶段超过 300 秒且无断言失败；这不覆盖
  required affected-package green。tasktree 缺失文档由 origin/main-v3 只读证据确认为既有基线。

## 2026-08-11 deep-offset 原生复验与精确回归

证据目录：
`C:\Users\ai03\AppData\Local\Temp\reasonix-anchor-final-20260811-025231\expert-retest-single-wheel-2026-08-11T03-50-41-702Z`。
该复验在 clean docs HEAD `b039acee`、生产运行 state `ce010d9` 上执行；Wails PID `16608`、app PID
`37552`，运行 SHA-256 为 `D4B103F6A42F452212521AD5001249BD9DB08435397A14792E7E213BDB1309E4`。

- baseline `120225`；唯一 wheel batch `209` 为 begin `1`、restart `0`、expire `1`，delta
  `-1083.333203125`。canonical 为 view `vce71b186`、opaque anchor `ab14cf0b8`、question index 5、
  offset `732.2001953125px`。
- 返回顺序为 pending→persisted，无 legacy；每 generation 只有一次 virtual seek，可见 jump/summary 都为 0。
- target row 曾挂载并测量；随后变量 row 高度收敛令总高度缩短，scrollTop 被浏览器夹在物理 max
  `6281.60009765625px`。目标 DOM 因 range 移至 bottom rows 后离开 overscan，但 row-key virtual geometry
  仍可信，给出的最近可达 offset 为 `611.60009765625px`。
- 差值 `120.60009765625px` 是继续向下滚动才能满足的 signed correction，而当前已在 bottom；不是 row-key
  复用、estimate、offset 定义或第二 seek 问题。旧实现只认绝对误差 `≤1px`，故 180 次 attempt 的
  stableFrames 始终为 0、layout revision 最终固定 68、`incomplete=1`，返回和额外 1.4 秒截图均为空白 failure。
- 未改生产代码的精确 OLD RED 为 expert `211 passed / 2 failed`：根因/唯一 seek/测量/clamp 断言均通过，
  仅“closest boundary 后应揭示”两条失败。批准后的最小实现保留 canonical 与 hidden restore，仅在同 identity
  的 measured virtual geometry、signed correction 指向已抵达 top/bottom、layout 连续稳定时累计 30 帧。
  顶部对称通过；近边界仍可达、未测量与 layout churn 均不得完成。GREEN 为 expert `213/213`、restore
  `21/21`。

该结果推翻本文件上一节的“最终”状态，但不否定其 cache miss→write→hit、无 legacy、普通长会话与零可见
跳动证据。新的 clean HEAD 原生验收与独立复审仍由任务 002 执行。

## 2026-08-11 不可达锚点最终权威验收

两条场景均来自 clean HEAD `0be686022134ebab1d89fe461e4f398b2c06b6a2` 的同一新 Wails 实例：Wails
PID `43656`、app PID `45004`，exe SHA-256
`2CBF289393487DCD9C37D5DC9041EBF64E07795460D6C8B1D8287E59973F97F1`，HMR `0`。

### Deep offset closest-boundary PASS

- 目录：`C:\Users\ai03\AppData\Local\Temp\reasonix-anchor-deep-offset-20260811-121307\deep-offset-authoritative-2026-08-11T04-21-25-307Z`；baseline `121309`。
- 唯一 wheel batch 2：begin `1`、restart `0`、expire `1`、delta `-1083.333203125`；canonical
  `ab14cf0b8`/question 5/offset `790.80029296875px`。
- 返回为 `team-pending → team-persisted`，legacy 0；seek 1、visible jump/summary/incomplete 均为 0。
- 恢复 39 帧，以可信 measured virtual geometry 连续稳定 30 帧：scrollTop `6281.60009765625px`、max
  `6281px`，measured offset `611.60009765625px`，signed correction `+179.2001953125px`，DOM 已正常离开
  overscan。canonical 没有错误改写，bottom 写入 0；稳定后 cache/seek/retry 事件 0。
- UI 正文可见、final marker 恰 1、failure 0；返回截图与额外 1.4 秒截图 hash 相同。

### Normal exact-offset PASS

- 目录：`C:\Users\ai03\AppData\Local\Temp\reasonix-anchor-deep-offset-20260811-121307\normal-exact-offset-2026-08-11T04-28-08-868Z`；既有 session
  `20260808-130641.387050600-gpt-gpt-5.6-luna`，view `v80ccb533`，baseline `121883`，normal path、
  item count 104、virtualized。
- 唯一 wheel batch 3；canonical `a4e3d9d47`/question 5/offset `-88.80000305175781px`。返回 seek 1、
  visible jump/summary/incomplete 均为 0；39 帧、连续稳定 30 帧，final error 0、DOM success。
- scrollTop `12223.2001953125px`、max `12663px`，明确不在边界，证明正常可达位置仍走 exact 分支；
  canonical 保持一致，late wrong writes 0，正文与 probe 正常可见且 failure 0。

同一进程较早的一轮记录为 `NOT_COVERED`：wheel 落在临时 view 且没有 semantic canonical，未用于产品
PASS/FAIL 判定。最终独立复审基于 `0be68602`，无 P0/P1/P2；独立 restore `21/21`、expert `213/213`，
clean/diff/LF 通过。实现侧 typecheck/build、anchor `32/32`、trace diagnostics `13/13` 与 OpenSpec strict 通过。
