# 专家会话返回可见跳动进度

## 当前阶段

- 阶段：Stage 3 已完成；任务 001/002 均完成。
- 最终验收基线：clean HEAD `0be686022134ebab1d89fe461e4f398b2c06b6a2`。
- 交付范围：persisted-only 专家 presentation、专家选择事务保存、共享虚拟几何恢复、稳定后一次揭示。
- 排除项保持不变：未做 Transcript keep-alive、历史无 sidecar 数据迁移或虚拟器替换。
- 远端状态：未 push，未创建或更新 PR。

## 当前结论

- clean Wails deep-offset 验收在 canonical offset 物理不可达时保持同一 identity，停在 signed correction
  指向的最近 bottom boundary；普通长会话同时证明非边界 exact offset 仍以误差 0 完成。
- 同一次专家真实返回内 trace 明确经历 cache miss→write→hit，覆盖已批准的 cache miss/hit 两个边界；
  cache 是 Transcript 实例内加速层，不需要把它误写为两个独立返回场景。
- deep-offset 返回 canonical `790.800293px`，可信 measured offset `611.600098px`，正向 correction
  `179.200195px`；scrollTop `6281.600098px` 已达 max `6281px`。恢复 39 帧、连续稳定 30 帧后正文揭示，
  seek 1、visible jump/summary/incomplete 均为 0，canonical 未改写且无 bottom 写入。
- normal exact 返回 canonical `-88.800003px`，最终 error 0、DOM success，scrollTop `12223.200195px`/
  max `12663px` 明确不在边界；同样 seek 1、39 帧/稳定 30 帧且无可见跳动。详细数据与证据路径见
  [evidence.md](evidence.md)，执行回执见 [任务 001](tasks/001-建立双恢复回归并最小收敛修复.md) 与
  [任务 002](tasks/002-完成原生验收与独立复审.md)。

## 当前验证快照

- Frontend：expert `213/213`；restore `21/21`；anchor `32/32`；trace diagnostics `13/13`；
  typecheck/build 通过。`test:typecheck` 不含本次两个测试文件错误，仅剩未改六类 baseline。
- Go：affected control/boot/serve tests、Code Guard、baseline、`go vet ./...` 通过。
- Contract：OpenSpec strict、独立 diff/LF/clean 复审均通过；独立复审无 P0/P1/P2。
- 完整 `go test ./...` 是非 required extra evidence；Windows 并行 boot 阶段两次超过 300 秒且无断言失败，
  tasktree 缺失验收文档由 origin/main-v3 只读证据确认为既有基线，不构成交付阻断。

## 后续关口

- 当前无实现、验证或产品决策阻塞。
- 如需发布或 PR 操作，必须另行获得授权并按仓库 PR Gate 执行。
