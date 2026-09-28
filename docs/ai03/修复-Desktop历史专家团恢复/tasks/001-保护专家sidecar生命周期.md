# 任务 001：保护专家 sidecar 生命周期

## 状态与交付

- 状态：已完成并通过聚焦验证；待主 Agent 最终验收。
- 执行组：Serial-01。
- 指派：批准后创建的单一实现子代理；不得继续委派。
- 依赖：[plan.md](../plan.md) 已提交并获实现批准。
- 交付：一个 Backend `fix:`提交；聚焦验证通过后紧随一个本任务完成回执`docs:`提交。

## 目标与产物

建立专家 sidecar 的持久化不变量，使“当前未选择专家”不再隐式删除父 Session 已有的
`team_active/team_archived`历史状态，同时保持单专家清除和 Session 删除的既有显式行为。

产物包括：

- 保存与删除语义分离；
- active退出时原子归档，archived普通保存时永久保留；
- 缺失旧 sidecar不重建，损坏 sidecar不被普通保存覆盖；
- active/archived会话继续禁止再次选择专家；
- 直接 Go 回归测试。

## 证据与实现依据

- `internal/agent/save.go:SaveSessionExpert`当前以空`ExpertID`执行`os.Remove`。
- `internal/control/selected_expert_refresh.go:saveExpertMeta`同时消费当前选择和内存历史 meta；普通自动保存、
  task生命周期和 SessionPath路径均会调用它。
- `restoreArchivedExpertTeam`已经正确地保留 archived meta并把当前选择恢复为 none。
- `ClearExpert`已经定义 active团队归档语义；本任务不重新定义状态机。
- [plan.md 的根因边界](../plan.md#23-根因边界)明确现场第一次丢失的触发未知；本任务只封闭已证明的
  空 meta删除机制。

## 允许范围

- `internal/agent/save.go`及直接测试；
- `internal/control/selected_expert_refresh.go`、`controller.go`中专家保存、恢复、清除所需的最小改动；
- 直接相关`internal/control`测试；
- 本任务完成回执。

实现前可只读审计其他 sidecar调用者；若必须修改未列文件，应先由主 Agent确认其直接属于同一生命周期
合同，不得自行扩大范围。

## 禁止范围

- Desktop frontend和团队历史 UI；由任务 002处理。
- 从 JSONL、host prompt、模型文本、task/job/child meta或相邻会话恢复 sidecar。
- 修改用户 AppData或两个隔离工作树。
- 新数据库、迁移器、sidecar双写或专家目录格式。
- task Runtime、artifact handoff、SubagentStore或 Task Tree重构。
- 推送、PR、rebase、amend或非本任务清理。

## 保持行为与约束

1. `team_active -> team_archived`只能由退出团队语义触发；普通自动保存不得擅自归档 active。
2. archived的当前选择仍为 none，不能为了保存 sidecar把主专家重新设为当前身份。
3. 单专家属于 normal；清除单专家后仍允许本会话重新选择专家。
4. whole-session、fork、rewind、trash和永久删除继续按既有所有权规则清理或复制 sidecar。
5. 空状态不再充当通用删除命令；需要删除的调用点使用语义明确、范围受控的入口。
6. 原子写入和 Windows文件行为沿用现有实现；失败不得先删除旧文件。
7. 已存在但格式损坏的 sidecar不自动覆盖；错误必须可由测试观察。
8. 已经缺失的旧 sidecar返回 normal/no-meta，不进行兼容恢复。

## 验收标准

- H-01：active sidecar经过普通保存和重载仍为 active。
- H-02：退出团队后 sidecar为 archived、CurrentExpert为 none。
- H-03：archived经过多次普通/自动保存和重载仍存在且字段不丢失。
- H-05：active/archived恢复后再次选择专家仍被拒绝。
- H-08：缺失 sidecar不创建新文件、不从其他 artifact推断。
- H-09：损坏 sidecar不被空快照删除或覆盖。
- 单专家清除、首次选择、fork/rewind及 Session删除相关现有测试不回归。

## 聚焦验证

至少运行：

```powershell
go test ./internal/agent -count=1
go test ./internal/control -count=1
go run ./scripts/code_guard
git diff --check
git ls-files --eol -- internal/agent internal/control
```

测试必须直接覆盖空快照、active、archived、损坏文件和显式清除，不接受只测序列化 round-trip。

## 完成回执

- 状态：实现完成，Backend 实现提交已创建。
- 变更文件：
  - `internal/agent/save.go`
  - `internal/agent/save_test.go`
  - `internal/control/controller.go`
  - `internal/control/selected_expert_refresh.go`
  - `internal/control/selected_expert_refresh_test.go`
- 实际变更：
  - `SaveSessionExpert`只接受带有效`ExpertID`的 meta；新增`DeleteSessionExpert`作为显式删除入口。
  - 普通空专家快照成为 no-op，fork的空专家快照同样不创建或删除 sidecar。
  - 单专家清除使用显式删除；active团队清除仍先原子保存为archived，再发布普通当前身份。
  - 自动保存写入前验证已有 sidecar可解析；损坏文件保留原内容并发出可观察 warning。
  - 新增空快照保留团队、损坏文件不覆盖、空保存拒绝和显式删除回归；既有 active/archived、
    重启恢复、选择锁定、fork/rewind及 Session删除测试继续由受影响包测试覆盖。
- 测试结果：
  - `go test ./internal/agent -count=1`：通过。
  - `go test ./internal/control -count=1`：通过。
  - `go run ./scripts/code_guard`：通过。
  - `git diff --check`：通过。
  - `git ls-files --eol -- internal/agent internal/control`：改动文件保持`i/lf w/lf`。
- 偏差、风险与交接：无实现范围偏差；未迁移或重建缺失 sidecar，未修改用户数据。真实 Desktop应用
  验收和最终 PR gate仍由主 Agent在任务 002完成后执行。
