# DeepSeek Harness 插件兼容宿主

当前源码 manifest 版本为 `0.1.227`。下方“最近验证状态”的 `0.1.193` 为历史运行态记录，不能代表当前源码已完成真实页面验收。

该插件是普通 Amitia 扩展，不是系统内置插件。

DSH 子插件安装在当前 Amitia 插件的插件数据目录中。兼容宿主负责把 DSH 子插件转换为 Amitia 通用插件贡献，不运行 DeepSeek Harness 本体，也不向 Amitia 核心引入 Cordis。

当前冻结的目标 ABI 版本为 `0.2.2-rc.2`，Client UI Primitives 使用同版本 `0.2.2-rc.2`。

当前开发阶段已提供：

- npm、Git 和本地 DSH 包安装。
- DSH 子插件登记、启停、移除和状态持久化。
- DSH Host ABI、工具调用和动态 Amitia Tool 注册。
- DSH Client ModuleLoader、Client ABI、React UI Host 和 Slot 映射。
- DSH `systemPrompt` 的 section、context、tools、variable 契约及 PromptExtension 注入。
- 官方 `dsh-commands` 斜杠命令：容器在 agent 上下文就绪后调用官方 `commands.execute`，`/compact`、`/goal` 均返回官方 `CommandResult`（`/goal <objective>` 经 `dsh-goal` 持久化目标）。
- DSH subagent provider registry 和 MCP resource provider registry 门面。
- 官方 `@deepseek-ai/dsh-tool-todo@0.2.2-rc.2` 包级导入、工具注册和执行冒烟测试。
- 官方 `@deepseek-ai/dsh-client-ui-jobs@0.2.2-rc.2` 和 `@deepseek-ai/dsh-client-ui-plan@0.2.2-rc.2` Client ABI、ModuleLoader 和 Slot 映射冒烟测试。
- 官方 `@deepseek-ai/dsh-client-ui-sidebar@0.2.2-rc.2` 已加载并映射会话头、侧栏和工作区侧栏 Slot。
- 官方 `@deepseek-ai/dsh-client-ui-sidebar-right@0.2.2-rc.2` 已加载并映射根右栏、会话右栏、右栏标签、页签操作和 Guide 子槽。
- 官方 `@deepseek-ai/dsh-client-ui-sidebar-files@0.2.2-rc.2` 已加载并映射文件页操作槽；官方 Slot Factory 使用独立 factory 适配面，不绕过普通未知 Slot 的失败规则。
- 官方 `@deepseek-ai/dsh-client-ui-sidebar-documentpreview@0.2.2-rc.2` 已加载，并映射右侧文档、文档操作、Office PDF 和不可预览子 Slot。
- 官方 `@deepseek-ai/dsh-client-ui-settings-models@0.2.2-rc.2` 已加载并注册 Models Settings 分节与引导项，设置壳标题、关闭、操作、侧栏启动器、通知、分节图标和模型登录位均已接入。
- 官方 `@deepseek-ai/dsh-client-ui-plugin-manager@0.2.2-rc.2` 已加载。`remote.pluginManager`、`remote.pluginInventory` 和 `remote.pluginRegistryProbe` 已提供；包列表、Bundle/单项启停、安装、移除和 Plugin Manager 视图接入 Amitia 当前 DSH 子插件状态，版本目录、详情文档和兼容豁免未具备时返回完整原始失败原因。
- 官方 `@deepseek-ai/dsh-client-ui-conversation@0.2.2-rc.2`、`@deepseek-ai/dsh-client-ui-workspace@0.2.2-rc.2`、`@deepseek-ai/dsh-client-ui-chat@0.2.2-rc.2`、`@deepseek-ai/dsh-client-ui-approval@0.2.2-rc.2`、`@deepseek-ai/dsh-client-ui-tool@0.2.2-rc.2` 和 `@deepseek-ai/dsh-client-ui-trajectory@0.2.2-rc.2` 已加载。会话根、Session、Header、View、消息流、Composer 链、空会话 Hero、命令、推理、Markdown 围栏、文本交互、审批详情、工具图片/Cordis 区和轨迹图片均接入对应 Amitia Slot；`fileUpload` 使用现有 Artifact 服务生成真实 durable receipt，桌面 `remote.directoryPicker`、`connection`、`shortcuts`、Cordis `ctx.plugin`、`uiConversation.events/views/groups` 与 `uiSession` 标准源提供接口均已补齐。
- DSH `remote.workspaceFiles.read/readBytes/stat/list/search/changes` 已接入会话项目主工作区。文本分页、完整字节、文件版本、目录列表、名称/路径搜索、隐藏项、忽略规则、排除路径和路径越界拒绝均在真实 Host 能力上执行；目录与文件变更使用有界 750ms 轮询生成 DSH `ready/change` 帧，脚本、构建产物和运行依赖不进入插件包。
- DSH Client `configForms`、`remote.settings`、`reflect.provide` 和浏览器 `fileDrag` 已提供可用桥接。
- DSH Session 事件和 Session projection 已通过 Amitia 扩展状态按会话持久化并可恢复。
- 官方 Session Lifecycle Event 63 项已全部按官方信封落地：每条事件包含 `type`、会话内单调 `seq`、`time`（epoch ms）和 `data`，`agent.session.append` 也走同一入口。未在冻结目录声明且未带 `ignorable: true` 的事件在写入与恢复两侧都会显式失败（`DSH_SESSION_EVENT_UNDECLARED` / `DSH_SESSION_EVENT_UNRECOGNIZED_REQUIRED`），不静默丢弃；带 `ignorable: true` 的未知事件保留并计入容忍计数。
- 63 项中 5 项 surface 事件（`system/message`、`developer/message`、`user/message`、`assistant/message`、`tool/result`）支持 `surfaceOp: 'append' | { op: 'replace', startSeq, endSeq }` 与 `sourceEventSeqs`，replace 两端点必须是当前 surface 节点且 `sourceEventSeqs` 必须覆盖全部被遮蔽节点，否则写入前失败；`agent.session.surfaceEvents()` 提供投影后的 surface 视图，log-only 事件携带 surface 元数据直接失败。
- 会话状态版本为 2，旧版本状态显式拒绝（`DSH_SESSION_STATE_VERSION_UNSUPPORTED`）。持久化不再做 `slice(-512)` 静默截断，改为上限 100000 条并在超限时报 `DSH_SESSION_EVENT_LIMIT_EXCEEDED`。
- 官方 `@deepseek-ai/dsh-mcp-client@0.2.2-rc.2` 已通过本地 MCP server 完成连接、工具发现和真实工具调用。
- DSH `agent.cancel()` 已通过通用 `host.conversation.turn.interrupt` 停止当前授权的活动 Turn。
- DSH `agent.whenIdle()` 已通过通用 `host.conversation.turn.await` 等待当前活动 Turn 完成。
- DSH `agent.model.current/select` 已持久化为 Session 事件，`runMaintenance` 已纳入 whenIdle 和 cancel 生命周期。
- DSH `sessions.create/resume` 已接入通用 `host.conversation.create` 和 Amitia Owner 会话事务。
- DSH Jobs Client 已支持会话任务列表、实时观察、有界输出和确认停止。
- DSH Client `layout` 已将官方 `conversation`、`plugins`、`schedules` 面板映射为明确的 Amitia 路由，并通过通用 Provider Action 切换侧栏。
- DSH Client `theme` 已提供主题快照、`theme/change` 事件、内置 light/dark 主题、第三方主题注册、`overrideTokens` 分层叠加、三档字号与字体族写入；`--dsw-*` 基础 token 直接映射 Amitia `--tp-*`，宿主调色板变化通过 `MutationObserver` 与 `prefers-color-scheme` 实时同步。宿主未暴露主题切换动作时，切换内置主题原样返回 `DshOperationUnavailableError`。
- DSH Client `timer` 已提供 Cordis 风格 `timeout`、`interval`、`throttle`、`debounce`，模块上下文同时注入这四个方法与 `sleep`；释放时清空全部挂起任务，之后调用原样返回已释放错误。
- DSH Client `theme/change` 事件由每个模块独占的事件总线分发，运行时服务与服务注入共用同一总线，插件 `ctx.on('theme/change')` 可直接收到快照。
- DSH Host `credentials` 已完整实现：`resolve` 先读 Amitia 通用 `host.state` 凭据存储（`source: 'file'`），未命中再回退进程环境变量（`source: 'env'`）；空值按官方语义视为未配置；`describe` 返回 `configured`、`source`、`writable` 与末两位可见的 `masked`；`set` 拒绝空值，`unset` 对不存在的引用是 no-op。record 半边 `readRecord`、`describeRecord`、`listRecords`、`modifyRecord`、`deleteRecord` 以 `host.state` 的版本化 CAS 作为唯一写路径，冲突自动重试 8 次后原样抛 `DSH_STATE_WRITE_CONFLICT`。
- DSH Host `commands` 已完整实现：`register` 校验命令名（小写、无斜杠）、冻结定义并拒绝重名；`list`/`find` 按名排序返回不含 handler 的描述符；`registerFileReceiptResolver` 只允许一个并提供释放函数；`execute` 严格写入成对的 `command/run` 与 `command/done` 事件，`rawInput` 保留原始分隔空白，`recordInput: false` 时不写 `args`；定义不接受附件时在 handler 之前结算为 error 结果且不写事件；handler 抛错结算为 error 结果，原始错误对象以不可枚举属性保留并原样进入 `text`。
- DSH Host `sessionQuery` 部分实现：`listSessions`、`readSession`、`listEvents`、`readSurface`、`readTitle`、`readTitleSnapshot`、`readTitleSnapshots` 接入了 Amitia 会话目录与持久化事件，surface 投影支持 `current`、`shadowed`、`log-only`；`observeSession`、`inspectSession`、`searchSessions`、`searchEvents`、`filterSessions`、`filterEvents`、`traceSession`、`traceEvent`、`readEvent` 缺少对应通用能力，调用原样抛 `DSH_HOST_OPERATION_UNAVAILABLE`。
- DSH Client `workspaces` 尚未实现：项目 create/rename/delete 与会话 archive/unarchive/insertSessionBefore 在 Amitia 核心没有对应通用 Host 能力，Catalog 中保留为未实现并记录原因。
- DSH Host `agentPresets` 暂缓：`mount`/`recompose` 需要按会话把 preset 的 plugins 组合成 Cordis 子组合，官方消费方还依赖 `agent.ctx` 与 `agent.session.header`，容器与 Core 均无对应能力，Catalog 中保留为未实现并记录原因。
- DSH Client `sessions.create`、`uiWorkspace.startSession` 和 `uiWorkspace.openSession` 已接入通用 `host.conversation.create` 与 Amitia 会话导航。
- 官方 DSH Schedule Client 已可在宿主中加载，`remote.schedule`、`remoteAccess`、`remote.$on`、侧栏 tab 查询等依赖已接入。现有 Amitia Schedule 执行器新增通用 `session_prompt` Target，支持固定 Session、`new_once` 和 `new` 目标，以及 `after/at/every/daily/weekly/cron/rrule`、IANA 本地绝对时间、create/catalog/list/view/update/delete/run/deleteInactive 和 delivery history；`new` 的 `cwd`、`agentPreset` 自定义创建参数仍返回明确失败。
- Client locale 已绑定真实 Amitia 字典和多语言回退，不再直接显示 locale key。
- DSH 右侧资源面板已映射到独立 content/title Slot，并支持 resource provider、`useTabInfo` 和 `useResource`。
- DSH 子插件安装已使用 staging、原子切换和失败保留旧版本。
- `DSH ABI Catalog v1`、`DSH UI Slot Catalog v1`、能力缺口、平台支持和兼容矩阵已从目标版本官方包生成并冻结。
- 生产 Slot 路由只读取冻结目录，不再使用名称包含关系或其他启发式 fallback；未映射 Slot 返回目录编号、阶段和明确原因。
- 管理状态包含 implemented、partial 和 unavailable 兼容矩阵，未完成项不会被误报为 100%。
- 普通 Amitia `web_page` 管理页面。
- 同一个 Amitiax 包已分别声明桌面 `web_module`、Android `mobile_web_runtime`、iOS `mobile_web_runtime` 和通用 mobile 入口。
- 手机容器管理页已支持状态、刷新、安装、启停、移除、完整错误展示和动态高度。
- 手机管理页通过通用 `AmitiaHost` Bridge 调用当前 contribution 的 `command` Action，不直接获得 Native 或后端权限。
- `/dsh-compatible-host` 路由由普通 `route.registry` Provider 注册，桌面和移动端使用同一页面 Provider。
- 手机 Web Runtime 会加载已启用的 DSH Client 插件，使用本地 React Slot Registry 渲染当前 Amitia Slot。
- DSH Slot 别名已全部映射到手机通用 Slot，共声明 40 个宿主贡献，包括聊天、消息、输入区、扩展、Portal、工作区侧栏和右侧资源面板。
- Client Slot 支持基础 `single/list/keyed/chain/hoc`、children、store、inject、locale、resource、Jobs 和右侧面板。
- 手机 WebView 的 Android/iOS 真机、跨 WebView 共享状态和恢复验收仍在继续。
- Android 已为真机门禁启用安装平台 `android`；iOS 待运行验证通过后再加入安装平台清单。

## 最近验证状态

- `0.2.103` 把预声明命中日志从 `console.warn` 降为 `console.debug`（子槽位已按宿主定义授权，命中属正常路径，不再占据告警面板）。插件测试 414/414 通过，构建 treeHash `f54f6c117d7baebaf7e1d8fbdbfff28fdee5ac5cdcd64b566060d2467c7589d2`，本地 Core 安装为 `0.2.103`（generation `generation-01e7924e-6830-400f-aff3-de99dac20049`），客户端模块在下次完整启动后生效。
- `0.2.102` 修复「归档会话」与 toast 渲染两处真实故障。归档失败根因是前端会话引用 `meshconv1:<owner>:<id>` 与 Core `host.conversation.*` 只认裸 uuid 的契约不一致，`conversation-read` / `conversation-archive` 一律返回 `会话不存在或不属于当前角色`；新增 `src/runtime/conversation-id.mjs`（`conversationReference`、`parseConversationReference`、`conversationIdOf`、`normalizeConversationRequest`，畸形引用抛错并带原始 cause），在 `shell.mjs` 挂载的 `conversationId`、`index.mjs` 的页面会话入口和 runtime `command` 入口统一归一化为裸 id，`scripts/build-package.mjs` 把该文件同时复制进 runtime 包。toast 失败根因是预声明子槽位 `app.workspace.toast` 返回空 children，没有渲染授权，`client:@deepseek-ai/dsh-client-ui-workspace:ui.portal.overlay:workspace.row-toast` 一渲染就报 `is not authorized to render child slot`；改为按宿主定义（`kind` / `scope` / `layout` / `supportedKinds` / `allowRootChild`）构造 child 声明条目并保留 `commonInject`，注册时登记 claim、卸载时回收，重复 claim 记警告并让位。联调中又发现前端 `ClientSlotContributionHost.vue` 把宿主贡献实例的 `actions` 对象写在后展开位置，覆盖了 DSH 渲染出现处传入的 `actions` 数组，`Toast` 抛 `actions?.map is not a function`；现改为宿主 `actions` 仅作默认值，出现处 props 优先。新增 `tests/conversation-id.test.mjs` 6 例、`tests/client-child-declaration.test.mjs` 1 例，插件测试 414/414 通过；treeHash `017431c8f57db4100bb43cab2a1f43145dbf5f8170acf034cf2b96f05d2aa41f`，本地 Core 安装为 `0.2.102`（generation `generation-f7f3d8d0-8026-4be6-98bb-1bf85fea7853`）。Electron + CDP 实测：`__dshCurrentSessionId` 为裸 uuid `4883a2f0-9df5-56c4-b8b1-6099bd108817`，点击「归档会话」返回 `archived:true`、`archivedAt:2026-11-25T06:34:26.479725Z`，toast 渲染「会话已归档，可撤销或筛选已归档会话」并带「撤销」「筛选已归档会话」两个按钮，点击「撤销」后回到 `archived:false`、`archivedAt:""`，控制台无 `actions?.map` 与 `is not authorized to render child slot` 报错。
- `0.2.101` 修复 DSH 客户端 UI 图标全部不显示：此前宿主页面没有官方 sprite，`productIcon()` 生成的 `<use href="#dsw-icon-…">` 无法解析，聊天头部「归档会话 / 置顶会话」两个 24×24 按钮只剩空白。新增 `src/client-ui/icon-sprite.mjs`，在 DSH 客户端 UI 挂载（桌面 Shell 与手机 Slot）时把官方 202 条 `ICON_SYMBOLS` 渲染成隐藏 sprite `#dsh-icon-sprite`，容器 `0×0`、`aria-hidden`、`pointer-events: none`，引用计数保证多实例共用一份、卸载时回收，外部移除后自动重建。插件测试 407/407 通过；本地 Core 更新到 `0.2.101`（generation `generation-7c0b3fb0-25b7-4ed0-a606-dab75ad84d32`，安装 treeHash `4ddd6e9ec76c4c140dae8a11eb9f965f7e63792ce050b4f83e70580db1b88a37`，`failureCount: 0`）。页面重载后 DOM 中仅 1 个 sprite 容器、202 个 `symbol`，两枚按钮 `use` 全部解析，按钮中心 `elementFromPoint` 命中自身，切换会话后 sprite 与图标保持。
- `0.1.227` 将官方 `uiWorkspace.openSession/startSession` 导航目标改为独立 DSH 会话页，携带 URL 编码的 `conversationId`，避免创建或选择会话后跳到 Amitia 原生聊天页。运行时服务定向测试 25/25、客户端定向测试 4/4 通过；treeHash `250a5cdfbcd2a66a3c0336997a543aee165132327a035fb79e11578a1173e996`，本地 Core 更新完成，`failureCount: 0`。完整项目重启后健康接口可访问；真实会话选择与发送仍待验收。
- `0.1.226` 已在本地 Core 安装并启用，更新操作完成，`failureCount: 0`。Electron 独立页 `#/dsh-conversation` 已真实显示官方 DSH 欢迎区、工作区选择器与禁用态输入栏；客户端模块装载 4/41。`0.1.225` 补齐官方会话快照的 `pendingSubmissions` 等字段，`0.1.226` 按官方 `InputState.compose()` 补齐 `attachmentIds`、`draftRev`、`phase`、`queue`，此前两处渲染异常均消失。插件客户端定向测试 4/4、独立页测试 2/2 通过。当前尚无会话，真实消息流和发送仍待带会话作用域验证；无会话入口会记录 `sessions.sessionOf` 无可解析作用域告警。
- `0.1.222` 已在本地 Core 安装并启用。独立页开始装载 4 个有客户端入口的 DSH 子插件，但旧管理页模块和新 Shell 模块各自打包 React，公共适配器标识复用后触发 `useRef` / `useSyncExternalStore` 的 React 实例冲突。`0.1.223` 为每个模块生成独立适配器标识，已打包安装，真实页面仍需重载到新 generation 验收。
- `0.1.221` 已在本地 Core 安装并启用，扩展详情显示 `installed`、`enabled`，管理页显示已安装的 DSH 子插件。Electron 中点击“打开官方会话页面”后路由进入 `/dsh-conversation`，但内容为空。已定位到独立路由的客户端装载器只筛选 `app.shell.` 等前缀，遗漏位于 `ui.portal.overlay` 的 `dsh-desktop-shell-host`；`0.1.222` 源码改为该路由精确装载这一贡献，并为 Shell 注入导航和会话上下文。后者的真实页面效果待本轮打包验证。
- 当前源码版本为 `0.1.193`（运行时发布态锁 `0.1.192`：应用级装载关闭，插件页与原生聊天页均正常）。安装包 `plugin/amitia-dsh-compatible-host-0.1.158.amitiax`，treeHash `04c148a5afd9a8028605dcf7eed2a541ad359f59b4bc3234710d4e7ccab5d480`，插件测试 215/215 通过。
- `0.1.158` 在 Electron 真实页面验证 DSH 客户端 UI：`#/extension/page/dsh-compatible-page?extensionId=com.amitia/dsh-compatible-host` 渲染 42 行插件表（插件/版本/状态/入口/操作），无 `Error` 文本、无控制台错误；页面内点击 `停用` 后宿主 `dsh-token-meter` 变为 `enabled:false`，点击 `启用` 后回到 `enabled:true`、`active:true`（39 包 active），证明 React 客户端模块经 Amitia 运行时模块通道真实执行并能回写宿主。
- `0.1.157` 修复类型声明型客户端入口：官方部分包（如 `dsh-token-meter`）的 `exports["./client"]` 指向 `lib/types/*.js`，只有类型没有运行时，按运行时模块导入会 403/失败；现在 `clientDependencies` 标记 `typeOnly`，页面改为注册空模块占位，插件自身类型入口也跳过实例化。
- `0.1.156` 为运行时模块导入加一次重取票据重试（首次偶发 `Failed to fetch dynamically imported module`）。
- `0.1.155` 补 `remote.job`（`list`/`follow`/`kill`）客户端服务：按官方 `JobListFrame`/`JobFollowFrame`/`JobKillValue` 契约，用宿主 `jobs-list`/`jobs-kill` 轮询与转发；`@deepseek-ai/dsh-api-job-controller` 客户端因此可实例化。
- `0.1.154` 客户端依赖按模块源码静态扫描：官方 `dsh-api-job-controller` 的 client bundle 里 `require("@deepseek-ai/dsh-api-gateway/client")`，但该包不在其 `package.json` 依赖中，改为扫描 `require(...)` 说明符并在共享树里解析；`client-source` 支持未登记但已安装的包，页面预加载依赖模块后再实例化插件。
- `0.1.153` `DshClientModuleLoader` 支持包级子路径依赖（`@pkg/client` 解析到已注册模块的导出，含循环与子路径校验），并新增对应测试。
- `0.1.152` 修复启用意图丢失：`set-enabled` 原先先激活成功才写状态，激活失败时用户意图随之丢失，依赖提供者后到也无法靠重试收敛，实测需要三轮才把 39 个包全部启起来；改为先持久化 `enabled` 再激活，失败保留 `enabled: true` 与 `activationError` 并立即触发一次失败重试。实测先禁用 `dsh-tools`（依赖 `systemPrompt`）与 `dsh-system-prompt`，单轮依次启用二者：`dsh-tools` 首次失败后保持 `enabled: true`，`systemPrompt` 启用后自动收敛，39 个包全部 active。
- `0.1.151` 把 DSH 子包安装从「每个包一套独立 npm 工作区」改为「一个共享依赖树」：官方组合依赖模块单例身份（`instanceof ManualCompactionError`、Brand 标记），独立工作区会让同版本 `@deepseek-ai/dsh-compaction` 出现多份副本，`/compact` 的预期失败映射因此失效并把内部错误直接抛出。现在 `dsh-tree/package.json` 合并全部依赖后统一 `npm install`，安装目录统一为 `dsh-tree/node_modules/<name>`，卸载只删该包目录与依赖声明；安装目录体积从 881MB 降到 143MB。同轮修复 `jobView` 二次包装丢 `owner`（`jobs-list` 的 `owner` 恒为空）与 `pluginDataRootFromPackageDir` 识别目录名。
- `0.1.150` 让 `diagnose` 支持 `commandLine`：在官方 `agents.create` 的 `setup` 中始终关闭并保存容器上下文与 agent，回合结束后通过官方 `commands.execute(agent, line, [], signal)` 执行斜杠命令，结果写入报告的 `command`。实测 `/compact` 返回官方 `CommandResult`（`commandId` `cmd-bcf67dfd-1`，`kind=error`，文案 `Compaction could not produce a useful summary...`），`/goal` 返回 `kind=success`（`No goal is currently set.` + 官方用法），`/goal 完成 DSH 兼容层验证` 返回 `Goal created`、`Status: active`、`Rounds: 0/256`，证明官方 `dsh-commands` 注册、解析与执行链在 Amitia 容器内真实可用。
- `0.1.149` 打通 `bash` 写命令：官方 `run` 返回的是 handle（`{done, cancel}`），结算信号在 `handle.done`，容器原先把返回值当完成信号立即标 `completed`，导致 `attached.process()` 恒为 `undefined`、工具报空错误；改为 `await value.done` 后再置 `completed` 并保留 handle 供 `cancel` 转发。实测 `touch /tmp/dsh-write-ok && echo wrote-ok`：审批 200、文件创建、模型回答 `wrote-ok`；只读复合命令同样正常。
- `0.1.148` 修正 `registry.wait` 返回类型：官方契约为 `Promise<JobView>`，容器原先返回 job 的 run 结果，导致 `view.status` 为 `undefined`、工具误判 job 未结算；改为等待 `job.done` 后返回 `jobView(job)` 并支持 `timeoutMs` 与 `signal.throwIfAborted`。修后写命令的输出已能回传（模型回答中出现 `wrote-ok`），但 `attached.process()` 在 escalation 场景仍为 `undefined`，工具仍把结果标成失败。
- `0.1.146`~`0.1.147` 修好 bash 的两处阻塞：`jobView` 补 `detail`（原缺失使失败分支抛空错误），`echo` 的 stdout 恢复原样回传（`sandbox-output-check`、`run-check-1`、`run-check-2` 实测通过）；包激活改为最多 4 轮重试，解决 `dsh-bash-sandbox` 依赖 `sandboxPolicy` 时激活失败后不再重试、重启后 `bash` 全部失效的问题（`echo retry-check` 通过）。写命令仍失败：审批 200、文件创建，但输出为空。
- `0.1.144`~`0.1.145` 继续定位 bash 输出回收：加 `[dsh-jobs] start/read/source` 诊断日志后确认**工具从未调用 `registry.read`**（只有 `start` 日志），`get`/`list` 已改为返回官方 `JobView` 投影仍无 `read` 调用——说明工具走了 job 已结算分支，输出渲染路径待进一步对齐。
- `0.1.143` 按 `dsh-jobs` 类型定义重写 jobs 门面：`kill` 返回 `requested`/`already-finished`，`read` 返回 `{ chunks, lossy, result, next, job }`（settlement 后首次携带 producer 结果），新增 `readAt(id, from, caller)`，chunk 统一为 `{ channel, text, from, to, at }`，`start` 消费 `options.output` pull source。`echo` 的输出回收仍失败（返回空错误），失败点已不在 read 契约表面。
- `0.1.142` 尝试对齐官方 jobs `output` pull source 契约（`start` 接收 `options.output`，`read` 逐 source 调 `read(fromByte)` 累积 chunk），`echo` 的输出回收仍失败并返回空错误——工具在 job 已完成分支走的读取路径与 `jobs.read` 尚未对齐，该缺口已记入开发计划 §23.14.17。
- `0.1.141` 完成沙箱升级审批闭环：`bash` 在 read-only 策略下执行 `touch` 触发 `approval/asked` → 容器 answerer 经 `host.approval.request` 进入 `ApprovalBroker` 等待 → 脚本批准后 `host.approval.request` 审计 `status=success`，`/tmp/dsh-sandbox-write-check` 被真实创建。遗留：工具层输出采集报"命令未产生任何输出"，`echo` 的 stdout 未被 `jobs.read` 正确回收。
- `0.1.140` 让沙箱写操作真正触发升级审批：`jobs.read` 改回官方 `{ chunks, lossy, job }` 结构（chunk 带 `data/from/to`），`touch /tmp/dsh-sandbox-write-check` 产生 `approval/asked` 1 与 `approval/decided` 1，工具返回 `sandbox escalation to "danger-full-access" requires approval, but no approval channel is available` —— 审批请求已进入 DSH 审批服务，但容器 answerer 未应答该路径，待与 `scopeTarget` 解析目标对齐。
- `0.1.138`~`0.1.139` 按官方 `jobs` 契约补齐容器门面：`start` 返回值增加 `id`（原先只有 `jobId`，`dsh-tool-bash` 取 `attached.id` 得到 `undefined` 后报 `DSH job [object Object] not found`），`kill` 同时接受 job 对象与字符串 id，新增 `wait` / `read`（支持 `consuming` 清空） / `remove`。补齐后只读命令（`echo dsh-sandbox-ok`）可执行；写命令路径仍受阻——`jobs.read` 的 chunk 结构与官方期望不一致，工具返回空错误消息（`Error:`），沙箱升级审批未触发。
- `0.1.136`~`0.1.137` 接入官方沙箱链并打通真实执行：安装 `dsh-sandbox` / `-local` / `-policy` / `dsh-bash-sandbox` / `dsh-shell-env` / `dsh-tool-bash`（37 包 36 `active`，`dsh-shell` 与 `dsh-bash-sandbox` 抢注服务 `shell` 故保留后者）；`dsh-tool-bash` 依赖 `ctx.jobs` 的 registry，容器 jobs 门面缺 `kill` 契约导致首次执行报 `registry.kill is not a function`，补 `kill(jobId, owner, reason)` 后模型真实调用 `bash` 执行 `echo dsh-sandbox-ok` 并回答 `dsh-sandbox-ok`。同时把审批应答器改为默认注册（真实工具审批也需要它），诊断总是先建 Amitia 会话以便宿主调用有作用域绑定。
- `0.1.135` 闭环审批决议：审批监听器改由容器 `context.on("approval/request", …)` 注入并在 `finally` 释放（挂在 agent setup 上时官方 waterfall 取不到，只会落到 `unavailable`）；`chat-runtime/snapshot` 外部复现需注意单聊会话不能带 `conversationRoleId`（会触发 group 校验报 `409 禁止将设备专属数据保存到其他设备`），以及会话级审批的 `requestId` 因 `omitempty` 在 JSON 中缺失（必须按字段不存在处理）。实测：`snapshot` 列出 `pending` 审批，`chat-runtime/approval` 返回 `200 {"approved": true}`，模型回答 `outcome 为 allowed-once`。渲染层由 `front/src/__tests__/agentApprovalGuard.test.ts` 覆盖：补齐 `createPinia` 与 `ExtensionSlot` stub 后原有 4 项恢复通过，新增会话级审批（无 `turnId`）用例断言弹窗可见并点击"本次允许"触发 `ownedChatRuntimeRequest("approval", { conversationId, turnId: "", approvalId, approved: true })`，该文件 5/5 通过。
- `0.1.128`~`0.1.134` 把 DSH 审批接到 Amitia 原生审批通道：新增 `host.approval.request` 宿主边界（权限 `approval.request`，走既有 `execution.ApprovalBroker.Await`），`src/runtime/approval-bridge.mjs` 注册 `approval/request` waterfall 监听器并经 `host.callTargeted` 定向到会话级快照；`diagnose` 新增 `approvalProbe`/`approvalTimeoutMs`/`approvalConversationId`；`runtime/plugin-host` 的 `host.call` 支持 `options.timeoutMs`，`SingleCallTimeout` 默认由 `30s` 调到 `5m`；Amitia 侧把 `requestId` 为空的审批当作会话级审批（`ownedRuntimeApprovals` 列出、`chat-runtime/approval` 放行决议、前端 `AgentApprovalGuard` 允许 `turnId` 为空）。实测审批请求真实进入 broker 并等待 179 秒后 fail-closed，`go test ./internal/extension/kernel` 与 `front` tsc 通过；用户点击"允许一次"的真实会话端到端仍待验证。
- `0.1.126`~`0.1.127` 打通 spill 自动写入：安装启用 `dsh-spill-policy`、`dsh-tool-fs-search`、`dsh-subprocess`（seam）与 `dsh-subprocess-local`（provider）；修复 `normalizeHostPlugin` 丢弃官方插件 `Config` 导致 schemastery 默认值不生效（`fs-search` 报 `globMaxResults must be a positive integer`）。真实 turn 中模型用 grep 搜出 42645 处匹配，`dsh-data/spill/session-f695d585c80d/9ec63dc2d7c4-grep-results.txt` 由官方 `spill-policy` 自动写入，4231146 字节、52809 行、SHA-256 `032fc1ff06c2f943fb7a84828ff2020207391d997c69823c82d06d274a7c752e`。运行时 30 个包全部 `active`。
- `0.1.121`~`0.1.125` 打通上下文自动压缩：`chat.ModelGatewayResult` 与 `kernel.ModelInvokeResult` 新增 `contextWindow`（取自激活模型配置），adapter 在 `resolveModel` 返回官方 `LlmModelContext`（`LlmRuntime.prepareCall` 只认 `resolveModel` 的 `context`）；安装启用 `dsh-token-meter` 与 `dsh-compaction-basic`（摘要模型绑定 `amitia/deepseek-chat`），模型配置补全 `context_window: 128000`。验证期低阈值下真实 turn 记录两次压缩：`compaction/summary` 的 `shadowedTokenCount` 分别为 699 与 464，两次 `compaction/end` 均为 `error: null`；`request/context` 含 `contextWindow: 128000`。验证后恢复官方默认阈值。同时修复 `start-all-services.sh` 的 `terminate_tree`（TERM 后等待并 `kill -9`），消除反复出现的 `Port 5178 is already in use`。
- `0.1.119`~`0.1.120` 完成剩余同名门面归位：`approval`→`amitiaApproval`、`commands`→`amitiaCommands`，安装并启用官方 `@deepseek-ai/dsh-user-approval@0.2.2-rc.2`（`policy` 默认 `ask`）与 `@deepseek-ai/dsh-commands@0.2.2-rc.2`，两者均为 `active: true`；`typert` 容器从未占用，无需让名。运行时共 24 个包全部 `active`。让名后 `diagnose`（`toolProbe: true`）仍为 `Status: success`，`tool/call` 1、`tool/result` 1、`assistantText` 为 `ok 的值为 true。`。
- `0.1.118` 打通 turn 内工具调用：`diagnose` 支持 `toolProbe`，在官方 `agents.create` 的 `setup(agentCtx)` 里用 `agentCtx.tools.register` 注册 `amitia_probe_<sessionId>`（open object 参数、`output.schema` 为 `{ok: boolean}`、`execute` 返回 `{ok: true}`）。真实 Core（`http://127.0.0.1:54918`）返回 `Status: success`、`DurationMS: 2222`，事件含 `tool/call` 1、`tool/result` 1、`step/start` 2、`step/end` 2、`assistant/message` 2；模型在第二个 step 基于工具返回值回答 `工具返回的 ok 值为 true。`；`dsh-diagnose-mwhykxtz.json` 的 `turnBoundary.lastStepBoundary` 为 `{"kind": "end", "seq": 14}`、`openTurnStartSeq` 为 null；spill 文件 33 字节与最终回答一致。
- `0.1.106`~`0.1.117` 打通真实模型 turn（开发计划 §23.14.16）：Go Core 新增 `host.model.invoke` 边界；`src/runtime/model-adapter.mjs` 实现官方 `LlmAdapter` 契约并 `registerAdapter(["amitia"], adapter)`；`reservedServiceNames` 追加 `tools`→`amitiaTools`、`systemPrompt`→`amitiaSystemPrompt`，每插件工具门面改挂 `entryContext.amitiaTools` 属性而不再占用服务名（否则共享容器 realm 报 `service "amitiaTools" has been registered at <root>`）；`diagnose` 动作改用官方 `ctx.agents.create({sessionId, meta: {cwd}, agentOptions})` 驱动 turn，用户消息必须带 `role: "user"` 与 `source: {kind: "user"}`。真实 Core（`http://127.0.0.1:52358`，`deepseek-chat`）执行 `{action: "diagnose", cwd: "/Users/untrammelled/Desktop/Project/U-AI"}` 返回 `Status: success`，11 条会话事件含 `turn/start`、`step/start`、`system/message`、`user/message`、`request/header`、`request/context`、`assistant/message`、`step/end`、`turn/end`；模型返回文本并记账 `inputTokens 18`、`outputTokens 119`；投影 checkpoint `dsh-diagnose-mwhy8qb1.json`（914 字节）的 `turnBoundary` 为 `openTurnStartSeq: null`、`lastTurn: 1`，`modelSelection.lastUsed` 为 `amitia/deepseek-chat`；spill 文件 `8d4b29ef6cfc-diagnose.txt`（141 字节）与 `assistantText` 逐字节一致。
- `0.1.106`~`0.1.117` 同时补齐宿主授权层：`host.conversation.read` 等会话级路由原先只认 `ScopeSnapshotID`，`callTargeted(method, input, scopeSnapshotId)` 一律报 `required roles [conversation] not satisfied`；`host_api` 新增 `TargetScopeChecker`，`ManagerScopeChecker.CheckTarget` 校验调用方与目标快照归属（extension/module/generation/space/character/conversation）并做角色判定，未实现该接口的 checker 直接 fail-closed。
- `0.1.103`~`0.1.105` 完成官方 agent 链路探路（开发计划 §23.14.15）：`0.1.103` 安装包大小 14722231 字节、SHA-256 `856f42ae878362e687f6a001e5b56bae9903a45ff386dba6ffe337e87871dd64`，把容器 `agents` 门面改名为 `amitiaAgents`，让出官方 `@deepseek-ai/dsh-agent` 的服务名，并新增 `official-agents-plugin` 反向断言（未安装官方包时按官方名注入报 `DSH_HOST_SERVICE_MISSING`）；`0.1.104` 安装包大小 14722285 字节、SHA-256 `7698b051ce992e55585aef53009df315afff216e68e97bd01955208f8c6d0254`，把 `sessionProjections` 门面改名为 `amitiaSessionProjections` 并安装启用官方 `@deepseek-ai/dsh-session-projection@0.2.2-rc.2`，此前 `ctx.agents.create()` 直穿官方 agent-loop 时在 `modelProjectionHandle.readerFor(...)` 抛原始错误 `invoke error: jsonrpc: -32603: this.modelProjectionHandle.readerFor is not a function`，根因是容器 facade 的 `register()` 只返回 `stateOf`、没有 `readerFor`；改名后 agent-loop 在该注册表登记 `turnBoundary`、`modelSelection`、`inbox` 三行投影，宿主 20 个官方包全部 `active: true`。
- `0.1.105` 复测到三处官方 provider 直写的真实落盘证据：`dsh-data/sessions/<cwd 编码>/amitia-probe-agent-3/session.v4.jsonl.zstd`（208 字节，mode 600，zstd 解压为 `{"type":"session","version":4,"id":"amitia-probe-agent-3","cwd":...,"isSeeded":false,"delegationDepth":0}`，SHA-256 `11c6b23422472a93b08b9e03a1fc5fc98ef22b0aa3502493f268872fa2e9c6c9`）连同 `session.lock`；`dsh-data/storage/session_projcache/sessions/amitia-probe-agent-3.json`（858 字节，`version: 7`，`rows` 含上述三行投影，SHA-256 `ba8daa18d93be1d9fddeedc6d99b7d09e0ce2d0a9750ba38f9f39d32b81387c7`）；`dsh-data/spill/session-55ce772d1796/2941c753b331-probe-0105.txt`（960 字节，mode 600，内容逐字节一致，SHA-256 `b4d0ad6c1d9dd2271818948ceeee5a7722c2055355b3bc60cb36f6751dc4caa4`）。catalog 把 `sessionPersistence`、`sessionProjectionCache`、`spillStore`、`sessionProjections` 四项推进为 `amitia-landed`，`implemented` 9→12、`partial` 12→9，缺口并集 47。
- `0.1.117` 起官方 agent-loop 已能真实驱动 Amitia 模型（见上文 §23.14.16 证据）；剩余缺口为 turn 内工具调用批次、上下文溢出自动 spill，以及容器门面与官方同名服务 `approval`、`commands`、`typert` 的归属确认。
- `0.1.101`~`0.1.102` 安装并启用官方 `@deepseek-ai/dsh-session-projection-cache@0.2.2-rc.2`：该包 `Config` 为 `z.object({writeEveryEvents, writeIntervalMs})` 且两项 `.required()`，容器原先不注入配置，激活时报原始错误 `invalid config: $.writeEveryEvents missing required value (at writeEveryEvents)`；`provider-wiring.mjs` 的 `seamConfigs` 按官方 README 注入 `writeEveryEvents: 200`、`writeIntervalMs: 5000` 后 `active: true`、`activationError: null`，`session_projcache` 存储域随之打开。catalog 把 `sessionProjectionCache` 从 `amitia`（pending）推进为 `amitia-wired`（partial），缺口从 49 降到 48（delegated 27、pending 8、unavailable 13），`resolved` 增补该项且 `hostServicePartialCount` 由 11 变为 12。`dsh-data` 下仍只有 `storage/workspace.json`，投影缓存的 checkpoint 落在会话创建、`turn/end` 与会话销毁三个切点，真实会话写入证据待补（开发计划 §23.14.14）。
- `0.1.100` 安装包 `plugin/amitia-dsh-compatible-host-0.1.100.amitiax` 大小 14721520 字节，SHA-256 `c444aba354dfcd5a14ed608e2aadb4552302706962f58ed2fa70d72edee63966`，treeHash `5d070bf686c0a4c264ce06dbd61ddf08871fb509c0f60276863270d39edde30d`，插件测试 169/169 通过。
- `0.1.100` 把官方 provider 落盘目录纳入 Amitia 备份与卸载清理：manifest 新增 `data.pluginData.include`（相对白名单路径，必须与 `data.backup=true` 同时出现，Core 校验空串、前后空格、绝对路径、反斜杠、`..` 越界与未规范化路径），DSH 声明 `include: ["dsh-data"]`，`dsh-packages`（npm 缓存，可重装）不进备份。Core 新增备份组件 `extension.plugin_data.v1`，按 `plugin-data/<safeID>` 逐文件记录 path/mode/size/sha256/base64，拒绝 symlink、非普通文件、未声明路径以及 size 或 sha256 不符（单文件 64MiB、总量 512MiB 上限），恢复写 `*.restore.tmp` 后 chmod + rename；`data.uninstall` 为 `delete`/`export` 时卸载事务与 `Runtime.Uninstall` 都会删除 `plugin-data/<safeID>`，`retain` 保留。真实 Core（Source Gate `414d3597cd9939e0f3ed609526bcab3ea0311d7ff01610e8fd02bd9288df69fc`、SHA-256 `d584af9d149cede3dc14f42b291fede8a59d145619b38f131251a1e9de12b9ed`）备份解密后 `datasets/extension.plugin_data.v1.ndjson` 仅 1 条记录 `dsh-data/storage/workspace.json`（227 字节、sha256 `8332e072a55f6c6ac287e41ac1a57640d630c2143cd55eeba907a3a33fb3240e`、mode 384），`dsh-packages` 未出现；`/api/health` 为 `ready: true`、`/readyz` 为 7 ready、0 degraded（开发计划 §23.14.13）。
- `0.1.99` 安装包 `plugin/amitia-dsh-compatible-host-0.1.99.amitiax` 大小 14716660 字节，SHA-256 `32091cb01b52d4a7f5fd1c79035d72af0a0b9f2407e2ce8b0edd546d4376a2ee`，treeHash `e69901c6dbe4464f782426802e76eafc526af329d1e49d47654a80c00832d2fc`，插件测试 169/169 通过。
- `0.1.98`~`0.1.99` 打通官方 durable provider 与 workspace：manifest 新增 `hostAccess.filesystem.durableSync`（必须与 `filesystem.mode == "read_write"` 同时出现），Core 据此只对声明方不再传 `--permission`，`fsync API is disabled when Permission Model is enabled.` 不再出现；宿主自造的 `sessions` 服务改名为 `amitiaSessions`，不再遮蔽官方 `@deepseek-ai/dsh-session` 的 `sessions` 契约（冲突原文 `service "sessions" has been registered at <root>`）。真实 Core 中 `@deepseek-ai/dsh-session`、`@deepseek-ai/dsh-workspace`、`@deepseek-ai/dsh-spill`、`@deepseek-ai/dsh-spill-local` 均 `active: true`，官方 storage-json provider 已写出 `plugin-data/dsh-data/storage/workspace.json`；`dsh-data/sessions` 与 `dsh-data/spill` 仍为空目录（开发计划 §23.14.9~§23.14.12）。
- `0.1.97` 安装包 `plugin/amitia-dsh-compatible-host-0.1.97.amitiax` 大小 14716593 字节，SHA-256 `6aa0136f1613692ef92bbd59ed867f069c4bb53db63f498ff5f2a25b5bcbae65`，treeHash `f9f73da1423ec6f35fd21ccf7d1e0a6180bb22d62c3ef00b215198c1282c6ed3`，插件测试 168/168 通过。
- `0.1.90`~`0.1.97` 完成 P11 批次 D 前半段（开发计划 §23.14）。`0.1.97` 修掉声明型包判定读取原型 getter 的问题（`@deepseek-ai/dsh-typert-registry` 曾因此报 `Cannot read properties of undefined (reading 'view')`），改为按属性描述符判断，`dsh-typert-registry` 与 `dsh-api-job-controller` 已恢复 `active: true`。官方 provider 落盘目录统一接到 `plugin-data/dsh-data/{storage,sessions,spill}`，`dsh-storage-domain` 注入必填 `backend: "json"`；`@deepseek-ai/dsh-session-persistence` 这类只导出抽象类的声明型契约包不再被当插件挂载并抢占服务名，`@deepseek-ai/dsh-lsp` 因带原型方法仍按真 seam 加载；npm 安装参数追加 `--ignore-scripts`，`koffi` 走平台预编译包正常加载。真实 Core 启用后 `dsh-storage`、`dsh-storage-json`、`dsh-storage-domain`、`dsh-session-persistence-jsonl` 均为 `active: true`，`dsh-data/storage` 与 `dsh-data/sessions` 已创建；`dsh-workspace` 越过 `storageDomain` 检查后停在原始错误 `fsync API is disabled when Permission Model is enabled.`（Node v26.12.1 任何 `--permission` 组合都会禁用 fsync，决策见开发计划 §23.14.6）。
- `0.1.89` 安装包 `plugin/amitia-dsh-compatible-host-0.1.89.amitiax` 大小 14711483 字节，SHA-256 `598854d3a0e01a8a4a99e3d46dfed404d0f9fdf2bfb930dda42e1583dea106ec`，treeHash `49645f85d38af11cc17d546726220cfd4455ef09dd956a52f11448fc4a36808e`，插件测试 164/164 通过。
- `0.1.89` 完成 P13：`ctx.workspaces` 的 `create`/`rename`/`delete`/`archiveSession`/`unarchiveSession` 映射到 Core 新增的通用 `host.project.list|create|update|delete` 与 `host.conversation.archive`，`insertSessionBefore` 保持原始 unsupported（Amitia 会话排序只有置顶与更新时间）。Core 新增权限 `project.read`、`project.manage`，`host.project.create` 支持 `directoryPath` 并复用 `workspace.Service.RegisterLocalMount` 生成真实本地挂载，没有第二套项目系统。插件新增 `dsh-workspace-index` 状态并在 `sessionCatalog.refresh` 中与会话派生项合并，使新建项目立即出现在 DSH Workspace 目录。catalog 中 `workspaces` 由 `unavailable` 变为 `partial`（`mapping: composite`），client services 8 项为 implemented 4、partial 4，host services 缺口仍为 delegated 27、pending 14、unavailable 13（另有 partial 9、implemented 6）。
- `0.1.89` 已通过 `scripts/install-package.mjs` 真实安装到本机 Core（`enabled: true`、`failureCount: 0`），以当前源码重建 darwin/arm64 `AmitiaCore` 并重启完整服务后 `/api/health` 为 `ready: true`、`/readyz` 为 `state: ready`（7 ready、0 degraded、0 blocking、0 failed）；`POST /api/extension/action/dsh-compatible-page/command` 调用 `project-catalog` 已进入 Core 的 `host.project.list`，无 page session 时按设计返回原始 `input_invalid`，带 page session 的 UI 端到端验证仍待补。
- `0.1.86` 安装包 `plugin/amitia-dsh-compatible-host-0.1.86.amitiax` 大小 14697789 字节，SHA-256 `f331550a1914f52636c7880c8ac52bb338f347918dbe80660b7d19c2ef409cd7`，treeHash `9a71b15ad6a292d6e6a95e5ecb5af5dc70619c57ef92f2f34507d407583a036a`，插件测试 157/157 通过。
- `0.1.86` 已通过 `scripts/install-package.mjs` 真实安装到本机 Core（`previousVersion: 0.1.85`），`/api/health` 与 `/readyz` 均为 200（7 ready、0 degraded、0 blocking、0 failed），Core 日志无 `[dsh-activate-failed]`，catalog 缺口分布为 delegated 27、pending 14、unavailable 13（合计 54）。
- `0.1.86` 修掉三个真实缺陷：一是 Host 入口只取包根导出（`./types`、`./typert`、`./remote` 等子路径是类型与 FaceModel 数据模块，交给 `ctx.plugin` 会抛 `invalid plugin ... received object`），纯 client 包不再被判为“缺少主入口”；二是激活失败不再先写 `enabled=true` 落库、不再阻断宿主启动，失败按包记入 `activationError` 并输出完整堆栈，摘要新增 `active`；三是安装器补齐 peerDependencies 闭包（排除 `@deepseek-ai/cordis`，避免与 vendored cordis 双实例），并从首次安装结果取目标包名，避免补装 peer 后按首个依赖键解析到错包。
- `0.1.86` 用 `scripts/test-official-host-chain.mjs` 实测官方链路：`typert-registry` 与 `api-job-controller`（严格注入检查）均激活成功并各带 1 个 client 入口，`jobs-local` 因 `service "jobs" has been registered` 与 Amitia 作业运行时互斥故不安装；`typert` / `jobs` / `jobController` 三项服务全部可见，`jobController` 路由改为 `provider: "none"`。
- `0.1.83` 已通过 `scripts/install-package.mjs` 真实安装到本机 Core（`previousVersion: 0.1.82`），扩展状态为 `installed`、`enabled`、`failureCount: 0`。`/api/health` 与 `/readyz` 均为 200（7 ready、0 degraded、0 blocking、0 failed），容器 `status` 动作返回 4 个官方插件启用，catalog 计数为 Host Service 69、Event 74、Tool 77、Configuration 163、Lifecycle Event 63、Client Service 8、Client Event 4、UI Slot 120，其中缺口分布为 delegated 26、pending 15、unavailable 13（合计 54），官方引用扫描包数 416。
- `0.1.83` 为 54 项 Host Service 缺口建立逐项路由（`scripts/host-service-routes.mjs`）：`delegated` 26 项由官方包原生托管并从实现缺口计数中移除，`pending` 15 项待映射 Amitia 通用能力的 provider，`unavailable` 13 项因需要真实进程 / PTY / 隔离执行 / 外部网络账号而保持官方失败态，每项原因写入 catalog，不使用统一兜底文案。
- `0.1.83` 新增 `scripts/scan-official-references.mjs` 与 `catalog/official-references.v1.json`：扫描 416 个官方包，记录 69 项服务的字面引用、属性引用、定义包与消费者数量。实测无零引用服务，因此“零引用即 unavailable”的假设不成立，实际改为按 provider 能力路由。`lsp` 在依赖闭包内无定义点（seam 包已单独发布），按实测写入原因。
- `0.1.81` 容器改为真实 `@deepseek-ai/cordis@4.1.0` 托管：服务注册、`ctx.plugin` 激活、`inject` 满足性、`ctx.isolate` 作用域与回收全部由 cordis 原生语义决定；每个包入口在 `isolate("tools")` 内独立注入 Amitia 工具门面。`events` 与 `runtime` 两个保留名重映射为 `amitiaEvents`、`amitiaRuntime`。错误只附加稳定 code（`DSH_HOST_SERVICE_MISSING`、`DSH_HOST_SERVICE_CONFLICT`、`DSH_HOST_SERVICE_UNINJECTED`），原始 message、stack 与错误对象原样抛出。
- `0.1.81` 兼容映射改为“真实包优先、`ERR_MODULE_NOT_FOUND` 才回退 shim”；导出面守卫改为扫描已安装官方包的真实具名导入，本轮据此补齐 `@deepseek-ai/dsh-subprocess` 的 `DSH_ENV_PREFIX` 并按官方语义重写 `scrubbedParentEnv`。`dsh-lsp` 与 `dsh-shell-env` 双试点通过，覆盖 `LSP_CONFLICT`、`LSP_UNAVAILABLE` 与 `shellEnv` 全生命周期。
- 运行时 ESM 图不得出现顶层 await：plugin-host 以 `require()` 加载扩展入口，顶层 await 会抛 `ERR_REQUIRE_ASYNC_MODULE`。vendor cordis 使用静态导入。
- `0.1.79` 重新生成 Catalog 后，`amitia-gap.v1.json` 中 `lifecycleEvents` 缺口为 0；剩余缺口为 Host Service 54 项、Client Service 1 项（`workspaces`）。`0.1.86` 起 Host Service 54 项全部带路由与原因（delegated 27 / pending 14 / unavailable 13），delegated 不再计入实现缺口。
- `0.1.77` 官方 `@deepseek-ai/dsh-client-ui-theme@0.2.2-rc.2` 在本机 Client ABI 容器中加载成功，注册 `system.settings.section` 两处；官方插件自行 `provide('theme')` 覆盖容器注入，容器主题服务仍在无官方主题插件时对外提供 `theme`。Client 冒烟脚本已补齐 jsdom 的 `DOMParser`、`requestAnimationFrame`、`ResizeObserver`、`IntersectionObserver`、`localStorage` 与 `getComputedStyle`。
- 工具在恢复会话后才执行，避免结果序号与历史事件撞号。工具执行和会话持久化同时失败时保留两个原始错误及堆栈。延迟恢复和双重失败均有回归测试。
- macOS 完整项目从当前 Core 源码重建并重启后，真实协调模式验证了 `cwd` 到设备项目的映射。手动触发 Schedule 后，Core 在该项目中创建了新会话，调度历史返回投递回执，设备消息接口读到了该会话中的对应消息。临时任务、会话、项目和工作区挂载已清理。
- 本机 Core 加载临时 DSH Host 插件，创建真实 DSH 会话并通过 Session API 持久化工具结果。UI Bridge 订阅从 `available: false` 转为 `true`，序号和内容匹配，取消正常结束。取消后丢弃在途响应由回归测试覆盖。负序号保留原始 Schema 错误。
- 本机 Core 与 JavaScript 运行宿主重建后，本地模拟模型通过正常聊天入口收到动态 DSH 工具定义、发起工具调用并读到结果。UI Bridge 在同一会话读取到持久化结果。临时模型配置、工具插件和会话已清理。
- 本机已验证 Schedule 创建和取消，以及未映射 `cwd` 时保留原始错误。已知工作区到 `projectId` 的映射由测试夹具覆盖，当前角色没有项目工作区可用于本机成功路径验证。
- `0.1.72` 验证时，运行模块在发送给浏览器前移除 `sourceMappingURL`，并在加载任何 DSH Client 插件前启用 Zod `jitless`。页面重载后 DevTools Issues 为 `0 page errors`。
- 运行态共有 41 个 DSH Client contribution、覆盖 19 个 Amitia Slot；`workspace.row-toast` 已获得 `app.workspace.toast` 子 Slot 渲染授权。
- React 适配器现统一使用扁平 props，并兼容 envelope 输入；observable injected hooks 使用 `useSyncExternalStore`，Vue VNode 通过稳定容器桥接为 React 节点，嵌套 Slot 继承父上下文。
- 根级预声明宿主 Slot 使用 `parentSlotId = root` 作为哨兵。Client contribution 可以显式声明并独占 claim；具体非 root 父 Slot 的越权声明仍直接失败。
- `0.1.72` 验证时，`internal/extension/kernel` 全包测试、Kernel `go vet`、前端 Slot/React Host/页面上下文定向测试 31/31、UI Slot Catalog 120/120 通过。前端全量 `vue-tsc` 被既有 `PetInstallationsView.vue` 缺失字段阻断。
- 当前 Core endpoint 为 `127.0.0.1:53794`，PID `76344`，SHA-256 `44e5d1ef28937e8078c5d6efe909b5a28c84b4990f68c0a50df1c31fff9bd3f8`；Core 构建 Source Gate 为 `1da0656f5e41ceb16a1fcce7802d4f2efb581205abf6cc5e16148c5824c70b0b`。本轮删除 Source Map 尾注、补充工作区搜索/变更桥接后，冻结清单共 11267 项并校验通过。
- `/api/health` 与 `/readyz` 均为 ready，运行时 7 ready、0 degraded、0 blocking、0 failed。

## 尚未完成

- `remote.directoryPicker` 的 Android/iOS 实现。
- 官方 DSH Client 对模型工具结果的真实界面渲染验证。
- `sessions.sessionOf` 在无可解析会话作用域时只上报 `no Amitia host capability` 告警，尚未补齐宿主能力。
- 真实前端页面上人工点击审批卡片的验证（当前用脚本复现 `chat-runtime/approval` 已得到 `allowed-once`，但尚未在渲染层点过）。
- Remote credentials、LLM providers、模型发现和模型目录。
- Session search、fork、retention、projection refresh 的完整语义。
- Agent setup、seed、fork history。
- MCP prompts、sampling、elicitation。
- Schedule `agentPreset` 仍缺少已验证的映射。
- React CSS 样式隔离、跨插件 Slot 清理压力测试和 Android/iOS 真机完整旅程。

## 验证入口

```text
../../AmitiaData/node/bin/node --test tests/*.test.mjs
../../AmitiaData/node/bin/node scripts/test-official-plugin.mjs @deepseek-ai/dsh-tool-todo@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-plugin.mjs @deepseek-ai/dsh-tool-subagent@0.2.2-rc.2 '{"provider":"spawn"}'
../../AmitiaData/node/bin/node scripts/test-official-plugin.mjs @deepseek-ai/dsh-tool-schedule@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-jobs@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-plan@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-sidebar@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-schedule@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-sidebar-right@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-sidebar-files@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-sidebar-documentpreview@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-settings-models@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-plugin-manager@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-conversation@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-workspace@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-chat@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-approval@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-tool@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-trajectory@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-client.mjs @deepseek-ai/dsh-client-ui-theme@0.2.2-rc.2
../../AmitiaData/node/bin/node scripts/test-official-mcp.mjs
../../AmitiaData/node/bin/node --experimental-strip-types scripts/generate-catalogs.mjs
../../AmitiaData/node/bin/node --test tests/catalog.test.mjs
../../AmitiaData/node/bin/node scripts/build-package.mjs
../../AmitiaData/node/bin/node scripts/install-package.mjs
```

当前 `Catalog v1` 覆盖 Host Service 69 项、Host Event 74 项、Host Tool 77 项、Host Configuration 163 项、Session Lifecycle Event 63 项、Client Service 8 项、Client Event 4 项和 UI Slot/Factory 120 项。UI Slot/Factory 120 项均已有明确映射，没有 P12 未映射项；Session Lifecycle Event 63 项全部 implemented（surface 5 项走 `session-envelope+surface`，log-only 58 项走 `session-envelope`）。

## 客户端 UI 装载现状（0.1.192/0.1.193）

- DSH 客户端模块在**插件页**（`#/extension/page/dsh-compatible-page`）内全量装载：40 个客户端贡献，含 `chat.message.renderer`、`chat.composer.dock.top`、`provider.conversation.messages`、`chat.conversation.header`、`provider.page.provider:conversation`。
- **应用级装载默认关闭**（`front/src/composables/useAppClientModules.ts` 的 `APP_CLIENT_MODULES_ENABLED`）。原因是它与 Amitia 原生会话页存在非确定性竞态：同一产物多次采样，`composer` 数量会在 1 / 0 之间摇摆，DSH 组件争抢 `chat.*` 槽位会把原生输入框卸载。
- 已修复的阻塞：`useSessionAccess` / `useWorkspaceAccess` 经 owner 上下文注入；子槽位授权（`sharesSessionSlotArea` 同时用于 `authorizeChildSlot` 与 `isAuthorizedChild`）；`useFactorySlot`；`sessions.retain/using/scopeOf/sessionOf/searchResultLimit`；`workspaces.initializeDefault/pinSession/unpinSession`；`remoteAccess.source/whenResourceAccess`；`ctx.waterfall`；React 挂载句柄 Promise 契约（`undefined.catch` 崩溃根因）。
- 下一步：把 `provider.page.provider` 的 **client 贡献**接入 Amitia 扩展页面列表并以 `scope=root` 隔离渲染，让官方会话 UI 独立成页，而不是共享原生 `chat.*` 槽位。

## /dsh-conversation 消息流现状（0.1.262）

- 会话页已用官方 `dsh-client-ui-conversation` + `dsh-client-ui-chat` 渲染真实消息：`chat.conversation.session` → `conversation.view` → `conversation.chat.flow` → `conversation.chat.node` 全链路挂载，`[data-chat-node-key]` 实测 2 个节点（`user:*`、`assistant-step:*`），助手消息正文来自真实模型回复，控制台无异常。
- 数据面：`host.conversation.read` 需要 `conversation` 角色作用域，内核新增通用 `host.conversation.scope`（入参 `{conversationId}`，复用 `createConversationTargetSnapshot`，权限沿用 `conversation.read`），插件侧 `conversation-scope` action 在 `dsh-session:<id>` 无持久化快照时兜底调用。
- 事件面：`conversation-read` 的消息被投影成 DSH `SessionEventLikeEntry`（`{type:'event', event}`），包含 `turn/start`、`step/start`、`user/message`、`assistant/message`、`step/end`、`turn/end`，消息类事件带 `surfaceOp:'append'`。
- 桥接关键修复：slot-level hook 工厂（`SlotHookFactory`）按 `(standard, hookContext)` 调用；子槽位 `commonInject` 合并；宿主非 keyed 槽位时把 `entryKey` 翻译为 `only` 过滤；`TurnLocation.start/end` 使用完整事件形状；user 消息 `source.form` 补 `null`。
- 消息渲染已双向完成：用户气泡与助手回复都由官方组件真实渲染。关键修复是 `chat.message.renderer` 属于 **chain** 槽位，工厂贡献必须按调用方标记在 `select` 里互斥选中，否则先注册的工厂会吞掉所有派发。
- 发送链路已打通并验证：`session.prompt` → `conversation-message-send` → 宿主落库，发送后同时刷新事件流与 `conversation-data` 投影（含 400/1200/3000/6000ms 延时补偿），界面无需重载即追加节点。
- 会话切换已验证：有消息会话分别渲染 6 / 2 个节点且无异常，空会话正确进入 hero 态。
- 界面发送已端到端验证：输入草稿后点击 `发送消息` 按钮，草稿清空、节点从 6 增至 7、宿主消息数同步增加、控制台无异常。
- 已知限制：合成回车（`Input.dispatchKeyEvent`）与合成鼠标点击都不触发提交，只有 `button.click()`（React 合成事件）生效；因此键盘回车路径仍需在真实窗口手工确认。
- 空会话（hero）态暂不可直接输入首条消息：DSH 在该状态下等待用户选择工作区（`chat.empty.workspace`，文案「选择一个工作区开始」），`InputBar` 的 `keyboard` 尚未就绪而渲染惰性占位。依赖 Workspaces 映射补全（`workspaces` 目前为 partial），不是槽位桥接缺失。

## 与原生界面的接入形态（0.1.303 起）

- DSH 客户端外壳由宿主 `front/src/components/AppLayout.vue` 常驻装载（`contributionIds: ["dsh-desktop-shell-host"]`），shell 组件渲染 `null`，只负责注册 DSH 客户端包。
- 插件 `src/client-ui/index.mjs` 的 `DSH_PAGE_SHELL_SLOTS` 明确排除页面级与原生已独占的槽位：`main.conversation`、`conversation.content/session/session.header/header/view/chat.flow`、`conversation.composer(+bar/dock)`、`conversation.input.*`、`conversation.chat.node`、`conversation.user.text(+node)`、`conversation.chat.user.content`、`conversation.chat.reasoning.content`。排除原因：这些槽位被 DSH 注册后会顶掉原生会话根/消息/输入框，导致原生聊天页白屏。
- 实测结果：原生聊天页消息与输入框正常（消息只渲染一次）、DSH 贡献 6 个（含头部"归档会话""置顶会话"按钮与 4 个 portal 覆盖层）、控制台 0 异常。
- 未接入：DSH 自有的消息节点渲染（与原生消息渲染重复，需要按原生 owner props 做适配后才能启用）。

