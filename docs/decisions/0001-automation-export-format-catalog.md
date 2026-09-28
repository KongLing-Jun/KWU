# ADR-0001：自动化导出格式由后端目录统一决定

## Status

Accepted

## Date

2026-07-08

## Context

自动化结果以 Markdown 保存，但桌面端曾硬编码导出按钮。若前端和后端分别维护格式列表，两端会出现按钮可见但后端不支持、或后端新增格式而 UI 不可见的漂移。

## Decision

后端 `internal/automation` 维护唯一的导出格式目录。读取存在且非空的运行结果时，`RunDetail.availableResultFormats` 返回当前可用格式；桌面端只根据该字段渲染按钮。直接导出请求也使用同一目录规范化和校验。

当前 Markdown 结果支持 `md`、`docx`、`txt`、`json`、`xlsx` 和 `pptx`。结果缺失或为空时不返回可用格式。

## Alternatives Considered

### 前端硬编码格式

实现简单，但会复制后端能力表并产生版本漂移，因此不采用。

### 根据内容猜测格式

例如只在存在表格时显示 XLSX。该方式让 UI 能力随内容启发式变化，难以稳定测试；当前所有格式均由明确转换器提供，因此不采用。

## Consequences

- 新增或移除格式必须先更新后端目录、转换器和测试。
- 前端无需了解转换能力，只消费后端 DTO。
- 已有 `result.md` 无需迁移，可在读取时派生格式目录。
- 格式目录表示转换能力，不保证特定文档的排版质量；转换失败仍作为显式错误返回。

## Evidence

- `internal/automation/export.go`
- `internal/automation/service.go`
- `internal/automation/service_test.go`
- `desktop/frontend/src/features/automation/heartbeat/HeartbeatPanel.tsx`
