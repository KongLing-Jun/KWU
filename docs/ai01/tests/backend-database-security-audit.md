# 后端、数据库与安全测试

## 接口与数据链路

静态盘点得到：

- Wails `App` 导出方法：302 个。
- 前端测试文件：129 个。
- Go 测试文件：572 个。

主要业务链路：

```text
React 页面/组件
  -> desktop/frontend/src/lib/bridge.ts
  -> Wails App 导出方法
  -> desktop 控制器与领域包
  -> JSONL / SQLite / 本地文件 / 外部供应商
  -> agent:event 等事件
  -> React 状态与页面渲染
```

覆盖领域包括：会话与消息、自动化、专家、技能/MCP、工作空间与文件、账户/云端、套餐与支付、模型与计费、Bot 渠道、设置、更新和运行时控制。

## 自动化结果

| 命令 | 结果 | 范围 |
| --- | --- | --- |
| 根目录 `go test ./... -count=1` | PASS | 所有根模块包 |
| `desktop/go test ./... -count=1` | PASS | 桌面 Wails、签名、更新 |
| 根目录 `go vet ./...` | PASS | 根模块静态检查 |
| `desktop/go vet ./...` | PASS | 桌面模块静态检查 |
| `go run ./scripts/code_guard` | PASS | 代码守卫；提示有可 ratchet 的旧基线变化，本轮未修改基线 |
| 关键包 `go test -race ...` | PASS | cloud、automation、modelbilling、bot、store、history |
| `pnpm audit --audit-level high` | PASS | 未发现已知高危依赖漏洞 |

race 命令覆盖：

```text
./internal/cloud
./internal/automation
./internal/modelbilling
./internal/bot/...
./internal/store
./internal/history
```

## 数据库与持久化

测试配置目录：

`D:\project\Reasonix-dev-profiles\integration-main-v3-20260724-win11-04`

所有数据库均以只读方式执行 `PRAGMA integrity_check`。

| 数据库 | 完整性 | 表与记录摘要 |
| --- | --- | --- |
| `experts.sqlite` | `ok` | `experts=24` |
| `model-billing.sqlite` | `ok` | 迁移记录 6；当前无模型调用/任务/结算记录 |
| `automation/templates.sqlite` | `ok` | 迁移记录 1；当前模板记录 0 |

当前隔离配置的 `sessions` 目录只有迁移标记，没有聊天 JSONL，因此本轮不能验证一次真实聊天写入后的 JSONL 追加与重载。现有 Go/前端测试覆盖了会话保存、列表、事件和恢复契约，但不等同于这次运行态的真实模型对话。

## 安全检查

已验证：

- 前端账户测试覆盖密码、短信 proof、CAPTCHA proof 不进入 Zustand/localStorage，并在完成或失败后清理。
- 支付测试覆盖重复下单、确认中阻止新购、轮询终止/退避、订单 scope 隔离。
- cloud、bot、automation、modelbilling、store、history 关键包通过 race 检查。
- Go 静态检查与代码守卫通过。
- 前端高危依赖审计无已知漏洞。
- 数据库以只读方式检查，无完整性错误。

未验证为真实外部成功：

- 云端鉴权、短信供应商、CAPTCHA 服务。
- 支付供应商创建订单与支付确认。
- QQ/飞书/Lark/微信联网和消息交换。
- 模型供应商请求、真实 token/费用结算。
- 自动化任务对外发送、文件写入与系统命令执行。

原因是缺少专用沙箱账号/渠道，且这些操作会发送数据、创建订单或改变外部状态。本轮以接口、状态机、错误路径、并发和存储测试替代，残余风险保留。

## 代码审查结论

`HEAD` 与 `origin/main-v3` 没有代码差异，因此按差异审查技能没有可审查 patch，也没有生成“标准/规格”双轴变更结论。本轮对基线执行的是运行测试、静态链路核对与安全/质量检查。

除 FE-QA-001 外，本轮没有形成具备复现步骤和影响证据的后端、数据库或安全缺陷。未执行的真实外部链路明确列为覆盖缺口，不能解读为已通过。
