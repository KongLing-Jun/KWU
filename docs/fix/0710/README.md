# Code Guard 484 条问题治理分组总览

> 本目录将原始 484 条静态度量问题按治理对象、超限程度、文件热点、核心运行链路和更新后的 CodeGraph 影响范围重新排序。原始审计快照保留在 [`../2026-07-10-code-guard-484-findings-audit.md`](../2026-07-10-code-guard-484-findings-audit.md)，未删除或缩减。

## 1. 数据快照

- 仓库：`Reasonix-v3.1`
- 分支：`codex/docs-0.9-consistency`
- Commit：`99997cb549303e70f6f2b59bb5defd1466afc589`
- CodeGraph：1169 文件、26131 节点、89253 边；执行 `codegraph sync .` 后状态为 up to date。
- 原始审计 SHA-256：`c3021f1bad7c3d71c7a0323c9b4605ceaca4c127d45a2bd5375f92b49c29b46f`
- Baseline SHA-256：`9cc3dc818cac3351465d77999b655df07186712083980ce5064494aed2a5b7c6`
- 问题总数：484
- 治理对象：417（同一函数多规则合并为同一对象）
- 分组：48 个 10 条组 + 1 个 4 条尾组，共 49 组。

## 2. 排序口径

综合分数采用 100 分模型：静态严重度 30、问题集中度 20、CodeGraph 影响面 30、治理可执行性 20。生产调用方与测试调用方分开计数，测试不进入影响面得分；未合并远端引用只作为串行/阻塞信息，不作为加分。每组首个治理对象按全局分数降序选取，组内再优先补入同文件、同模块、同运行链路对象，以避免多个负责人同时修改热点文件；同一函数的复杂度、长度和嵌套记录绝不拆组。

冲突风险中的分支信息来自本地现有 `origin/*` 引用，不等同于 GitHub 当前开放 PR；认领前仍需刷新远端并重新核对。

CodeGraph 调用边按置信度分级：`>=0.9` 记为可信静态边，`0.7–0.89` 仅记为待源码复核候选；`heuristic`、低于 0.7 的边和生产代码误指向测试实现的边不进入清单。即使是可信边也只代表静态索引记录；反射、接口动态分派、注册表和 unresolved call 仍是索引盲区。

### 规则对账

| 规则 | 数量 |
|---|---|
| 圈复杂度 | 394 |
| 函数有效行数 | 51 |
| 文件有效行数 | 18 |
| 嵌套深度 | 20 |
| 单包生产文件数 | 1 |

## 3. 分组索引

| 组 | 整体优先级 | 标题 | 条数 | 主要文件 | 问题 ID | 前置/串行关系 | 状态 |
|---|---|---|---|---|---|---|---|
| [G01](./group-01-p0-governance-batch.md) | P0 | 启动装配与运行依赖编排 | 10 | `internal/boot/boot.go`<br>`internal/agent/migrate.go`<br>`internal/boot/token_profile.go` | `CX-079` `FN-011` `ND-005` `CX-032` `FN-006` `ND-003` `FL-004` `CX-037` `CX-081` `CX-080` | 先于 G05 处理 | 已完成 |
| [G02](./group-02-p0-cli-tui.md) | P0 | CLI/TUI 状态更新与交互渲染 | 10 | `internal/cli/chat_tui.go` | `CX-106` `FN-018` `ND-007` `CX-117` `FN-021` `FL-006` `CX-116` `FN-020` `CX-112` `FN-019` | 先于 G15、G36、G39 处理 | 待认领 |
| [G03](./group-03-p1-internal-cli-select.md) | P1 | internal/cli · select 治理批次 | 10 | `internal/cli/select.go`<br>`internal/cli/mcp_manager.go`<br>`internal/cli/bot.go` | `CX-160` `FN-028` `ND-009` `CX-159` `FN-027` `CX-146` `FN-026` `CX-104` `FN-017` `CX-147` | 先于 G22 处理 | 待认领 |
| [G04](./group-04-p1-governance-batch.md) | P1 | 配置渲染与差异输出 | 10 | `internal/config/render.go`<br>`internal/config/effort.go`<br>`internal/config/edit.go` | `CX-195` `FN-030` `CX-196` `FN-031` `CX-181` `CX-180` `FL-009` `CX-179` `FL-011` `CX-197` | 先于 G43 处理 | 待认领 |
| [G05](./group-05-p1-internal-agent-migrate.md) | P1 | internal/agent · migrate 治理批次 | 10 | `internal/agent/migrate.go`<br>`internal/agent/parallel_tasks.go` | `CX-035` `FN-007` `ND-004` `CX-040` `FN-008` `CX-036` `CX-033` `CX-034` `CX-038` `CX-039` | 等待 G01 的同文件变更合并 | 待认领 |
| [G06](./group-06-p1-controller.md) | P1 | Controller 会话控制与提交链路 | 10 | `internal/control/controller.go` | `CX-209` `FN-032` `ND-011` `FL-012` `CX-213` `CX-219` `CX-215` `CX-218` `CX-212` `CX-210` | 先于 G31 处理 | 待认领 |
| [G07](./group-07-p1-memory-compiler.md) | P1 | Memory Compiler 运行时图与策略 | 10 | `internal/memorycompiler/runtime.go` | `FL-016` `CX-305` `FN-042` `CX-307` `CX-308` `FN-043` `CX-310` `CX-312` `CX-311` `CX-315` | 先于 G19 处理 | 待认领 |
| [G08](./group-08-p1-internal-tool-webfetch.md) | P1 | internal/tool · webfetch 治理批次 | 10 | `internal/tool/builtin/webfetch.go`<br>`internal/tool/builtin/grep.go`<br>`internal/tool/builtin/delete_symbol.go` | `CX-391` `FN-051` `ND-020` `CX-386` `FN-050` `CX-384` `ND-019` `CX-379` `CX-383` `CX-392` | 先于 G32 处理 | 待认领 |
| [G09](./group-09-p1-agent.md) | P1 | Agent 主循环与工具执行链路 | 10 | `internal/agent/task.go`<br>`internal/agent/agent.go` | `CX-053` `FN-009` `CX-024` `FN-005` `CX-019` `FN-002` `CX-023` `FN-004` `CX-055` `CX-054` | 先于 G16 处理 | 待认领 |
| [G10](./group-10-p1-internal-control-slash.md) | P1 | internal/control · slash 治理批次 | 10 | `internal/control/slash.go`<br>`internal/control/auto_plan.go` | `CX-234` `FN-033` `CX-206` `CX-231` `CX-204` `CX-205` `CX-232` `CX-236` `CX-233` `CX-235` | 无同文件组依赖 | 待认领 |
| [G11](./group-11-p1-internal-provider-anthropic.md) | P1 | internal/provider · anthropic 治理批次 | 10 | `internal/provider/anthropic/anthropic.go`<br>`internal/provider/openai/openai.go`<br>`internal/provider/retry.go` | `CX-342` `FN-048` `ND-017` `CX-341` `FN-047` `ND-016` `CX-343` `CX-348` `CX-349` `CX-344` | 无同文件组依赖 | 待认领 |
| [G12](./group-12-p2-internal-tasktree-planner.md) | P2 | internal/tasktree · planner 治理批次 | 10 | `internal/tasktree/planner.go`<br>`internal/tasktree/parallel.go`<br>`internal/tasktree/executor.go` | `CX-366` `FN-049` `CX-363` `CX-364` `CX-361` `CX-365` `CX-368` `CX-369` `CX-367` `CX-362` | 无同文件组依赖 | 待认领 |
| [G13](./group-13-p2-internal-plugin-lazy.md) | P2 | internal/plugin · lazy 治理批次 | 10 | `internal/plugin/lazy.go`<br>`internal/plugin/plugin.go`<br>`internal/tool/builtin/codeindex.go` | `CX-325` `FN-045` `ND-015` `CX-326` `FN-046` `CX-377` `ND-018` `FL-017` `CX-328` `CX-327` | 先于 G32 处理 | 待认领 |
| [G14](./group-14-p2-internal-cli-cli.md) | P2 | internal/cli · cli 治理批次 | 10 | `internal/cli`<br>`internal/cli/cli.go` | `PF-001` `CX-123` `FN-024` `CX-121` `FN-022` `FL-007` `CX-122` `FN-023` `CX-120` `CX-124` | 先于 G42 处理 | 待认领 |
| [G15](./group-15-p2-cli-tui.md) | P2 | CLI/TUI 状态更新与交互渲染 | 10 | `internal/cli/upgrade.go`<br>`internal/cli/effort.go`<br>`internal/cli/acp.go` | `CX-170` `FN-029` `CX-134` `FN-025` `CX-102` `FN-015` `CX-109` `ND-008` `CX-114` `CX-115` | 等待 G02 的同文件变更合并 | 待认领 |
| [G16](./group-16-p2-agent.md) | P2 | Agent 主循环与工具执行链路 | 10 | `internal/agent/agent.go`<br>`internal/agent/textsink.go`<br>`internal/agent/branch.go` | `FL-002` `CX-057` `CX-022` `FN-003` `CX-026` `CX-047` `CX-027` `CX-058` `CX-020` `CX-021` | 等待 G09 的同文件变更合并 | 待认领 |
| [G17](./group-17-p2-internal-fileref-render.md) | P2 | internal/fileref · render 治理批次 | 10 | `internal/fileref/search.go`<br>`internal/bot/render.go`<br>`internal/bot/qq/gateway.go` | `CX-261` `FN-036` `CX-090` `FN-014` `CX-089` `FN-013` `CX-093` `CX-092` `CX-088` `CX-091` | 无同文件组依赖 | 待认领 |
| [G18](./group-18-p2-internal-migration-refs.md) | P2 | internal/migration · refs 治理批次 | 10 | `internal/migration/migration.go`<br>`internal/control/goal.go`<br>`internal/control/refs.go` | `CX-318` `FN-044` `CX-221` `CX-229` `CX-228` `CX-225` `CX-226` `CX-227` `CX-317` `CX-316` | 先于 G31 处理 | 待认领 |
| [G19](./group-19-p2-internal-memorycompiler-compression.md) | P2 | internal/memorycompiler · compression 治理批次 | 10 | `internal/memorycompiler/compression.go`<br>`internal/memorycompiler/runtime.go` | `FL-015` `CX-303` `CX-302` `CX-304` `CX-301` `CX-309` `ND-014` `CX-313` `CX-306` `CX-314` | 等待 G07 的同文件变更合并 | 待认领 |
| [G20](./group-20-p2-internal-control-attachments.md) | P2 | internal/control · attachments 治理批次 | 10 | `internal/control/attachments.go`<br>`internal/control/profile_management.go`<br>`internal/control/input.go` | `CX-201` `CX-224` `CX-202` `CX-223` `CX-237` `CX-199` `CX-200` `CX-207` `CX-208` `CX-198` | 无同文件组依赖 | 待认领 |
| [G21](./group-21-p2-internal-guardian-complete.md) | P2 | internal/guardian · complete 治理批次 | 10 | `internal/guardian/transcript.go`<br>`internal/cli/complete.go`<br>`internal/cli/latex.go` | `CX-266` `FN-037` `CX-131` `CX-137` `CX-139` `CX-129` `CX-132` `CX-130` `CX-264` `CX-265` | 先于 G30 处理 | 待认领 |
| [G22](./group-22-p2-internal-cli-md.md) | P2 | internal/cli · md 治理批次 | 10 | `internal/cli/md.go`<br>`internal/cli/bot.go`<br>`internal/cli/mcp.go` | `CX-152` `CX-103` `FN-016` `CX-148` `CX-149` `CX-154` `CX-119` `CX-133` `CX-151` `CX-150` | 等待 G03 的同文件变更合并 | 待认领 |
| [G23](./group-23-p2-internal-installsource-plan.md) | P2 | internal/installsource · plan 治理批次 | 10 | `internal/installsource/plan.go`<br>`internal/installsource/install_source.go`<br>`internal/installsource/mcp.go` | `CX-284` `FN-040` `CX-274` `FN-039` `CX-275` `CX-283` `CX-281` `CX-279` `CX-280` `CX-282` | 先于 G37 处理 | 待认领 |
| [G24](./group-24-p2-internal-history-search.md) | P2 | internal/history · search 治理批次 | 10 | `internal/history/search.go`<br>`internal/agent/compact.go` | `CX-270` `ND-012` `CX-028` `CX-030` `CX-029` `CX-267` `FN-038` `CX-031` `CX-268` `CX-269` | 无同文件组依赖 | 待认领 |
| [G25](./group-25-p2-internal-agent-subagent-store.md) | P2 | internal/agent · subagent_store 治理批次 | 10 | `internal/agent/parallel_tasks.go`<br>`internal/agent/reasoning_language.go`<br>`internal/agent/subagent_store.go` | `CX-042` `CX-045` `CX-051` `CX-048` `CX-044` `CX-050` `CX-052` `CX-043` `CX-049` `CX-041` | 等待 G05 的同文件变更合并 | 待认领 |
| [G26](./group-26-p2-internal-config-load.md) | P2 | internal/config · load 治理批次 | 10 | `internal/config/load.go`<br>`internal/config/migrate.go` | `FL-010` `CX-188` `CX-183` `ND-010` `CX-189` `CX-191` `CX-190` `CX-182` `CX-185` `CX-184` | 无同文件组依赖 | 待认领 |
| [G27](./group-27-p2-internal-jobs-jobs.md) | P2 | internal/jobs · jobs 治理批次 | 10 | `internal/jobs/jobs.go`<br>`cmd/e2ebench/diff.go` | `FL-014` `CX-287` `FN-041` `CX-002` `ND-002` `CX-001` `CX-288` `CX-291` `CX-290` `CX-289` | 先于 G41 处理 | 待认领 |
| [G28](./group-28-p2-internal-automation-schedule.md) | P2 | internal/automation · schedule 治理批次 | 10 | `internal/automation/migration.go`<br>`internal/automation/engine.go`<br>`internal/automation/schedule.go` | `CX-069` `FN-010` `CX-063` `CX-072` `CX-073` `CX-071` `CX-070` `CX-064` `FL-003` `CX-060` | 先于 G46 处理 | 待认领 |
| [G29](./group-29-p2-internal-bot-gateway.md) | P2 | internal/bot · gateway 治理批次 | 10 | `internal/bot/gateway.go`<br>`internal/botruntime/runtime.go`<br>`internal/bot/feishu/feishu.go` | `CX-086` `FN-012` `CX-098` `CX-099` `CX-082` `CX-083` `CX-084` `CX-094` `FL-005` `CX-085` | 先于 G36、G49 处理 | 待认领 |
| [G30](./group-30-p2-internal-cli-skill-picker.md) | P2 | internal/cli · skill_picker 治理批次 | 10 | `internal/cli/latex.go`<br>`internal/cli/skill_picker.go`<br>`internal/cli/mcp_manager_view.go` | `CX-138` `CX-163` `CX-144` `CX-158` `CX-145` `CX-143` `CX-164` `CX-166` `CX-167` `CX-165` | 等待 G21 的同文件变更合并 | 待认领 |
| [G31](./group-31-p2-controller.md) | P2 | Controller 会话控制与提交链路 | 10 | `internal/control/errmsg.go`<br>`internal/control/imagecompress.go`<br>`internal/control/refs.go` | `CX-220` `CX-222` `CX-230` `CX-025` `CX-046` `CX-203` `CX-214` `CX-211` `CX-216` `CX-217` | 等待 G06、G16、G18 的同文件变更合并 | 待认领 |
| [G32](./group-32-p2-internal-doctor-report.md) | P2 | internal/doctor · report 治理批次 | 10 | `internal/doctor/report.go`<br>`internal/tool/builtin/codeindex.go`<br>`internal/tool/builtin/completestep.go` | `CX-245` `FN-034` `CX-376` `CX-381` `CX-388` `CX-393` `CX-380` `CX-375` `CX-378` `CX-246` | 等待 G08、G13 的同文件变更合并 | 待认领 |
| [G33](./group-33-p3-internal-config-credentials.md) | P3 | internal/config · credentials 治理批次 | 10 | `internal/config/config.go`<br>`internal/config/credentials.go`<br>`internal/config/ccswitch.go` | `FL-008` `CX-172` `CX-171` `CX-173` `CX-174` `CX-187` `CX-194` `CX-186` `CX-193` `CX-192` | 无同文件组依赖 | 待认领 |
| [G34](./group-34-p3-internal-eventwire-policy.md) | P3 | internal/eventwire · policy 治理批次 | 10 | `internal/eventwire/wire.go`<br>`internal/planmode/policy.go`<br>`internal/acp/service.go` | `CX-248` `FN-035` `CX-323` `CX-011` `FN-001` `CX-324` `FL-001` `CX-322` `CX-321` `CX-249` | 先于 G41、G48、G49 处理 | 待认领 |
| [G35](./group-35-p3-internal-profile-store.md) | P3 | internal/profile · store 治理批次 | 10 | `internal/profile/selector.go`<br>`internal/profile/store.go`<br>`internal/profile/event.go` | `CX-336` `CX-338` `CX-330` `CX-331` `CX-329` `CX-333` `CX-340` `CX-337` `CX-332` `CX-339` | 无同文件组依赖 | 待认领 |
| [G36](./group-36-p3-internal-sandbox-runtime.md) | P3 | internal/sandbox · runtime 治理批次 | 10 | `internal/sandbox/shell.go`<br>`internal/botruntime/runtime.go`<br>`internal/cli/chat_tui_paste.go` | `CX-351` `ND-006` `CX-105` `CX-140` `CX-157` `CX-118` `CX-096` `CX-097` `CX-095` `CX-100` | 等待 G02、G15、G29 的同文件变更合并 | 待认领 |
| [G37](./group-37-p3-internal-installsource-store.md) | P3 | internal/installsource · store 治理批次 | 10 | `internal/installsource/skill.go`<br>`internal/memory/store.go`<br>`internal/installsource/names.go` | `CX-285` `CX-296` `ND-013` `CX-278` `CX-273` `CX-300` `CX-272` `CX-276` `CX-277` `CX-286` | 等待 G23 的同文件变更合并 | 待认领 |
| [G38](./group-38-p3-internal-serve-serve.md) | P3 | internal/serve · serve 治理批次 | 10 | `internal/serve/serve.go`<br>`internal/skill/skill.go`<br>`internal/serve/auth.go` | `FL-018` `CX-356` `CX-352` `CX-355` `CX-358` `CX-360` `CX-357` `CX-359` `CX-354` `CX-353` | 无同文件组依赖 | 待认领 |
| [G39](./group-39-p3-cli-tui.md) | P3 | CLI/TUI 状态更新与交互渲染 | 10 | `internal/cli/chat_tui.go`<br>`internal/cli/mcp_manager_actions.go`<br>`internal/cli/gitstatus.go` | `CX-108` `CX-142` `CX-110` `CX-135` `CX-136` `CX-155` `CX-161` `CX-113` `CX-111` `CX-107` | 等待 G02、G15、G36 的同文件变更合并 | 待认领 |
| [G40](./group-40-p3-internal-tool-bash.md) | P3 | internal/tool · bash 治理批次 | 10 | `internal/tool/builtin/bash.go`<br>`internal/tool/builtin/delete_range.go`<br>`internal/tool/builtin/glob.go` | `CX-372` `CX-382` `CX-385` `CX-370` `CX-374` `CX-371` `CX-373` `CX-389` `CX-390` `CX-387` | 无同文件组依赖 | 待认领 |
| [G41](./group-41-p3-cmd-e2ebench-diff.md) | P3 | cmd/e2ebench · diff 治理批次 | 10 | `cmd/e2ebench/diff.go`<br>`internal/acp/dispatch.go`<br>`cmd/e2ebench/main.go` | `ND-001` `CX-010` `CX-009` `CX-005` `CX-006` `CX-007` `CX-003` `CX-004` `CX-013` `CX-016` | 等待 G27、G34 的同文件变更合并 | 待认领 |
| [G42](./group-42-p3-internal-cli-cli.md) | P3 | internal/cli · cli 治理批次 | 10 | `internal/cli/skill_hooks.go`<br>`internal/cli/theme_osc_unix.go`<br>`internal/cli/cli.go` | `CX-162` `CX-168` `CX-128` `CX-153` `CX-156` `CX-141` `CX-127` `CX-169` `CX-126` `CX-125` | 等待 G14、G39 的同文件变更合并 | 待认领 |
| [G43](./group-43-p3-internal-config-edit.md) | P3 | internal/config · edit 治理批次 | 10 | `internal/config/edit.go`<br>`internal/config/dotenv.go`<br>`internal/memory/recall.go` | `CX-176` `CX-178` `CX-175` `CX-177` `CX-295` `CX-297` `CX-298` `CX-334` `CX-335` `CX-299` | 等待 G04、G37 的同文件变更合并 | 待认领 |
| [G44](./group-44-p3-internal-controlplane-evidence.md) | P3 | internal/controlplane · evidence 治理批次 | 10 | `internal/controlplane/policy_nodes/weighting.go`<br>`internal/evidence/evidence.go`<br>`internal/agent/testutil/mock_provider.go` | `CX-241` `CX-252` `CX-256` `CX-056` `CX-238` `FL-013` `CX-253` `CX-239` `CX-240` `CX-255` | 先于 G47 处理 | 待认领 |
| [G45](./group-45-p3-internal-fileutil-diff.md) | P3 | internal/fileutil · diff 治理批次 | 10 | `internal/fileutil/encoding/encoding.go`<br>`cmd/suiyuan-plugin-example/main.go`<br>`internal/diff/diff.go` | `CX-262` `CX-008` `CX-242` `CX-259` `CX-244` `CX-258` `CX-260` `CX-263` `CX-243` `CX-257` | 无同文件组依赖 | 待认领 |
| [G46](./group-46-p3-internal-automation-lease.md) | P3 | internal/automation · lease 治理批次 | 10 | `internal/automation/lease.go`<br>`internal/automation/condition.go`<br>`internal/automation/export.go` | `CX-066` `CX-067` `CX-062` `CX-065` `CX-068` `CX-059` `CX-078` `CX-077` `CX-061` `CX-074` | 等待 G28 的同文件变更合并 | 待认领 |
| [G47](./group-47-p3-internal-mcpdiag-provider.md) | P3 | internal/mcpdiag · provider 治理批次 | 10 | `internal/mcpdiag/auth.go`<br>`internal/permission/permission.go`<br>`internal/provider/provider.go` | `CX-294` `CX-320` `CX-345` `CX-394` `CX-101` `CX-251` `CX-254` `CX-346` `CX-250` `CX-347` | 等待 G44 的同文件变更合并 | 待认领 |
| [G48](./group-48-p3-internal-netclient-service.md) | P3 | internal/netclient · service 治理批次 | 10 | `internal/netclient/netclient.go`<br>`internal/equilibrium/convergence/global_equilibrium.go`<br>`internal/lsp/results.go` | `CX-319` `CX-247` `CX-293` `CX-271` `CX-350` `CX-292` `CX-017` `CX-012` `CX-014` `CX-015` | 等待 G34、G41 的同文件变更合并 | 待认领 |
| [G49](./group-49-p3-internal-acp-store.md) | P3 | internal/acp · store 治理批次 | 4 | `internal/automation/store.go`<br>`internal/bot/gateway.go`<br>`internal/acp/service.go` | `CX-075` `CX-076` `CX-087` `CX-018` | 等待 G29、G34、G41、G46、G48 的同文件变更合并 | 待认领 |

## 4. 完整性证明

| 核查项 | 结果 |
|---|---|
| Finding / 唯一 ID / 唯一 baseline key | 484 / 484 / 484 |
| 治理对象 | 417（单条 360、双规则 47、三规则 10） |
| 分组容量 | 48 个 10 条组；尾组 4 条 |
| 同一 file+func 跨组 | 0（生成器以治理对象为不可拆原子） |
| CodeGraph callable 映射 | 484 条已映射；0 条明确标注未映射/歧义 |
| CodeGraph DB integrity_check | ok |
| 原审计处理 | 保留原文件；仅在本目录新增索引、manifest 与分组文档 |

机器可读分配与哈希见 [`manifest.json`](./manifest.json)；交付前还需用独立校验命令复核上述不变量，而不是只信任生成器输出。

## 5. 团队认领规则

1. 同一文件跨组时按组号串行处理，后组等待前组合并后重新同步 CodeGraph 和 baseline。
2. 核心热点建议单一负责人；不得让两组同时修改同一文件。
3. 每组只处理列明职责，不顺带重构相邻代码。
4. 修复后先跑聚焦测试和局部 Code Guard，再跑仓库规定的全量检查。
5. baseline 只在最新 `origin/main-v3` 上显式刷新并审查差异。
