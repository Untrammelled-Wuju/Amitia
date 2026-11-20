# DSH 兼容宿主改动清单（2026-11-18）

用于 review 与提交判断。所有改动均在工作区，未执行 `git add` / commit / push。

## 宿主后端（顺序字段补齐新增/改动）

| 文件 | 改动 |
| --- | --- |
| `backend/internal/devicemesh/coordination/chat_management.go` | `ChatManagementRequest` 增加 `Draft bool` |
| `backend/cmd/server/mesh_owned_chat_actions.go` | 抽出 `createManagedConversation`（create/prepare 共用）；归档同时写 `archived` 布尔与 `archivedAt`；fork 保持 |
| `backend/cmd/server/mesh_local_chat_management.go` | 默认目录排除 draft（有消息即视为已发布）；`chatManagementBool` 辅助 |
| `backend/cmd/server/kernel_host_adapters.go` | `ConversationCreateRequest.Draft` 透传到 chat management |
| `backend/internal/extension/kernel/host_api/types.go` | 新增 `host.directory.pick` |
| `backend/internal/extension/kernel/host_api/permission_mapping.go` | pick 映射 `workspace.read`、RiskMedium、SideEffectReadOnly |
| `backend/internal/extension/kernel/host_api/capability_matrix.go` | Directory 能力加入 pick 与理由 |
| `backend/internal/extension/kernel/host_api_routes.go` | `directoryPickRouteHandler` + 路由注册（5 分钟超时）；create 支持 `draft` |
| `backend/internal/extension/kernel/ui_host_notifier.go` | `DirectoryPickHost` 接口、`ErrDirectoryPickHostUnavailable`、Default 桩 |
| `backend/internal/extension/kernel/sse_ui_host.go` | `PickDirectory` / `ResolveDirectoryPick` / `FailDirectoryPick` 与 pending 表 |
| `backend/internal/extension/kernel/ui_handler/handler.go` | `DirectoryPickResolver`、`/api/extensions/ui/directory-pick-response`、feature 白名单加 `ui.directory.pick` |
| `backend/internal/extension/ui_api.go` | 接线 `SetDirectoryPickResolver` |
| `backend/cmd/server/mesh_owned_chat_prepare_test.go`（新） | 草稿隐藏/发布、create 升级、冲突拒绝 |
| `backend/cmd/server/mesh_owned_chat_archive_test.go`（新） | 归档布尔与时间戳同步、`ResourceAvailable` 判定 |
| `backend/internal/extension/kernel/host_api_routes_directory_pick_test.go`（新） | pick 成功/不可用/原始错误透传 |
| `backend/internal/devicemesh/business/projects.go` | `Project.Order`；创建时按最大有效顺序 +1024；`ProjectSortOrder` 未写 order 时回退 `createdAt` 毫秒 |
| `backend/internal/devicemesh/business/edit.go` | project/conversation 允许键加入 `order` 并校验非负 |
| `backend/internal/devicemesh/coordination/chat_management.go` | 增加 `BeforeConversationID` |
| `backend/cmd/server/mesh_order_plan.go`（新） | `planSparseOrder`：中点、端部、间隙耗尽重排 |
| `backend/cmd/server/mesh_owned_chat_actions.go` | `conversation/reorder` 工作区分组重排；创建会话写入 `order` |
| `backend/cmd/server/mesh_local_chat_management.go` | 注册 `conversation/reorder` 操作 |
| `backend/cmd/server/kernel_host_adapters.go` | 会话目录按实际消息数回填 `messageCount`/`blank`（文档计数为 0 时用 search 的 total）；会话读取透出消息 `id` 与 `attachments`（sha256 只读副本 + absolutePath）；会话目录透出 `parentSession`（取会话文档 `forkOrigin.conversationId`）；`ReorderConversation` / `ReorderProject`；项目列表按 `order` 排序并排除已删除、已隐藏；`projectUpdateChanges` 显式判空（修复 typed-nil 写入 `null` 导致的“内容不能为空”） |
| `backend/internal/extension/kernel/host_api/types.go` | 新增 `host.project.reorder`、`host.conversation.reorder` |
| `backend/internal/extension/kernel/host_api/permission_mapping.go` | 两个方法：manage 权限、RiskMedium、SideEffectWrite、namespaced |
| `backend/internal/extension/kernel/host_api/capability_matrix.go` | Project 路由与 Conversation Order 条目 |
| `backend/internal/extension/kernel/host_api_routes.go` | 两个 reorder 路由实现与注册；`projectUpdate` 支持 `order` |
| `backend/internal/extension/kernel/container_builder.go` | `WithProjectReorderer` / `WithConversationReorderer` |
| `backend/cmd/server/services.go` | 装配两个 reorderer |
| `backend/cmd/server/kernel_attachment_materialize_test.go`（新） | 附件物化：只读副本、哈希校验、非法编码 |
| `backend/cmd/server/mesh_order_plan_test.go`（新） | 中点/端部/重排/异常锚点、会话重排落库、`projectUpdateChanges` 判空 |

## 宿主前端（1 个文件）

| 文件 | 改动 |
| --- | --- |
| `front/src/ui-runtime/mainPanelRegistry.ts`（新） | 主面板注册表：`registerMainPanel`/`resolveMainPanel`/`listMainPanels`，拒绝空 id、空标题与重复注册 |
| `front/src/views/main-panel/MainPanelHostView.vue`（新） | `/main-panel/:panelId` 容器页：已注册面板渲染组件，未注册显示 id 与已注册列表 |
| `front/src/router/builtinRoutes.ts` | 新增 `mainPanelHost` 路由 |
| `front/src/composables/useUIHostSSE.ts` | 注册 `ui.directory.pick` 端点；`ui_directory_pick` 事件处理与回填 |

## 插件（`plugins-code/dsh-compatible-host`）

| 文件 | 改动 |
| --- | --- |
| `src/runtime/host-services.mjs` | workspaceFiles.changes 轮询产出 ready/change/absent 帧；directoryPickerController 以 native 能力委派 pick/list/createDirectory；workspaceController.insertBefore/insertSessionBefore 接两个 reorder；sessionsByProject 按 order 排序；subagents.sendMessage 按会话血缘树根授权后按作用域追加；sessions.prepare 客户端化、subagents.prompt/resolveName、agents.setFactory/announce、workspaceController、`llm.fileRequestText`、jobs.readErrors、`waitForTreeWork`/`drainDescendants` 真实语义 |
| `src/runtime/host-service-bridges.mjs` | sessionQuery：traceEvent 按 surfaceOp/sourceEventSeqs 求替换链与来源；observeSession/readEvent 窗口/filterSessions/filterEvents/traceSession、sessionHeader 增 `parentSession` |
| `src/runtime/index.mjs` | 新增 `agent-preset-write` / `agent-preset-read` 命令；容器上下文注入 runtimeHooks（供 agents.announce）；conversation-create 支持 draft；新增 `project-reorder` / `conversation-reorder` 命令 |
| `src/client-abi/runtime-services.mjs` | `uiWorkspace.startSession` 支持 `prompt` 与 `agentPreset`、`openWorkspace` 支持工作区级 `agentPreset`；`sessions.prepare`、`uiWorkspace.forkSession`、`workspaces.insertBefore` / `workspaces.insertSessionBefore` |
| `amitia-extension.json` | 版本 0.2.80；command schema 增 `draft`、`beforeProjectId`、`beforeConversationId`；action 枚举增两个 reorder |
| `scripts/catalog-policy.mjs` / `scripts/host-service-routes.mjs` | 能力状态与 reason 按证据修正 |
| `catalog/*.json`、`src/runtime/catalog-summary.generated.mjs` | 重新生成 |
| `tests/*` | 覆盖新增能力、顺序方法与记账修正（顺序相关 5 项由“明确失败”改为真实行为断言） |

## 文档与交付物（未提交）

| 文件 | 用途 |
| --- | --- |
| `docs/architecture/dsh-remaining-gaps-20261118.md` | 缺口清单（更新） |
| `docs/architecture/dsh-compatibility-final-report-20261118.md` | 终版报告（新） |
| `docs/architecture/dsh-compatibility-delivery-summary-20261118.md` | 一页纸摘要（新） |
| `artifacts/dsh-acceptance-20261118/{acceptance.mjs, acceptance-result.json, README.md}` | 运行时验收 19/19（含项目与会话重排） |
| `artifacts/dsh-acceptance-20261118/{audit-catalog.mjs, audit-services.mjs, verify-consistency.mjs}` | 一致性审计（9/9） |
| `DESKTOP_PET_FINALIZATION_SHA256SUMS.txt` | freeze 清单随源码改动刷新 |

## 最后一次验证

- 插件单测：372/372
- 一致性校验：9/9（catalog 计数、包版本、文档统计、运行版本）
- 运行时验收：19/19（0.2.80）
- 后端：`go vet` 覆盖 `cmd/server`、`internal/extension/kernel/...`、`internal/devicemesh/...`；`internal/extension/kernel` 全子包、`internal/devicemesh/business`、`cmd/server` 排序与聊天定向用例通过（`trusted_service` 的生命周期端口用例在并行负载下偶发超时，单独复跑通过；`TestOwnedChatCleanup*` 三条为既有失败，未改动其语义）

## 未完成与待决策

见 `dsh-compatibility-delivery-summary-20261118.md`：顺序字段已落地（不再属于待决策），剩余为 `directoryPicker` 人工点选与需要宿主新增数据源或 DSH 内部对象的结构性缺口。
