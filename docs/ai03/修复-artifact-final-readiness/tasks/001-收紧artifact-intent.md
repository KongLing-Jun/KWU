# 任务 001：收紧 artifact intent

## 目标与产物

阻止工具、命令、API、函数等元语言中的中文动作词制造匿名 artifact，同时保留明确命名、
真实匿名和真实多产物合同。交付 parser 实现、正负回归测试和任务完成回执。

## 允许范围

- `internal/agent/artifact_contract.go`
- `internal/agent/artifact_contract_helpers.go`
- 对应 parser/readiness 测试
- 本任务回执和 `../progress.md`

禁止放宽 evidence distinct matcher、freshness、source exclusion，禁止为完整提示字符串加白名单。

## 实施与验收

先增加失败测试，至少覆盖：

- `调用一次写文件工具，创建 marker.txt` → 一个命名 target；
- `使用图片生成 API 创建 cover.png` → 一个命名 target；
- `写一个文件` → 一个匿名 target；
- `写一个说明文件，再创建 marker.txt` → 两个 targets；
- 英文 `use the write file tool to create marker.txt` 不产生额外 target。

实现采用局部分句的元语言资格判断，不能事后删除所有 anonymous+named 组合。

聚焦验证：

```powershell
go test ./internal/agent -run 'ArtifactRequirement|FinalReadiness|ArtifactIntent' -count=1
```

## 状态与回执

| 项目 | 值 |
| --- | --- |
| 状态 | 已完成 |
| 修改文件 | `artifact_contract.go`、`artifact_contract_helpers.go`、`artifact_contract_test.go` |
| 测试结果 | 新增红测先失败；实现后 parser/readiness 聚焦测试通过 |
| 偏差与风险 | 未改变 matcher；剩余风险是工具名称长尾，由高置信度局部结构和正负测试控制 |

实现把 action 后的“artifact object + 工具/命令/API/接口/函数”，以及 action 前 artifact
object、action 后工具类型的中文复合名称识别为元语言。判断只跳过该 action，后续真实
`create marker.txt` 仍独立解析；“写一份工具说明文档”和真实 generic+named 双交付保持不变。

验证：

```text
go test ./internal/agent -run TestArtifactRequirementIgnoresToolReferenceActions -count=1
go test ./internal/agent -run 'RF003|ArtifactRequirement|FinalReadiness|ArtifactIntent|StructuredArtifact' -count=1
```

两项均通过。
