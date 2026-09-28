<p align="center">
  <img src="docs/logo.svg" alt="云智企办 · Clouvia" width="640"/>
</p>

> [!IMPORTANT]
> **云智企办（Clouvia）1.0 是用 Go 从零重写的版本** —— 本分支（`main-v3`）已是新的默认分支，后续开发都在这里。
> 早期的 `0.x` TypeScript 版本转为 **legacy**，保留在 `v1` 分支（仅维护）。
> 详见 **[迁移指南](./docs/MIGRATING.md)**。桌面版本通过项目发布流程维护，版本说明见 **[桌面版本记录](./docs/releases/README.md)**；CLI 也可以从源码构建。

<p align="center">
  <img src="https://img.shields.io/badge/project-Clouvia-58a6ff.svg?style=flat-square&labelColor=161b22" alt="Clouvia"/>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-8b949e.svg?style=flat-square&labelColor=161b22" alt="license"/></a>
</p>

<br/>

<h3 align="center">本地优先的桌面 Agent OS 与智能工作台。</h3>
<p align="center">以 Go Agent 核心连接桌面、CLI、HTTP/ACP 与 IM Bot，通过模型、技能、MCP、自动化和专家协作完成办公与开发任务。</p>

<br/>

> [!IMPORTANT]
> **加入社区 · Community** — 双语 Discord，提供安装答疑（`#help` / `#求助`）、工作流展示与功能想法。→ **<https://discord.gg/XF78rEME2D>**

## 特性

- **配置驱动**：provider、agent、启用的工具、插件全部在 `clouvia.toml` 中声明，
  内核无硬编码模型。
- **多模型 · 可组合**：DeepSeek 作为预设内置；任何 OpenAI 兼容
  端点都只是一条配置。可选让两个模型协同（执行器 + 规划器），各自独立、缓存稳定的 session。
- **插件驱动**：外部工具以子进程形式运行，通过 stdio JSON-RPC 通信（MCP 兼容）；
  内置工具在编译期自注册。
- **多形态交付**：CLI 核心可构建为 `CGO_ENABLED=0` 单二进制；桌面端采用 Wails + React。
  项目依赖以根目录和 `desktop/` 两个 Go module 及前端 lockfile 为准。

## 安装

桌面端安装包由项目发布流程生成，并支持应用内检查更新；具体版本内容见
[`docs/releases/`](./docs/releases/README.md)。CLI 可从源码构建：

```sh
make build      # -> bin/kwu(.exe)
```

### 代码签名

Windows 构建使用 [SignPath 基金会](https://signpath.org/) 提供的免费代码签名证书，
通过 [SignPath.io](https://signpath.io/) 完成签名。

### 从源码构建

```sh
make build      # -> bin/kwu(.exe)
make cross      # -> dist/（darwin|linux|windows × amd64|arm64）
```

## 快速开始

```sh
kwu setup                      # 配置向导 → 当前配置档案的用户配置和凭据
kwu setup --local              # 可选的项目配置 → ./clouvia.toml；凭据仍保存在配置档案中
kwu                            # 然后在会话里运行 /init 生成 AGENTS.md（项目记忆）
kwu run "把 main.go 里的 TODO 实现掉"
kwu run --model deepseek-pro "给这个函数补单元测试"
echo "解释这段代码" | kwu run
```

## 配置

一个最小的项目 `clouvia.toml`——一个 provider 加一个默认模型——就够跑起来：

```toml
default_model = "deepseek-flash"

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
```

优先级为 **flag > `./clouvia.toml` > 当前配置档案的用户配置文件 > 内置默认值**。
新安装的用户配置位于 macOS/Linux 的 `~/.clouvia/config.toml`，
Windows 为 `%AppData%\Clouvia\config.toml`。迁移细节见
**[配置路径](./docs/CONFIG_PATHS.zh-CN.md)**，其中也说明了全局 `config.toml`
和 `.env` 的完整结构。Provider 通过 `api_key_env` 命名密钥，真实密钥值保存在
CLI 与桌面端共用的当前云智企办配置档案 state `.env`；项目 `.env` 不再作为
provider key 的运行时 fallback，但仍会作为当前 workspace 范围内的 MCP/plugin 非 provider `${VAR}` 展开来源，不导入稳定的 `KWU_*`
控制变量。权限、沙盒、插件（MCP）、斜杠命令、`@` 引用与双模型设置，全部在
**[指南](./docs/GUIDE.zh-CN.md)** 里。

## 文档

- **[文档中心](./docs/README.md)** —— 稳定指南、工程参考、发布手册、在研计划与历史归档的统一入口。
- **[指南](./docs/GUIDE.zh-CN.md)** —— 配置、权限与沙盒、插件（MCP）、斜杠命令、
  `@` 引用、双模型协同。
- **[机器人使用指南](./docs/BOT_GUIDE.zh-CN.md)** —— 桌面端连接飞书、Lark、微信
  Bot，以及 IM 里的审批、YOLO 和命令交互。
- **[规格](./docs/SPEC.md)** —— 工程契约：架构、registry、数据类型与路线图。
- **[从 0.x 迁移](./docs/MIGRATING.md)** —— 从 legacy TypeScript 版本迁到 1.0 Go 重写版。
- **[Checkpoints 与 rewind](./docs/CHECKPOINTS.md)** —— 基于快照的编辑安全网
  （Esc-Esc / `/rewind`）。

<br/>
