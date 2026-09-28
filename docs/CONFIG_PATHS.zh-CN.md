# 配置路径

从 **云智企办 v1.8.1** 开始，云智企办使用一个用户可见的全局目录存放配置和用户状态。CLI 与桌面端共用这个目录。

## Clouvia Home

| 平台 | Clouvia home |
| --- | --- |
| macOS | `~/.clouvia` |
| Linux | `~/.clouvia` |
| Windows | `%APPDATA%\Clouvia` |

可以设置 `CLOUVIA_HOME` 覆盖 Clouvia home，主要用于测试、CI 或便携安装。`KWU_HOME`、`SUIYUAN_HOME` 和 `REASONIX_HOME` 仅作为旧版兼容别名继续识别，普通用户通常不需要设置。

## 目录内容

| 数据 | 路径 |
| --- | --- |
| 全局配置 | `<Clouvia home>/config.toml` |
| 全局 provider 凭据 | `<Clouvia home>/.env` |
| 旧 credentials 导入来源 | `<Clouvia home>/credentials` |
| 全局斜杠命令 | `<Clouvia home>/commands/` |
| 全局 skills | `<Clouvia home>/skills/` |
| 项目本地 MCP 信任状态 | `<Clouvia home>/trust.json` |
| 会话 | `<Clouvia home>/sessions/` |
| 归档 | `<Clouvia home>/archive/` |
| 记忆 | `<Clouvia home>/memory/` 与 `<Clouvia home>/projects/` |

全局用户配置文件名是 `config.toml`。新项目本地配置使用 `clouvia.toml`，已有的 `kwu.toml`、`suiyuan.toml` 和 `reasonix.toml` 继续兼容读取。
如果有人说“全局 kwu.toml”，通常指的是 `<Clouvia home>/config.toml`。

## 全局 `config.toml`

`<Clouvia home>/config.toml` 存放 CLI 与桌面端共用的非密钥配置。它可以包含云智企办写入用户配置的 provider、plugin、UI、desktop、tool、skill、sandbox、
bot 和 agent 设置。Provider 条目只保存 `api_key_env` 里的凭据变量名，不保存真实密钥值。

示例：

```toml
config_version = 1
default_model = "deepseek/deepseek-v4-flash"
language = "zh"
credentials_store = "auto"   # 旧兼容字段；provider key 保存在 .env

[ui]
theme = "auto"

[desktop]
provider_access = ["deepseek"]

[agent]
auto_plan = "off"
max_steps = 0

[[providers]]
name        = "deepseek"
kind        = "openai"
base_url    = "https://api.deepseek.com"
models      = ["deepseek-v4-flash", "deepseek-v4-pro"]
default     = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[plugins]]
name    = "example"
command = "example-mcp-server"
```

不要把 API key 的真实值写进 `config.toml`。这个文件是普通配置：可以查看、编辑、
迁移，也可以在常规脱敏后用于诊断。密钥值属于下面的全局 `.env`。

## 全局 `.env`

`<Clouvia home>/.env` 是云智企办保存的 provider API key 的唯一运行时来源。
setup 向导、桌面端设置页、CLI 缺 key 提示以及删除 provider key 的操作，都会通过同一套凭据 helper 读写这个文件。

结构：

```dotenv
DEEPSEEK_API_KEY=sk-...
GEMINI_API_KEY=...
ANTHROPIC_API_KEY=...
# kwu-cleared OLD_API_KEY
```

规则：

- 每行一个 `KEY=value`；
- 空行和 `#` 注释会被忽略；
- 读取时接受 `export KEY=value` 和带引号的值；
- 云智企办写入时会拒绝多行值；
- key 必须是类似 `DEEPSEEK_API_KEY` 的 shell 风格变量名；
- `# kwu-cleared KEY` 是删除 key 后写入的非密钥标记，用来防止旧存储把它静默迁回；
- 在操作系统支持的情况下，云智企办会用受限权限写入该文件。

Provider 请求只会从这个全局 `.env` 解析 key。项目 `.env`、home `.env`、继承的 shell
环境变量、旧 `credentials` 文件和系统 keyring 都不再作为运行时 provider key fallback。项目 `.env`、home `.env` 和继承的 shell 环境变量不会自动导入到全局凭据文件。
旧 `credentials` 文件和旧 keyring 条目只会在新全局 `.env` 缺少对应 key 时作为非破坏性迁移来源读取。
项目 `.env` 仍会作为当前 workspace 范围内的非 provider 变量展开来源，例如 MCP/plugin 的 env、headers、URL、command 和 args 中的 `${VAR}`；这些值不会写入进程环境，`CLOUVIA_HOME`、`CLOUVIA_STATE_HOME`、`KWU_HOME`、`KWU_STATE_HOME`、`XDG_CONFIG_HOME` 等云智企办控制变量也会被忽略。

缓存仍放在系统缓存目录，例如 macOS 的 `~/Library/Caches/Clouvia`、
Linux 的 `$XDG_CACHE_HOME/Clouvia` 或 `~/.cache/Clouvia`、Windows 的
`%LOCALAPPDATA%\Clouvia`。可以设置 `CLOUVIA_CACHE_HOME` 覆盖缓存根目录；旧的 `KWU_CACHE_HOME`、`SUIYUAN_CACHE_HOME` 和 `REASONIX_CACHE_HOME` 继续作为兼容别名。

## 配置优先级

运行时配置按下面顺序解析：

```text
命令行参数
> 项目 ./clouvia.toml（或识别到的旧版项目配置名）
> 全局 <Clouvia home>/config.toml
> 兼容读取的旧全局配置
> 内置默认值
```

写配置时始终写入新的全局路径：

```text
macOS/Linux: ~/.clouvia/config.toml
Windows:     %APPDATA%\Clouvia\config.toml
```

## 旧路径迁移

从 **v1.8.1** 开始，云智企办启动时会在第一次加载配置前自动检查旧路径。迁移是同步、一次性、非破坏性的：旧文件会被复制或转换到云智企办 home，原文件保留。

旧配置来源包括：

```text
~/Library/Application Support/kwu/config.toml
~/.config/kwu/config.toml
~/.kwu/kwu.toml
~/.kwu/config.json
```

旧 credentials、memory 文件和 sessions 也会在新目标不存在时导入到 Clouvia home。
旧 provider key 只会在 `<Clouvia home>/.env` 尚未包含同名 key 时复制进去。若新的全局配置已经存在，则新配置优先；旧配置只作为兼容 fallback 保留。

从 **v1.9.1** 开始，云智企办还会在升级时把已知旧路径、legacy `config.json`、
桌面端已登记项目和恢复 tabs 对应项目里的 MCP 配置汇总补齐到全局
`<Clouvia home>/config.toml`。已有的全局 `[[plugins]]` 按名称优先，不会被旧
配置或项目配置覆盖；源文件会保留不变。该补齐会写入一次性 marker，避免用户之后
主动删除某个全局 MCP 时又被旧项目配置反复恢复。

## 手动补救迁移

如果云智企办已经创建了新的 home 目录，但当时旧数据还不在可扫描路径里；或者先打开了桌面端，导致自动迁移没有把旧路径数据补齐，可以在任一前端运行补救命令：

```text
/migrate
```

在 CLI TUI 中，把 `/migrate` 输入到聊天输入框。在桌面端中，把同一个命令输入到 composer。命令会显示进度提示：

1. 检查旧配置和 credentials；
2. 扫描已知旧 memory 位置；
3. 扫描已知旧 sessions 目录；
4. 导入尚未迁移过的 memory 文件和 sessions；
5. 输出最终汇总。

该补救命令仍然是非破坏性的。它不会覆盖已有的
`<Clouvia home>/config.toml`；如果新配置已经存在，需要手动把旧配置里缺失的设置复制过去。旧 memory 文件只会在目标文件不存在时复制。它也会尊重 session 导入 marker，因此已经迁移过、之后又被用户删除的会话，不会在后续 `/migrate` 中被重新恢复。

版本限制：

- 自动迁移从 **v1.8.1** 开始。
- `/migrate` 只存在于包含该命令的 Go 版云智企办构建中。如果云智企办提示 `unknown command`，请先升级后再运行。
- legacy `0.x` TypeScript 线没有这个命令。
- 它只会重新扫描上面列出的旧路径；它不是备份恢复工具、降级导入工具，也不是任意目录导入器。
