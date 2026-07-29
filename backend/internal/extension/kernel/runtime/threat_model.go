package runtime

type Threat struct {
	ID          string
	Category    ThreatCategory
	Description string
	Vector      string
	Mitigation  string
	Severity    string
}

type ThreatCategory string

const (
	ThreatCategoryProcessEscalation ThreatCategory = "process_escalation"
	ThreatCategoryIPCBackdoor       ThreatCategory = "ipc_backdoor"
	ThreatCategoryModuleLoading     ThreatCategory = "module_loading"
	ThreatCategoryNativeModule      ThreatCategory = "native_module"
	ThreatCategoryFileSystemAccess  ThreatCategory = "filesystem_access"
	ThreatCategoryNetworkAccess     ThreatCategory = "network_access"
	ThreatCategorySecretLeak        ThreatCategory = "secret_leak"
	ThreatCategoryElectronAccess    ThreatCategory = "electron_access"
	ThreatCategoryDenialOfService   ThreatCategory = "denial_of_service"
	ThreatCategorySupplyChain       ThreatCategory = "supply_chain"
	ThreatCategoryRuntimeCrash      ThreatCategory = "runtime_crash"
	ThreatCategorySideChannel       ThreatCategory = "side_channel"
)

func ThreatModel() []Threat {
	return []Threat{
		{
			ID:          "T-001",
			Category:    ThreatCategoryProcessEscalation,
			Description: "插件尝试通过漏洞提权到宿主进程",
			Vector:      "利用 Node.js 漏洞或原生模块漏洞",
			Mitigation:  "独立进程、最小权限 OS 账户、Host API Gateway、不提供宿主路径",
			Severity:    "high",
		},
		{
			ID:          "T-002",
			Category:    ThreatCategoryIPCBackdoor,
			Description: "插件尝试直接访问 Go IPC 或建立后门",
			Vector:      "通过环境变量、共享句柄、调试端口",
			Mitigation:  "Bootstrap Spec 通过安全 IPC 传入、禁止自建 Socket、过滤环境变量",
			Severity:    "critical",
		},
		{
			ID:          "T-003",
			Category:    ThreatCategoryModuleLoading,
			Description: "插件尝试加载未声明模块或绝对路径",
			Vector:      "动态 require、eval、动态 import",
			Mitigation:  "自定义模块加载器、白名单、Manifest 声明匹配",
			Severity:    "high",
		},
		{
			ID:          "T-004",
			Category:    ThreatCategoryNativeModule,
			Description: "插件加载原生 .node 模块绕过隔离",
			Vector:      "通过 .node 文件执行任意原生代码",
			Mitigation:  "禁止原生模块加载、Integrity 校验、安装时不执行",
			Severity:    "critical",
		},
		{
			ID:          "T-005",
			Category:    ThreatCategoryFileSystemAccess,
			Description: "插件直接访问任意文件系统",
			Vector:      "通过 fs 模块读取宿主配置、Secret、其他扩展数据",
			Mitigation:  "禁止任意 fs、统一 host.resource/host.storage、命名空间隔离",
			Severity:    "high",
		},
		{
			ID:          "T-006",
			Category:    ThreatCategoryNetworkAccess,
			Description: "插件直接发起任意网络请求",
			Vector:      "通过 http/https/net 模块绕过域名和审计",
			Mitigation:  "禁止原生网络 API、统一 host.network.request、域名约束、审计",
			Severity:    "high",
		},
		{
			ID:          "T-007",
			Category:    ThreatCategorySecretLeak,
			Description: "插件读取其他扩展或宿主 Secret",
			Vector:      "通过文件系统、环境变量、共享内存",
			Mitigation:  "SecretBroker 加密、命名空间隔离、不通过环境变量传递、Redact 日志",
			Severity:    "critical",
		},
		{
			ID:          "T-008",
			Category:    ThreatCategoryElectronAccess,
			Description: "插件尝试访问 Electron 主进程或 IPC",
			Vector:      "通过 require('electron') 或环境变量",
			Mitigation:  "独立子进程不运行在 Electron 上下文、禁止 electron 模块",
			Severity:    "critical",
		},
		{
			ID:          "T-009",
			Category:    ThreatCategoryDenialOfService,
			Description: "插件通过资源耗尽影响宿主或其他扩展",
			Vector:      "内存泄漏、CPU 循环、句柄耗尽、日志洪泛",
			Mitigation:  "资源限制、Circuit Breaker、日志速率限制、Host API 速率限制",
			Severity:    "medium",
		},
		{
			ID:          "T-010",
			Category:    ThreatCategorySupplyChain,
			Description: "插件依赖包含恶意代码",
			Vector:      "通过 npm 依赖树引入恶意包",
			Mitigation:  "禁止运行时 npm install、依赖快照、Integrity 校验、License 收集",
			Severity:    "high",
		},
		{
			ID:          "T-011",
			Category:    ThreatCategoryRuntimeCrash,
			Description: "插件进程崩溃影响宿主或其他扩展",
			Vector:      "未捕获异常、内存破坏、原生模块崩溃",
			Mitigation:  "独立进程、Circuit Breaker、重启策略、Health Check",
			Severity:    "medium",
		},
		{
			ID:          "T-012",
			Category:    ThreatCategorySideChannel,
			Description: "插件通过侧信道获取其他扩展信息",
			Vector:      "共享缓存、临时文件、计时攻击",
			Mitigation:  "命名空间隔离、独立工作目录、临时目录分离、独立进程",
			Severity:    "low",
		},
	}
}

type SecurityBoundary struct {
	Name        string
	Description string
	Layer       string
	Threats     []string
}

func SecurityBoundaries() []SecurityBoundary {
	return []SecurityBoundary{
		{
			Name:        "Independent Process",
			Description: "独立子进程隔离",
			Layer:       "OS",
			Threats:     []string{"T-001", "T-002", "T-008", "T-011"},
		},
		{
			Name:        "Runtime Supervisor",
			Description: "Runtime Supervisor 管理生命周期和健康检查",
			Layer:       "Host",
			Threats:     []string{"T-009", "T-011"},
		},
		{
			Name:        "Host API Gateway",
			Description: "Host API Gateway 统一访问宿主能力",
			Layer:       "Host",
			Threats:     []string{"T-001", "T-002", "T-005", "T-006", "T-007"},
		},
		{
			Name:        "Permission System",
			Description: "权限系统声明和授予",
			Layer:       "Kernel",
			Threats:     []string{"T-005", "T-006", "T-007"},
		},
		{
			Name:        "Scope System",
			Description: "Scope 隔离角色和会话",
			Layer:       "Kernel",
			Threats:     []string{"T-007", "T-012"},
		},
		{
			Name:        "Resource Ownership",
			Description: "资源所有权命名空间隔离",
			Layer:       "Kernel",
			Threats:     []string{"T-005", "T-007", "T-012"},
		},
		{
			Name:        "Module Loader",
			Description: "自定义模块加载器白名单",
			Layer:       "Runtime",
			Threats:     []string{"T-003", "T-004", "T-010"},
		},
		{
			Name:        "Integrity Verification",
			Description: "包完整性校验和签名验证",
			Layer:       "Package",
			Threats:     []string{"T-004", "T-010"},
		},
	}
}

type DevProdDifference struct {
	Category  string
	DevMode   string
	ProdMode  string
	Rationale string
}

func DevProdDifferences() []DevProdDifference {
	return []DevProdDifference{
		{
			Category:  "source_maps",
			DevMode:   "加载 Source Map",
			ProdMode:  "不加载 Source Map",
			Rationale: "生产模式减小内存和加载时间",
		},
		{
			Category:  "hot_reload",
			DevMode:   "支持热重载",
			ProdMode:  "禁止未签名热替换",
			Rationale: "生产模式需要完整性保证",
		},
		{
			Category:  "debug_port",
			DevMode:   "受控 Debug Port",
			ProdMode:  "不开调试端口",
			Rationale: "防止远程调试攻击",
		},
		{
			Category:  "log_level",
			DevMode:   "详细调试日志",
			ProdMode:  "摘要化错误",
			Rationale: "生产模式避免日志泄露",
		},
		{
			Category:  "source_dir",
			DevMode:   "读取 src 目录",
			ProdMode:  "只读 dist",
			Rationale: "生产模式不暴露源码",
		},
		{
			Category:  "secret_display",
			DevMode:   "可显示 Secret（开发信任）",
			ProdMode:  "Redact Secret",
			Rationale: "防止 Secret 泄露",
		},
		{
			Category:  "trust_level",
			DevMode:   "Development Trust（仅本地工作区）",
			ProdMode:  "需要正式签名和信任",
			Rationale: "开发信任不能外溢为正式信任",
		},
	}
}

type PerformanceBaseline struct {
	Category    string
	Metric      string
	Target      string
	Measurement string
}

func PerformanceBaselines() []PerformanceBaseline {
	return []PerformanceBaseline{
		{Category: "bootstrap", Metric: "cold_start", Target: "<2s", Measurement: "从进程启动到 ready"},
		{Category: "bootstrap", Metric: "warm_start", Target: "<500ms", Measurement: "复用 Runtime 的启动"},
		{Category: "invocation", Metric: "tool_call_p50", Target: "<50ms", Measurement: "Tool 调用 50 分位"},
		{Category: "invocation", Metric: "tool_call_p99", Target: "<500ms", Measurement: "Tool 调用 99 分位"},
		{Category: "memory", Metric: "baseline_rss", Target: "<100MB", Measurement: "Runtime 基线内存"},
		{Category: "memory", Metric: "peak_rss", Target: "<256MB", Measurement: "Runtime 峰值内存"},
		{Category: "rpc", Metric: "round_trip", Target: "<5ms", Measurement: "JSON-RPC 往返"},
		{Category: "host_api", Metric: "latency", Target: "<10ms", Measurement: "Host API 调用延迟"},
	}
}
