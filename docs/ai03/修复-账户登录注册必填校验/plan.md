# 账户登录注册必填校验修复计划

## 背景与问题

账户页主登录/注册表单目前直接把输入交给云端调用：

- 登录或注册缺少必填输入时，仍可能进入真实滑块或调用 `CloudLogin`/`CloudRegister`，最终显示云端错误。
- 注册手机号为空时，真实模式仍可进入滑块流程；Fixture 模式也可先完成验证再请求短信。
- 必填字段没有红色 `*`，用户无法在提交前识别必填项。
- 登录失败产生的顶部错误横幅存放在账户共享状态中；切换到注册标签只改变 `mode`，横幅继续显示。
- 云端已提供稳定账户业务错误码，但账户页没有账户专属中文映射，常见登录、注册和短信失败会统一落入“云请求失败”。

用户已确认：按钮保持可点击以便显示具体字段错误，但任何空值都必须在前端拦截，不能打开不应开始的滑块流程或发送云端请求。

## 当前与预期行为

### 登录

- 当前：Fixture 登录按钮可直接调用 `login`；真实登录按钮在缺少手机号或密码时被禁用，用户看不到具体原因。
- 预期：手机号、密码标签显示红色 `*`。按钮在非忙碌且滑块已就绪时可点击；点击后同时显示所有空字段错误、聚焦第一个空字段，并且不打开真实滑块、不调用 `CloudLogin`。

### 注册与短信

- 当前：注册按钮不检查三项输入；真实发送验证码按钮不检查手机号；Fixture 验证步骤不检查手机号。
- 预期：手机号、密码、短信验证码标签显示红色 `*`。注册空值时显示全部对应错误并阻止 `CloudRegister`；发送验证码前手机号为空时显示错误，不开始验证、不调用 `CloudSendRegisterSMS`。

### 模式切换

- 当前：顶部横幅来自账户 Store 的共享 `error`，登录/注册标签只调用 `setMode`，所以旧横幅跨模式保留。
- 预期：登录与注册双向切换时清除顶部错误横幅、字段错误和已通过的 Fixture CAPTCHA 状态；保留用户输入和短信重发倒计时。

### 云端账户业务错误

- 当前：`AccountCenter` 的错误提交与最终横幅展示都只提供通用 fallback，未利用 `clientError` 的 `knownMessages` 扩展点。
- 预期：已确认的普通用户账户错误码显示确定、可行动的中文横幅；未知码继续显示受控 fallback，同时保留错误码和请求 ID 技术详情，不展示未经控制的云端原始消息。

## 目标

- 在主登录/注册表单明确标识必填字段，并提供字段级中文错误和首错聚焦。
- 在所有登录、注册和发送验证码入口前完成一致的空值拦截。
- 切换认证模式时清除不再属于当前上下文的错误。
- 为普通用户登录、注册和短信请求提供账户模块局部的常见错误中文提示。
- 保持现有真实/Fixture CAPTCHA、云端错误、安全清理和倒计时行为不变。

## 非目标与边界

- 不增加格式校验，不修改云端 API、Wails bridge、Go 校验、错误码或服务端行为。
- 不修改账号切换表单及其校验。
- 不改变输入值的跨标签保留行为，也不重置短信重发倒计时。
- 不给“完成滑块验证”或“发送验证码”等操作标题加必填星号。
- 不把字段空值错误写入账户全局 Store 或遥测。
- 不修改全局 `clientError` 错误码表，不影响账户外页面。
- 不映射仅用于管理员登录的 `ACCOUNT_NOT_FOUND`、`INVALID_PASSWORD`，不通过提示区分普通账号是否存在。
- 不展示 `VALIDATION_FAILED` 或其他未知错误的云端原始消息。

## 用户场景与验收标准

1. 登录页手机号和密码为空时点击登录：
   - 同时显示“手机号不能为空”“密码不能为空”。
   - 聚焦手机号。
   - Fixture 模式不调用 `CloudLogin`；真实模式不打开滑块且不调用 `CloudLogin`。
2. 注册页三项为空时点击注册：
   - 同时显示三项“不能为空”。
   - 聚焦手机号。
   - 不调用 `CloudRegister`。
3. 注册页手机号为空时点击验证/发送验证码：
   - 显示“手机号不能为空”并聚焦手机号。
   - 不打开真实滑块，不推进 Fixture 验证，不调用 `CloudSendRegisterSMS`。
4. 用户为报错字段输入非空内容后，该字段错误立即消失；其他字段错误保留。
5. 登录失败出现顶部横幅后切换注册，横幅和字段错误消失；注册切回登录行为相同。
6. 手机号或短信验证码仅含空白时视为空；密码沿用后端现有语义，仅空字符串视为空。
7. 请求进行中、真实 CAPTCHA 配置加载中及短信重发倒计时的既有禁用规则不变。
8. 必填标识、错误文本和输入错误状态具备可访问性关联。
9. 已知账户错误码显示确定中文提示；未知码显示“云请求失败”，错误码和请求 ID 仍可在技术详情查看，原始云端消息不进入横幅。
10. 账户专属映射不改变其他页面使用的全局错误提示。

## 调查基线与 Codegraph 证据

- 主工作区：`D:\Reasonix`，分支 `main-v3`。
- Codegraph/计划基线：`be9085f6bd71524ea54639af368cb8e4bf530d7a`。
- Codegraph 精确查询：
  `AccountCenter.tsx AccountCenter login register sendSMS beginOperation commitAccountError setMode setError account-center__error AccountPasswordField RealCaptchaButton`。
- Codegraph 返回当前磁盘源码，并报告：
  - `AccountCenter` 有 6 个调用/导出消费者；相关测试包括 `account-center-registration`、`account-center-states`、`account-center-switch` 和支付集成测试。
  - `accountStore.setError` 有 11 个调用者，横跨多个账户 UI；因此不修改 Store 契约，只在 `AccountCenter` 的模式切换入口清理。
  - `AccountCenter` 实例化 `AliyunCaptchaAdapter`，真实登录和真实发送短信都经 `RealCaptchaButton`。
- 独立工作树从同一提交创建，`core.autocrlf=false`；以下结论均由工作树实际源码复核。

### 提交与错误链路

- `AccountCenter.tsx:425-444` 的 `sendSMS` 在 CAPTCHA 条件满足后立即 `beginOperation` 并调用 `CloudSendRegisterSMS`，没有手机号空值检查。
- `AccountCenter.tsx:463-486` 的 `login` 直接构造请求并调用 `CloudLogin`；真实模式只检查是否获得 proof。
- `AccountCenter.tsx:488-514` 的 `register` 直接构造三字段请求并调用 `CloudRegister`。
- `AccountCenter.tsx:336-355` 的 `beginOperation` 只在操作真正开始时清除全局错误；字段预检必须发生在它之前，才能保证空值不改变 busy 状态、不产生云端调用。
- Go 端 `internal/cloud/auth_application.go:189-194,220-222` 已有最终防线，但它返回聚合错误，无法提供字段级反馈；前端预检是体验层补充，不替代后端校验。

### 模式切换与横幅根因

- `AccountCenter.tsx:294,301,307` 分别读取共享 `error`、取得 `setError` 并维护本地 `mode`。
- `AccountCenter.tsx:841-867` 只要共享 `error` 非空就渲染顶部横幅。
- `AccountCenter.tsx:1059-1072` 的登录/注册按钮只调用 `setMode`，不清除 `error`，因此横幅跨模式保留。
- 操作期间模式按钮已由 `busy` 禁用，模式切换清错不会与进行中的认证请求竞争。

### 表单、CAPTCHA 与样式扩展点

- `AccountCenter.tsx:111-155` 的 `AccountPasswordField` 同时用于主认证和账号切换；新增属性必须可选，确保账号切换默认行为不变。
- `AccountCenter.tsx:1080-1148` 是主认证表单唯一渲染入口，适合集中加入必填标识、错误关联和预检入口。
- `AccountCenter.tsx:170-239` 的 `RealCaptchaButton` 把真实按钮交给 `AliyunCaptchaAdapter`。
- `aliyunCaptcha.ts:143-176` 显示适配器在按钮捕获阶段记录意图，并由 SDK 绑定同一按钮。真实模式的空值预检应通过 React 捕获阶段先阻止事件，再由既有适配器处理有效点击；无需修改 SDK 协议或 proof 生命周期。
- `features.css:454-478,695-732` 已集中定义认证字段、输入焦点和 CAPTCHA 布局，可局部增加必填、字段错误和 `aria-invalid` 视觉样式。

### 账户错误映射证据

- 工作树源码确认 `cloudWailsError` 保留安全 allowlist 错误码和 `request_id`，但 `AccountCenter` 的 `commitAccountError` 与 `visibleErrorMessage` 两次 presentation 都只传 `fallbackMessage`；任一处缺少同一 `knownMessages` 都会把已知码再次降级为通用 fallback。
- `desktop/frontend/src/lib/clientError.ts` 已支持调用方局部 `knownMessages`，且局部表优先于全局表；因此扩展点位于账户组件，无需改变其他页面。
- Reasonix-Cloud `main` 提交 `cdbebd3d8f1cf9f7bac45dac20aff6396f23dd59` 的 `internal/platform/errcode.go` 与 `internal/auth/handler.go` 对照确认：普通用户认证稳定返回本计划所列账户错误码；`ACCOUNT_NOT_FOUND`、`INVALID_PASSWORD` 仅用于 `handleAdminLogin`，普通桌面登录由 `failAuth` 统一为 `INVALID_CREDENTIALS`。
- 同一 Cloud 源码确认额外稳定普通用户错误码为 `SLIDER_CAPTCHA_EXPIRED`、`ACCOUNT_DISABLED`、`AUTH_CRYPTO_CONFIG_UNAVAILABLE`、`PASSWORD_DECRYPT_FAILED`；`VALIDATION_FAILED` 属于 JSON/通用输入校验，继续使用未知码 fallback。

## 选定方案

### 1. 本地字段错误模型

在 `AccountCenter` 内维护仅含 `phone`、`password`、`smsCode` 的本地错误状态和输入引用。提供两个纯前端预检：

- 认证提交预检：按当前模式检查登录两项或注册三项，一次返回全部错误并聚焦第一项。
- 发送短信预检：只检查手机号。

手机号和短信验证码使用 `trim()` 判断空值；密码使用原值判断，和 Go 端现有必填语义一致。同一校验既在真实 CAPTCHA 打开前执行，也在 `login`、`register`、`sendSMS` 处理函数调用 `beginOperation`/bridge 前再次执行，任一失败立即返回。

### 2. 可访问的字段展示

- 主认证手机号、密码、短信验证码标签追加红色、装饰性的 `*`。
- 输入增加 `required`、`aria-invalid` 和错误存在时的 `aria-describedby`。
- 字段错误紧邻输入显示，并使用稳定 ID 与输入关联。
- `AccountPasswordField` 通过可选属性接收主认证所需的必填、错误和 ref；账号切换调用不传这些属性。

### 3. 统一覆盖真实与 Fixture 入口

- Fixture 登录、注册及注册验证/发送短信入口先预检，再调用原函数或推进验证状态。
- `login`、`register`、`sendSMS` 均保留处理函数级防御性检查，防止真实 proof 回调或其他调用路径绕过。
- `RealCaptchaButton` 增加可选的点击前置检查，在捕获阶段阻止无效点击；真实登录检查手机号和密码，真实发送短信检查手机号。
- 有效输入继续进入原 CAPTCHA、proof、请求和重建路径，不改变敏感值生命周期。

### 4. 模式切换清理

封装登录/注册模式切换函数，统一执行：

- 切换 `mode`。
- `setError(null)` 清除顶部共享横幅。
- 清除本地字段错误。
- 清除 `captchaPassed`，避免 Fixture 验证状态跨模式复用。

保留手机号、密码、短信验证码输入和 `smsCooldownSeconds`。不修改账户 Store 结构，也不清除其他页面状态。

### 5. 账户专属错误映射

在 `AccountCenter` 内构造一份本地化 `knownMessages`，同时传给错误提交和最终横幅 presentation。映射覆盖普通用户登录、注册、滑块、短信、限流、网络及账号状态稳定码；未知码继续使用 `account.requestFailed`。横幅只消费受控中文文案，技术详情继续展示安全保留的 code/request ID。

- `INVALID_CREDENTIALS`：手机号未注册或密码错误。
- `SMS_CODE_INVALID`：短信验证码错误，请重新输入；`SMS_CODE_EXPIRED`：短信验证码已过期，请重新获取；`SMS_CODE_ATTEMPTS_EXCEEDED`：验证码错误次数过多，请重新获取。
- `PHONE_INVALID`：手机号格式不正确；`PHONE_ALREADY_REGISTERED`：该手机号已注册，请直接登录。
- `PASSWORD_INVALID`：密码格式不正确，请输入 8–20 个字符；`PASSWORD_DECRYPT_FAILED`：密码校验失败，请重新输入后再试。
- `SLIDER_CAPTCHA_REQUIRED`、`SLIDER_CAPTCHA_REUSED`：请重新完成滑块验证；`SLIDER_CAPTCHA_EXPIRED`：滑块验证已过期，请重新完成验证。
- `SMS_RATE_LIMITED`、`AUTH_RATE_LIMITED`：操作过于频繁，请稍后再试。
- `SMS_PROVIDER_UNAVAILABLE`：短信服务暂不可用，请稍后重试；`NETWORK_UNAVAILABLE`：网络连接失败，请检查网络后重试。
- `ACCOUNT_DISABLED`：账号已被禁用，请联系管理员；`AUTH_CRYPTO_CONFIG_UNAVAILABLE`：注册服务暂不可用，请稍后重试。

## 预计影响文件

- `desktop/frontend/src/features/account/ui/AccountCenter.tsx`
  - 字段错误、引用、预检、模式切换、必填标识和 CAPTCHA 前置拦截。
- `desktop/frontend/src/styles/features.css`
  - 必填星号、字段错误、无效输入样式。
- `desktop/frontend/src/locales/zh.ts`
  - 必填与账户常见错误中文文案。
- `desktop/frontend/src/__tests__/account-center-required-validation.test.tsx`（优先新建）
  - 独立覆盖必填、聚焦、请求拦截、真实 CAPTCHA 拦截、逐字段清错和模式切换。
- `desktop/frontend/src/__tests__/account-center-registration.test.tsx`
  - 保护真实 CAPTCHA 成功路径，并覆盖 structured object、Wails string、已知码、未知 fallback、技术详情与全局映射隔离。
- 本目录 `plan.md`、`progress.md` 与任务文档同步记录扩展范围和验收状态。

明确不修改 `accountStore.ts`、`aliyunCaptcha.ts`、bridge/Go 接口和账号切换生产逻辑。

## 兼容性、风险与控制

- **真实 CAPTCHA 事件顺序**：捕获阶段若处理不当可能仍弹出 SDK。用真实模式 SDK stub 证明无效点击不记录有效 intent、不提交 proof；有效点击仍只发一次请求。
- **共享横幅被清除**：切换模式会隐藏当前通用云端错误。该行为已由用户确认；下一次操作仍会重新产生真实错误，不改变账户状态。
- **账号切换误受影响**：密码字段组件被复用。新增属性保持可选，并用既有账号切换测试确认默认渲染和提交不变。
- **倒计时被绕过**：模式切换不清除 `smsCooldownSeconds`，发送短信仍保留 cooldown 判断。
- **浏览器原生校验干扰**：现有按钮均为 `type="button"`；`required` 仅提供语义，错误展示由受控逻辑完成。
- **敏感值处理回归**：不把错误写入全局 Store，不把密码或验证码放入错误消息；既有提交结束后清空密码逻辑保持。
- **视觉布局变化**：错误只在字段下方占位，使用现有 token；在账户页实际尺寸验证不遮挡按钮或溢出。
- **双重 presentation**：只在错误提交处映射会被最终渲染再次降级；两处复用同一映射并断言最终 `role=alert` 文本。
- **账号存在性泄露**：不得映射管理员专用的账号不存在/密码错误细分码；普通登录只使用统一 `INVALID_CREDENTIALS` 提示。
- **原始消息泄露**：未知码和通用校验错误始终使用受控 fallback；测试注入原始诊断文本并证明横幅不显示。
- 无 API、协议、持久化或数据迁移。

## 验证计划

### 聚焦自动化

- 运行新增必填校验测试，覆盖 8 个用户场景及 bridge 调用次数为零。
- 运行 `account-center-registration.test.tsx`，保护 Fixture/真实 CAPTCHA、短信 cooldown、注册和登录成功路径。
- 在账户注册测试中覆盖登录 `INVALID_CREDENTIALS`、注册短信码错误/过期、滑块过期、账号禁用、注册服务不可用、密码解密失败、短信限流/网络失败、未知码 fallback，以及 structured object/Wails string 的 code/request ID 保留。
- 直接调用全局 `clientErrorPresentation` 证明账户专属码在无局部表时仍走调用方 fallback。
- 运行 `account-center-password.test.tsx`，保护密码提交后清理和敏感错误过滤。
- 运行 `account-center-states.test.tsx` 与 `account-center-switch.test.tsx` 的受影响账户状态回归。
- 前端实现完成后运行 `pnpm build`；本改动不涉及 Go 文件，不运行 Go 测试。

### 应用验收

优先使用 Codex 内置浏览器打开桌面前端实际页面：

1. 分别在登录和注册页以全空、部分空输入点击提交，核对星号、全部错误、首错聚焦和无网络调用。
2. 注册页空手机号点击验证/发送短信，确认真实滑块不弹出。
3. 制造一次登录错误横幅，切换注册并确认横幅消失；反向重复。
4. 补齐字段后完成一次真实或最近似的 CAPTCHA/登录注册路径，确认正常请求未受影响。
5. 检查短信倒计时跨模式切换仍保留。

若真实 CAPTCHA 或账号凭据只能由用户完成，则提供最短步骤、预期结果和网络观察点，状态标记为等待应用验收。

### PR Gate

获得 PR 授权后，在同步最新 `origin/main-v3` 的最终树上运行仓库规定的通用检查和：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

前端分类应运行生产构建；不得因本改动运行无关 Go 套件。

## 回滚

回退本地字段错误、模式切换封装、样式和文案即可。没有数据、协议或持久状态需要恢复；后端既有必填防线仍在。
