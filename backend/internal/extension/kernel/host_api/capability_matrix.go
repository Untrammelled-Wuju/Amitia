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
	Name        string
	Route       string
	Resolution  CapabilityResolution
	Reason      string
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
		Route:      "host.conversation.read",
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #13-#15 直接复用",
	},
	{
		Name:       "Memory",
		Route:      "host.memory.query",
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #16-#19 直接复用",
	},
	{
		Name:       "Resource",
		Route:      string(MethodResourceOpen) + "/" + string(MethodResourceRead) + "/" + string(MethodResourceWrite) + "/" + string(MethodResourceClose) + "/" + string(MethodResourceStat),
		Resolution: ResolutionReuseExisting,
		Reason:     "Existing Gateway 已注册；G12 #20-#21 直接复用",
	},
	{
		Name:       "Provider/Model",
		Route:      string(MethodProviderInvoke),
		Resolution: ResolutionCompose,
		Reason:     "Permission/Method 常量已定义；G12 #22-#26 插件可自管模型或走 Provider Invoke（骨架就绪）",
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
		string(MethodResourceOpen), string(MethodResourceRead), string(MethodResourceWrite), string(MethodResourceClose), string(MethodResourceStat),
		// Event
		string(MethodEventEmit), string(MethodEventSubscribe), string(MethodEventUnsubscribe),
		// Schedule
		string(MethodScheduleCreate), string(MethodScheduleCancel), string(MethodScheduleList),
		// UI
		string(MethodUINotify), string(MethodUIDialog), string(MethodUINavigate),
		// Clipboard
		string(MethodClipboardWrite), string(MethodClipboardRead),
		// Data
		string(MethodCharacterRead), string(MethodConversationRead), string(MethodMemoryQuery),
		// Provider / Tool / Runtime
		string(MethodProviderInvoke), string(MethodToolExecute), string(MethodRuntimeHealth),
		// Migration
		string(MethodMigrationSQLExecute), string(MethodMigrationSQLQuery),
	}
}
