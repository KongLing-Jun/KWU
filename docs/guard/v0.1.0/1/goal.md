# 阶段 1：将现有守卫建设成可靠的 CI 增量门禁

## 1. 阶段名称

可靠的 CI 增量门禁。

## 2. 阶段背景

现有 Code Guard 能发现结构问题，但没有直接进入 GitHub CI，默认检查还可能写回 baseline。本地 pre-push 只执行 `go vet ./...` 且可以不安装或被绕过。必须先建立可靠基础门禁，再增加高误报规则。

## 3. 当前问题

- CI 不直接运行 Code Guard；
- 普通检查可能修改 baseline；
- PR 比较依赖当前工作区 baseline，而不是基础分支；
- Root、Desktop 和 Frontend 没有一个统一的完整入口；
- Frontend `test:all` 未进入主 CI；
- pre-push 与 PR Ready 能力不一致；
- 报告没有统一区分新增、恶化和 debt。

## 4. 阶段目标

将已有确定性检查组织成默认只读、跨模块、基础分支感知、可在本地复现且 CI 不可绕过的增量门禁。

## 5. 非目标

- 不把碎片率和重复代码设为 error；
- 不启用新的 40/60 行结构阈值；
- 不治理 484 条历史 debt；
- 不引入变更覆盖率门槛；
- 不实现全量调用链推断。

## 6. 前置依赖

- 阶段 0 完成；
- baseline、严重度和豁免语义冻结；
- Root、Desktop、Frontend 的权威命令明确；
- CI 能获取 PR 基础分支提交。

## 7. 输入条件

- 当前 Root baseline 484 条；
- 当前 Code Guard 支持 strict、JSON 和 freeze；
- Makefile 已有 `code-guard`、`pr-ready`、`hooks`；
- `.githooks/pre-push` 当前执行 `go vet ./...`；
- GitHub CI 已有 Root、race、Desktop、lint、coverage jobs；
- Frontend `package.json` 已定义 `test:all`。

## 8. 扫描对象

- Root Go 生产代码和既有 baseline；
- Root Go `gofmt` 洁净、全包 build、vet、test、race、lint；
- Desktop Go 格式、tidy、build、vet、test、lint；
- Frontend lockfile、build、typecheck、`test:all`；
- baseline 文件本身及其 Git 状态；
- PR 基础分支和当前分支的 finding 集合。

## 9. 本阶段新增守卫

- 默认只读 Code Guard；
- 基础分支 baseline 比较；
- 新增/恶化/debt/改善/消失分类；
- baseline 洁净与跟踪状态检查；
- Root/Desktop/Frontend 统一 PR Ready；
- Frontend `test:all` CI 门禁；
- 可选 pre-push 统一入口；
- 人读文本和 JSON 统一报告。

## 10. 规则定义

### 增量比较

- 基础分支不存在的 key、当前超限：`new`；
- 同 key 当前值大于基础值：`worsened`；
- 同 key 当前值小于基础值但仍超限：`improved`；
- 基础 key 当前不再超限：`resolved`；
- 同 key、同值：`debt`；
- 规则阈值或版本变化必须单独报告，不能伪装成代码变化。

### 检查与更新分离

- `make code-guard`：只读检查；
- `make code-guard-baseline-update`：唯一显式更新入口；必须先对基础分支执行只读比较，存在新增或恶化时拒绝更新，只允许写入已经确认的改善和消失；如复用现有 `--freeze`，只能用于生成临时候选，不能直接覆盖工作区 baseline 绕过比较；
- `make pr-ready`：完整且只读；
- CI 不调用 baseline 更新入口。

## 11. 阈值

阶段 1 沿用现有 error 阈值：

| 指标 | Error |
|---|---:|
| 文件有效行数 | 800 |
| 函数有效行数 | 80 |
| 嵌套深度 | 4 |
| 圈复杂度 | 10 |
| 单包生产文件数 | 30 |

本阶段不启用新的 warning 结构阈值。

## 12. 硬门禁

本阶段建立后续阶段必须持续继承的基础门禁：

- 新增 finding；
- 历史 finding 恶化；
- baseline 缺失、未跟踪、损坏、重复 key 或版本不兼容；
- 普通检查修改 baseline；
- Root Go `gofmt`、build、vet、test 失败；
- race 或 lint 失败；
- Desktop 格式、tidy、build、vet/test/lint 失败；
- Frontend install、build、typecheck 或 `test:all` 失败；
- 无法读取基础分支 baseline；
- 本地完整检查与 CI 使用不同规则。

## 13. Warning

- finding 改善但 baseline 尚未显式刷新；
- finding 仅发生行号漂移；
- pre-push 未安装；
- 可选的 govulncheck 信息性结果；
- CI 时间显著增加但尚未超过项目约定预算。

## 14. Debt 处理

- 当前 484 条只展示汇总，默认不逐条刷屏；
- 支持通过 JSON artifact 查看完整 debt；
- debt 值保持不导致失败；
- debt 不能因规则 key 漂移而静默丢失。

## 15. Baseline 处理

- CI 从基础分支提取 baseline 到临时路径；
- 当前分支扫描结果与该快照比较；
- 工作区 baseline 只用于完整性和显式更新检查；
- 检查结束后验证工作区和暂存区 baseline diff 均为空；
- 改善后由开发者运行独立更新命令并评审 diff；
- 显式更新命令在存在新增或恶化时必须失败，不得将新问题吸收为历史 debt。

## 16. 豁免方式

阶段 1 只接受阶段 0 格式完整、未过期的具体规则豁免。不得豁免：

- 编译或测试失败；
- baseline 格式损坏；
- 检查写回 baseline；
- 无法取得基础分支；
- Frontend/Root/Desktop 验证命令未执行。

## 17. 本地触发方式

### 17.1 从当前过渡门禁升级

阶段 1 以现有“同步 `main-v3`、解决冲突、执行六项检查、任一失败即停止”的过渡门禁为起点，将其中的检查能力升级为真正只读、跨模块并与 CI 对齐的完整门禁：

- 普通 Code Guard 不再因 finding 改善而写回 baseline，更新只允许通过显式入口完成；
- PR Ready 从当前 Root Go 与 baseline 最小检查扩展到 Root Go、Desktop Go 和 Frontend；
- 本地完整入口与 CI 使用同一规则、阈值和验证组合；
- 同步最新 `main-v3` 和解决冲突继续作为运行完整验证前的流程前提，检查命令本身不隐式修改分支历史。

### 17.2 计划入口

计划提供：

```sh
make guard-fast
make code-guard
make pr-ready
make hooks
```

- `guard-fast`：变更文件、受影响包和快速静态检查；
- `code-guard`：全量只读 Code Guard；
- `pr-ready`：与 CI 对齐的完整验证；
- `hooks`：可选安装统一 pre-push 入口。

## 18. CI 触发方式

- 对 `main-v3` 的 pull request 和 push 执行；
- Code Guard job 在 checkout 后获取基础分支 baseline；
- Root、race、Desktop、Frontend、lint 与 Code Guard 结果均进入合并检查；
- 保存 JSON 报告和简要 step summary；
- CI 只调用检查命令，不调用更新命令。

## 19. 失败输出格式

文本示例：

```text
ERROR [complexity] new internal/x.go:42 Service.Run base=none current=14 limit=10
  remediation: split independent decision branches and add focused tests
```

汇总必须显示：新增数、恶化数、改善数、消失数、debt 数、warning 数和最终退出结论。

## 20. 开发建议

- 优先改造现有 `scripts/code_guard`，不创建平行命令；
- 比较和报告逻辑从扫描逻辑中分离；
- Makefile、Hook 和 CI 只做编排；
- JSON schema 保持版本字段；
- 基础分支获取失败时 fail closed；
- Frontend 复用现有 `test:all`。

## 21. 实施注意事项

- 默认模式行为变化需要覆盖“改善但未更新 baseline”的迁移说明；
- Windows 不依赖 Bash-only Hook 实现；
- CI PR 安全边界不得执行不受信任分支提供的高权限脚本；
- baseline compare 使用基础分支内容，不能相信 PR 修改后的比较器输入；
- 不清理用户现有工作区变更。

## 22. 对应测试

- 默认检查对改善场景不写 baseline；
- 显式更新命令能够写入且只写 baseline；
- 显式更新只接受改善和消失，存在新增或恶化时失败；
- 新增、恶化、改善、消失和 debt 分类；
- baseline 缺失、损坏、重复、未知版本；
- 局部扫描不错误删除全量 baseline 项；
- 文本和 JSON 数量一致；
- Hook 在 macOS、Linux、Windows 使用统一入口；
- CI 命令组合的契约测试；
- Frontend `test:all` 在 CI 路径实际执行。

## 23. 阶段交付物

- 只读 Code Guard；
- 显式 baseline 更新命令；
- 基础分支比较；
- 分类和报告模型；
- Root/Desktop/Frontend 统一入口；
- CI Code Guard job；
- Frontend `test:all` 门禁；
- 可选跨平台 pre-push；
- 单元和集成测试；
- 使用说明。

## 24. 验收标准

- 人为新增或恶化一条问题时本地和 CI 均失败；
- 改善或删除问题时检查通过且 baseline 不被修改；
- 显式更新后 baseline diff 可审查；
- 新增或恶化存在时显式更新入口拒绝写入；
- 当前 484 条保持 debt，不阻塞；
- Root/Desktop/Frontend 验证全部进入 CI；
- 未安装或绕过 Hook 时 CI 仍失败；
- 输出能定位问题并解释分类；
- CI 与本地完整检查结论一致。

## 25. 完成定义

现有确定性守卫已经成为默认只读、跨模块、基础分支感知、CI 不可绕过的增量门禁，且测试证明不会静默修改 baseline。

## 26. 进入下一阶段的条件

- 阶段 1 完成定义满足；
- 增量比较和报告稳定；
- CI 运行时间可接受；
- Root/Desktop/Frontend 无漏检；
- 阶段 2 可以复用 finding、baseline、豁免和报告模型。

## 27. 风险

- 基础分支获取或浅克隆导致比较错误；
- 默认行为迁移造成开发者误解；
- 多模块命令增加 CI 时间；
- Hook 平台差异造成结论不一致；
- 改善不自动写 baseline 后可能出现 stale baseline，需要清晰提示。

## 28. 暂不处理事项

- 40/60 新结构阈值；
- 占位与注册链路规则；
- 碎片和重复代码；
- 变更行覆盖率门槛；
- 484 条历史 debt 修复。
