# 桌面专家与专家团闭环调查证据

## 1. 基线与方法

- Codegraph 导航基线：主工作区
  `f1777a582545d3146ec184a1a89c1f0a9c754910`。
- 最终源码复核基线：独立 worktree
  `origin/main-v3@36e2623305f1f42e4b27cdd26b5f1b8edc568a49`。
- 主工作区只读；worktree 以 `core.autocrlf=false` 检出。

## 2. 运行与云端证据

2026-07-28 匿名实测：

```text
GET http://192.168.10.243:8080/api/v1/expert-assets/system
Authorization: <absent>
HTTP 200
Content-Type: application/json; charset=utf-8
Body: 18125 bytes，包含 experts 与 expert_groups
```

云仓库 `xiaoxiaotest9527-bit/Reasonix-Cloud` 默认分支当时指向
`cc92852668599821161d88a9c45e8aa24cc1c79b`。其
`internal/experts/handler.go` 仍注册：

```go
r.GET("/expert-assets/system", requireAuth, h.listSystemAssets)
```

因此 8080 部署事实与云仓库源码不一致。本次范围明确不修改云端。

## 3. 桌面根因与重复错误

| 源码位置 | 当前事实 | 结论 |
| --- | --- | --- |
| `desktop/experts_app.go:59-71` | 云模式下 `ListExperts` 每次进入 `listCloudExperts` | 当前不是全局单次获取 |
| `desktop/experts_app.go:271-305` | HTTP 前检查账户状态，匿名直接返回 `AUTH_REQUIRED` | 专家页空白主根因在桌面 |
| `internal/cloud/expert_assets_client.go:98-109` | 系统 GET 使用认证执行器和 Token | 客户端契约仍按保护接口实现 |
| `useExpertCatalog.ts:31-84` | 系统目录、tab 选择和本地状态共用刷新 | 任一状态变化都会重拉目录 |
| `App.tsx:1106,1124-1127` | `canClearSelection` 随 ready/hydrating 改变 | 会话恢复会再次触发 effect |
| `App.tsx:3233,3363,3516` | 会话打开路径显式再次刷新 | 形成典型第三个错误入口 |
| `useExpertCatalog.ts:76-79` | 每个完成的错误各调用一次 `showToast` | 同一错误被多个入口放大 |
| `frontend/src/main.tsx:84-93` | DEV 使用 React `StrictMode` | effect 可能被开发模式重放 |
| `HeartbeatPanel.tsx:2291-2296` | 自动化面板独立调用 `ListExperts` | 系统目录没有唯一前端所有者 |

请求序列保护会抑制部分迟到结果，所以 Toast 数量受时序影响，不保证永远刚好三个。
截图中的三个错误不是三类专家分别失败，而是同一鉴权失败在不同刷新时点完成。

## 4. 缓存与本地数据证据

| 源码位置 | 当前事实 | 结论 |
| --- | --- | --- |
| `desktop/experts_app.go:255-264` | 目录库为用户目录下 `experts.sqlite` | 已有缓存文件 |
| `desktop/experts_app.go:281-289` | 云响应成功后调用 `ReplaceOfficial` | 当前只写缓存，不在失败时读 |
| `internal/expert/store.go:155-193` | 事务删除、重写 official 行 | 失败不会留下半写状态 |
| `internal/expert/store_test.go:305-345` | 测试证明 user 行被保留 | 刷新不必覆盖本地用户记录 |
| `internal/config/paths.go:664-671` | 用户专家路径为设备全局 `experts.json` | 已不按云账号隔离 |
| `internal/control/controller.go:1583-1688` | 已有 JSON 创建、删除、复制 | 本地 CRUD 主体已存在 |
| `selected_expert_refresh.go:11-40` | 已有 JSON 更新和运行中保护 | 无需新建更新体系 |

本机当前 Clouvia/KWU 数据只发现 official SQLite 行，没有当前 schema 的 SQLite user 行；
但源码明确允许并保留旧 `source=user`，不能据单机状态删除兼容路径。

## 5. 云创建分流与本地权限证据

| 源码位置 | 当前事实 | 结论 |
| --- | --- | --- |
| `desktop/app.go:1428-1451` | 有云客户端时创建不走 Controller JSON CRUD | 登录状态改变创建路由 |
| `desktop/app.go:1454-1523` | POST 成功后再写 JSON 镜像 | 存在不完整双写 |
| `desktop/app.go:1526-1568` | 云 ID 禁止本地更新、删除 | 镜像无法闭环 |
| `desktop/app.go:1583-1631` | 云 ID 复制再次调用云创建 | 匿名或云端缺路由时失败 |
| `ExpertDirectory.tsx:940-955,1315-1328` | 云 ID 隐藏编辑、删除 | UI 延续了该不完整模型 |

`isCloudUserExpertAssetID` 只按 `exp_`/`exg_` 前缀判断，没有云端事实校验。删除此特判后，
权限仍可由 `source=official/user` 安全区分，系统 official 保持只读。

## 6. Fixture 证据

- `desktop/cloud_runtime.go:366-424` 只在 DEV 且 `KWU_CLOUD_FAKE=1` 时选择 Fixture。
- `desktop/cloud_runtime.go:516-618` 构造 Fixture 账户、支付和事件能力，但没有设置
  `expertAssets`。
- `desktop/experts_app.go:255-264` 在没有专家云客户端时用 `BuiltinDefinitions` 初始化。

所以 Fixture 专家目录实际是本地内置数据；“刷新”必须重载内置 official 行，不能访问
8080，也不能向用户暗示数据来自云平台。

## 7. 空缓存恢复旧专家会话

| 源码位置 | 当前事实 | 结论 |
| --- | --- | --- |
| `desktop/app.go:553-572` | startup 立即异步恢复 tab，当前没有后端目录初始化前置协调 | 前端稍后刷新无法保证先于旧会话构建 |
| `desktop/model_billing.go:1559-1565` | 真实云模式禁用内置专家 | 空 SQLite 时不能靠 builtin 解析系统专家 |
| `desktop/tabs.go:2873-2937` | sidecar 的专家 ID 在 Controller 构建时立即传入 | 是否恢复专家取决于构建当时的本地目录 |
| `internal/boot/build_config.go:222-239` | 未知的恢复专家会重试为 ExplicitNone | 当前会静默降级为无专家 |
| `internal/boot/boot_test.go:563-592` | 测试明确断言未知恢复专家变为 SourceNone | 该路径是既有确定行为，不是推测 |
| `internal/control/controller.go:2375-2395` | Draft/Locked Resume 用新 prompt 替换 transcript system message | 保存过的旧 system message 不能兜底 |
| `internal/control/scene_state_test.go:61-85` | 测试区分 Draft/Locked 重建与 Legacy 保留 | 当前主流程的场景会实际覆盖 prompt |
| `internal/control/controller.go:1799-1814` | ReloadExpertCatalog 只替换目录指针 | 迟到刷新不会恢复已丢失的选择和 prompt |
| `internal/agent/save.go:120-182` | `.expert` sidecar 只保存 ID、名称、来源 | 本地没有完整专家定义快照可供离线重建 |

该状态可由缓存删除/损坏、profile 迁移，以及现有“重置专家”先清表后云请求失败触发。
因此修复必须协调后台初始化与未解析 tab；保存完整 prompt sidecar 的迁移方案风险更高，
不纳入本次。

## 8. 重置边界

- `desktop/experts_app.go:201-225` 当前先调用 `Store.Reset`，随后云模式再次 `ListExperts`；
  后者失败时数据库已经被清空。
- `internal/expert/store.go:127-152` 的 `Reset` 会 DROP 并重建整张表，无法只清 user。
- 页面确认文案明确承诺删除“自定义专家、收藏和最近使用”，没有承诺清除或联网重载系统目录。

因此新实现需要独立的事务型 user 清理边界；“重置专家”不再复用系统目录刷新。
