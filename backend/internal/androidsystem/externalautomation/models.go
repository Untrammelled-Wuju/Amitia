package externalautomation

type CapabilityState struct {
	Supported          bool   `json:"supported"`
	CanResolveApps     bool   `json:"canResolveApps"`
	CanLaunchApps      bool   `json:"canLaunchApps"`
	CanResolveURI      bool   `json:"canResolveUri"`
	CanOpenURI         bool   `json:"canOpenUri"`
	CanOpenSettings    bool   `json:"canOpenSettings"`
	CanInvokeIntent    bool   `json:"canInvokeIntent"`
	CanInspectForeground bool `json:"canInspectForeground"`
	CanWaitForeground  bool   `json:"canWaitForeground"`
	State              string `json:"state"`
	Reason             string `json:"reason,omitempty"`
}

type AppTarget struct {
	PackageName string `json:"packageName,omitempty"`
	Component   string `json:"component,omitempty"`
	Label       string `json:"label,omitempty"`
}

type ResolveAppRequest struct {
	Query string `json:"query"`
}

type ResolvedApp struct {
	PackageName string `json:"packageName"`
	Component   string `json:"component,omitempty"`
	Label       string `json:"label,omitempty"`
	Launchable  bool   `json:"launchable"`
	SystemApp   bool   `json:"systemApp,omitempty"`
}

type OpenAppRequest struct {
	PackageName string         `json:"packageName"`
	Component   string         `json:"component,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"`
	NewTask     bool           `json:"newTask,omitempty"`
}

type ResolveURIRequest struct {
	URI    string `json:"uri"`
	Action string `json:"action,omitempty"`
}

type ResolvedURI struct {
	URI            string        `json:"uri"`
	Scheme         string        `json:"scheme"`
	Resolved       bool          `json:"resolved"`
	Handlers       []ResolvedApp `json:"handlers,omitempty"`
	DefaultHandler *ResolvedApp  `json:"defaultHandler,omitempty"`
}

type OpenURIRequest struct {
	URI            string `json:"uri"`
	PackageName    string `json:"packageName,omitempty"`
	PreferExternal bool   `json:"preferExternal,omitempty"`
}

type OpenSettingsRequest struct {
	Page        string `json:"page"`
	PackageName string `json:"packageName,omitempty"`
}

type IntentSpec struct {
	Action      string         `json:"action"`
	Data        string         `json:"data,omitempty"`
	PackageName string         `json:"packageName,omitempty"`
	Component   string         `json:"component,omitempty"`
	Categories  []string       `json:"categories,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"`
	Mode        string         `json:"mode,omitempty"`
}

type ForegroundState struct {
	PackageName string `json:"packageName,omitempty"`
	Component   string `json:"component,omitempty"`
	Label       string `json:"label,omitempty"`
	DisplayID   int    `json:"displayId,omitempty"`
	ObservedAt  int64  `json:"observedAt"`
	Source      string `json:"source"`
	Confidence  string `json:"confidence,omitempty"`
}

type WaitForegroundRequest struct {
	PackageName string `json:"packageName"`
	Component   string `json:"component,omitempty"`
	TimeoutMS   int    `json:"timeoutMs,omitempty"`
}

type ActionResult struct {
	Success          bool   `json:"success"`
	Operation        string `json:"operation"`
	TargetPackage    string `json:"targetPackage,omitempty"`
	TargetComponent  string `json:"targetComponent,omitempty"`
	Resolved         bool   `json:"resolved,omitempty"`
	Started          bool   `json:"started,omitempty"`
	UserActionRequired bool `json:"userActionRequired,omitempty"`
	Timestamp        int64  `json:"timestamp"`
}

const (
	StateAvailable       = "available"
	StateUnsupported     = "unsupported"
	StateHostUnavailable = "host_unavailable"
	StateReady           = "ready"
)

const (
	SettingsAppDetails     = "app_details"
	SettingsAccessibility  = "accessibility"
	SettingsOverlay        = "overlay"
	SettingsNotifications  = "notifications"
	SettingsBattery        = "battery"
	SettingsUnknownSources = "unknown_sources"
	SettingsWireless       = "wireless"
	SettingsBluetooth      = "bluetooth"
	SettingsLocation       = "location"
	SettingsDefaultApps    = "default_apps"
)

const (
	ModeActivity = "activity"
)

const (
	SourceAccessibility = "accessibility"
	SourceUsageStats    = "usage_stats"
	SourceActivityLifecycle = "activity_lifecycle"
)

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)
