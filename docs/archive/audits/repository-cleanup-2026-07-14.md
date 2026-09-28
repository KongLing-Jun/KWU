# 仓库文档与个人信息清理审计

日期：2026-07-14

基线：`main-v3` / `b47237ac`

范围：Git 跟踪的文档、文档媒体、历史计划、个人工作痕迹；不包含构建产物和本机 profile 凭据。
状态：**已执行首轮清理；本文件归档，仅用于追溯当时的判断和结果。**

## 结论

仓库当前有 1,852 个跟踪文件。按文档、表格和 JSON 数据口径统计，共 452 个文件、5,861,345 字节；其中 `docs/` 的主要膨胀来源是：

| 目录 | 文件数 | 字节数 | 当前判断 |
| --- | ---: | ---: | --- |
| `docs/fix` | 52 | 3,387,658 | 体积最大，但 49 个治理组中仅 G01 已完成；不能直接整目录删除 |
| `docs/todos` | 281 | 1,160,059 | 大量阶段计划；应按功能完成度归档，不应一次性清空 |
| `docs/ai01` | 25 | 177,165 | 个人/代理工作报告，未被运行代码或构建引用，可优先移出仓库 |
| `docs/superpowers` | 17 | 141,763 | 实施计划与设计稿，功能落地后可归档 |
| `docs/guard` | 7 | 78,803 | 代码守卫路线图，仍有工程治理价值 |
| `docs/ai02` | 2 | 18,832 | 个人/代理命名的计划，可移出或改为中性目录 |
| `docs/bugs` | 3 | 14,691 | 历史 bug 计划，修复完成后可删 |

第一批有充分证据、无需改变程序功能即可删除的候选共 **25 个文件、1,054,761 字节**：

- 15 个未被任何跟踪文本引用的文档媒体：992,326 字节。
- 9 个逐字节重复的 `new-expert3` 文档：48,153 字节。
- 1 个已完成且无引用的隐藏进度文档：14,282 字节。

这些数字不包含本报告自身，也不包含需要产品或历史保留判断的第二、第三批候选。

## 执行记录

2026-07-14 已按本报告完成第一轮清理；本文其余数量均为清理前基线：

- 删除 54 个文件，共 1,313,441 字节：第一批 25 个文件、`docs/ai01` 25 个文件、`docs/ai02` 2 个文件，以及两份 0.9.0 内部协作进度文档。
- 将 `docs/todos/01/ai02/new-expert2` 和 `new-expert4` 移至 `docs/todos/01/` 下的同名中性路径，共保留 32 个文件、95,966 字节。
- 将仍在使用的 `docs/fix/0710/manifest.json` 中绝对本机项目路径改为相对路径 `.`。
- 保留仍有未完成治理任务的 `docs/fix`、功能状态不明的阶段 TODO、当前 Pages 站点和全部运行时资源。
- 清理后 `go run ./scripts/code_guard`、代码守卫基线检查、`go vet ./...` 和 `go test ./...` 均通过。

## 审计依据

本次判断使用以下证据：

1. `git ls-files` 只盘点 Git 跟踪内容。
2. 对文档重复项使用 `git hash-object`；下列 9 对文件的 blob hash 完全相同。
3. 使用 `git grep` 和 `rg` 检查 Markdown、HTML、Astro、TypeScript、Go、CI、Makefile 中的引用。
4. 检查全部 `//go:embed`：嵌入对象只有桌面前端构建产物、应用图标、云测试 fixture、内置专家、guardian policy 和 serve 页面；本文候选目录均未被嵌入。
5. `.github/workflows/pages.yml` 只构建并发布 `site/dist`，不发布 `docs/`。
6. 扫描邮箱、电话、绝对用户路径、个人账号链接和常见密钥格式；没有发现真实提交密钥。`sk-...` 命中来自文件名片段 `task-pages` 和 `task-bug-scan`，均属于误报。
7. 当前 worktree 已有用户修改：`README.md` 删除 47 行支持/致谢内容。本报告不修改、不覆盖该变更。

## 第一批：可直接清理

### 1. 未引用的文档媒体

这些文件的文件名在所有跟踪的 Markdown、HTML、CSS、Astro、JS/TS、YAML 和 Go 文件中均无引用。删除只会移除孤立素材，不影响程序、构建或现有文档页面。

| 字节数 | 文件 | 建议 |
| ---: | --- | --- |
| 141,819 | `.github/sponsor/wechat-pay.jpg` | 删除；个人收款二维码。当前 `README.md` 工作区变更已移除唯一展示入口 |
| 92,294 | `docs/assets/agent-skill-sources.png` | 删除；无引用的历史截图 |
| 11,759 | `docs/assets/approval-authorization-mocks.svg` | 删除；无引用的 mock 图 |
| 90,290 | `docs/assets/issue-2605-plan-approval-shelf.png` | 删除；无引用的 issue 截图 |
| 78,730 | `docs/assets/issue-2605-tool-approval-shelf.png` | 删除；无引用的 issue 截图 |
| 104,120 | `docs/claude-desktop-layout-preview.png` | 删除；旧产品对照图，无引用 |
| 2,211 | `docs/cli-image-paste-effect.svg` | 删除；无引用演示图 |
| 3,559 | `docs/cli-theme-effect.svg` | 删除；无引用演示图 |
| 52,824 | `docs/desktop-theme-dark.jpg` | 删除；无引用主题截图 |
| 51,084 | `docs/desktop-theme-light.jpg` | 删除；无引用主题截图 |
| 75,220 | `docs/desktop-v2-layout-preview.png` | 删除；无引用旧版桌面截图 |
| 81,977 | `docs/desktop-v2-skills-preview.png` | 删除；无引用旧版桌面截图 |
| 3,671 | `docs/paste-folding-effect.svg` | 删除；无引用演示图 |
| 199,773 | `docs/project-topic-tabs-context.png` | 删除；无引用的大型上下文截图 |
| 2,995 | `docs/slash-command-views.svg` | 删除；无引用演示图 |

注意：`docs/logo.svg` 仍被 README 使用；桌面图标、专家卡片图片和提示音被应用代码或 Wails 嵌入使用，不属于清理候选。

### 2. 逐字节重复的 `new-expert3` 文档

建议保留中性路径 `docs/todos/01/new-expert3/`，删除带个人/代理命名的副本 `docs/todos/01/ai02/new-expert3/`：

| 删除 | 保留 |
| --- | --- |
| `docs/todos/01/ai02/new-expert3/01-current-gap-and-reference-analysis.md` | `docs/todos/01/new-expert3/01-current-gap-and-reference-analysis.md` |
| `docs/todos/01/ai02/new-expert3/02-card-expansion-taxonomy.md` | `docs/todos/01/new-expert3/02-card-expansion-taxonomy.md` |
| `docs/todos/01/ai02/new-expert3/03-prompt-template-and-adaptation.md` | `docs/todos/01/new-expert3/03-prompt-template-and-adaptation.md` |
| `docs/todos/01/ai02/new-expert3/04-backend-catalog-and-data-model.md` | `docs/todos/01/new-expert3/04-backend-catalog-and-data-model.md` |
| `docs/todos/01/ai02/new-expert3/05-frontend-card-integration.md` | `docs/todos/01/new-expert3/05-frontend-card-integration.md` |
| `docs/todos/01/ai02/new-expert3/06-team-runtime-roadmap.md` | `docs/todos/01/new-expert3/06-team-runtime-roadmap.md` |
| `docs/todos/01/ai02/new-expert3/07-tests-rollout.md` | `docs/todos/01/new-expert3/07-tests-rollout.md` |
| `docs/todos/01/ai02/new-expert3/08-implementation-task-list.md` | `docs/todos/01/new-expert3/08-implementation-task-list.md` |
| `docs/todos/01/ai02/new-expert3/goal.md` | `docs/todos/01/new-expert3/goal.md` |

两套文件内部只互相使用 `new-expert3` 名称，没有外部代码、构建或文档入口依赖其中一套具体路径。

### 3. 已完成的隐藏进度文档

| 字节数 | 文件 | 证据与建议 |
| ---: | --- | --- |
| 14,282 | `.superpowers/sdd/progress.md` | 全部任务已标记完成或 review clean；无入站引用；包含本地 `D:\Reasonix...` worktree 路径。删除 |

`.kwu/commands/review.md` 不应删除：它是仓库显式保留的本地命令定义，`.gitignore` 中有专门的反忽略规则。

### 4. 旧静态站残留（确认后删除）

当前 Pages 工作流只发布 `site/dist`。下列旧 `docs/` 静态站文件不参与现有部署：

| 字节数 | 文件 | 建议 |
| ---: | --- | --- |
| 20,468 | `docs/index.html` | 若 GitHub Pages 设置已确定使用 Actions，删除旧手写首页 |
| 518 | `docs/favicon.svg` | 只被旧 `docs/index.html` 使用；随旧首页删除 |
| 0 | `docs/.nojekyll` | 旧 `docs/` Pages 标记；随旧首页删除 |

这 20,986 字节未计入“可直接清理”的 1,054,761 字节，因为仓库外的 GitHub Pages 设置不能仅靠代码证明。

## 第二批：个人信息与个人工作痕迹

### 1. README 与收款/致谢信息

当前未提交的 `README.md` 变更已经删除：

- PayPal 个人收款链接。
- `.github/sponsor/wechat-pay.jpg` 微信收款二维码入口。
- 多位贡献者姓名和个人 GitHub 账号列表。
- 小红书推广链接及推广者致谢。

建议保留该工作区变更，并同时删除收款二维码文件。贡献者致谢是否删除属于项目治理决定；它不影响运行功能，但可能影响社区归属感和贡献可见性。

`README.md` / `README.zh-CN.md` 中的 Discord 社区链接属于项目支持入口，不是个人隐私，建议保留。

### 2. `docs/ai01`：个人/代理结果与测试留痕

该目录共 25 个文件、177,165 字节，没有代码、构建、CI 或 README 引用。部分报告包含本机用户目录、历史命令路径和内部审计过程。若不需要保留开发过程，建议整目录移出 Git；若需要留档，放到外部私有归档并在仓库只保留一份去标识化的汇总。

`docs/ai01/result/`（22 个文件、128,053 字节）：

- `2026-06-27-automation-task-1-2-progress.md`
- `2026-06-28-business-diagnostics-template.md`
- `2026-06-28-user-profile-completion-audit.md`
- `2026-06-28-user-profile-stage-2-report.md`
- `2026-06-28-user-profile-stage-4-result.md`
- `2026-06-28-user-profile-stage1-result.md`
- `2026-06-30-automation-editor-model-enabled-prompt.md`
- `2026-06-30-user-profile-rendering-ui-followup.md`
- `2026-07-01-full-test-report.md`
- `2026-07-01-sqlite-template-store.md`
- `2026-07-09-frontend-audit-fixes-batch-1.md`
- `2026-07-09-module-audit-fixes-batch-1.md`
- `automation-task-pages-report.md`
- `business-diagnostics-template-test-report.md`
- `business-diagnostics-template-test-suite.md`
- `template-delete-report.md`
- `business-diagnostics-template-data/finance.csv`
- `business-diagnostics-template-data/inventory.csv`
- `business-diagnostics-template-data/inventory.xlsx`
- `business-diagnostics-template-data/purchase.csv`
- `business-diagnostics-template-data/returns.csv`
- `business-diagnostics-template-data/sales.csv`

`docs/ai01/tests/`（3 个文件、49,112 字节）：

- `automation-task-bug-scan-20260707.md`
- `frontend-page-p0-p3-audit-20260708.md`
- `module-p0-p3-audit-20260708.md`

优先处理的具体内容：

- `2026-06-28-business-diagnostics-template.md`：包含 `C:\Users\<user>\...` 形式的运行时工具路径。
- `automation-task-pages-report.md`：包含当前机器的 AppData 路径。
- `module-p0-p3-audit-20260708.md`：包含本机路径、历史重命名和安全审计过程。

### 3. `docs/ai02` 与 `docs/todos/01/ai02`

| 范围 | 文件数 | 字节数 | 建议 |
| --- | ---: | ---: | --- |
| `docs/ai02` | 2 | 18,832 | 删除或迁移到中性功能目录 |
| `docs/todos/01/ai02/new-expert2` | 15 | 46,909 | 功能已落地时删除；仍活跃时移动到中性目录 |
| `docs/todos/01/ai02/new-expert3` | 9 | 48,153 | 第一批直接删除，已有逐字节相同的中性路径副本 |
| `docs/todos/01/ai02/new-expert4` | 17 | 49,057 | 功能已落地时删除；仍活跃时移动到中性目录 |

`docs/ai02` 的两个文件为：

- `docs/ai02/goal.md`
- `docs/ai02/模型广场开发计划第一版.md`

目录名 `ai01` / `ai02` 暴露个人或代理分工，不利于长期文档信息架构。需要保留的内容应按功能命名，例如 `docs/archive/model-marketplace/`、`docs/archive/expert-library/`。

### 4. 包含本机路径、会话 ID 或内部协作记录的文件

| 文件 | 风险 | 建议 |
| --- | --- | --- |
| `docs/todos/10/0.9.0-注册及积分闭环/A线开发进度.md` | 36,084 字节；包含本机用户名、绝对 worktree 路径、大量会话 ID、分支、提交和内部流程偏差记录 | 功能结论已写明完成；优先删除，或压缩成不含身份/路径/会话 ID 的一页里程碑摘要 |
| `docs/fix/0710/manifest.json` | 71,407 字节；`project` 字段包含 `/Users/<user>/...` 绝对路径 | `docs/fix` 尚活跃时只脱敏为相对路径；整套治理迁移后与 `docs/fix` 一起删除 |
| `docs/todos/10/0.9.0-注册及积分闭环/两人工作安排.md` | 26,599 字节；长期保存 A/B 分工、owner 和 worktree 流程 | 不含明确真实姓名，但属于内部协作细节；0.9.0 收尾后删除或外部归档 |

扫描到的两个手机号位于 `04-双端接口契约.md` 的 JSON 示例中，是测试/协议样例，没有证据表明是真实个人号码。建议改为显式保留号段或明显占位值，而不是因为扫描命中直接删除整个契约。

### 5. 项目账号链接

`xiaoxiaotest9527-bit/Reasonix` 和 `KWU-release` 出现在升级代码、发布工作流、站点和测试中。这些链接属于当前发布基础设施，不能按“个人账号”直接删除。只有仓库迁移到组织账号后，才应统一替换并运行升级/发布契约测试。

## 第三批：迁移或归档后清理

### 1. `docs/fix`：最大体积，但仍是活跃治理台账

总计 52 个文件、3,387,658 字节：

- `docs/fix/2026-07-10-code-guard-484-findings-audit.md`
- `docs/fix/0710/README.md`
- `docs/fix/0710/manifest.json`
- `docs/fix/0710/group-01-p0-governance-batch.md`
- `docs/fix/0710/group-02-p0-cli-tui.md`
- `docs/fix/0710/group-03-p1-internal-cli-select.md`
- `docs/fix/0710/group-04-p1-governance-batch.md`
- `docs/fix/0710/group-05-p1-internal-agent-migrate.md`
- `docs/fix/0710/group-06-p1-controller.md`
- `docs/fix/0710/group-07-p1-memory-compiler.md`
- `docs/fix/0710/group-08-p1-internal-tool-webfetch.md`
- `docs/fix/0710/group-09-p1-agent.md`
- `docs/fix/0710/group-10-p1-internal-control-slash.md`
- `docs/fix/0710/group-11-p1-internal-provider-anthropic.md`
- `docs/fix/0710/group-12-p2-internal-tasktree-planner.md`
- `docs/fix/0710/group-13-p2-internal-plugin-lazy.md`
- `docs/fix/0710/group-14-p2-internal-cli-cli.md`
- `docs/fix/0710/group-15-p2-cli-tui.md`
- `docs/fix/0710/group-16-p2-agent.md`
- `docs/fix/0710/group-17-p2-internal-fileref-render.md`
- `docs/fix/0710/group-18-p2-internal-migration-refs.md`
- `docs/fix/0710/group-19-p2-internal-memorycompiler-compression.md`
- `docs/fix/0710/group-20-p2-internal-control-attachments.md`
- `docs/fix/0710/group-21-p2-internal-guardian-complete.md`
- `docs/fix/0710/group-22-p2-internal-cli-md.md`
- `docs/fix/0710/group-23-p2-internal-installsource-plan.md`
- `docs/fix/0710/group-24-p2-internal-history-search.md`
- `docs/fix/0710/group-25-p2-internal-agent-subagent-store.md`
- `docs/fix/0710/group-26-p2-internal-config-load.md`
- `docs/fix/0710/group-27-p2-internal-jobs-jobs.md`
- `docs/fix/0710/group-28-p2-internal-automation-schedule.md`
- `docs/fix/0710/group-29-p2-internal-bot-gateway.md`
- `docs/fix/0710/group-30-p2-internal-cli-skill-picker.md`
- `docs/fix/0710/group-31-p2-controller.md`
- `docs/fix/0710/group-32-p2-internal-doctor-report.md`
- `docs/fix/0710/group-33-p3-internal-config-credentials.md`
- `docs/fix/0710/group-34-p3-internal-eventwire-policy.md`
- `docs/fix/0710/group-35-p3-internal-profile-store.md`
- `docs/fix/0710/group-36-p3-internal-sandbox-runtime.md`
- `docs/fix/0710/group-37-p3-internal-installsource-store.md`
- `docs/fix/0710/group-38-p3-internal-serve-serve.md`
- `docs/fix/0710/group-39-p3-cli-tui.md`
- `docs/fix/0710/group-40-p3-internal-tool-bash.md`
- `docs/fix/0710/group-41-p3-cmd-e2ebench-diff.md`
- `docs/fix/0710/group-42-p3-internal-cli-cli.md`
- `docs/fix/0710/group-43-p3-internal-config-edit.md`
- `docs/fix/0710/group-44-p3-internal-controlplane-evidence.md`
- `docs/fix/0710/group-45-p3-internal-fileutil-diff.md`
- `docs/fix/0710/group-46-p3-internal-automation-lease.md`
- `docs/fix/0710/group-47-p3-internal-mcpdiag-provider.md`
- `docs/fix/0710/group-48-p3-internal-netclient-service.md`
- `docs/fix/0710/group-49-p3-internal-acp-store.md`

`README.md` 显示只有 G01 已完成，G02-G49 仍为“待认领”；总审计也将 484 条标记为“待语义治理评审”。因此：

1. 现在可以删除或脱敏生成器中的绝对项目路径。
2. 先把仍需处理的条目迁移到 GitHub Issues、项目看板或一份紧凑台账。
3. 每个条目保留 ID、位置、优先级、状态和验收命令即可，不必保留 50-80 KB 的重复调用链模板。
4. 迁移并核对完成后，整目录删除可释放约 3.39 MB。

### 2. `docs/todos`：按功能完成度分批归档

| 范围 | 文件数 | 字节数 | 建议 |
| --- | ---: | ---: | --- |
| `docs/todos/01/ai02` | 41 | 144,119 | 先删重复 9 个；其余移到中性路径或在专家功能稳定后删除 |
| `docs/todos/01/new-expert3` | 9 | 48,153 | 保留作为重复项的中性副本；专家库稳定后可归档 |
| `docs/todos/01/专家库` | 15 | 50,556 | 与 new-expert2/3/4 内容重叠；合并成一份最终架构说明后删除阶段任务 |
| `docs/todos/02/任务树` | 33 | 83,391 | 任务树代码和测试稳定后，只保留设计边界与验收摘要 |
| `docs/todos/03/自动化任务` | 33 | 128,519 | 自动化功能稳定后，只保留用户文档和最终设计边界 |
| `docs/todos/04/用户系统` | 82 | 302,823 | 当前最大 TODO 子树；阶段文件高度碎片化，完成后合并为 profile/credential/session 架构文档 |
| `docs/todos/05/用户画像` | 47 | 137,748 | 功能完成后合并为 `USER_PROFILE` 设计/用户文档，删除阶段 1-9 任务拆分 |
| `docs/todos/08/office基础套件` | 2 | 26,080 | 若仍未实现则保留；取消或实现后删除计划 |
| `docs/todos/10/0.9.0-注册及积分闭环` | 18 | 209,926 | 保留接口契约和设计边界；优先清理 A 线进度、两人安排和阶段性进度文件 |
| `docs/todos/11/更换官方厂商` | 1 | 28,744 | 已有完整完成记录；新厂商切换稳定后删除实施计划 |

原则：`goal.md`、阶段任务和进度记录属于临时执行材料；稳定后的长期事实应进入 `docs/SPEC.md`、用户指南或一份功能架构文档。

### 3. `docs/superpowers`、`docs/guard`、`docs/bugs`

| 范围 | 文件数 | 字节数 | 建议 |
| --- | ---: | ---: | --- |
| `docs/superpowers/plans` + `docs/superpowers/specs` | 17 | 141,763 | 实施完成后删除 plan；仅在仍记录关键设计决策时保留精简 spec |
| `docs/guard/v0.1.0` | 7 | 78,803 | 守卫仍在演进时保留；完成后合并为一份维护文档 |
| `docs/bugs` | 3 | 14,691 | 对照当前测试确认 bug 已修复后删除；回归事实应由测试承担 |

## 明确保留

以下文件虽属于文档或较大资产，但当前有明确功能、发布或用户入口，不应纳入瘦身删除：

- `AGENTS.md`：PR 门禁和仓库协作规则。
- `LICENSE`：许可证。
- `README.md`、`README.zh-CN.md`：项目入口；只清理个人信息段落，不删除文件。
- README 直接链接的文档：
  - `docs/MIGRATING.md`
  - `docs/CONFIG_PATHS.md`
  - `docs/CONFIG_PATHS.zh-CN.md`
  - `docs/GUIDE.md`
  - `docs/GUIDE.zh-CN.md`
  - `docs/BOT_GUIDE.md`
  - `docs/BOT_GUIDE.zh-CN.md`
  - `docs/SPEC.md`
  - `docs/CHECKPOINTS.md`
- `scripts/code_guard_baseline.json`：代码守卫运行基线，不是普通历史报告。
- `docs/codegraph/README.md` 及其 SVG：仍可作为架构可视化；若团队不使用 CodeGraph，可另行整组删除。
- `desktop/build/appicon.*`、平台图标：Wails 和托盘代码通过 `go:embed` 使用。
- `desktop/frontend/src/assets/experts/*`：专家目录 UI 使用。
- `desktop/frontend/public/sounds/*`：桌面通知声音使用。
- `site/`：当前 GitHub Pages 的真实构建来源。
- `.kwu/commands/review.md`：运行时命令定义。

## 推荐执行顺序

1. 确认并保留当前 `README.md` 的个人收款/致谢清理变更。
2. 删除第一批 15 个孤立媒体、9 个重复文档和 `.superpowers/sdd/progress.md`。
3. 删除或私有归档 `docs/ai01`、`docs/ai02`，处理 `docs/todos/01/ai02` 的个人命名。
4. 删除或脱敏 `A线开发进度.md`、`manifest.json` 和内部工作安排。
5. 把 `docs/fix` 未完成条目迁移到可追踪系统，再删除生成式大文档。
6. 每个已完成功能将阶段 TODO 合并成一份长期架构/用户文档，再删除阶段拆分文件。

建议每批单独提交，便于回滚和审查：

- `docs: remove unreferenced media and duplicate plans`
- `docs: remove personal development artifacts`
- `docs: archive completed implementation plans`

## 删除后的验证

第一批清理后至少执行：

```powershell
git diff --check
git grep -n "wechat-pay.jpg"
git grep -n "docs/todos/01/ai02/new-expert3"
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
```

因为第一批包含图片和非 Markdown 文件，准备 PR 前仍按 `AGENTS.md` 执行完整门禁：

```powershell
go vet ./...
go test ./...
```

若删除旧静态站，还需执行：

```powershell
npm --prefix site ci
npm --prefix site run build
```

最终必须检查：

- `git status --short --branch` 只包含预期清理和本报告。
- `git diff --name-status` 没有误删应用资产、用户指南或代码守卫基线。
- README 中所有相对链接目标仍存在。
- `site/dist` 可正常构建。
- 桌面 Wails 构建仍能找到所有嵌入资源。
