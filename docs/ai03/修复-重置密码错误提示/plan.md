# 重置密码错误提示修复方案

## 背景与问题

重置密码页在两次新密码不一致时显示“云请求失败”，技术详情为
`PASSWORD_MISMATCH`。这会让用户误以为云平台故障，无法知道应修改哪项输入。

当前前端实际已在云请求前校验密码长度、两次密码一致性和短信验证码格式；
问题发生在统一安全错误展示阶段：本地 `PASSWORD_MISMATCH` 未进入已知码映射，
明确的本地文案被受控 fallback 覆盖。

## 当前行为与预期

- 当前：密码不一致不调用云端，但顶部横幅显示“云请求失败”。
- 预期：顶部横幅显示“两次输入的新密码不一致”，仍不调用云端。
- 当前：重置接口返回 `INVALID_CREDENTIALS` 时复用登录文案“手机号未注册或密码错误”。
- 预期：显示“无法完成密码重置，请确认手机号后重试”，不暴露手机号是否注册或账号角色。
- 其他验证码、滑块、限流、网络错误继续复用已验收的账户中文映射。
- 未知码继续使用安全 fallback，不展示云端原始消息。
- 当前：验证码、新密码和确认新密码没有必填标识；按钮在字段为空时直接禁用，
  用户看不到缺少哪一项。
- 预期：三个字段显示红色必填标识；点击确认后在缺失字段下方显示明确错误，
  聚焦第一个缺失项并阻止云请求。顶部横幅仍只显示格式和业务错误。

## 目标与边界

- 为重置密码模式增加局部、上下文相关的错误映射。
- 保持登录、注册和全局 `clientError` 映射不变。
- 保留安全错误码和 request ID 技术详情。
- 仅为重置密码页复用现有字段级错误和必填样式；不改变登录、注册行为。
- 不新增通用 UI 组件、依赖、后端接口或持久化结构。
- 不修改 CAPTCHA、短信 cooldown、密码清理和成功重置流程。

## 调查基线与证据

- KWU/Codegraph 基线：`10c4581d44f787fd87ee9d612faaf02dce022750`。
- Codegraph 确认 `AccountCenter` 是账户页唯一实现入口，直接测试集中在
  `account-center-registration.test.tsx`；变更影响登录、注册、重置共用的错误展示点，
  因此映射必须按当前模式选择。
- Codegraph 与工作树源码确认登录/注册已使用 `AccountPasswordField.required`、
  `.account-center__required`、字段下方错误和首个缺失项聚焦；重置页可局部复用这些
  能力，无需新增展示体系。
- 工作树源码 `AccountCenter.tsx:596-609` 证明本地校验在
  `CloudResetPassword` 之前返回；`PASSWORD_MISMATCH` 不是该路径的云端响应。
- `AccountCenter.tsx:417-424,840-845` 证明错误提交和最终横幅各执行一次安全 presentation，
  两处必须使用同一模式映射。
- Reasonix-Cloud `main@6e216d7d0d6cb930295d045c402a9b73ca83780b`
  的 `docs/api/auth.md`、`internal/auth/handler.go` 和 `internal/auth/service.go`
  证明密码重置稳定错误码不包含 `PASSWORD_MISMATCH`；未知手机号或非普通用户统一为
  `INVALID_CREDENTIALS`，不能在客户端细分。

## 方案

1. 在已有账户映射上派生重置密码专属映射。
2. 覆盖 `PASSWORD_MISMATCH`、`PASSWORD_INVALID` 和 `INVALID_CREDENTIALS`。
3. 错误提交和最终横幅根据 `mode` 复用同一映射对象。
4. 为验证码、新密码和确认新密码补齐必填标识、HTML 必填语义、字段错误与焦点；
   空值校验通过前不调用 `CloudResetPassword`。
5. 增加重置密码回归测试，覆盖空值、本地拦截、稳定云码、原始消息隔离和既有成功路径。

## 影响文件

- `desktop/frontend/src/features/account/ui/AccountCenter.tsx`
- `desktop/frontend/src/locales/zh.ts`
- `desktop/frontend/src/__tests__/account-center-registration.test.tsx`
- 本目录方案、进度和任务文档

## 验收标准

- 两次新密码不一致时显示明确中文，`CloudResetPassword` 调用为零。
- 验证码、新密码和确认新密码均显示红色 `*` 并暴露 HTML `required` 语义。
- 点击空表单时三个缺失项分别在字段下方显示“不能为空”，焦点落在短信验证码，
  顶部不新增空值横幅，`CloudResetPassword` 调用为零。
- 云端 `PASSWORD_INVALID` 使用“新密码”文案。
- 重置场景 `INVALID_CREDENTIALS` 使用不枚举账号状态的文案，不复用登录提示。
- 短信验证码错误继续显示已有确定中文。
- 所有已知和未知错误均不展示未经控制的云端原始消息。
- 登录和注册错误文案不变；密码重置成功流程不回归。

## 验证与应用测试

- 聚焦运行 `account-center-registration.test.tsx` 和 `client-error.test.ts`。
- 运行 `pnpm build`。
- 在真实 Wails 应用验证必填标识、空值字段错误和密码不一致横幅且不触发云请求；
  选测错误/过期验证码。
- 成功重置会修改真实密码并撤销会话，仅在专用测试账号下选测，不作为本修复阻塞项。

## 风险与回滚

- 风险低：仅改变重置密码页必填交互、账户页错误展示映射和中文文案。
- 主要回归风险是模式选择错误导致登录/注册文案被覆盖，由跨模式测试锁定。
- 无数据迁移；可整体回退本 PR 恢复原行为。
