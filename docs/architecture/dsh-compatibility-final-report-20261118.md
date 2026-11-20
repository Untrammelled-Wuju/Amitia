# DSH 兼容宿主终版状态报告（2026-11-18）

## 1. 范围与口径

目标：把 DeepSeek Harness（DSH）插件的能力完整映射到 Amitia 原生界面与宿主服务，不新增并行的第二套系统，也不为 DSH 单独做页面。

口径以 `plugins-code/dsh-compatible-host/catalog/compatibility.v1.json` 为准；运行时证据以 `artifacts/dsh-acceptance-20261118/` 为准。

## 2. 当前统计

| 维度 | 结果 |
| --- | --- |
| UI 槽位 | 120 / 120 已映射到 Amitia 原生页面与组件 |
| Client Service | 8 项：implemented 5、partial 3（`layout`、`uiWorkspace`、`workspaces`） |
| Host Service | 69 项：implemented 25、partial 4、delegated 27（官方包提供）、unavailable 13（官方失败态）、pending 0 |

`delegated` 表示能力由官方 DSH 包在容器内原生托管，`unavailable` 是 DSH 自身的失败态定义，二者都不是本插件未完成的工作。

## 3. 本轮累计完成的能力

| 能力 | 落点 |
| --- | --- |
| 轮次事实层 | `host.conversation.turns` 读取已落库轮次检查点，按 `turnSequence` 排序并透出 `text`/`reasoning`/`tool_call`（含 `callId`、参数与结果）块 |
| 消息折叠 | `session-projection.mjs` 把宿主消息与轮次块折叠成 DSH `Message`/`parts`：`user`（含附件 `file` 块）、`assistant`（文本/推理/工具调用）、`tool/result`，并对未配对轮次补发 assistant 消息 |
| 会话表面 | `sessionQuery` 的 `readSession`/`listEvents`/`readSurface`/`readEvent`/`traceEvent` 与 `inspectSession` 统一走折叠结果；插件可 `sessions.registerMessageProjection` 注入自定义投影 |
| 会话写入 | `host.conversation.update` 改标题（带 `ExpectedRevision`）、`host.conversation.turn.steer` 追加用户引导，均走作用域快照 |
| 宿主服务 | `webServer`（含 index tap/injection、升级路由）、`agentDefaultModel`、`connection` 补齐，第三方 DSH 插件可正常激活 |
| 客户端装载 | 客户端源码经宿主 chunk 通道下发并 blob 导入；索引注入的脚本经 CSP 允许的 `/dsh-inline/*` 路由提供；`commandUi`（含 `decorate`）、`commands`、locale 字典注册可用 |
| 会话归档 / 置顶 / 派生 | mesh 层真实写库（`archived`、`archivedAt`、`pinned`、`pinnedAt`、`forkOrigin`），双向可逆 |
| 草稿会话（prepare） | `conversation/create` 的 `draft` 参数：首条消息前不进目录，同 `requestId` 正式创建以 `publishedAt` 升级 |
| 目录选择 | `host.directory.pick` → `ui.directory.pick` 端点 → `window.amitiaDesktop.selectDirectory`，复用 `ui.dialog` 的 SSE 往返 |
| 附件句柄文本 | `llm.fileRequestText` 按官方 `fileHandleText` 语义生成，`ref.uri` 可解析时给出只读副本路径 |
| 子代理命名 | `subagents.resolveName` 按活动 run 表解析，重复/未命中保留候选列表 |
| agent 工厂与公布 | `agents.setFactory` 工厂栈；`agents.announce` 在容器 cordis 上串行派发 `agent/created` |
| 会话查询 | `observeSession` 租约、`readEvent` 前后窗口、`filterSessions` 五类谓词、`filterEvents` 语义文档过滤、`traceSession` 血缘 |
| 工作区控制器 | `list`/`create`/`rename`/`delete`/`initializeDefault` + `archive`/`unarchive`/`pin`/`unpin`（按会话作用域快照写） |
| 子代理提示 | `subagents.prompt` 文本片段按作用域追加，`source=dsh-subagent`；`waitForTreeWork`/`drainDescendants` 改为真实语义 |
| 客户端派生会话 | `uiWorkspace.forkSession` 走 `conversation-fork` 并回调 childId，宿主失败原样抛出 |
| 作业读取错误 | `jobs.read` 的 source 读取失败不再静默跳过，记入 `readErrors` 并由 `jobView` 暴露 |

另修正若干记账错误（5 项 pending 实际已实现；4 项 partial 实际完备）与一处真实缺陷（归档布尔与 `archivedAt` 不同步导致取消归档后资源仍不可用）。

## 4. 运行时验收

`artifacts/dsh-acceptance-20261118/acceptance.mjs`：19/19 通过。

- Core `ready=true`、`/readyz` 7 ready / 0 degraded / 0 blocking
- 插件 0.2.95 `effectiveState=installed`、`failureCount=0`
- UI bridge 可用、无错误边界、composer 正常、会话目录 53 条、目录列举 5 条
- 草稿会话：prepare → 目录不可见 → 同 id 发布 → 可见 → 删除
- 归档/置顶：写入与回滚双向验证（`archived/archivedAt`、`pinned/pinnedAt`）
- 派生：fork `messageCount=9`、目录项 `parentSession` 指回源会话并清理
- 项目顺序：新建两个工作区 → `project-reorder` 把后建项移到前项之前 → 目录顺序与 `order` 值同时前移 → 删除后不再出现在目录
- 会话顺序：新建两条会话 → `conversation-reorder` 把后建项移到前项之前 → 返回顺序与落库 `order` 同时前移

单元与集成：插件 394 项测试全通过；`searchSessions`/`searchEvents` 在会话日志上实现 DSH 命中页结构（分词命中计数、片段、游标分页），不再借用 `host.conversation.search`；后端 `kernel`、`host_api`、`ui_handler`、`devicemesh/business`、`cmd/server` 定向测试通过。

## 5. 未完成项

| 项 | 原因 | 证据 |
| --- | --- | --- |
| 节点 `tool-call` / `steering` 渲染 | Amitia 消息表无 `tool_calls` 字段，`role: "tool"` 只出现在发给模型的请求里，不落库 | `backend/internal/chat/model.go` 与 `llm_client.go` |
| `sessions.enter` / `announce` / `registerMessageProjection` | 需要 DSH `Session` 实例与消息表面（`Message` 对象 + surface 折叠），Amitia 只有会话快照与 `contentSummary` 文本 | `dsh-session/lib/types/surface.d.ts` |
| `sessionQuery.inspectSession` / `searchEvents` / `traceEvent` | 分别需要可传入回调的 Session 实例、跨会话事件检索、事件级来源引用 | 同上 |
| `subagents.sendMessage` | 需要子代理树父子关系与发送者准入（`assertAdmitting`）；缺树结构时实现等于放开任意会话写入，故明确拒绝 | `dsh-subagent` 实现 |
| `workspaceFiles.changes` | 缺 `host.workspace.watch` 的持续变更帧，只有快照读取 | `src/runtime/host-services.mjs` |
| `llm` 附件可读路径 | 只认 `ref.uri` 的 `file://` 或绝对路径；Amitia 无附件落盘与 `attachmentId → 路径` 解析 | 同上 |
| `directoryPickerController` | 依赖 `workspaceRegistry`（官方 Typert 栈），Amitia 对应物是 project 服务 | `dsh-api-workspace-controller` |
| `directoryPicker.pick` 点选 | 端点注册与路由已验证，原生对话框的实际点选需在桌面端人工操作一次 | 本次未执行 |

## 6. 交付清单

| 类别 | 位置 |
| --- | --- |
| 插件源码 | `plugins-code/dsh-compatible-host/src/**`（runtime / client-abi / client-ui） |
| 插件包 | `plugin/amitia-dsh-compatible-host-0.2.95.amitiax` |
| 兼容目录 | `plugins-code/dsh-compatible-host/catalog/*.json`（compatibility / amitia-gap / dsh-abi / platform-support） |
| 单测 | `plugins-code/dsh-compatible-host/tests/*.test.mjs`（367 项） |
| 运行时验收 | `artifacts/dsh-acceptance-20261118/{acceptance.mjs, acceptance-result.json, README.md}`（19/19） |
| 一致性审计 | `artifacts/dsh-acceptance-20261118/{audit-catalog.mjs, audit-services.mjs}` |
| 缺口清单 | `docs/architecture/dsh-remaining-gaps-20261118.md` |
| 后端改动 | `backend/cmd/server/mesh_owned_chat_actions.go`（archive 布尔同步）、`mesh_local_chat_management.go`（draft 可见性）、`kernel_host_adapters.go`、`internal/extension/kernel/{host_api_routes,ui_host_notifier,sse_ui_host}.go` |

## 7. 顺序字段实施结果（已执行）

项目与会话的顺序落在各自文档的 `order` 字段（mesh 资源 body，无需建表迁移），采用稀疏步长 1024 的相邻项中点插入，间隙耗尽时在单个写事务内统一重排：

1. `backend/internal/devicemesh/business/projects.go`：`Project.Order`，创建项目时按当前最大有效顺序 +1024 取值；`ProjectSortOrder` 对未写 order 的历史行回退到 `createdAt` 毫秒值
2. `backend/internal/devicemesh/business/edit.go`：`project` 与 `conversation` 的允许键加入 `order`，并校验非负整数
3. `backend/internal/devicemesh/coordination/chat_management.go`：新增 `beforeConversationId`
4. `cmd/server/mesh_order_plan.go`：`planSparseOrder` 统一计算中点、端部与重排三种情况
5. `cmd/server/mesh_owned_chat_actions.go`：`conversation/reorder` 在工作区内按 `projectId` 分组、跳过草稿与归档项、单次写入
6. `cmd/server/kernel_host_adapters.go`：`host.project.reorder` / `host.conversation.reorder` 适配，项目列表按 `order` 排序并排除已删除、已隐藏项目
7. `internal/extension/kernel`：新增 `host.project.reorder`、`host.conversation.reorder` 方法、权限（`project.manage` / `conversation.manage`）、风险（medium）、副效应（write）、作用域（namespaced）与能力矩阵条目
8. 插件 `workspaceController.insertBefore` / `insertSessionBefore` 接上述两个 host 方法；`sessionsByProject` 按 `order` 排序，`insertSessionBefore` 用会话作用域快照调用

## 8. 结论

`workspaceController` 的顺序方法补齐后，可达成的映射已全部完成并留有测试与运行时证据。剩余未完成项只剩两类：需要宿主新增数据源（工具事件、附件落盘、工作区变更帧）与需要 DSH 内部对象模型（Session/Message/Registry）才能成立的接口；这两类不属于插件层能闭环的范围，已逐条写入缺口清单并保留原始错误语义，未做静默降级或伪造成功。

顺序字段只影响 DSH 侧的 `host.project.list` / `host.conversation.list` 排序；Amitia 原生侧栏仍按置顶与更新时间展示，两者是同一份数据的不同视图，未改动原生侧栏排序。
