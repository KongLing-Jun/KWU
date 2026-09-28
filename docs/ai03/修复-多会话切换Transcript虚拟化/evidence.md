# Composer 专家闪烁时间线证据

## 当前结论

版本化原子交接完成后，2026-08-04 再用同一 dev profile、同一对会话连续切换
5 次。每条 trace 的全部 `app.commit` 和 `app.after-paint` 只有目标值，修复前的
`目标 → 错误中间值 → 目标` 序列不再出现。完整修复后结果见本文末尾。

## 修复前结论

2026-08-04 使用 dev profile 真实历史会话复验，连续 5 轮均出现可绘制的错误中间
专家状态。此前“专家不会闪”的结论无效。

实际显示序列不是原子切换，而是：

```text
目标 transition expert → 独立 activeExpert 的旧值/空值 → 后端刷新后的目标 expert
```

## 观测方法

- 项目树 click 为 `t=0`，每次点击生成 trace ID。
- 记录 transition begin/finish、active tab、专家 refresh start/resolve/apply/ignored、
  App layout commit、`requestAnimationFrame` 和 paint 后宏任务。
- 事件写入 dev-only 1,000 条 ring buffer，并通过 Wails `LogInfo` 输出。
- 锚点不参与状态选择，不改变 transition、专家刷新或 React 状态值。
- 运行日志：`%TEMP%/reasonix-codex-acceptance/wails.stderr.log`。

## 五轮呈现时间线

`<none>` 表示 Composer 未显示专家。时间均相对本轮点击。

| Trace | 目标 | App commit 序列 | paint 后序列 |
| --- | --- | --- | --- |
| `1785846788411-1` | `<none>` | 80ms `<none>` → 465.3ms 会计核算专家 → 611ms `<none>` | 124.6ms `<none>` → 495ms 会计核算专家 → 629.5ms `<none>` |
| `1785846815142-2` | 会计核算专家 | 79.3ms 会计核算专家 → 385.6ms `<none>` → 1833.8ms 会计核算专家 | 125.3ms 会计核算专家 → 419.4ms `<none>` → 1860.1ms 会计核算专家 |
| `1785846860044-3` | `<none>` | 82.1ms `<none>` → 519.8ms 会计核算专家 → 730.9ms `<none>` | 120.7ms `<none>` → 557.4ms 会计核算专家 → 752.6ms `<none>` |
| `1785846870529-4` | 会计核算专家 | 84.5ms 会计核算专家 → 383.8ms `<none>` → 1701.3ms 会计核算专家 | 132.7ms 会计核算专家 → 415.1ms `<none>` → 1731.7ms 会计核算专家 |
| `1785846883307-5` | `<none>` | 79ms `<none>` → 441.4ms 会计核算专家 → 643.9ms `<none>` | 116ms `<none>` → 477.3ms 会计核算专家 → 663.5ms `<none>` |

错误状态至少跨过一次真实 paint，并非日志顺序或不可见 React 中间状态：

- 专家 → 无专家：旧专家回闪约 134–196ms。
- 无专家 → 专家：专家消失约 1.32–1.44s。

## 写入链路

1. `ProjectTree` 点击立即用 sidecar identity 建立目标 transition，首帧值正确；真正
   `onOpenTopic` 仍由 200ms timer 延后。
2. `App` 在 transition active 时显示 `transitionActiveExpert`，结束后立即改用独立
   hook 状态 `activeExpert`。
3. 缓存会话在 `ActivateTopic` 返回、切换 active tab 后立即
   `finishSurfaceTransition`，没有等待目标专家状态确认。
4. `runOpenTopicRequest` 随后调用的是当前 render 闭包中的 `refreshExpertState`；该
   闭包仍可能绑定源 tab。trace 2 中 refresh sequence 18 仍查询源 tab，后来被忽略。
5. active tab 改变触发目标 tab refresh，但专家 runtime 尚未恢复时
   `CurrentExpertForTab` 先返回空。trace 2 的 sequence 19 在 467.3ms 返回空；直到
   1703.6ms 的 sequence 20 再查，1744.1ms 才返回会计核算专家。
6. 因此 transition 正确目标值被过早撤下，独立 `activeExpert` 的旧值或暂时空值
   获得真实 paint；后续 refresh 才再次写成目标。

## 源码锚点

- 点击及 200ms 延时：`ProjectTree.tsx` 的 topic `onClick`。
- transition 展示与 `activeExpert` 回退：`App.tsx` 的 `displayedActiveExpert`。
- transition 完成：`useController.ts` 的 `finishSurfaceTransition` 与缓存命中分支。
- 异步专家状态：`useExpertCatalog.ts` 的 `refreshExpertState`、effect 和
  `resolveExpertRefreshSelection`。

## 修复前门槛

在用户确认修复策略前不修改状态选择。后续修复必须保证 transition identity 与
目标 runtime expert 完成一次有版本的 handoff；不能通过延长固定 loading 时间、
隐藏专家控件或增加截图延时掩盖错误状态。

## 修复后实现与五轮验收

实现采用版本化原子交接，不增加固定等待时间：

1. `renderedViewTarget.composerIdentity` 在 transition 完成后仍拥有专家展示权。
2. 专家刷新捕获目标 `viewKey`、目标 tab 和 refresh sequence；旧 tab 或旧 view
   的迟到结果会被忽略。
3. 仅在目标 runtime ready、允许清空选择且刷新结果与当前 `viewKey + tabId`
   精确匹配时，发布 authoritative resolution。
4. resolution 发布前即使独立 `activeExpert` 暂时为空或仍是源专家，Composer
   仍显示目标 identity；发布时目标值与 runtime 值相同，因此交接不产生视觉变化。
5. 删除打开历史会话路径中使用旧 render 闭包主动触发的专家刷新，刷新只由目标
   tab/view/ready 状态驱动。

| Trace | 目标 | 首次 paint | authoritative | 最后 paint | commit/paint 唯一显示值 |
| --- | --- | ---: | ---: | ---: | --- |
| `1785847447895-6` | 会计核算专家 | 107.6ms | 1740.1ms | 1836.4ms | `exp_accounting` |
| `1785847461362-7` | `<none>` | 118.4ms | 490.5ms | 545.7ms | `<none>` |
| `1785847472750-8` | 会计核算专家 | 120.7ms | 1770.4ms | 1825.2ms | `exp_accounting` |
| `1785847484187-9` | `<none>` | 125.1ms | 478.0ms | 537.0ms | `<none>` |
| `1785847494852-10` | 会计核算专家 | 121.3ms | 1824.0ms | 1906.4ms | `exp_accounting` |

每轮显示来源都会从 `target-identity` 变为 `active-expert`，但值在交接前后相同。
这证明等待的是权威所有权交接，不是隐藏控件或延迟绘制。最终原生窗口中，无专家
会话没有专家 chip；专家会话直接显示“会计核算专家”。`kwu-desktop-dev` 在切换后
仍响应，日志未出现 panic、fatal 或 renderer crash。

## 无权模型原子展示证据

### 修复前

非登录 dev profile 从“你好4”切换到“当前是v4”时，模型选择器连续出现：

```text
354–533ms  gpt/gpt-5.5
654–1032ms 无权订阅模型
1149ms+    请选择模型
```

根因是 transition 把目标会话原始 `modelRef` 传入 pending 的 ModelSwitcher；
后者会先格式化原始 ref，等目录与 entitlement facts 结算、过滤无权路由后才显示
“请选择模型”。这不是后端改写模型，而是同一选择器先后采用了两套展示策略。

### 修复后方法

- 保持 dev 应用登录状态、历史 profile 和测试会话不变。
- 每轮先切到 BYOK 会话“你好4”，确认 `gpt/gpt-5.5`，再切到订阅模型会话
  “当前是v4”。
- 通过 Windows UI Automation 连续读取会话标题与模型按钮，每轮持续
  0.7–1.2s；共执行 5 轮并观察窗口存活状态。

### 五轮结果

| 轮次 | 无权目标样本数 | 首个样本 | 最后样本 | 可见模型唯一值 |
| --- | ---: | ---: | ---: | --- |
| 1 | 10 | 92ms | 1201ms | `请选择模型` |
| 2 | 8 | 83ms | 975ms | `请选择模型` |
| 3 | 6 | 88ms | 710ms | `请选择模型` |
| 4 | 6 | 96ms | 732ms | `请选择模型` |
| 5 | 8 | 98ms | 1002ms | `请选择模型` |

38 个目标样本的标题均为 `Global / 当前是v4`，没有出现源 BYOK 模型或任何订阅
模型名。每轮回到“你好4”后选择器都恢复 `gpt/gpt-5.5`，证明修复没有把 BYOK
统一清空。最终原生窗口仍响应，Composer 几何稳定且选择器显示最终的“请选择
模型”，没有 skeleton 或占位条。
