// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package mcp

type Server struct {
	ID                      string `gorm:"column:id;primaryKey" json:"id"`
	Name                    string `gorm:"column:name;not null" json:"name"`
	DisplayName             string `gorm:"column:display_name;not null" json:"displayName"`
	Description             string `gorm:"column:description;not null" json:"description"`
	Transport               string `gorm:"column:transport;not null" json:"transport"`
	Endpoint                string `gorm:"column:endpoint;not null" json:"endpoint"`
	Command                 string `gorm:"column:command;not null" json:"command"`
	ArgsJSON                string `gorm:"column:args_json;not null" json:"args"`
	WorkDir                 string `gorm:"column:work_dir;not null" json:"workDir"`
	ProtocolVersion         string `gorm:"column:protocol_version;not null" json:"protocolVersion"`
	ServerInfoJSON          string `gorm:"column:server_info_json;not null" json:"serverInfo"`
	CapabilitiesJSON        string `gorm:"column:capabilities_json;not null" json:"capabilities"`
	Instructions            string `gorm:"column:instructions;not null" json:"instructions"`
	AuthType                string `gorm:"column:auth_type;not null" json:"authType"`
	Enabled                 int    `gorm:"column:enabled;not null" json:"enabled"`
	Status                  string `gorm:"column:status;not null" json:"status"`
	Source                  string `gorm:"column:source;not null" json:"source"`
	NormalizedIdentity      string `gorm:"column:normalized_identity;not null;uniqueIndex" json:"normalizedIdentity"`
	ConfigurationHash       string `gorm:"column:configuration_hash;not null" json:"configurationHash"`
	LastConnectedAt         string `gorm:"column:last_connected_at;not null" json:"lastConnectedAt"`
	LastErrorCode           string `gorm:"column:last_error_code;not null" json:"lastErrorCode"`
	LastErrorMessage        string `gorm:"column:last_error_message;not null" json:"lastErrorMessage"`
	CreatedAt               string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt               string `gorm:"column:updated_at;not null" json:"updatedAt"`
	PrivateNetworkConfirmed bool   `gorm:"-" json:"privateNetworkConfirmed"`
}

func (Server) TableName() string { return "mcp_servers" }

type ServerScopeBinding struct {
	ID        string `gorm:"column:id;primaryKey" json:"id"`
	ServerID  string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_binding" json:"serverId"`
	ScopeType string `gorm:"column:scope_type;not null;uniqueIndex:idx_mcp_binding" json:"scopeType"`
	ScopeID   string `gorm:"column:scope_id;not null;uniqueIndex:idx_mcp_binding" json:"scopeId"`
	Enabled   int    `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ServerScopeBinding) TableName() string { return "mcp_server_scope_bindings" }

type ServerCredential struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	ServerID        string `gorm:"column:server_id;not null;index" json:"serverId"`
	CredentialType  string `gorm:"column:credential_type;not null" json:"credentialType"`
	SecretReference string `gorm:"column:secret_reference;not null" json:"secretReference"`
	ExpiresAt       string `gorm:"column:expires_at;not null" json:"expiresAt"`
	ScopesJSON      string `gorm:"column:scopes_json;not null" json:"scopes"`
	CreatedAt       string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ServerCredential) TableName() string { return "mcp_server_credentials" }

type ServerCapability struct {
	ID            string `gorm:"column:id;primaryKey" json:"id"`
	ServerID      string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_capability" json:"serverId"`
	Capability    string `gorm:"column:capability;not null;uniqueIndex:idx_mcp_capability" json:"capability"`
	Configuration string `gorm:"column:configuration_json;not null" json:"configuration"`
	Enabled       int    `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt     string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ServerCapability) TableName() string { return "mcp_server_capabilities" }

type ToolDefinition struct {
	ID                  string `gorm:"column:id;primaryKey" json:"id"`
	ServerID            string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_tool_remote" json:"serverId"`
	RemoteName          string `gorm:"column:remote_name;not null;uniqueIndex:idx_mcp_tool_remote" json:"remoteName"`
	SkillID             string `gorm:"column:skill_id;not null;uniqueIndex" json:"skillId"`
	Title               string `gorm:"column:title;not null" json:"title"`
	Description         string `gorm:"column:description;not null" json:"description"`
	InputSchemaJSON     string `gorm:"column:input_schema_json;not null" json:"inputSchema"`
	OutputSchemaJSON    string `gorm:"column:output_schema_json;not null" json:"outputSchema"`
	AnnotationsJSON     string `gorm:"column:annotations_json;not null" json:"annotations"`
	ExecutionJSON       string `gorm:"column:execution_json;not null" json:"execution"`
	CapabilityHintsJSON string `gorm:"column:capability_hints_json;not null" json:"capabilityHints"`
	RiskLevel           string `gorm:"column:risk_level;not null" json:"riskLevel"`
	Enabled             int    `gorm:"column:enabled;not null" json:"enabled"`
	Hash                string `gorm:"column:hash;not null" json:"hash"`
	CreatedAt           string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt           string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ToolDefinition) TableName() string { return "mcp_tools" }

type ResourceDefinition struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	ServerID    string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_resource_uri" json:"serverId"`
	URI         string `gorm:"column:uri;not null;uniqueIndex:idx_mcp_resource_uri" json:"uri"`
	Name        string `gorm:"column:name;not null" json:"name"`
	Title       string `gorm:"column:title;not null" json:"title"`
	Description string `gorm:"column:description;not null" json:"description"`
	MIMEType    string `gorm:"column:mime_type;not null" json:"mimeType"`
	Enabled     int    `gorm:"column:enabled;not null" json:"enabled"`
	Hash        string `gorm:"column:hash;not null" json:"hash"`
	CreatedAt   string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ResourceDefinition) TableName() string { return "mcp_resources" }

type ResourceTemplate struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	ServerID    string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_template_uri" json:"serverId"`
	URITemplate string `gorm:"column:uri_template;not null;uniqueIndex:idx_mcp_template_uri" json:"uriTemplate"`
	Name        string `gorm:"column:name;not null" json:"name"`
	Title       string `gorm:"column:title;not null" json:"title"`
	Description string `gorm:"column:description;not null" json:"description"`
	MIMEType    string `gorm:"column:mime_type;not null" json:"mimeType"`
	Enabled     int    `gorm:"column:enabled;not null" json:"enabled"`
	Hash        string `gorm:"column:hash;not null" json:"hash"`
	CreatedAt   string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ResourceTemplate) TableName() string { return "mcp_resource_templates" }

type PromptDefinition struct {
	ID            string `gorm:"column:id;primaryKey" json:"id"`
	ServerID      string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_prompt_remote" json:"serverId"`
	RemoteName    string `gorm:"column:remote_name;not null;uniqueIndex:idx_mcp_prompt_remote" json:"remoteName"`
	Title         string `gorm:"column:title;not null" json:"title"`
	Description   string `gorm:"column:description;not null" json:"description"`
	ArgumentsJSON string `gorm:"column:arguments_json;not null" json:"arguments"`
	Enabled       int    `gorm:"column:enabled;not null" json:"enabled"`
	Hash          string `gorm:"column:hash;not null" json:"hash"`
	CreatedAt     string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (PromptDefinition) TableName() string { return "mcp_prompts" }

type DependencyLink struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	AgentSkillExtensionID string `gorm:"column:agent_skill_extension_id;not null;uniqueIndex:idx_mcp_dependency" json:"agentSkillExtensionId"`
	ServerID              string `gorm:"column:server_id;not null;uniqueIndex:idx_mcp_dependency" json:"serverId"`
	DependencyName        string `gorm:"column:dependency_name;not null;uniqueIndex:idx_mcp_dependency" json:"dependencyName"`
	Required              int    `gorm:"column:required;not null" json:"required"`
	InstallStatus         string `gorm:"column:install_status;not null" json:"installStatus"`
	BindingStatus         string `gorm:"column:binding_status;not null" json:"bindingStatus"`
	CreatedAt             string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (DependencyLink) TableName() string { return "mcp_dependency_links" }

type Operation struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	Type         string `gorm:"column:type;not null" json:"type"`
	Status       string `gorm:"column:status;not null" json:"status"`
	ServerID     string `gorm:"column:server_id;not null" json:"serverId"`
	AgentSkillID string `gorm:"column:agent_skill_id;not null" json:"agentSkillId"`
	ScopeType    string `gorm:"column:scope_type;not null" json:"scopeType"`
	ScopeID      string `gorm:"column:scope_id;not null" json:"scopeId"`
	PlanJSON     string `gorm:"column:plan_json;not null" json:"plan"`
	ResultJSON   string `gorm:"column:result_json;not null" json:"result"`
	ErrorCode    string `gorm:"column:error_code;not null" json:"errorCode"`
	ErrorMessage string `gorm:"column:error_message;not null" json:"errorMessage"`
	CreatedAt    string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Operation) TableName() string { return "mcp_operations" }

type OAuthSession struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	ServerID              string `gorm:"column:server_id;not null;index" json:"serverId"`
	StateHash             string `gorm:"column:state_hash;not null;uniqueIndex" json:"stateHash"`
	CodeVerifierReference string `gorm:"column:code_verifier_reference;not null" json:"codeVerifierReference"`
	RedirectURI           string `gorm:"column:redirect_uri;not null" json:"redirectUri"`
	RequestedScopesJSON   string `gorm:"column:requested_scopes_json;not null" json:"requestedScopes"`
	Status                string `gorm:"column:status;not null" json:"status"`
	ExpiresAt             string `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt             string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (OAuthSession) TableName() string { return "mcp_oauth_sessions" }

type Task struct {
	ID            string `gorm:"column:id;primaryKey" json:"id"`
	ServerID      string `gorm:"column:server_id;not null;index;uniqueIndex:idx_mcp_task_remote" json:"serverId"`
	RemoteTaskID  string `gorm:"column:remote_task_id;not null;uniqueIndex:idx_mcp_task_remote" json:"remoteTaskId"`
	CharacterID   string `gorm:"column:character_id;not null" json:"characterId"`
	RunID         string `gorm:"column:run_id;not null" json:"runId"`
	Status        string `gorm:"column:status;not null" json:"status"`
	StatusMessage string `gorm:"column:status_message;not null" json:"statusMessage"`
	ResultJSON    string `gorm:"column:result_json;not null" json:"result"`
	ExpiresAt     string `gorm:"column:expires_at;not null" json:"expiresAt"`
	LastUpdatedAt string `gorm:"column:last_updated_at;not null" json:"lastUpdatedAt"`
	CreatedAt     string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Task) TableName() string { return "mcp_tasks" }

type AuditLog struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	ServerID       string `gorm:"column:server_id;not null;index" json:"serverId"`
	Operation      string `gorm:"column:operation;not null" json:"operation"`
	ToolName       string `gorm:"column:tool_name;not null" json:"toolName"`
	CharacterID    string `gorm:"column:character_id;not null" json:"characterId"`
	ConversationID string `gorm:"column:conversation_id;not null" json:"conversationId"`
	Channel        string `gorm:"column:channel;not null" json:"channel"`
	TraceID        string `gorm:"column:trace_id;not null" json:"traceId"`
	OperationID    string `gorm:"column:operation_id;not null" json:"operationId"`
	Status         string `gorm:"column:status;not null" json:"status"`
	DurationMS     int64  `gorm:"column:duration_ms;not null" json:"durationMs"`
	ErrorCode      string `gorm:"column:error_code;not null" json:"errorCode"`
	SummaryJSON    string `gorm:"column:summary_json;not null" json:"summary"`
	CreatedAt      string `gorm:"column:created_at;not null" json:"createdAt"`
}

func (AuditLog) TableName() string { return "mcp_audit_logs" }
