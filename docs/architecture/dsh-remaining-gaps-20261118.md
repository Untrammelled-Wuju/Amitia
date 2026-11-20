# DSH 兼容宿主剩余缺口清单（2026-11-18）

口径：以 `plugins-code/dsh-compatible-host/catalog/compatibility.v1.json` 为准，缺口说明逐条取自代码中的错误文案，不写推测。

## 统计

- UI 槽位：120/120 已映射。
- Host Service：69 项 —— implemented 25、partial 4、delegated 27、unavailable 13、pending 0（逐条状态以 `catalog/compatibility.v1.json` 为准，2026-11-18 夜更新）。
- Client Service：8 项 —— implemented 5、partial 3。
- sessions 的 client 侧已全部接入（含 prepare 草稿语义）；host 侧仅剩 SessionStore 原语。

## partial 4 项的逐条缺口

| 能力 | 缺的宿主接口 | 证据 |
| --- | --- | --- |
| directoryPicker | `host.directory.pick` 已实现并接到 `window.amitiaDesktop.selectDirectory`，桌面端已注册 `ui.directory.pick` 端点，`directoryPickerController` 以 native 能力委派 pick/list/createDirectory；剩原生对话框点选未自动化实测、移动端未注册该端点（会原样返回 `ui_host: directory pick host unavailable`） | `sse_ui_host.go` / `useUIHostSSE.ts` |
| llm | 宿主侧已补数据面：`host.conversation.read` 现在返回消息 `id` 与 `attachments[{id,kind,filename,mimeType,sizeBytes,sha256,absolutePath}]`，附件字节按 sha256 落成只读副本（0444）；仍缺的是把这些附件映射成 DSH 消息表面里的 `FileAttachmentRef.uri`（`fileRequestText` 是同步函数，无法在调用内查宿主），因此仍报缺口 | `cmd/server/kernel_host_adapters.go` / `src/runtime/host-services.mjs` |
| sessions | `enter`/`announce`/`registerMessageProjection` 需要 DSH Session 实例与消息表面（`Message` + surface 折叠），Amitia 只有会话快照与 contentSummary 文本 | `host-services.mjs` |
| workspaceFiles | `changes` 缺 `host.workspace.watch`（没有持续变更帧订阅，只有快照读取） | `host-services.mjs` |
| sessionQuery | `observeSession`/`readEvent`（request 窗口）/`filterSessions`（id、cwd、created-at、parent、availability）/`filterEvents`（seq、time、type、surface、text）/`traceSession`（parentSession 血缘）/`searchSessions`（跨会话按事件命中分组）/`searchEvents`（会话内命中、片段、游标分页）/`traceEvent`（surfaceOp 替换链 + sourceEventSeqs 来源与派生）已实现；剩 `inspectSession` 需要可传入回调的 DSH Session 实例 | `src/runtime/host-service-bridges.mjs` |




## sessions 说明

`host.conversation.prepare` 已经落地：`conversation/create` 的 `draft` 参数写入 `draft` 标记与 `preparedAt`，在会话目录中首条消息之前不可见；同名 `requestId` 的正式 `conversation/create` 会以 `publishedAt` 升级同一会话。客户端 `sessions.prepare` 走该参数，返回 `{ sessionId, draft, preparedAt, publish(), drop() }`。
剩余缺口只在宿主侧 DSH `SessionStore` 原语：`enter`、`announce`、`registerMessageProjection` 需要 Session 对象的 append/公布语义，Amitia 侧没有对应的持久事件流写入口。

## 服务面一致性审计（2026-11-18 夜）

用 `artifacts/dsh-acceptance-20261118/audit-services.mjs` 实例化宿主服务并与 catalog 逐条比对：

- catalog `summary` 的 implemented/partial/delegated/unavailable/pending 计数与逐条 recount 完全一致
- 21 项 implemented 均能在 `src/runtime/**` 找到实现入口
- 服务面 32 个键；`storage`、`workspaceRegistry` 等 47 个 catalog 能力不在本插件 services 对象里，其中 27 项为官方包 `delegated`、13 项为 DSH 失败态，其余（含 `storage`、`workspaceRegistry`）由官方包在容器内自行 `ctx.provide` 注册。这两条的 catalog note 已补上该审计事实，status 保持 landed（能力确实可用，只是注册方不是本插件）

## 静默假实现修正（2026-11-18 夜）

`jobs.collectJobChunks` 原先用空 `catch {}` 跳过读取失败的 source，并伴随两条每次调用都打印的调试日志（每次读 chunk 都输出 channel/cursor 长度）。现改为：读取失败记入 `job.readErrors`（channel/cursor/原始 message）并通过 `jobView` 暴露，同时打印 `[original-error]`；两条调试日志删除。
`subagents.waitForTreeWork()` 原先恒返回 `false`、`drainDescendants()` 原先空实现，两者都会让调用方误判"子代理树已空闲"。现改为：`waitForTreeWork` 按活动 run 数量返回真实结果；`drainDescendants` 对已知 run 逐个 `dispose` 收敛，失败不吞（`allSettled` 后抛第一个原始错误）。`sendMessage` 也从无理由的 `unsupportedOperation` 补成带原因的拒绝。

## 记账修正（2026-11-18 夜）

`fileReferences`、`sessionFileReferences`、`sessionSkillCatalog`、`skills`、`workspaceFiles` 此前在 catalog 里还是 `providerPending`，实际实现早已落地并被单测覆盖。本次按证据修正：前四项转 `amitia-landed`（implemented），`workspaceFiles` 转 `amitia-wired`（partial，仅 `changes` 缺 `host.workspace.watch`）。修正后 host 侧 pending 归零。

## workspaceController 说明

`@deepseek-ai/dsh-workspace`、`dsh-api-workspace-controller`、`dsh-typert-protocol`、`dsh-typert-registry` 实际都在运行时安装树里（此前判断"本安装树没有该包"有误）。官方 `WorkspaceController` 继承 `TypertRemoteService`，业务方法全部委托 `ctx.workspaceRegistry`，而容器没有 workspaceRegistry 与 typert remote 栈。

因此改为在既有 host-services 里按官方 `WorkspaceView` 结构实现业务面：`workspaceId`/`path`/`title`/`sessionIds`/`createdAt`/`updatedAt`，底层全部复用 `host.project.*` 与 `host.conversation.list`。typert remote 命名空间（`ctx.remote.workspace`）没有复制，因为 Amitia 的浏览器通道是插件自己的 client 侧 workspaces 服务。

## 数据源缺口（非接缝）

节点 `conversation.chat.node` 的 `steering` 与 `tool-call` 渲染器已注册成功，但 Amitia 消息投影只有 `contentSummary` / `role` / `timestamp`；扫描 8 个会话、单会话最多 200 条消息，角色只有 `assistant` / `user`，没有任何工具调用字段，因此没有可喂给渲染器的事件。

## 已验证的正向能力（同日）

归档/取消归档、置顶/取消置顶（真实点击，双向可逆）、`llm.fileRequestText`（官方 fileHandleText 等价文本，`ref.uri` 为 file:// 或绝对路径时给出可读副本路径）、`subagents.resolveName`（活动子代理命名表，名字重复与未命中都保留原始候选列表）、DSH→Amitia 发送（`content` 字段，消息 7→8）、`subagents.prompt`（文本片段 → `host.conversation.message.append`，带 `source=dsh-subagent` 与会话作用域快照，拒绝图片片段）、`workspaceController` 的 list/create/rename/delete/initializeDefault（复用 project 注册表、重名与空标题拒绝、缺失工作区报 `session/not-found`）、archive/unarchive/pin/unpin（作用域快照经 `host.state.get` 命中或 `host.conversation.scope` 铸造，写入走 `callTargeted`，归档态置顶拒绝）、insertBefore/insertSessionBefore（`host.project.reorder` / `host.conversation.reorder`，中点插入 + 间隙耗尽重排，跨工作区会话与未知锚点原样报错）、`sessionQuery` 的 observeSession 租约、readEvent 前后窗口、filterSessions 五类谓词、filterEvents 语义文档过滤、traceSession 祖先/后代血缘（单测覆盖到非法谓词与循环血缘）、`agents.setFactory` 与 `agents.announce`（announce 在容器 cordis 上下文串行派发 `agent/created`，未绑定容器时报不可用；重复公布、非活动 agent、缺 source 都保留原始错误）、`host.directory.pick` 桌面链路（宿主注册 `ui.directory.pick` → `host.directory.pick` 路由 → SSE → `window.amitiaDesktop.selectDirectory`，路由单测覆盖成功/不可用/原始错误透传）、`sessions.prepare` 草稿闭环（prepare→目录不可见 53/53→同 id create 升级→目录可见→删除）、`host.directory.list/create`、`host.skill.list`、`sessionQuery.readEvent`、P8 子包快照与 `npm ci` 确定性还原（101 pins，mismatched=0）、P14 陈旧 `requires_recovery` 终结。
