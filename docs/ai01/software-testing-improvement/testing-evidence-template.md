# 测试证据记录模板

> 本模板用于自动化、人工矩阵、失败复现和发布候选复核。计划、命令超时、
> blocked、skipped 和 pass 必须分开记录；不得把重试成功覆盖首次失败。

## 任务与范围

- Task / Goal：
- Scenario ID：
- 风险级别：
- 受影响模块：
- 明确不在范围内：

## 来源与环境

- Source commit：
- Branch / worktree：
- Dirty worktree 摘要：
- OS build / architecture：
- CPU / RAM / VM：
- 用户权限：
- 网络 / proxy / CA：
- EDR / application control：
- WebView2 / Go / Node / pnpm：

## 工具与权限

- Tool / skill：
- 固定版本 / 来源 / hash：
- 可读目录：
- 可写目录：
- 网络访问：
- 凭据访问：无 / 测试凭据 / 其他（说明）
- 生产或客户数据：无 / 已脱敏（说明）

## 执行

### Command 1

```text
command
```

- 开始/结束时间：
- Exit code：
- 首次结果：pass / fail / blocked / timeout / skipped / not-run
- 失败分类：
- 原始证据路径：

### Retry 1

- 是否执行：
- 触发原因：
- 修改内容：
- 命令和 Exit code：
- 结果：
- 是否保留首次失败：是 / 否（否必须说明）

## 制品与敏感信息复核

- Installer / executable SHA-256：
- Signature / timestamp：
- SBOM / vulnerability DB snapshot：
- Trace / screenshot / log / dump：
- 脱敏检查：
- 保留期限：

## 结论

- Final result：pass / fail / blocked / partial / not-run
- 已证明：
- 未证明：
- Known gaps / skipped checks：
- Issue / owner / due date：
- Human reviewer：
- Decision / ADR：
