# DSH 兼容宿主审计（2026-11-18）

配套交付清单见 `dsh-delivery-final-20261118.md`（验收 19 项、能力清单、复现命令）。

口径：以 `plugins-code/dsh-compatible-host/catalog/compatibility.v1.json` 为准；插件版本 0.2.80；Core 为同一份源码重建（Core SHA-256 `d04a8be2faf1a3d4df33688cdd0d24087d225c5d2381d53ac8c11c3c03ce0f9f`，与 `desktop/resources/core/macos-arm64/AmitiaCore` 实测哈希一致）。

## 1. 服务面盘点

| 面 | 数量 | 状态 |
| --- | --- | --- |
| UI 槽位 | 120 | 120 已映射，0 待办 |
| Host Service | 69 | implemented 25、partial 4、delegated 27、unavailable 13、pending 0 |
| Client Service | 8 | implemented 5、partial 3 |
| Host Event / Tool / Configuration | 74 / 77 / 163 | 由官方包提供或已接线 |

自动校验：`node artifacts/dsh-acceptance-20261118/audit-services.mjs`（实例化宿主服务并与 catalog 逐条比对）、`node artifacts/dsh-acceptance-20261118/verify-consistency.mjs`（汇总计数与重算一致、包/运行时/文档版本一致）。

## 2. 剩余 partial 4 项

| 能力 | 缺的宿主能力 | 当前行为 | 证据 |
| --- | --- | --- | --- |
| `sessions`（`enter`/`announce`/`registerMessageProjection`） | DSH `SessionStore` 的 Session 对象、append 公布原语与 surface 折叠；Amitia 只有会话快照与事件桥 | 三个方法保留原始缺口报错，`create`/`prepare`/`fork`/`list`/`flush` 已接 Amitia 会话目录 | `src/runtime/host-services.mjs` |
| `sessionQuery.inspectSession` | 可传入回调的 live DSH `Session` 实例 | 报 `DSH_HOST_OPERATION_UNAVAILABLE`，其余 12 个方法已实现（含 `searchSessions`/`searchEvents`/`traceEvent`） | `src/runtime/host-service-bridges.mjs` |
| `llm` 附件可读路径 | 会话消息投影把 `message_attachments` 的 `resourceUri`/落盘路径带进 DSH 消息表面；官方 `fileRequestText(ref): string` 是同步函数，无法在调用内查宿主 | 无 `uri` 时返回“执行环境无法访问可读路径，不要声称已读取”的原始句柄文本，不伪造成功 | `src/runtime/host-services.mjs` |
| `directoryPicker` | 桌面原生对话框点选的人工实测；移动端未注册 `ui.directory.pick` | 桌面端点已注册并可点选；移动端原样返回 `ui_host: directory pick host unavailable`；`directoryPickerController` 已按 native 能力委派 pick/list/createDirectory | `sse_ui_host.go` / `useUIHostSSE.ts` |

结论：5 项都不是“实现忘了做”，而是需要宿主新增数据源或 DSH 运行时对象。按项目规则不在插件内另造一套与 Amitia 会话并行的 Session 存储，保持原样报错、逐条记录。

## 2.1 为什么这 4 项不应该在插件里硬做

核查证据：

- `amitia-extension.json` 没有 entries，插件的 `activatePackage` 只加载本插件条目；`dsh-agent-loop`、`dsh-tool-*` 等官方运行时包**未被装载**（`agent-loop` 只出现在 `sessionProjections` 缺投影的错误文案里）。
- 因此 `SessionStore.enter/announce/registerMessageProjection`、`Session.deriveMessages()`、`inspectSession` 的回调 Session、以及模型工具（`tool.title.sendMessage` 等）在容器内都没有调用方。
- 唯一会消费它们的前提是“在容器里跑 DSH 自己的 agent loop / 工具运行时”。那等于把 Amitia 的会话与模型调用链换成 DSH 的，属于另建一套系统，与项目“不在已有系统上重复造第二套”的规则冲突。
- `llm` 附件路径同理：即使补了同步路径解析，也没有 DSH 消息投影产生带 `uri` 的 `FileAttachmentRef`。

因此这 4 项按“缺宿主能力”记录并保留原样报错，而不是用简化实现冒充完成；插件侧改为把能力用在真正被消费的面上（DSH 客户端 UI 经 `ctx.workspaceController`/`workspaceFiles`/`sessionQuery`/`directoryPicker` 等服务访问 Amitia 数据）。


## 2.2 消息表面：事实层 → 折叠层 → DSH Message

此前 DSH 会话视图只有 `contentSummary` 文本快照，工具调用、推理段、附件在兼容层不可见。本轮按「单一事实源 + 复用一份折叠」重建消息表面：

1. **事实层（宿主已有数据，不新建存储）**：`host.conversation.turns` 读设备归属业务层已持久化的轮次检查点（`kind=checkpoint`、`runtime.blocks`），返回 `text` / `reasoning` / `tool_call` 块（含 `callId`、`payload.arguments`、`payload.result`、`status`）；`host.conversation.read` 的消息记录补充 `requestId`，用于把消息与轮次配对。两个接口都只读，权限沿用 `conversation.read`，Owner 校验在 handler 内按会话作用域完成。
2. **折叠层（唯一实现）**：`modules/dsh-compatible-runtime/dist/session-projection.mjs` 负责 `buildSessionProjection`（消息 + 轮次 → DSH 契约事件：`turn/start`、`step/start`、`user/message`、`session/title`、`assistant/message`、`tool/result`、`step/end`、`turn/end`）与 `foldSessionSurface`（append/replace 表面折叠 + 注册式消息投影）。附件折叠为 `file` 块（`FileAttachmentRef`：`attachmentId`/`name`/`bytes`/`mediaType`/`uri`），`llm.fileRequestText` 直接读取其中的可读路径。消息按 `timestamp` 归位为时间正序（宿主返回为倒序），未配对的轮次块仍会补出助手消息与工具结果，保证工具调用不丢失。
3. **消费面**：`sessionQuery.readSession/listEvents/readSurface/readEvent/observeSession/traceEvent/searchEvents/traceSession` 全部基于折叠视图；`sessionQuery.inspectSession(sessionId, read, signal)` 把只读会话视图（`id`/`header`/`events`/`surfaceEvents()`/`messages()`/`readSurface()`/`title()`）交给读取回调，写入调用直接返回 `DSH_SESSION_INSPECT_READ_ONLY`；`sessions.registerMessageProjection` 注册宿主侧折叠投影（同 `type` 不可重复注册，返回释放函数），投影返回非法结构或改写不在表面上的消息时原样报错。
4. **不回退到快照**：持久化 `dsh-session:<id>` 内已存在表面事件时以该日志为准（容器内 DSH 会话自有日志），否则由事实层派生，两者不混算、不改写既有 seq。

运行时证据：真实会话 `84d454cc-a0bb-5900-b922-36672cc3ce31` 折叠出 33 个事件、12 个表面节点，助手消息含 2 个 `tool-call` 块，`tool` 消息携带 `call_01`/`call_02` 对应 `toolCallId`，折叠结果与原始 `payload.result` 一致；工具调用块来自持久化检查点，非实时流补造。

## 3. 验证矩阵（可复现）

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| 插件单测 | `AmitiaData/node/bin/node --test plugins-code/dsh-compatible-host/tests/` | 372/372 通过 |
| 运行时验收 | `AMITIA_CORE_BASE=http://127.0.0.1:<port> AMITIA_LOCAL_TOKEN=$(cat AmitiaData/security/local-token) node artifacts/dsh-acceptance-20261118/acceptance.mjs` | 19/19 通过 |
| 一致性审计 | `node artifacts/dsh-acceptance-20261118/verify-consistency.mjs` | 9/9 通过 |
| 后端编译 | `go build ./...`（backend） | 通过 |
| 后端静态检查 | `go vet ./cmd/server/ ./internal/extension/kernel/... ./internal/devicemesh/...` | 通过 |
| devicemesh 全量 | `go test ./internal/devicemesh/... -count=1` | 全部通过 |
| extension 全量 | `go test ./internal/extension/... -count=1` | 78 个包通过；`trusted_service` 的 `TestLifecycleStopRemovesChildrenAfterParentExits` 并行负载下超时，单独复跑 2.4s 通过（既有抖动） |
| Core 构建 | `node desktop/scripts/build-amitiacore.mjs` | PASS，输出 `CORE_BUILD_SHA256=41b75e3b…`（该值即 Core 二进制 SHA-256） |
| 运行时健康 | `GET /api/health`、`GET /readyz` | 200，7 ready / 0 degraded / 0 blocking |

运行时覆盖到的真实链路：会话草稿（`draft` → 发布 → 可见 → 清理）、归档/取消归档/置顶/取消置顶、fork（目录项 `parentSession` 指回源会话）、项目重排、会话重排、目录列举、UI 桥就绪。

产物级覆盖（直接加载已安装 generation 的 `dist/*.mjs` 驱动）：`searchEvents`/`searchSessions` 命中与分组、`workspaceFiles.changes` 的 `ready`/`change`/`absent` 帧、`sessionQuery.traceEvent` 的替换链与来源派生。

## 3.1 已修复缺陷：会话目录计数与 blank 失真

- 现象：`host.conversation.list` 的所有会话 `messageCount=0`、`blank=true`，但 `host.conversation.read` 能读到 9 条消息。
- 根因：目录从会话文档的 `messageCount` 字段取值，而 mesh 消息追加路径不维护该字段（全仓仅 `chat_import.go` 在导入时写入）。
- 修复：目录构建时若文档计数为 0，用 `ManageChat` 的 `search` 操作（`PageSize=1` 返回 `total`）按会话统计真实消息数；统计失败直接返回错误，不回退成 0。
- 运行时验证：同一批会话现在返回 `messageCount=9 / blank=false`、`messageCount=4 / blank=false`，空会话仍为 `0 / true`，与 `host.conversation.read` 的 9 条一致。

## 4. 已知既存失败（与本轮改动无关，未做范围外改造）

- `cmd/server` 的 3 条 `TestOwnedChatCleanup*` 用例断言「请至少选择一个清理条件」，为既有失效断言。
- `internal/extension/kernel/trusted_service` 的生命周期用例在整树并行跑时偶发超时，单独复跑通过。

### 会话写入与运行时控制（0.2.80）

- `sessions.rename(title)` → `host.conversation.update`：复用设备归属业务层 `Edit` 的 `conversation.title` 键（与侧边栏重命名同一写入路径），按 `ReadResource` 的 `revision` 做 CAS，返回值带 `revision` 与 `acknowledgement`。实测：创建 → 改名（revision 2）→ 目录标题变为新标题 → 删除。
- `sessions.cancel()` → 插件命令 `conversation-interrupt` → `host.conversation.turn.interrupt`（与既有中断通道同一条），无运行中轮次时返回宿主的 `state_conflict`，不再报「没有 client command action」。
- 转向：插件命令 `conversation-steer` → `host.conversation.turn.steer` → `conversationstream.Manager.Steer`（复用既有 Steer/ConsumeSteer 通道）；轮次不存在时明确报「当前会话没有可转向的轮次」。
- `sessions.loadThrough(seq)` 改为按 `seq` 截取事件窗口，未覆盖时按翻页上限扩展，非法 `seq` 直接报错。
