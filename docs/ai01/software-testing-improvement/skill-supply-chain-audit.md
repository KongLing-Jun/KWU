# 测试任务 Skill 供应链与权限审查

> 审查日期：2026-07-23
> 审查范围：本轮测试改进任务实际调用或由用户明确指定的 9 个 skill。
> 结论边界：审查本机已安装快照，不把 README、star 数或远端最新内容当作
> 已批准来源；任何 skill 都不直接构成 required gate。

## 1. 决策摘要

| Skill | Trigger | 来源/版本证据 | 权限与数据边界 | 决策 |
|---|---|---|---|---|
| `incremental-implementation` | 多文件测试基础设施改动 | 本地 `SKILL.md` 哈希；无 release、许可证或上游来源元数据 | 读取仓库、修改测试文件、执行项目命令；不得自行安装依赖或发布 | 条件接受，仅约束增量方法 |
| `karpathy-guidelines` | 需要最小、可验证改动 | 本地 `SKILL.md` 哈希；无 release、许可证或上游来源元数据 | 读取/修改任务范围文件并执行验证；无独立门禁权限 | 条件接受，仅约束编码方法 |
| `debugging-and-error-recovery` | 首次测试失败和根因定位 | 本地 `SKILL.md` 哈希；无 release、许可证或上游来源元数据 | 文档包含 `git bisect`、`npm install` 等通用命令；执行前必须由仓库事实和当前授权约束 | 条件接受，仅用于诊断；不得自动改依赖或 Git 历史 |
| `playwright` | 终端真实浏览器探索 | Apache-2.0/NOTICE 存在；本地 `SKILL.md` 哈希；未记录 skill release | wrapper 使用未固定版本的 `npx --package @playwright/cli`，可能联网下载并执行包；可读取页面、输入、截图 | required gate 拒绝；本项目只使用锁文件固定的 Playwright |
| `security-and-hardening` | 安全、凭据和供应链边界审查 | 本地 `SKILL.md` 哈希；无 release、许可证或上游来源元数据 | 以审查和建议为主；引用的 `references/security-checklist.md` 缺失 | 条件接受为人工审查方法；不得进入 required gate |
| `browser:control-in-app-browser` | 用户明确要求内置浏览器或本地页面交互 | OpenAI bundled plugin `26.715.52143`，Proprietary；manifest 和 client 均在本地 | Interactive/Read/Write；可导航、点击、输入和截图；禁止读取 Cookie、密码和 session store | 仅人工 local/browser smoke；不批准发布 |
| `chrome:control-chrome` | 用户明确要求现有 Chrome 状态 | OpenAI bundled plugin `26.715.52143`，Proprietary；manifest 和 client 均在本地 | Interactive/Read；可能接触登录页面内容；禁止检查 Cookie、密码和 session store | 仅用户明确授权的人工探索；不处理客户/生产会话 |
| `web-design-guidelines` | UI/无障碍评审 | metadata `1.0.0`、author `vercel`；无许可证/commit | 每次从 GitHub `main` 分支抓取最新规则，存在网络访问和规则漂移 | required gate 拒绝；只作人工建议 |
| `vercel-react-best-practices` | React 性能评审 | metadata `1.0.0`、Vercel Engineering、January 2026；无许可证/commit | 读取代码并提出/实施建议；规则目录本地存在，不应自动安装 README 中依赖 | 仅建议性评审；进入门禁前需补来源与许可证 |

## 2. 本地快照证据

以下 SHA-256 只固定本机审查时的文件内容，不等价于上游签名、commit 或
许可证证明：

```text
incremental-implementation
960f2ed08ccdd51527c347d29d2d00c80edb9992e95d67030d8a289ba04eed4e

karpathy-guidelines
39096973a96f8fa7d5aae223135037171cc628a2741751dec0e5007ebeff0a27

debugging-and-error-recovery
927ad6917fa2891827351cfa3470cd715423faaa5c5f27fe74534d9324c35eea

playwright
c52cc95125b720cdc3c9829b0294c2752d4ae9b85a5282aa153da8bba929af87

security-and-hardening
266c60e073bc81651ad070a1427a93af4d062dc44470580eae31806b38561833

browser:control-in-app-browser
28927858747b869471858458777e1e45fb95315bd80bd967bead0616f602f3a3

chrome:control-chrome
c804ef161231038d261137f3fe44af783bfe482b93f9b233455b1a97601d0563

web-design-guidelines
f4647ca866a3accf763777f83e7682954f0187cd6bea7eea0399796652414e8f

vercel-react-best-practices
71ed7794962fa6e803ee83030517b5b93a9f70fbfeb431ec4535c5480a8d8355
```

Browser 与 Chrome 的 `scripts/browser-client.mjs` 均存在，内容 SHA-256 为：

```text
583bf903a6cfb9b00a01e1583381ce921c71d7585085c6fa6b805baa0f60230b
```

## 3. 发现

### GOV-TEST-003：Playwright skill 执行包未固定

- 严重度：High（若进入 CI/required gate）。
- 证据：`scripts/playwright_cli.sh` 执行
  `npx --yes --package @playwright/cli playwright-cli`，没有版本或完整性固定。
- 影响：同一命令可能随 registry 当前状态执行不同代码，也可能触发首次网络下载。
- 处置：不得把该 wrapper 用于 Reasonix required gate；继续使用仓库
  `pnpm-lock.yaml` 固定的 Playwright 测试入口。

### GOV-TEST-004：Security skill 引用不完整

- 严重度：Medium。
- 证据：`SKILL.md` 三处引用 `references/security-checklist.md`，本地 skill
  目录只有 `SKILL.md`，引用文件不存在。
- 影响：无法完整执行其自述的供应链版本矩阵和详细 pre-commit 清单。
- 处置：本轮只采用 `SKILL.md` 内可核验的最小权限原则；不臆造缺失内容，
  不允许该 skill 直接成为门禁。

### GOV-TEST-005：Web Design 规则源不可复现

- 严重度：Medium。
- 证据：skill 要求每次从
  `vercel-labs/web-interface-guidelines/main/command.md` 获取最新规则。
- 影响：远端 `main` 可变、依赖网络，历史评审无法仅凭仓库 SHA 离线复现。
- 处置：只作人工建议；若将来进入门禁，必须固定 commit、缓存规则并记录许可证。

### GOV-TEST-006：方法类 skill 缺少统一来源元数据

- 严重度：Medium。
- 范围：`incremental-implementation`、`karpathy-guidelines`、
  `debugging-and-error-recovery`、`security-and-hardening`，以及当前
  `vercel-react-best-practices` 本地包。
- 影响：本机文件哈希可证明审查快照，但不能证明上游维护状态、许可证或来源。
- 处置：继续限定为方法建议；不得复制进 CI 或自动批准例外。进入共享工具链前，
  需要固定仓库 commit/release、许可证、依赖和维护 owner。

## 4. 输入、输出、数据保留与回滚

- 输入仅限仓库源码、合成 fixture、本地 loopback 页面和脱敏测试结果。
- 禁止向 browser/Chrome/远端 guideline 请求发送客户数据、生产日志、凭据、
  Cookie、token、手机号、密码、支付信息或完整会话。
- 输出仅为测试代码、测试制品和 Markdown 证据；视觉或 skill 建议不得直接
  改写 required gate 结论。
- 本轮未安装新 skill、插件或依赖，也未向 CI 增加 skill 运行时。
- 回滚方式是停止调用对应 skill，并继续执行仓库固定测试命令；Browser/Chrome
  插件缓存不是 Reasonix 构建依赖，移除不会改变仓库门禁。

## 5. 审查结论

T6.3 的“逐个审查、不整包盲装”已对本任务实际使用的 9 个 skill 完成。
审查结果包含条件接受和拒绝进入 required gate，而不是统一批准。

Checkpoint M6 仍未完成：本地方法类 skill 的统一来源、许可证、release 和
维护 owner 尚缺失；发布、安装、签名、SBOM 和长稳 required gate 也尚未达到
任意合格 runner 可离线复现的状态。
