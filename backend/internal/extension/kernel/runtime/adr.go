package runtime

type RuntimeADR struct {
	DecisionID          string
	Title               string
	Status              string
	DecidedAt           string
	PrimaryRoute        PrimaryRoute
	SupplementaryRoutes []SupplementaryRoute
	RejectedRoutes      []RejectedRoute
	Rationale           []string
	Constraints         []string
}

type PrimaryRoute struct {
	Name        string
	Description string
	Components  []string
}

type SupplementaryRoute struct {
	Name        string
	Description string
	UseCase     string
}

type RejectedRoute struct {
	Name   string
	Reason string
}

var ADR = RuntimeADR{
	DecisionID: "ADR-001-RUNTIME",
	Title:      "Amitia 桌面第三方插件 Runtime 技术方案",
	Status:     "accepted",
	DecidedAt:  "2026-07-26",
	PrimaryRoute: PrimaryRoute{
		Name:        "Independent Node.js Subprocess",
		Description: "独立 Node.js 子进程 + TypeScript/JavaScript 插件 SDK + 自定义受控模块加载器 + 内部 JSON-RPC + Host API Gateway + 每 Module 独立实例",
		Components: []string{
			"independent_node_subprocess",
			"typescript_sdk",
			"controlled_module_loader",
			"internal_json_rpc",
			"host_api_gateway",
			"singleton_per_module",
		},
	},
	SupplementaryRoutes: []SupplementaryRoute{
		{
			Name:        "Task Runtime",
			Description: "短期或隔离运行的任务 Runtime",
			UseCase:     "数据迁移、长计算、批量导入、可取消后台任务",
		},
		{
			Name:        "Trusted Service Runtime",
			Description: "受信任服务 Runtime",
			UseCase:     "原生二进制、MCP Server、Service Module",
		},
		{
			Name:        "WASM Runtime",
			Description: "WebAssembly 计算 Runtime",
			UseCase:     "纯计算、确定性转换、资源受限算法",
		},
		{
			Name:        "Restricted UI Runtime",
			Description: "受限 UI Runtime（Sandbox WebUI）",
			UseCase:     "扩展 UI Contribution",
		},
	},
	RejectedRoutes: []RejectedRoute{
		{
			Name:   "Electron Renderer 执行插件",
			Reason: "Renderer 与 UI 生命周期耦合，难以稳定管理后台任务",
		},
		{
			Name:   "Electron Main Process require",
			Reason: "Main 是桌面宿主关键进程，第三方代码进入会导致完全失控",
		},
		{
			Name:   "动态 Go Plugin",
			Reason: "平台支持不一致，ABI 耦合，无法安全卸载，Windows 支持问题",
		},
		{
			Name:   "仅使用 WASM 作为全部插件能力",
			Reason: "异步宿主 API 复杂，桌面/网络/流式任务 Host Binding 成本高",
		},
		{
			Name:   "宿主进程内弱隔离 JavaScript VM",
			Reason: "不能作为主要安全边界",
		},
	},
	Rationale: []string{
		"与 Electron/Vue/TypeScript 技术栈一致",
		"插件开发者生态成熟",
		"支持 TypeScript SDK 和异步任务",
		"跨 Windows/macOS/Linux 支持",
		"独立进程崩溃隔离",
		"通过 IPC 完全阻断插件直接访问 Go 内部服务",
		"可独立设置工作目录、环境变量、资源限制和进程组",
	},
	Constraints: []string{
		"不直接复用 Electron Main",
		"不直接开放 Node Integration",
		"不依赖系统 Node",
		"禁止在线 npm install",
		"禁止任意 child_process/fs/net",
		"禁止第三方 .node 原生模块",
		"禁止插件自建 IPC 后门",
	},
}

type ProcessGranularity string

const (
	GranularitySingletonPerModule    ProcessGranularity = "singleton_per_module"
	GranularitySingletonPerExtension ProcessGranularity = "singleton_per_extension"
	GranularityPool                  ProcessGranularity = "pool"
	GranularityPerInvocation         ProcessGranularity = "per_invocation"
)

type RuntimeType string

const (
	RuntimeTypeMain           RuntimeType = "javascript_main"
	RuntimeTypeTask           RuntimeType = "task"
	RuntimeTypeTrustedService RuntimeType = "trusted_service"
	RuntimeTypeWASM           RuntimeType = "wasm"
	RuntimeTypeUI             RuntimeType = "ui"
	RuntimeTypeLegacyGo       RuntimeType = "legacy_go"
)

type ProcessBoundary struct {
	ExecutablePath    string
	WorkingDirectory  string
	EnvironmentFilter []string
	ResourceLimits    ResourceLimits
	IsolationLevel    IsolationLevel
	StdioConfig       StdioConfig
}

type ResourceLimits struct {
	MaxMemoryMB        int
	MaxConcurrentCalls int
	MaxQueueDepth      int
	SingleCallTimeout  string
	LogRatePerSecond   int
	HostAPIRatePerSec  int
	MaxOpenHandles     int
	MaxMessageSizeKB   int
}

type IsolationLevel string

const (
	IsolationProcess      IsolationLevel = "process"
	IsolationProcessGroup IsolationLevel = "process_group"
	IsolationContainer    IsolationLevel = "container"
)

type StdioConfig struct {
	StdinMode  string
	StdoutMode string
	StderrMode string
}

type HostBinary struct {
	Name                 string
	Version              string
	Path                 string
	Platform             string
	Architecture         string
	SupportedAPIs        []string
	CompatibilityVersion int
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemoryMB:        256,
		MaxConcurrentCalls: 8,
		MaxQueueDepth:      64,
		SingleCallTimeout:  "30s",
		LogRatePerSecond:   100,
		HostAPIRatePerSec:  50,
		MaxOpenHandles:     64,
		MaxMessageSizeKB:   1024,
	}
}

func DefaultProcessBoundary() ProcessBoundary {
	return ProcessBoundary{
		IsolationLevel: IsolationProcess,
		StdioConfig: StdioConfig{
			StdinMode:  "pipe",
			StdoutMode: "pipe",
			StderrMode: "pipe",
		},
		ResourceLimits: DefaultResourceLimits(),
	}
}
