# Composer 会话配置加载回退验收方案

## 目的

验证已有会话在标签切换、历史重开和应用重启恢复时立即显示目标会话的 Composer 配置，
同时证明权限门禁、会话隔离、原子更新和失败恢复没有回退。

## 测试环境与数据

- 使用独立 Wails 开发配置和测试 profile，不修改用户生产配置。
- 准备两个可识别的 BYOK 测试模型，端点可指向本机不可达地址；验收不发送模型请求。
- 如可安全使用测试 subscription fixture，再增加允许/拒绝/loading 三态；否则通过现有前端
  合同测试覆盖权限三态，真实应用只验证可用账户范围。
- 会话 A：支持 effort，模型 `provider-a/model-a`，强度 `low`，权限 ask，协作 normal。
- 会话 B：支持 effort，模型 `provider-b/model-b`，强度 `high`，权限 auto 或 yolo，协作 plan，
  并选择一个专家或专家团。
- 会话 C：不支持 effort，用于验证只显示模型。
- 为避免误调用外部服务，只创建/恢复会话和切换配置，不发送 prompt。

## 观察方式

- 优先使用 Codex 内置浏览器控制可访问的开发页面；原生 WebView 无法覆盖时使用 Computer Use。
- 在测试构建中使用 DOM 文本、`disabled`、`aria-busy` 和 selection presentation 属性作为主要
  观察点；必要时录制截图序列或短视频证明没有中间空白帧。
- 同时查看客户端 breadcrumb/控制台，确认目标 tab id、surface transition 和 composer selection
  seq；不得仅凭最终稳定画面判定通过。

## 场景 1：已打开标签切换

步骤：

1. 打开 A、B，分别确认稳定状态为 `model-a · 低`、`model-b · 高`。
2. 从 A 点击 B，在点击后的第一个目标 Composer 渲染读取模型文字、强度、权限、协作和专家。
3. 后端激活完成前读取选择器 disabled/aria-busy；完成后再次读取。
4. B→A 重复，并连续切换至少 10 次。

预期：

- 首个目标帧已经是目标会话组合，没有 loading、空白或上一会话强度。
- 激活期间值可见且选择器禁用；稳定后解除禁用。
- 权限、协作、专家与模型/强度属于同一目标会话。

## 场景 2：快速连续切换

步骤：A→B→A→B 快速点击，期间人为延迟 A 或 B 的 `EffortForTab` 回读。

预期：最终 B 只显示 B 的组合；任何旧 seq 返回都不造成回闪或覆盖。

## 场景 3：历史会话重开

步骤：关闭 B 的运行标签，从历史列表和项目树分别重新打开 B，观察首帧与稳定帧。

预期：首帧由 B 的 branch meta/Composer identity 恢复 `model-b · 高` 及现有 profile/专家身份；
权威回读相同时无第二次文字替换。

## 场景 4：应用重启恢复

步骤：让 A、B 保持打开，正常关闭并重启应用；在后端返回 tabs 后读取 Composer 首个配置帧。

预期：恢复的活动会话一次显示完整模型/强度；不先显示模型再补强度，也不借用另一个标签。

## 场景 5：订阅权限

分别制造：access loading、ready+allowed、unavailable/denied。

预期：

- loading：持久化模型/强度可见但禁用，发送门禁保持关闭。
- ready+allowed：保持组合并解除禁用。
- unavailable/denied：沿用现有逻辑显示“请选择模型”，不泄露为可选项，不隐式选模型。
- 账号或 binding scope 切换时旧账户快照不得用于新账户授权判断。

## 场景 6：BYOK 与不支持 effort

步骤：切换到 BYOK 会话，并让共享目录读取延迟；随后切换到会话 C。

预期：BYOK 持久化组合不因订阅读取清空；C 只显示模型，不出现伪造的“自动”强度入口。

## 场景 7：主动模型与强度切换

步骤：在稳定 A 中切换模型，再切换强度；分别模拟成功和失败。

预期：

- 模型切换期间上一确认组合保持可见且禁用，新组合 ready 后一次替换；失败恢复完整旧组合。
- 强度切换立即显示新强度且保留模型；失败恢复旧强度并显示现有警告。

## 自动化与构建门槛

- 所有任务文档列出的聚焦测试通过。
- 直接改动 Desktop Go package 范围测试通过。
- `pnpm build` 通过。
- 最终分支无 EOL 异常，`git diff --check origin/main-v3...HEAD` 通过。
- 若随后获准创建/更新 PR，先同步最新 `origin/main-v3`，重跑关键应用路径和完整 PR Gate。

## 失败判定与证据记录

以下任一情况判定失败：出现 loading/空白帧、模型和强度来自不同会话、激活前控件可写、
无权模型继续显示为可用、旧请求覆盖最终会话、主动切换行为回退、构建或必需测试失败。

实际执行后在本文追加紧凑结果表：环境、场景、结果、关键观察和证据位置；详细日志只记录
必要片段，不把本文变成时间线。

## 实施者准备与自动化结果

| 环境 | 场景 | 结果 | 关键证据 |
| --- | --- | --- | --- |
| jsdom/controller | 标签切换、历史重开、重启恢复、快速切换 | 通过 | `tab-switch-hydration.test.tsx` 147/147；首帧断言同帧包含目标 model/effort，迟到结果不覆盖最终目标 |
| 纯状态/快照 | identity 隔离、缓存组合、legacy unknown、unsupported model-only | 通过 | `session-view-snapshot` 18/18，`model-switcher-transition` 19/19 |
| jsdom/switcher | BYOK/subscription loading、known-empty/权威 empty auto、禁用快照、definitive choose | 通过 | `model-switcher-stable-snapshot` 26/26，权威 auto 不被旧快照覆盖，快照不生成 effort 选项 |
| reducer/controller | 原子组合、主动模型/强度切换、失败恢复、专家 | 通过 | `use-controller-meta` 97/97，`composer-selection-atomic` 33/33，专家回归通过 |
| 生产构建 | TypeScript、CSS、Vite production build | 通过 | `pnpm build`；仅既有 chunk-size/plugin-timing 警告 |
| 隔离 Wails 构建/启动 | 当前工作树真实 Go + WebView 产物 | 通过 | `wails build` 成功；独立 `KWU_DEV_PROFILE_ROOT` 与本地云环境启动当前 `kwu-desktop`/`kwu-desktop-dev`，生成独立 `dev-environment.json`、tabs、sessions 与 WebView 数据 |
| 现有 dev 配置 Wails 交互 | 场景 1、4、5 的关键路径与连续切换 | 通过 | 从当前工作树执行 `wails dev -nosyncgomod -nocolour`，未注入 `KWU_*`/`CLOUVIA_*` 覆盖；进程使用现有 dev 环境 `a6defe83d02e4e84`，Vite `http://127.0.0.1:5179/`，Wails 代理 `http://localhost:34115` |

### 真实 dev 环境验收结果

- 从无可用模型会话切到“你好”后，第一个可读取目标帧已同时显示
  `gpt/gpt-5.6-luna · 自动`，`aria-busy=true` 且按钮禁用；约 60 ms 后权威回读完成，
  文案保持不变并解除禁用。权限同帧保持该会话的“需要批准”。
- 切到最终不可用的会话“你是谁，你能做什么”时，首帧和稳定帧均沿用既有
  “请选择模型”，没有把保存快照泄露为可用选项。
- 在上述两个会话间连续切换 10 次，最终标题、模型呈现和可用性均属于最后目标会话；
  返回“你好”后仍显示完整模型/强度组合。
- 停止并重新启动当前工作树的 Wails dev 进程后，活动会话恢复为“你好”，稳定状态仍为
  `gpt/gpt-5.6-luna · 自动`。重启后的精确首帧由
  `tab-switch-hydration.test.tsx` 的 deferred `EffortForTab` 合同断言覆盖；本次浏览器控制未在
  reload 返回前截获该瞬时帧，因此不把稳定画面冒充逐帧证据。
- 未关闭/归档现有 dev 数据中的会话，故历史列表真实重开未在本轮执行；场景 3 已由聚焦合同
  测试覆盖。专家团、主动切换失败和完整权限三态同样保留自动化证据，不以本轮现有账号样本
  过度声明。

## 用户最终确认（最短路径）

无需重新造测试数据；在包含本分支的可运行产物中，用两个已经保存了不同模型/强度的会话：

1. 在 A、B 之间切换，点击后应立即看到目标模型与强度；加载期间文字保留、控件禁用，不出现
   空白或 loading 文案。
2. 从历史关闭并重开 B，再重启应用；首个可见配置仍应一次出现完整模型与强度。
3. 对绑定订阅 provider 的会话，在账号权限尚在获取时仍显示保存值并禁用；若最终未登录或
   无权，再切换为既有“请选择模型”。
4. 确认权限、协作方式、专家/专家团没有串到另一个会话。

只需反馈首个失败步骤、看到的模型/强度文字和是否可点击；其余竞态、失败恢复、BYOK、
unsupported effort 与快速切换已由上表自动化覆盖。
