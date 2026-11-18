package host_api

// CapabilityResolution 记录 Host API 每项宿主能力的收敛路径。
// G12 先做 Existing Gateway Route Inventory，再逐项判定是 REUSE_EXISTING 还是 NEW_MINIMAL。
type CapabilityResolution string

const (
	ResolutionReuseExisting CapabilityResolution = "REUSE_EXISTING"
	ResolutionCompose       CapabilityResolution = "COMPOSE"
	ResolutionNewMinimal    CapabilityResolution = "NEW_MINIMAL"
)

// CapabilityEntry 记录单项宿主能力的 G12 收敛结果。
type CapabilityEntry struct {
	Name       string
	Route      string
	Resolution CapabilityResolution
	Reason     string
}

// CapabilityMatrix 是 G12 正式收敛结果表。
// 每项 Existing Route 唯一注册于 Gateway，未创建任何 GameHost 平行 Host API。
var CapabilityMatrix = []CapabilityEntry{
	{
		Name:       "Character",
		Route:      "host.character.read",
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #8-#12 直接复用",
	},
	{
		Name:       "Conversation",
		Route:      string(MethodConversationList) + "/host.conversation.read/" + string(MethodConversationSearch) + "/" + string(MethodConversationCreate) + "/" + string(MethodConversationDelete),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；会话读取与创建复用 Owner 会话管理事务",
	},
	{
		Name:       "Conversation Message",
		Route:      string(MethodConversationMessageSend) + "/" + string(MethodConversationMessageAppend) + "/" + string(MethodConversationTurnInterrupt) + "/" + string(MethodConversationTurnAwait),
		Resolution: ResolutionReuseExisting,
		Reason:     "统一会话消息、追加和活动轮次中断能力，插件按当前会话或显式目标复用",
	},
	{
		Name:       "Memory",
		Route:      "host.memory.query",
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #16-#19 直接复用",
	},
	{
		Name:       "Vector",
		Route:      string(MethodVectorUpsert) + "/" + string(MethodVectorSearch) + "/" + string(MethodVectorDelete),
		Resolution: ResolutionNewMinimal,
		Reason:     "按扩展和模块命名空间隔离的通用向量读写能力",
	},
	{
		Name:       "Resource",
		Route:      string(MethodResourceOpen) + "/" + string(MethodResourceRead) + "/" + string(MethodResourceWrite) + "/" + string(MethodResourceClose) + "/" + string(MethodResourceStat) + "/" + string(MethodResourceLink) + "/" + string(MethodResourceDelete),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #20-#21 直接复用",
	},
		{
		Name:       "Model Defaults",
		Route:      string(MethodModelDefaults),
		Resolution: ResolutionReuseExisting,
		Reason:     "复用当前启用的聊天模型配置（温度、最大 token、上下文窗口），不新建设置存储",
	},
{
		Name:       "Provider/Model",
		Route:      string(MethodProviderInvoke) + "/" + string(MethodModelInvoke),
		Resolution: ResolutionCompose,
		Reason:     "provider.invoke 承载 capability provider 调用，host.model.invoke 复用当前模型配置与 modelprotocol 承载模型推理",
	},
	{
		Name:       "UI Notification",
		Route:      string(MethodUINotify),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #27-#29 直接复用",
	},
	{
		Name:       "UI Dialog",
		Route:      string(MethodUIDialog),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；其 pending/resolve 模式可复用为 User Approval 前端（G12 #52-#53）",
	},
	{
		Name:       "Runtime Health",
		Route:      string(MethodRuntimeHealth),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #30-#32 直接复用",
	},
	{
		Name:       "Runtime Snapshot",
		Route:      string(MethodRuntimeSnapshot),
		Resolution: ResolutionNewMinimal,
		Reason:     "运行时调试与决策页面通过统一只读快照契约读取数据，不直接访问管理员 HTTP 接口",
	},
	{
		Name:       "Runtime Maintenance",
		Route:      "(composed)",
		Resolution: ResolutionCompose,
		Reason:     "长期运行维护页面通过同一 Runtime Maintenance Bridge 访问任务状态、配置和受控维护动作",
	},
	{
		Name:       "Workspace",
		Route:      string(MethodWorkspaceRead) + "/" + string(MethodWorkspaceWrite),
		Resolution: ResolutionNewMinimal,
		Reason:     "工作区业务页面通过统一 Bridge 访问受控文件与 Git 能力，路径策略、只读限制和 Git 策略仍由核心执行",
	},
	{
		Name:       "Conversation Import",
		Route:      string(MethodImportExecute),
		Resolution: ResolutionNewMinimal,
		Reason:     "聊天导入页面通过受控 Host API 完成解析、批次管理和设备会话晋升，不直接访问管理员导入路由",
	},
	{
		Name:       "Storage Maintenance",
		Route:      string(MethodStorageCleanup) + "/" + string(MethodStorageBackup) + "/" + string(MethodStorageRestore),
		Resolution: ResolutionNewMinimal,
		Reason:     "存储业务页通过分离的清理、备份、恢复 Host API 访问 Owner 业务和数据可移植性事务",
	},
	{
		Name:       "Conversation Archive",
		Route:      string(MethodConversationLogsRead) + "/" + string(MethodConversationLogsManage) + "/" + string(MethodConversationLogsExport),
		Resolution: ResolutionNewMinimal,
		Reason:     "归档对话页面通过 Owner 会话管理读取、恢复、删除、生成摘要和导出，不直连设备管理 HTTP",
	},
	{
		Name:       "Adapter Invocation",
		Route:      "(composed)",
		Resolution: ResolutionCompose,
		Reason:     "外部 Adapter Provider 通过 plugin.adapter.invoke/cancel 承载新的执行语义，调用受 Manifest AllowedAdapters 与权限并集约束",
	},
	{
		Name:       "State",
		Route:      string(MethodStateGet),
		Resolution: ResolutionReuseExisting,
		Reason:     "host.state.* 已全部注册为正式 Route；runtime state 访问走此处",
	},
	{
		Name:       "Event",
		Route:      string(MethodEventEmit),
		Resolution: ResolutionReuseExisting,
		Reason:     "host.event.{emit,subscribe,unsubscribe} 已注册；业务事件走此处",
	},
	{
		Name:       "Restricted Network",
		Route:      string(MethodNetworkRequest),
		Resolution: ResolutionNewMinimal,
		Reason:     "restricted 模式保持插件进程无直连网络，由宿主统一代理 HTTP/TCP/UDP/WebSocket 并逐次校验 transport、目标、DNS/IP 与端口",
	},
	{
		Name:       "Vision",
		Route:      "(composed)",
		Resolution: ResolutionCompose,
		Reason:     "G12 #33-#41：Existing Provider 链路 + internal/vision 服务可服务于游戏截图分析；无需新建专用 Vision Route",
	},
	{
		Name:       "Structured Logging",
		Route:      "(composed)",
		Resolution: ResolutionCompose,
		Reason:     "G12 #42-#51：observability.RuntimeEventStore + log.WithFields + GameHost log channel 后端齐全；诊断日志可由 GameHost 桥接，无需新建专用 Route",
	},
	{
		Name:       "Approval",
		Route:      "(composed)",
		Resolution: ResolutionCompose,
		Reason:     "G12 #52-#62：host.ui.dialog (pendingDialog) + PermissionBroker.RecordApproval + Execution StatusAwaitingApproval 基础设施已齐全；不新建专用 Route",
	},
	{
		Name:       "Plugin Composition",
		Route:      string(MethodPluginContributionRegister) + "/" + string(MethodPluginContributionUnregister) + "/" + string(MethodPluginServiceProvide) + "/" + string(MethodPluginServiceUnprovide) + "/" + string(MethodPluginServiceRequire) + "/" + string(MethodPluginServiceRelease),
		Resolution: ResolutionNewMinimal,
		Reason:     "动态贡献与插件服务通过统一 Broker 注册、校验和撤销",
	},
	{
		Name:       "Project",
		Route:      string(MethodProjectList) + "/" + string(MethodProjectCreate) + "/" + string(MethodProjectUpdate) + "/" + string(MethodProjectDelete),
		Resolution: ResolutionReuseExisting,
		Reason:     "复用设备归属业务层 Query/CreateProject/Edit 项目通道，未新建第二套项目系统",
	},
	{
		Name:       "Conversation Archive",
		Route:      string(MethodConversationArchive),
		Resolution: ResolutionReuseExisting,
		Reason:     "复用设备归属业务层 Edit 的 conversation archived 键，与侧边栏归档同一写入路径",
	},
	{
		Name:       "Skill Catalog",
		Route:      string(MethodSkillList),
		Resolution: ResolutionReuseExisting,
		Reason:     "复用扩展内核已有的 agent_skill 目录，按当前会话作用域列出技能；技能目录属于会话上下文，权限沿用 conversation.read，不新建第二套技能系统",
	},
		{
		Name:       "Conversation Fork",
		Route:      string(MethodConversationFork),
		Resolution: ResolutionNewMinimal,
		Reason:     "会话派生需要复制消息到新会话（含附件），复用 chat 服务的存储层新增 ForkConversation，不复用只读接口",
	},
	{
		Name:       "Conversation Prepare",
		Route:      string(MethodConversationPrepare),
		Resolution: ResolutionReuseExisting,
		Reason:     "会话预分配复用已有的 mesh 会话存储，只写入 draft 标记且首条消息前不进会话目录，不新建第二套会话系统",
	},
	{
		Name:       "Conversation Pin",
		Route:      string(MethodConversationPin),
		Resolution: ResolutionReuseExisting,
		Reason:     "复用设备归属业务层 Edit 的 conversation pinned 键，与侧边栏置顶同一写入路径",
	},
	{
		Name:       "Directory",
		Route:      string(MethodDirectoryList) + "/" + string(MethodDirectoryCreate),
		Resolution: ResolutionNewMinimal,
		Reason:     "DSH directoryPicker seam 需要宿主列目录与建目录；沿用 workspace.read/workspace.write 权限，仅接受绝对路径并拒绝父目录逃逸",
	},
}

// AllRegisteredMethods 返回当前 Gateway 中实际已注册的全部 Method。
// 这是 G14 #4-#5 能力盘点的正式 Source of Truth。
func AllRegisteredMethods() []string {
	return []string{
		// State
		string(MethodStateGet), string(MethodStateCAS), string(MethodStateDelete), string(MethodStateList),
		// Secret
		string(MethodSecretGet),
		// Resource
		string(MethodResourceOpen), string(MethodResourceRead), string(MethodResourceWrite), string(MethodResourceClose), string(MethodResourceStat), string(MethodResourceLink), string(MethodResourceDelete),
		// Event
		string(MethodEventEmit), string(MethodEventSubscribe), string(MethodEventUnsubscribe),
		// Schedule
		string(MethodScheduleCreate), string(MethodScheduleCancel), string(MethodScheduleList), string(MethodScheduleRun), string(MethodScheduleUpdate), string(MethodScheduleHistory),
		// UI
		string(MethodUINotify), string(MethodUIDialog), string(MethodUINavigate),
		// Clipboard
		string(MethodClipboardWrite), string(MethodClipboardRead),
		// Data
		string(MethodCharacterRead), string(MethodCharacterList), string(MethodConversationList), string(MethodConversationRead), string(MethodConversationSearch), string(MethodConversationScope), string(MethodMemoryQuery),
		// Provider / Tool / Runtime / mediated network
		string(MethodProviderInvoke), string(MethodToolExecute), string(MethodPluginPreview), string(MethodPluginInstall), string(MethodPluginUninstall), string(MethodPluginList), string(MethodRuntimeHealth), string(MethodNetworkRequest),
		string(MethodWorkspaceRead), string(MethodWorkspaceWrite),
		string(MethodImportExecute),
		string(MethodStorageCleanup), string(MethodStorageBackup), string(MethodStorageRestore),
		string(MethodConversationLogsRead), string(MethodConversationLogsManage), string(MethodConversationLogsExport),
		string(MethodPluginContributionRegister), string(MethodPluginContributionUnregister),
		string(MethodPluginServiceProvide), string(MethodPluginServiceUnprovide), string(MethodPluginServiceRequire), string(MethodPluginServiceRelease),
		string(MethodConversationMessageSend),
		string(MethodConversationMessageAppend),
		string(MethodConversationTurnInterrupt),
		string(MethodConversationTurnAwait),
		string(MethodConversationCreate),
		string(MethodConversationDelete),
		string(MethodConversationArchive), string(MethodConversationPin), string(MethodConversationFork), string(MethodConversationPrepare), string(MethodDirectoryList), string(MethodDirectoryCreate), string(MethodSkillList),
		string(MethodProjectList), string(MethodProjectCreate), string(MethodProjectUpdate), string(MethodProjectDelete),
		string(MethodVectorUpsert), string(MethodVectorSearch), string(MethodVectorDelete),
		string(MethodNetworkTCPOpen), string(MethodNetworkTCPRead), string(MethodNetworkTCPWrite), string(MethodNetworkTCPClose),
		string(MethodNetworkUDPOpen), string(MethodNetworkUDPReceive), string(MethodNetworkUDPSend), string(MethodNetworkUDPClose),
		string(MethodNetworkWebSocketOpen), string(MethodNetworkWebSocketReceive), string(MethodNetworkWebSocketSend), string(MethodNetworkWebSocketClose),
		// Migration
		string(MethodMigrationSQLExecute), string(MethodMigrationSQLQuery),
	}
}
