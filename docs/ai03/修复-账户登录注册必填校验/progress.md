# 账户登录注册必填校验进度

## 当前阶段

- 阶段：Stage 3，扩展实施。
- 状态：必填校验与账户常见错误中文映射均已完成主审、自动化验证、主会话独立应用验收和用户真实应用验收，等待实现 PR 授权。
- 工作分支：`codex/feat-account-required-validation`。
- 工作树：`D:\Reasonix-worktrees\account-required-validation-impl`，`core.autocrlf=false`。

## 已确认决策

- 必填校验、真实 CAPTCHA 前置拦截、模式切换清理及输入/倒计时保留行为维持已验收结果。
- 已知普通用户账户错误码使用账户模块局部中文映射；未知码继续显示 `account.requestFailed`。
- 错误提交和最终横幅两次 presentation 复用同一 `knownMessages`。
- 保留安全 code/request ID 技术详情，不展示未经控制的云端原始消息。
- 不修改全局 `clientError` 表，不影响账户外页面。
- 普通登录只映射统一 `INVALID_CREDENTIALS`；不加入管理员专用 `ACCOUNT_NOT_FOUND`、`INVALID_PASSWORD`。

## 调查与证据

- 工作树源码确认 `commitAccountError` 与 `visibleErrorMessage` 是两个独立 presentation 点，必须同时传入同一账户映射。
- `clientError.ts` 已提供局部 `knownMessages` 扩展点，优先级高于全局表。
- 桌面 Wails 层保留安全 allowlist code/request ID，structured object 与 Wails string 都可进入现有解析链路。
- Reasonix-Cloud `main` 提交 `cdbebd3d8f1cf9f7bac45dac20aff6396f23dd59` 的 `internal/platform/errcode.go`、`internal/auth/handler.go` 证明普通认证稳定码范围，以及管理员细分码与普通 `INVALID_CREDENTIALS` 的边界。

## 交付状态

- 方案 PR #263 与任务拆解 PR #264 已合并。
- 必填校验实现、测试、应用验收及用户反馈回归已完成。
- 账户常见错误局部映射、中文文案和账户测试已在本分支完成。
- `plan.md`、本进度快照和任务合同同步扩展范围。
- 尚未推送或创建/更新实现 PR。

## 验证状态

- 既有必填校验聚焦测试最近结果：32 passed / 0 failed。
- 主会话独立复跑账户注册测试：89 passed / 0 failed，逐项覆盖全部账户局部映射，并覆盖最终横幅、structured object、Wails string、未知 fallback、技术详情和全局映射隔离。
- 账户密码、状态和切换测试分别为 17、98、44 passed / 0 failed；全局 `client-error` 测试 10 passed / 0 failed。
- 主会话独立复跑 `pnpm build` 通过；CSS、z-index token、TypeScript 和 Vite 生产构建均成功，仅有既存 bundle 提示。
- 内置浏览器在 `http://127.0.0.1:5293/` 验收真实渲染通过：`PASSWORD_INVALID` 显示确定中文提示和错误码；使用错误短信码直连云平台真实得到 `SMS_CODE_EXPIRED`，页面显示确定中文提示并保留请求 ID。
- 2026-07-31，用户在当前工作树启动的真实 Wails 应用中完成交付清单并反馈全部通过；清单覆盖登录/注册必填状态、注册缺少密码、普通登录凭据错误、短信验证码错误、双向切换清除横幅、技术详情安全性和成功登录。

## 风险与阻塞

- 当前无技术阻塞。
- 用户已完成真实 CAPTCHA 参与的失败与成功登录验收并反馈通过；具有真实账号创建副作用的成功注册仅在有专用测试手机号时选测，不作为本缺陷验收阻塞。
- 真实短信 cooldown 仍由 SDK stub/Fixture 自动化覆盖，未单独进行限流压力测试。
- 未知码必须保持受控 fallback；管理员专用错误码不得用于普通登录提示。

## 下一道门

1. 获得实现 PR 授权后同步最新基线、复验关键路径并运行最终 PR Gate。

## 剩余工作

- 等待实现 PR 授权。
