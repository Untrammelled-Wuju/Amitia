package runtime

type BootstrapSpec struct {
	InstanceID         string
	ExtensionID        string
	ModuleID           string
	DefinitionHash     string
	Generation         int
	Entry              string
	HostAPIVersion     string
	ResourceLimits     ResourceLimits
	LogPolicy          LogPolicy
	DevelopmentMode    bool
	SessionToken       string
	SessionTokenTTL    string
	AllowedContributions []string
	AllowedNamespaces []string
}

type LogPolicy struct {
	Level       string
	Structured  bool
	RedactSecrets bool
	MaxRatePerSec int
	Destination string
}

type BootstrapSequence struct {
	Steps []BootstrapStep
}

type BootstrapStep struct {
	Name        string
	Description string
	Required    bool
}

func DefaultBootstrapSequence() BootstrapSequence {
	return BootstrapSequence{
		Steps: []BootstrapStep{
			{Name: "process_start", Description: "Process Start", Required: true},
			{Name: "read_bootstrap_spec", Description: "Read Runtime Bootstrap Spec", Required: true},
			{Name: "open_rpc_channel", Description: "Open RPC Channel", Required: true},
			{Name: "authenticate_session", Description: "Authenticate Runtime Session", Required: true},
			{Name: "verify_definition", Description: "Verify Definition Hash/Generation", Required: true},
			{Name: "initialize_sdk", Description: "Initialize SDK", Required: true},
			{Name: "load_entry_module", Description: "Load Entry Module", Required: true},
			{Name: "call_activate", Description: "Call activate", Required: true},
			{Name: "report_ready", Description: "Report Ready", Required: true},
		},
	}
}

type BootstrapResult struct {
	InstanceID    string
	Success       bool
	FailedStep    string
	Reason        string
	StartedAt     string
	ReadyAt       string
	Duration      string
}

type RuntimeSession struct {
	InstanceID         string
	ExtensionID        string
	ModuleID           string
	Generation         int
	SessionToken       string
	DefinitionHash     string
	State              SessionState
	StartedAt          string
	Ready              bool
}

type SessionState string

const (
	SessionStateStarting   SessionState = "starting"
	SessionStateAuthenticating SessionState = "authenticating"
	SessionStateLoading    SessionState = "loading"
	SessionStateActivating SessionState = "activating"
	SessionStateReady      SessionState = "ready"
	SessionStateStopping   SessionState = "stopping"
	SessionStateStopped    SessionState = "stopped"
	SessionStateCrashed    SessionState = "crashed"
)

type ModuleLoaderPolicy struct {
	AllowedModuleTypes  []string
	DeniedModules       []string
	AllowedStdlibSubset []string
	AllowBundleRequire  bool
	AllowSDKVirtuals    bool
	AllowResourceRead   bool
	DenyAbsolutePaths   bool
	DenyDynamicDownload bool
	DenyNativeModules   bool
	DenyElectronAccess  bool
	DenyGoIPCAccess     bool
	DenyShellExecution  bool
}

func DefaultModuleLoaderPolicy() ModuleLoaderPolicy {
	return ModuleLoaderPolicy{
		AllowedModuleTypes: []string{
			"package_bundle",
			"manifest_entry",
			"sdk_virtual",
		},
		DeniedModules: []string{
			"child_process",
			"cluster",
			"dgram",
			"vm",
			"worker_threads",
		},
		AllowedStdlibSubset: []string{
			"buffer",
			"crypto",
			"events",
			"path",
			"stream",
			"string_decoder",
			"url",
			"util",
			"zlib",
		},
		AllowBundleRequire:  true,
		AllowSDKVirtuals:    true,
		AllowResourceRead:   true,
		DenyAbsolutePaths:   true,
		DenyDynamicDownload: true,
		DenyNativeModules:   true,
		DenyElectronAccess:  true,
		DenyGoIPCAccess:     true,
		DenyShellExecution:  true,
	}
}

type HostAPIReplacement struct {
	Category    string
	APIPath     string
	Description string
	DeniedNative []string
}

func HostAPIReplacements() []HostAPIReplacement {
	return []HostAPIReplacement{
		{
			Category: "network",
			APIPath:  "host.network.request",
			Description: "HTTP/HTTPS 请求由宿主代理，执行域名约束、方法、Header、Secret、代理、TLS、响应大小、审计和取消",
			DeniedNative: []string{"http", "https", "net", "dns"},
		},
		{
			Category: "file",
			APIPath:  "host.resource.*",
			Description: "包内只读资源由 Runtime Loader 提供受控读取",
			DeniedNative: []string{"fs"},
		},
		{
			Category: "storage",
			APIPath:  "host.storage.*",
			Description: "结构化存储通过 StorageBroker 命名空间隔离",
			DeniedNative: []string{"fs"},
		},
		{
			Category: "secret",
			APIPath:  "host.secret.*",
			Description: "Secret 通过 SecretBroker 加密读取",
			DeniedNative: []string{"fs", "process"},
		},
		{
			Category: "process",
			APIPath:  "host.process.*",
			Description: "进程操作由宿主审批",
			DeniedNative: []string{"child_process"},
		},
		{
			Category: "schedule",
			APIPath:  "host.schedule.*",
			Description: "计划任务由宿主执行",
			DeniedNative: []string{"timers"},
		},
	}
}
