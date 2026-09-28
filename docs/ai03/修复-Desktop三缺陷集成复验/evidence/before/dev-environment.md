# 持久化 dev 环境与连接确认

- 记录时间：`2026-08-12T15:04:28Z`。
- dev environment id：`a6defe83d02e4e84`。
- Desktop binary：`desktop/build/bin/kwu-desktop.exe`；SHA-256
  `FC1FA59E7927B6C95723021F57F2B214E5AF4D093113DB0B40D538FD618F0A86`。
- 启动方式：不设置 `KWU_DEV_PROFILE_ROOT`，直接进入持久化 dev 环境。
- 启动前可见历史：190 个任务、8 个 workspace；保留记录，不纳入本轮样本。
- 用户本轮重新确认：自定义 Provider 服务根地址、API Key、模型 `gpt-5.6-luna`。
- Reasonix 有效 OpenAI-compatible base：`http://192.168.10.231:8080/v1`。源码合同在 base 后追加
  `/chat/completions`，因此用户确认的服务根地址需使用标准 `/v1` API base；旧候选也使用同一路径口径。
- API Key：已由 Computer Use 在 Provider 配置页输入并覆盖保存；本文和所有证据均不记录值。
- 连通性：Desktop 的“测试并获取模型”成功返回 20 个模型；13 个启用模型保存成功，包含
  `gpt-5.6-luna`。这是 `/models` 请求，不是模型生成调用。
- 默认模型：`gpt-5.6-luna`，Provider 卡片显示已设密钥。
- 子代理 effort：Low；旧候选可比 child meta 同为 low。
- execution/planning steps：不限；默认审批：询问；auto plan：关闭；冷启动精简：关闭。
- 配置完成时模型生成请求：0。

一次性隔离 Profile 启动发生在用户明确要求 dev 环境之前，只验证了 Desktop 启动和设置入口；未输入凭据、
未发起 Provider 或模型请求，之后已终止。其文件保留但标记为 abandoned，不计入 30 次矩阵。
