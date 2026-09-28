# 统一会话加载态进度

## 当前状态

- 阶段：任务 001 保持完成；任务 002 的全屏新会话等待事务被用户否决并已从工作树移除，正在恢复基线后重新建立窄回归。
- 产品决策：稳定外壳；一套 `正在加载会话…` 仅用于已有会话；保护立即生效；反馈约 180ms 后显示；删除三块骨架；失败面板独立保留。新建空白会话不得显示白屏或加载文案，不得破坏目标 Composer 草稿首帧。
- 实现边界：保留统一历史等待视觉、Transcript 延迟所有权与全部锚点修复；撤回 App 全屏遮罩、全局 inert 和过宽导航门禁，不改变 Go/Wails 或底层 blank helper。

## 已完成调查

- Codegraph 主工作区基线为 `f98f9878...`，当前结论已由独立 worktree 源码复核。
- 已定位首次加载三块 placeholder、四类重复等待 JSX、overlay/failure CSS、现有约 120ms Controller 提示计时及邻近回归。
- 已确认新方案必须保留专家/普通长会话稳定恢复期间的即时 hidden/inert，加载态只是视觉层，不替代恢复正确性。
- 已完成 `activeTabId` 调用链复核：模型、专家、技能/连接器、Composer、右侧工作区、运行时操作与滚动恢复均依赖当前会话；进入其他页面不得清空选中态。
- 已定位 A 历史回闪为 `handleNewTab` 先 `showChatView`、后等待 B 的展示顺序；确认不能用通用伪 blank target，且 `EnsureBlankSurface` 无可取消 prepare/rollback。
- 任务 002 已先保留独立实现提交，再 merge 最新 `origin/main-v3`；App 自动合并无文本冲突，新增的模型/推理强度原子呈现与本导航事务经聚焦复核共同通过。
- 任务 001 OLD RED 为 existing loading 49/57；GREEN 为 existing 57/57、expert 217/217、restore 21/21、anchor 32/32、diagnostics 2/2。placeholder/shimmer 已删除，pending→persisted 的原始 180ms timer 不重置。
- 任务 002 OLD RED 为原子导航 4/10（6 个精确失败）；GREEN 为静态与 promise 竞态 17/17、new-session race 25/25、existing 57/57、expert 217/217、restore 21/21、anchor 32/32、diagnostics 2/2，CSS/z-index 与直接 Vite production bundle 通过。
- 最新基线同步后再次 GREEN：原子导航 18/18、existing 57/57、new-session race 25/25、模型/推理强度 33/33 与 15/15、expert 217/217、restore 21/21、anchor 32/32、diagnostics 2/2、CSS/z-index 与直接 Vite production bundle。
- clean HEAD `7793238a...` 原生验收 PASS：专家与普通长会话均只出现来源页→统一等待表面→新会话 Welcome，没有 A 历史回闪；等待表面只暴露 loading status，主工作区不可交互。
- 返回专家 A 后仍为 `ab14cf0b8/q5/795.60009765625`，返回普通 A 后仍为 `a4e3d9d47/q5/63.200000286102295`；两者均为 virtual seek 1、visible jump/incomplete 0、stableFrames 30，稳定后无迟到 seek/cache rewrite。专家深虚拟行按既有 closest-boundary 合同收敛且 canonical 未被覆盖。
- 用户随后否决该新建体验：全屏遮罩在 180ms 反馈阈值前表现为纯白，并把新建错误建模为历史加载；上述 PASS 仅证明无 A 回闪与锚点不回归，不再作为任务 002 的产品验收结论。
- 源码复核确认既有 Composer 固定壳体合同要求目标草稿绘制前恢复；`EnsureBlankSurface` 还可能重建同一 blank topic 的 tab/session path，因此后续不能用临时空 Composer 冒充 B。
- 止损工作树验证：Composer draft 43/43、new-session race 25/25、existing loading 57/57、模型/推理强度 33/33 与 15/15、TypeScript 与直接 Vite production bundle 通过；`app-chrome-tabs` 保持既存 88/91 三项源码正则失败。
- 止损后的 `pnpm typecheck` 与完整 `pnpm build` 已通过；`app-chrome-tabs` 保持既存 88/91 三项源码正则失败，未在本任务扩域修复。

## 阻塞与下一门禁

- 任务 002 尚未完成：止损实现与聚焦验证已完成；下一步在恢复的生产行为上建立“模型广场闪 A”和 blank 草稿往返的精确旧红，最终呈现方案仍需用户确认。
- 被否决方案的原生证据保留在 `C:\Users\ai03\AppData\Local\Temp\reasonix-new-session-atomic-20260811-172635`，仅作为回退原因与锚点未回归证据，不作产品 PASS。
- 当前不 push、不创建/更新 PR、不标记 Ready、不合并。
- 未获得 push/PR/Ready/merge 授权。
