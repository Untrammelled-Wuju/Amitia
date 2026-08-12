package permission

import "github.com/u-ai/backend/internal/extension/kernel/capability"

type PermissionCategory string

const (
	CategoryHostData   PermissionCategory = "host_data"
	CategoryFilesystem PermissionCategory = "filesystem"
	CategoryNetwork    PermissionCategory = "network"
	CategoryDesktop    PermissionCategory = "desktop"
	CategoryExtension  PermissionCategory = "extension"
	CategoryMCP        PermissionCategory = "mcp"
	CategoryWorkflow   PermissionCategory = "workflow"
	CategoryProvider   PermissionCategory = "provider"
	CategoryService    PermissionCategory = "service"
	CategoryGameHost   PermissionCategory = "gamehost"
	CategoryMedia      PermissionCategory = "media"
)

type ApprovalMode string

const (
	ApprovalAuto        ApprovalMode = "auto"
	ApprovalManual      ApprovalMode = "manual"
	ApprovalDeny        ApprovalMode = "deny"
	ApprovalFullControl ApprovalMode = "full_control"
)

type ChildInvocationPolicy string

const (
	ChildInherit    ChildInvocationPolicy = "inherit"
	ChildReevaluate ChildInvocationPolicy = "reevaluate"
	ChildDeny       ChildInvocationPolicy = "deny"
)

type PermissionDefinition struct {
	ID                  string                `json:"id"`
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	Category            PermissionCategory    `json:"category"`
	RiskLevel           capability.RiskLevel  `json:"riskLevel"`
	AllowedScopes       []ScopeType           `json:"allowedScopes"`
	PersistentGrantable bool                  `json:"persistentGrantable"`
	RequiresPerUse      bool                  `json:"requiresPerUse"`
	BackgroundAllowed   bool                  `json:"backgroundAllowed"`
	ChildInvocation     ChildInvocationPolicy `json:"childInvocation"`
	TrustedOnly         bool                  `json:"trustedOnly"`
	DefaultApproval     ApprovalMode          `json:"defaultApproval"`
}

type PermissionDefinitionRegistry struct {
	definitions map[string]PermissionDefinition
}

func NewPermissionDefinitionRegistry() *PermissionDefinitionRegistry {
	r := &PermissionDefinitionRegistry{
		definitions: make(map[string]PermissionDefinition),
	}
	r.registerBuiltin()
	return r
}

func (r *PermissionDefinitionRegistry) Register(def PermissionDefinition) {
	r.definitions[def.ID] = def
}

func (r *PermissionDefinitionRegistry) Get(id string) (PermissionDefinition, bool) {
	def, ok := r.definitions[id]
	return def, ok
}

func (r *PermissionDefinitionRegistry) List() []PermissionDefinition {
	result := make([]PermissionDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		result = append(result, def)
	}
	return result
}

func (r *PermissionDefinitionRegistry) ValidateAll(ids []string) []string {
	unknown := make([]string, 0)
	for _, id := range ids {
		if _, ok := r.definitions[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	return unknown
}

func (r *PermissionDefinitionRegistry) ListByCategory(cat PermissionCategory) []PermissionDefinition {
	result := make([]PermissionDefinition, 0)
	for _, def := range r.definitions {
		if def.Category == cat {
			result = append(result, def)
		}
	}
	return result
}

func (r *PermissionDefinitionRegistry) registerBuiltin() {
	builtins := []PermissionDefinition{
		{ID: "character.read", Name: "Read Character", Description: "Read character profile and settings", Category: CategoryHostData, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "character.write", Name: "Write Character", Description: "Modify character profile and settings", Category: CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "conversation.read", Name: "Read Conversation", Description: "Read conversation messages", Category: CategoryHostData, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "conversation.write", Name: "Write Conversation", Description: "Insert messages into conversation", Category: CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "message.send", Name: "Send Message", Description: "Send message to user", Category: CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "memory.read", Name: "Read Memory", Description: "Read character memories", Category: CategoryHostData, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "memory.write", Name: "Write Memory", Description: "Create or update memories", Category: CategoryHostData, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "memory.delete", Name: "Delete Memory", Description: "Delete memories", Category: CategoryHostData, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "files.read", Name: "Read Files", Description: "Read files on disk", Category: CategoryFilesystem, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeResource}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
		{ID: "files.write", Name: "Write Files", Description: "Write or create files", Category: CategoryFilesystem, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeResource}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "files.delete", Name: "Delete Files", Description: "Delete files", Category: CategoryFilesystem, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeResource}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "network.request", Name: "Network Request", Description: "Make HTTP requests", Category: CategoryNetwork, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeResource}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
		{ID: "desktop.capture", Name: "Desktop Capture", Description: "Capture screen content", Category: CategoryDesktop, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "desktop.input", Name: "Desktop Input", Description: "Control keyboard and mouse", Category: CategoryDesktop, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalFullControl},
		{ID: "desktop.notification", Name: "Desktop Notification", Description: "Show desktop notifications", Category: CategoryDesktop, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "extensions.install", Name: "Install Extension", Description: "Install extensions", Category: CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "extensions.enable", Name: "Enable Extension", Description: "Enable or disable extensions", Category: CategoryExtension, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "extensions.invoke", Name: "Invoke Extension", Description: "Invoke extension tools", Category: CategoryExtension, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "mcp.server.connect", Name: "Connect MCP Server", Description: "Connect to MCP servers", Category: CategoryMCP, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "mcp.tools.invoke", Name: "Invoke MCP Tools", Description: "Call MCP server tools", Category: CategoryMCP, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
		{ID: "workflow.execute", Name: "Execute Workflow", Description: "Execute automated workflows", Category: CategoryWorkflow, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildReevaluate, DefaultApproval: ApprovalManual},
		{ID: "provider.use", Name: "Use Provider", Description: "Use AI provider", Category: CategoryProvider, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "provider.configure", Name: "Configure Provider", Description: "Configure provider settings", Category: CategoryProvider, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "secrets.read", Name: "Read Secrets", Description: "Read stored secrets and credentials", Category: CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalFullControl},
		{ID: "secrets.write", Name: "Write Secrets", Description: "Store secrets and credentials", Category: CategoryExtension, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalFullControl},
		{ID: "ui.contribute", Name: "UI Contribution", Description: "Register UI components", Category: CategoryExtension, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalAuto},
		{ID: "scheduler.create", Name: "Create Schedule", Description: "Create scheduled tasks", Category: CategoryExtension, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "process.spawn", Name: "Spawn Process", Description: "Start external processes", Category: CategoryFilesystem, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalFullControl},
		{ID: "service.runtime.execute", Name: "Execute Service Runtime", Description: "Execute code in the trusted service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.process.spawn", Name: "Spawn Service Process", Description: "Spawn child processes from the trusted service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.network.listen_loopback", Name: "Listen on Loopback", Description: "Listen for inbound connections on loopback interface from the service runtime", Category: CategoryService, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, BackgroundAllowed: false, ChildInvocation: ChildReevaluate, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.network.request", Name: "Service Network Request", Description: "Make outbound network requests from the trusted service runtime", Category: CategoryService, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildInherit, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.files.package_read", Name: "Read Package Files", Description: "Read files bundled within the extension package", Category: CategoryService, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, TrustedOnly: true, DefaultApproval: ApprovalAuto},
		{ID: "service.files.extension_data", Name: "Read Extension Data", Description: "Read extension-managed data files from the service runtime", Category: CategoryService, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, TrustedOnly: true, DefaultApproval: ApprovalAuto},
		{ID: "service.files.user_resource", Name: "Access User Resource", Description: "Access user-owned resources through the service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.secret.use", Name: "Use Secret", Description: "Use stored secrets and credentials within the service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.provider.register", Name: "Register Service Provider", Description: "Register capability providers from the service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, BackgroundAllowed: false, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.tool.execute", Name: "Execute Service Tool", Description: "Execute registered tools through the service runtime", Category: CategoryService, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: true, BackgroundAllowed: true, ChildInvocation: ChildReevaluate, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "service.background.run", Name: "Run Background Task", Description: "Run long-lived background tasks within the service runtime", Category: CategoryService, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeExtension, ScopeModule, ScopeGlobal}, PersistentGrantable: false, BackgroundAllowed: true, ChildInvocation: ChildDeny, TrustedOnly: true, DefaultApproval: ApprovalManual},
		{ID: "gamehost.control", Name: "GameHost Control Output", Description: "Allow the plugin to participate in GameHost-managed control output flow", Category: CategoryGameHost, RiskLevel: capability.RiskHigh, AllowedScopes: []ScopeType{ScopeGlobal, ScopeExtension, ScopeModule, ScopeResource}, PersistentGrantable: false, RequiresPerUse: true, BackgroundAllowed: false, ChildInvocation: ChildDeny, DefaultApproval: ApprovalManual},
		{ID: "gamehost.channel.use", Name: "GameHost Channel Use", Description: "Allow the runtime to use declared GameHost runtime channels (IPC streams)", Category: CategoryGameHost, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeExtension, ScopeModule}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
		{ID: "gamehost.host_api.invoke", Name: "GameHost Host API Invoke", Description: "Allow entering the GameHost Host API gateway; still requires route-specific permissions", Category: CategoryGameHost, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeExtension, ScopeModule}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
		{ID: "media.image.read", Name: "Read Image", Description: "Read and analyze images, including OCR and visual understanding", Category: CategoryMedia, RiskLevel: capability.RiskLow, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation, ScopeResource}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalAuto},
		{ID: "media.image.generate", Name: "Generate Image", Description: "Generate images using AI providers", Category: CategoryMedia, RiskLevel: capability.RiskMedium, AllowedScopes: []ScopeType{ScopeGlobal, ScopeCharacter, ScopeConversation}, PersistentGrantable: true, BackgroundAllowed: false, ChildInvocation: ChildInherit, DefaultApproval: ApprovalManual},
	}
	for _, def := range builtins {
		r.Register(def)
	}
}
