export const HOST_SERVICE_ROUTES = Object.freeze({
  agentDefaultModel: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方包 @deepseek-ai/dsh-agent-default-model 提供，插件声明 inject 并自带该依赖后经真实 cordis 原生托管",
  }),
  agentLoop: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 bundle @deepseek-ai/dsh-agent-loop 提供，容器不复制 agent 驱动",
  }),
  agentPresets: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-agent-preset 提供注册表；mount/recompose 动态子组合由 cordis 原生托管",
  }),
  authorization: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-authorization 提供凭据流程注册表，记录写入复用已实现的 credentials",
  }),
  clientModules: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-client-modules 提供 Web 插件表，桌面端不消费",
  }),
  compaction: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-compaction seam 提供，具体 provider 为官方 compaction-basic",
  }),
  connection: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 client 连接包提供，桌面端仅需服务可见性",
  }),
  credentialsController: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-api-settings-controller 提供，底层 credentials 已由容器实现",
  }),
  deepseekAccount: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-deepseek-account 提供，登录需要 DeepSeek 账号网络通道，容器不代理第三方账号，保持官方失败态",
  }),
  deepseekLlmApiExtensions: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-deepseek-llm-api-extensions 提供请求字段注册表，纯内存",
  }),
  directoryPicker: Object.freeze({
    route: "official-package",
    provider: "amitia-wired",
    note: "list/create 走 host.directory.*，pick 走 host.directory.pick → 已注册 ui.directory.pick 的渲染宿主 → window.amitiaDesktop.selectDirectory；桌面端已注册该端点，移动端未注册时原样返回 ui_host: directory pick host unavailable；原生对话框点选尚未自动化实测",
  }),
  directoryPickerController: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "capability() 报 native 并给出 pick；pick/list/createDirectory 全部委派宿主目录接口（host.directory.pick/list/create），未知动词不伪造方法",
  }),
  fileReferences: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "list/search/stat/read 已复用 host.workspace.read 的 file_list/file_search/file_stat/file_read_text",
  }),
  goals: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-goal 提供，以会话日志为唯一来源",
  }),
  jobController: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "官方 @deepseek-ai/dsh-api-job-controller 实测在严格注入检查下直接激活（根入口 ./lib/index.js 为唯一 Host 入口，typert 与 jobs 均满足，激活后注册 jobController 并提供 1 个 client 入口）。typert 来自官方 @deepseek-ai/dsh-typert-registry；jobs 来自 Amitia host-services 已实现的作业运行时，官方 jobs-local 与之冲突（DSH_HOST_SERVICE_CONFLICT service \"jobs\" has been registered），二者不共存，容器不安装 jobs-local",
  }),
  lsp: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 包 @deepseek-ai/dsh-lsp@0.2.2-rc.2 已发布，但不在 @deepseek-ai/dsh@0.2.2-rc.2 依赖闭包内（416 个官方包实测无定义点），需显式安装；provider lsp-stdio 需真实子进程与语言服务器二进制，容器不提供，保持官方 LSP_UNAVAILABLE",
  }),
  permissionPresets: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "由官方 @deepseek-ai/dsh-permission-presets 提供，依赖 ctx.shell 执行器，容器不提供真实进程执行",
  }),
  planMode: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-plan-mode 提供，状态写入会话日志",
  }),
  ptcRuntime: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-ptc-runtime 提供；provider 需隔离执行程序，容器 Node 权限不变时保持官方失败态",
  }),
  remoteAccess: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-api-remote-access 提供访问策略注册表，纯内存",
  }),
  sandbox: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-sandbox 提供；confining 实现需宿主提供真实隔离，容器不放开即失败关闭",
  }),
  sandboxPolicy: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-sandbox-policy 提供策略服务，执行侧才需要 sandbox provider",
  }),
  sessionController: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-api-session-controller 提供，依赖会话持久化与标题服务",
  }),
  sessionFileReferences: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "同上并带 sessionId 作用域，复用 host.workspace.read 的文件枚举与读取",
  }),
  sessionLogDeepSeek: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-session-log-deepseek 提供日志贡献者，纯内存过滤规则",
  }),
  sessionPersistence: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "seam 由官方 @deepseek-ai/dsh-session-persistence 提供，jsonl 后端已接线到插件数据目录 dsh-data/sessions；0.1.104 真实运行中经官方 agents.create 写出 sessions/<cwd>/<sessionId>/session.v4.jsonl.zstd（208 字节，zstd 解压后为 session 头事件）",
  }),
  sessionProjectionCache: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "官方 @deepseek-ai/dsh-session-projection-cache 已安装并 active，容器按官方 README 注入必填 writeEveryEvents=200 与 writeIntervalMs=5000，打开 session_projcache 存储域；0.1.104 真实运行中会话创建即写出 storage/session_projcache/sessions/<sessionId>.json，含 turnBoundary/modelSelection/inbox 三行投影",
  }),
  sessionProjections: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "seam 由官方 @deepseek-ai/dsh-session-projection 提供（0.1.104 安装并 active），容器注册表改名 amitiaSessionProjections 让出官方服务名；官方 agent-loop 已在该注册表登记 turnBoundary/modelSelection/inbox 三行投影",
  }),
  sessionReferenceResolver: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-session-reference 提供，读取会话日志精确内容",
  }),
  sessionSkillCatalog: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "合并 host.skill.list 与会话本地注册，按当前会话作用域过滤",
  }),
  sessionTelemetry: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-session-telemetry 提供；Amitia 无遥测上报通道，provider 保持官方失败态",
  }),
  sessionTitle: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-session-title 提供，标题以会话日志折叠为准",
  }),
  settings: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-settings 提供 Config 投影，纯内存",
  }),
  settingsController: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-api-settings-controller 提供，底层 settings 由官方包提供",
  }),
  shell: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-shell 提供（实测可加载）；provider 需真实进程，容器 Node 权限不变时不放开",
  }),
  shellEnv: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-shell-env 提供（实测 register/list/collect/release 全通）",
  }),
  skills: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "list/get/read 走 host.skill.list 与本地 provider 注册表，返回 name/skillId/description/version/enabled",
  }),
  spillStore: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "seam 由官方 @deepseek-ai/dsh-spill 提供，本地 provider 已接线到插件数据目录 dsh-data/spill；0.1.104 真实运行中 saveText 写出 spill/session-0588e96b2117/685d18d77a52-probe.txt（1728 字节，0600，内容逐字节比对一致）",
  }),
  storage: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "seam 由官方 @deepseek-ai/dsh-storage 提供，JSON 后端已实测落盘 dsh-data/storage/workspace.json（0.1.99 真实运行）；0.2.59 服务面审计确认该键由官方包在容器内注册，不在本插件 services 对象里",
  }),
  storageDomain: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "由官方 @deepseek-ai/dsh-storage-domain 提供，域数据已实测经 storage provider 落盘（0.1.99 真实运行）",
  }),
  subagentModelSelection: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-tool-subagent 提供 settings 贡献项，纯内存记录",
  }),
  subprocess: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-subprocess 提供；provider 需真实子进程，容器 Node 权限不变时保持官方失败态",
  }),
  terminalController: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "由官方 @deepseek-ai/dsh-api-terminal-controller 提供，依赖 terminals 与真实 PTY",
  }),
  terminals: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-terminal 提供；PTY 后端需子进程，容器不放开",
  }),
  tokenMeter: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-token-meter 提供，用于本地估算与 usage 折叠",
  }),
  toolResultPruner: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-compaction-tool-result-pruner 提供确定性裁剪，纯计算",
  }),
  typert: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-typert-registry 提供 schema 注册表，纯内存",
  }),
  typertGateway: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-api-gateway 提供，活跃客户端通道就绪后可用",
  }),
  web: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-web 提供；搜索与抓取 provider 需外部网络与凭据，容器不代理",
  }),
  webApp: Object.freeze({
    route: "official-package",
    provider: "none",
    note: "由官方 @deepseek-ai/dsh-web-app 提供，属 Web 端专用，桌面端不安装不加载",
  }),
  webServer: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-host-webserver 提供；HTTP 服务端端口与路由须由宿主提供，容器不开放监听",
  }),
  workflowEngine: Object.freeze({
    route: "official-package",
    provider: "unavailable",
    note: "seam 由官方 @deepseek-ai/dsh-workflow 提供；provider workflow-ptc 需隔离执行，容器不放开",
  }),
  workspaceController: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "list/create/rename/delete/initializeDefault 接 host.project.*（路径取 primary workspace 的 rootUri，sessionIds 按会话 projectId 分组并按持久顺序字段排序）；archiveSession/unarchiveSession/pinSession/unpinSession 按会话作用域快照调 host.conversation.archive/pin；insertBefore 调 host.project.reorder、insertSessionBefore 调 host.conversation.reorder，顺序值写在项目与会话文档的 order 字段，取相邻项中点，间隙不足时统一重排",
  }),
  workspaceFiles: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "list/search/stat/read/readBytes 走 host.workspace.read；changes 以 file_list/file_stat 轮询产出 DSH ready/change 帧（文件按 stat.version 判变；目录按直接子项逐个 stat 的 version 判变，并发 8，目录项按类型标记，目标消失发 absent 帧）",
  }),
  workspaceRegistry: Object.freeze({
    route: "official-package",
    provider: "amitia-landed",
    note: "由官方 @deepseek-ai/dsh-workspace 提供，0.1.99 已激活并完成默认工作区初始化落盘；0.2.59 服务面审计确认该键由官方包在容器内注册，不在本插件 services 对象里，故 workspaceController 改为在 host-services 内直接映射 project 服务",
  }),
});

export const HOST_SERVICE_PROVIDER_STATUS = Object.freeze({
  none: Object.freeze({ status: "delegated", label: "官方包提供" }),
  amitia: Object.freeze({ status: "pending", label: "待实现 Amitia provider" }),
  "amitia-landed": Object.freeze({ status: "implemented", label: "Amitia provider 已实测落盘" }),
  "amitia-wired": Object.freeze({ status: "partial", label: "Amitia provider 已接线，写入证据待补" }),
  unavailable: Object.freeze({ status: "unavailable", label: "官方失败态" }),
});
