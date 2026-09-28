# Composer 会话审批姿态验收

## 判定口径

每个成功场景同时观察：Composer 选中项、backend `TabMeta/Meta`、controller 实际模式、对应 `<session>.meta`。会话有 sidecar 后，四者必须一致。legacy missing 显示/运行 Ask，但 sidecar 保持缺失，直到用户修改。

## 自动化与真实应用矩阵

| 场景 | 结果 | 证据 |
| --- | --- | --- |
| 普通新会话默认 | 通过 | 隔离配置默认 Ask，新建全局和项目会话均为 Ask；Go/前端测试覆盖 Ask/Auto/YOLO 三种设置默认 |
| 三种显式值 | 通过 | native Composer 实测 Ask、Auto、YOLO 均写入对应 sidecar，配置默认始终为 Ask |
| A/B 隔离 | 通过 | A=YOLO、B=Auto，连续三轮历史往返均恢复各自值 |
| 全局/项目 | 通过 | 全局 B sidecar 为 Auto；隔离本地项目会话 sidecar 为 YOLO |
| 单表面复用 | 通过 | `desktop-tabs.json` 始终仅一个表面 tab，切换全局 A/B 及项目会话时 sidecar 保持独立 |
| 历史重开 | 通过 | 全局 A/B 从历史重开恢复；项目恢复路径由 desktop 包级测试覆盖 |
| 应用重启 | 通过 | 完全停止并重启隔离实例后，A 恢复 YOLO、B 恢复 Auto；项目启动恢复由包级测试覆盖 |
| legacy missing | 通过 | 同时移除 sidecar 与 tab snapshot 字段后启动，UI/runtime 为 Ask，二者均保持字段缺失 |
| legacy 首次修改 | 通过 | 用户改 Auto 后 sidecar 与 tab snapshot 写入显式 Auto，配置默认仍为 Ask |
| fork/检查点 | 通过（自动化） | `internal/agent` 指针深拷贝/父子独立测试覆盖 Ask/Auto/YOLO 与 missing；当前 UI 无直接 checkpoint/fork 入口 |
| 持久化失败 | 通过 | 独占锁定 B sidecar 后 Auto→YOLO：UI 保持 Auto，sidecar 保持 Auto，并显示“保存会话审批姿态失败，请稍后重试。” |
| 快速切换 | 通过 | native UI 三轮 A/B 快速切换无串扰；事务身份重验由 desktop 测试覆盖 |
| Composer 不改默认 | 通过 | 会话内多次改值、项目设 YOLO 后，隔离 `config.json` 的默认仍为 Ask；新会话仍为 Ask |
| bot/automation 边界 | 通过（静态/自动化） | 最终 changed-path 分类未触碰 bot、IM、automation；相关仓库 Gate 通过 |

## 真实验收数据约束

- 使用隔离 `KWU_HOME`/dev profile 和临时全局/项目目录。
- 不读取、修改或清理真实用户会话与设置。
- 失败注入只针对隔离 session sidecar；结束后保留最少必要证据或删除纯临时数据。

## 验收记录

- 状态：通过。
- 环境：隔离 profile `C:\Users\ai03\AppData\Local\Temp\reasonix-composer-approval-ba0f`；Vite `5209`、Wails `34209`；隔离项目 `acceptance-project`。验收结束后进程、端口和文件锁均已释放，数据暂留用于复核。
- 工具：浏览器用于页面与静态状态观察；native 文件夹选择、Composer 操作与 toast 观察使用 Computer Use。
- 自动化：`go test ./internal/agent -count=1 -timeout=180s`、desktop module 的 `go test . -count=1 -timeout=240s`、相关前端定向测试、`pnpm build` 和最终 PR Gate 均通过。
- 偏差/剩余风险：当前产品 UI 没有直接 checkpoint/fork 操作入口，该场景以真实 sidecar 合同和 `internal/agent` 集成测试验收；项目历史/重启以真实项目持久化加 desktop 恢复测试组合覆盖。未推送、未创建 PR。
