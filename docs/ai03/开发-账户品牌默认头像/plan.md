# 账户品牌默认头像开发计划

## 1. 背景与当前行为

创建模式侧栏的账户入口在没有本地头像时，以账户标签首字回退：未登录显示“登”，已登录可能显示手机号或用户 ID 首位；账户中心身份卡片则显示通用 `UserRound`，视觉不统一。

当前头像不是云端用户资料：

- `useAccountSidebar.ts:88-126` 用 `FileReader` 将用户选择的 PNG/JPEG/WebP 写入本机 `localStorage`，限制 2 MiB，没有桥接或 HTTP 调用。
- 存储键为 `clouvia.sidebar-avatar.<accountID>`，当前依次取
  `cloud_user_id`、`display_name`、`local`。
- `App.tsx:254-260,3526-3532,3788,3802` 负责侧栏两处文字头像。
- `AccountCenter.tsx:775-777` 负责已登录身份卡片的通用图标回退。

退出后认证视图会变为 `anonymous`，但绑定仍保留最近账号 ID；若只按 ID 选键，会继续显示上一账号的本地头像。

## 2. 期望行为

所有实际存在的账户头像位置采用同一优先级：

1. 当前本地槽位有用户手选图片：显示该图片。
2. 当前槽位没有图片：显示现有实心
   `desktop/frontend/src/assets/brand-symbol.png`。
3. 不再以“登”、手机号/用户 ID 首位、显示名首字或通用用户图标作为头像回退。

本地槽位按身份隔离：

- `authenticated` 以及带稳定用户 ID 的 `offline`、`expired` 使用
  `cloud_user_id` 槽位。
- 其他状态使用 `local` 槽位，即使视图保留最近绑定 ID。
- 缺少稳定用户 ID 时不得用可变、可重复的 `display_name` 代替账号身份。

## 3. 目标、非目标与边界

### 目标

- 侧栏入口、展开菜单、已登录账户中心身份卡使用一致的品牌回退。
- 未登录状态和不同账号的本地图片互不串用。
- 退出后不残留上一账号头像，离线/过期账号仍保留自身头像。
- 保留现有上传行为和既有本地数据。

### 非目标

- 不开发或调用云端头像上传、对象存储、裁剪、压缩或同步能力。
- 不新增提示、设置页、“删除头像”或“恢复默认”入口。
- 不修改品牌资产、认证语义、账户文案、Reasonix-Cloud、Wails 绑定或依赖。

### 边界

- 继续读取 `clouvia.sidebar-avatar.local` 和
  `clouvia.sidebar-avatar.<cloud_user_id>`，不清空、不迁移、不重写。
- 头像图片保持装饰语义；现有按钮名称和账户文字标签不变。
- 实心品牌图标是渲染回退，不写入 `localStorage`。

## 4. 用户场景与验收标准

| 场景 | 验收结果 |
| --- | --- |
| 首次启动、未登录、无本地图片 | 侧栏入口和菜单显示实心品牌图标 |
| 未登录槽位已有图片 | 侧栏入口和菜单显示该图片，重启后恢复 |
| 账号 A 登录且无本地图片 | 侧栏入口、菜单、账户中心身份卡均显示品牌图标 |
| 账号 A 已有图片 | 三处均显示 A 的图片，重启后恢复 |
| A 切换到无图片的 B | 新状态首次渲染起不得出现 A 的图片；B 显示品牌图标 |
| B 有自己的图片 | 三处显示 B 的图片，不读取 A 或 `local` 槽位 |
| B 退出 | 新状态首次渲染起使用 `local` 图片或品牌图标，不出现 B 的图片 |
| A 进入 `offline`/`expired` 且有稳定 ID | 继续使用 A 的账号槽位 |

同时满足：

- 现有 PNG/JPEG/WebP、2 MiB 校验和错误提示不变。
- 账户中心未登录分支仍显示现有登录/注册内容；该分支没有身份卡片，不增加一个。
- 不新增任何云端请求或持久化格式。
- 侧栏菜单、账户中心、切换、退出等业务交互不回归。

## 5. 调查证据与实现依据

### 基线

- Codegraph 主工作区：`D:\Reasonix`
- 调查基线：`main-v3@80b7cae56adbe4cd4776591c8a9feac8d70e1fc5`
- 独立 worktree 在同一基线直接复核了相关源码、测试与 EOL。

### 关键证据

- `accountStore.ts:187-190` 已定义可恢复身份：
  `authenticated`，或带 ID 的 `offline`/`expired`。
- `auth_application.go:343-382,795-797` 证明退出转匿名时保留绑定。
- `cloud_api.go:769-789` 会把绑定 ID投影到匿名 `CloudAccountView`。
- `useAccountSidebar.ts:93-99` 以普通 `useEffect` 读取新键；若状态只保存
  `avatarSrc`，账号切换后的首次渲染仍可能携带旧槽位图片。
- `CloudUser.Validate` 要求 `cloud_user_id`，生产账号不需要显示名身份兜底。

Reasonix-Cloud 提交 `7993c3d10d0af6d5f63b6ce25742b4a3cfddcfe2`
虽预留 `avatar_url` 字段和读取响应，但没有当前用户自助修改/上传接口；桌面端 `authUserWire` 也未接收该字段。因此云端能力不属于本次范围。

## 6. 选定方案与备选方案

### 选定方案

1. 复用实心 `brand-symbol.png`，不新增资源。
2. 侧栏头像组件只渲染图片：本地图片为空时使用品牌图标，不再接收首字符。
3. 账户中心身份卡片使用相同回退；标题和操作菜单里的功能图标不属于头像，不改变。
4. `useAccountSidebar` 根据可恢复身份和稳定 `cloud_user_id` 解析存储键；否则使用 `local`。
5. 头像状态必须同时关联“已加载的存储键”和图片值。当前键与已加载键不一致时立即显示品牌图标，直到目标键读取完成；不得让旧图片参与新身份的首次渲染。
6. 上传完成后只更新当前键对应的状态和 `localStorage`。

自动化测试必须检查切换/退出后的首次渲染，而不只等待 `useEffect` 后的最终状态。

### 未选择

- 全设备共用一张本地图片：会让不同账号串用身份图像。
- 将品牌图标写入每个存储键：产生无意义写入并混淆“默认”和“用户设置”。
- 接入云端头像：缺少当前用户修改接口，且显著扩大本次范围。

## 7. 影响范围

预计修改：

- `desktop/frontend/src/App.tsx`
- `desktop/frontend/src/features/account/controller/useAccountSidebar.ts`
- `desktop/frontend/src/features/account/ui/AccountCenter.tsx`
- `desktop/frontend/src/__tests__/account-sidebar-bootstrap.test.tsx`
- `desktop/frontend/src/__tests__/account-center-states.test.tsx`
- `desktop/frontend/src/__tests__/brand-contract.test.ts`

仅当 22/28/38 px 实际检查需要时，局部调整
`styles/theme-overrides.css` 或 `styles/features.css`。不得修改品牌 PNG、
`scripts/code_guard_baseline.json`、Go/云端代码或依赖文件。

## 8. 兼容性、风险与回滚

### 兼容性

- 无数据库、云端 API、Wails 或本地存储迁移。
- 既有 `local` 和账号 ID 键继续生效。
- 从匿名状态误用最近账号键改为固定 `local` 是有意的身份隔离修正；重新登录账号后其原头像仍可读取。

### 风险与控制

- **旧头像短暂串用**：采用“存储键 + 图片值”关联状态，并测试首次渲染。
- **小尺寸品牌图标不清晰**：检查 22/28/38 px 和明暗主题，仅做必要的局部样式调整。
- **读取失败被品牌回退掩盖**：测试直接断言读取的键，而不只断言最终有图片。
- **性能**：不新增网络或持久化扫描；每次身份变化仍只读取一个键。
- **隐私**：退出和切换不再显示上一身份的本地图片。

### 回滚

回退实现提交即可恢复文字/通用图标回退和旧键选择。由于不删除或迁移本地数据，回滚不会丢失已有图片，也无需云端操作。

## 9. 验证计划

### 应用验收

由主会话或独立验证者执行：

1. 在明暗主题检查未登录侧栏入口和菜单的品牌图标。
2. 验证未登录 `local` 图片的选择、显示和重启恢复。
3. 登录 A，验证无图时三处品牌回退、有图时三处同步。
4. 从 A 切换 B，观察首次渲染不得出现 A 图片；验证 B 独立槽位。
5. 退出 B，观察首次渲染立即回到 `local` 图片或品牌图标。
6. 验证 `offline`/`expired` 仍使用账号槽位。
7. 回归文件格式、大小校验和账户菜单操作。

优先使用内置浏览器；若真实 Wails 状态或本机文件选择无法覆盖，再使用
Computer Use。仅在工具无法可靠完成或需要用户凭据时请用户验收。

### 自动化验证

迭代期间：

```powershell
cd desktop/frontend
pnpm exec tsx src/__tests__/account-sidebar-bootstrap.test.tsx
pnpm exec tsx src/__tests__/account-center-states.test.tsx
pnpm exec tsx src/__tests__/brand-contract.test.ts
pnpm typecheck
pnpm check:css
```

行为稳定后运行 `pnpm build`。是否扩展到 `pnpm test:all` 由最终变更路径、
依赖影响和 PR Gate 分类决定，不因本计划预先扩大。

创建或更新 PR 前，先同步最新 `origin/main-v3`，完成应用验收，再运行仓库
`AGENTS.md` 规定的 Common checks 和：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

## 10. 未解决问题与停止条件

没有待用户决定的问题。实施时仅需确认实心图标是否需要局部样式。

若需要改变外部行为、存储格式、云端契约、依赖、风险等级或新增 UI 控件，停止实施，更新本计划并重新批准。
