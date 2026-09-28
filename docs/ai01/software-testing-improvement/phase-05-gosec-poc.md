# Phase 5 gosec 报告模式 PoC

> 执行日期：2026-07-23
> 工具：`github.com/securego/gosec/v2/cmd/gosec@v2.28.0`
> 版本证据：Go module proxy 返回发布时间 `2026-07-13T08:40:30Z`，
> module `go 1.25.8`。
> 边界：只扫描和分级，不修改生产实现、不添加 suppress、不把数量当作漏洞数量。

## 1. 执行结果

使用 `-quiet -no-fail -fmt=json` 捕获 JSON，再由 PowerShell 解析。`-no-fail`
只用于报告 PoC 继续生成汇总，不会把发现解释成安全通过。

| Module | 文件/行 | 总发现 | High | Medium | Low |
|---|---:|---:|---:|---:|---:|
| Root | 490 / 149,929 | 393 | 53 | 249 | 91 |
| Desktop | 54 / 32,652 | 153 | 22 | 100 | 31 |

Root 数量最多的规则：

| Rule | 数量 | 初筛 |
|---|---:|---|
| G304 路径输入 | 112 | 需要结合现有 confinement/allowlist 逐路径审查 |
| G104 未检查错误 | 85 | Low；区分明确丢弃与资源/安全失败 |
| G301 目录权限 | 53 | Windows 与 Unix 权限语义需分开 |
| G204 命令执行 | 38 | Agent 工具本身需要执行命令，检查授权和参数边界 |
| G306 文件权限 | 29 | 结合客户数据和本地多用户模型 |
| G101 硬编码凭据 | 25 | High severity/Low confidence，大量为 URL、环境变量名和普通常量 |
| G115 整数转换 | 13 | High/Medium confidence，需边界测试 |
| G703 路径穿越 taint | 9 | High/High confidence，需逐入口复核 |

Desktop 主要为 G304 41、G301 29、G104 26、G306 20、G703 11；
另有 G704 SSRF 2、G702 命令注入 1 和 G115 整数转换 3。

## 2. 需要独立跟进的候选

### GOSEC-TEST-001：Skill 复制存在 TOCTOU 候选

- 规则：G122，High/Medium。
- 位置：`internal/installsource/skill.go:384`。
- 证据：`filepath.WalkDir` callback 随后按路径 `os.Open(path)`；目录项检查和
  打开之间可能被符号链接替换。
- 风险边界：install source 属于外部/用户可控输入时，该路径比普通本地工具
  路径更敏感。
- 处理：生产修复不在本测试任务授权内；建议以并发替换 fixture 先建立回归测试，
  再评估 `os.Root`/handle-relative 打开。

### GOSEC-TEST-002：Unix EDITOR 通过 shell 拼接

- 规则：G702，High/High。
- 位置：`internal/cli/mcp_manager_actions.go:422`。
- 证据：非 Windows 路径执行
  `exec.Command("sh", "-lc", editor+" "+shellQuote(path))`。
- 初筛：`path` 被 quote，但 `EDITOR` 整体作为 shell 程序文本执行。若环境变量
  只由当前用户控制，这是预期能力；若由不可信启动配置注入，则可执行任意命令。
- 处理：需要明确 `EDITOR` 信任模型和 Linux 测试；不得直接 suppress。

### GOSEC-TEST-003：Updater URL 的 SSRF 候选

- 规则：G704，High/High，两条。
- 位置：`desktop/updater.go:677`、`:681`。
- 证据：URL 进入 `http.NewRequestWithContext` 和 `Client.Do`。
- 初筛：是否可利用取决于 URL 是否只能来自已验证 manifest/固定更新源，以及
  redirect、DNS 和私网地址限制；当前静态结果无法证明这些前置条件。
- 处理：需要沿调用链建立恶意 manifest、redirect、loopback/private IP 测试。

### GOSEC-TEST-004：无 timeout 的 HTTP server

- 规则：G114，Medium/High。
- 位置：`internal/serve/run.go:18`。
- 证据：直接调用 `http.ListenAndServe`，无法设置
  `ReadHeaderTimeout`/`ReadTimeout`/`IdleTimeout`。
- 影响：非可信网络可达时存在慢请求资源耗尽风险。
- 处理：确认默认 bind 和部署边界后补本地慢请求测试；生产修改另行授权。

### GOSEC-TEST-005：整数转换边界

- 规则：G115，Root 13 条、Desktop 3 条，High/Medium。
- 代表位置：
  - `internal/fileextract/office/extract.go:148`，`uint64 -> int64`；
  - `internal/cloudquota/client.go:245`，`uint64 -> int64`；
  - 多个 model-billing DB `int64 -> uint64`；
  - `internal/proc/kill_windows.go:52/74`，PID `int -> uint32`；
  - `desktop/devinfo.go:26`，`uint64 -> int`。
- 处理：按来源分别证明上界/非负不变量并补边界测试，不进行全局机械 cast。

### GOSEC-TEST-006：高置信路径输入簇

- 规则：G703，Root 9 条、Desktop 11 条。
- 范围包括 builtin 文件工具、memory、atomic write、e2ebench 以及
  `desktop/cmd/sign`。
- 初筛：部分路径在上游已有 workspace confinement 或属于签名 CLI 显式参数，
  但单个 sink 的高置信 taint 不能靠整体架构假设关闭。
- 处理：按入口到 sink 的真实 allowlist/root 检查逐条审阅；签名工具应优先。

## 3. 误报或重复候选

以下只能标为候选，不能批量 suppress：

- G101 的 25/5 条多为 `APIKeyEnv`、URL、字段名和普通文本，置信度均为 Low；
  没有从这些样例发现真实密钥。
- G404 命中 `internal/provider/retry.go:151` 的 `math/rand`，用途是 retry jitter，
  不是密钥、nonce 或 token。
- G407 命中 `TestPasswordEncryptor` 的固定 IV；代码注释和生产 factory 表明它
  是 fixture seam，生产 encryptor 当前 fail closed。但该类型仍编译且导出，
  应保持“误用风险 review”而非直接忽略。
- G124 三条 cookie 报告中，调用点已设置 `HttpOnly`/`SameSite`，helper 根据
  TLS/loopback 设置 `Secure`，且已有 CodeQL 定向说明；属于 analyzer 跨函数
  数据流不足的强误报候选。
- Desktop G702 命中 `exec.Command(explorer, "/select,", path)`；参数没有经过
  shell，直接命令注入可能性低，但 `SystemRoot`/`windir` 的信任边界仍需审查。
- G118 命中的 goroutine 正在等待同一 `ctx.Done()`，随后用独立 background
  context 执行 shutdown；更像资源清理设计，而不是忽略 request context。

CodeQL 的 security-and-quality 查询与命令注入、路径、cookie 等类别存在潜在
重叠；本轮没有可比较的同 SHA CodeQL SARIF，因此只记录“潜在重复”，不声称
已经去重。G115、Windows 权限语义和 Agent 工具信任边界仍体现 gosec PoC 的
独立价值。

## 4. 首次失败与复核

1. 首次 `go run ... -out=output/gosec/root.json` 完成扫描并返回发现退出码 1，
   但没有生成 JSON 文件。
2. 改为固定版本临时二进制后，`-out` 仍未落盘。
3. 最终使用 stdout JSON + `-no-fail` 报告模式成功解析 Root/Desktop 汇总。

前两次属于报告归档失败，不记为扫描通过；没有通过删除规则、降低 severity、
加入 `#nosec` 或跳过目录解决。

## 5. 结论

T5.4 的固定版本报告模式 PoC 和首轮误报/重叠审查已完成。当前 393/153 是
需要分流的 analyzer finding 数，不是确认漏洞数，也不能直接作为 required
gate 阈值。上述六个候选组在生产修复或逐项风险关闭前应保持可见。
