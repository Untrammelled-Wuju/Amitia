package agentplatform

import "time"

type AgentProfile struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID           string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	CharacterID       string     `gorm:"column:character_id;not null;index" json:"characterId"`
	Name              string     `gorm:"column:name;not null" json:"name"`
	Description       string     `gorm:"column:description;not null;default:''" json:"description"`
	Archetype         string     `gorm:"column:archetype;not null;default:'companion'" json:"archetype"`
	Status            string     `gorm:"column:status;not null;default:'draft'" json:"status"`
	OwnerID           string     `gorm:"column:owner_id;not null;default:'';index" json:"ownerId"`
	TeamID            string     `gorm:"column:team_id;not null;default:'';index" json:"teamId"`
	CurrentRevisionID string     `gorm:"column:current_revision_id;not null;default:''" json:"currentRevisionId"`
	CurrentReleaseID  string     `gorm:"column:current_release_id;not null;default:''" json:"currentReleaseId"`
	Revision          int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt         *time.Time `gorm:"column:deleted_at;index" json:"deletedAt,omitempty"`
}

type AgentRevision struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID      string    `gorm:"column:agent_id;not null;index" json:"agentId"`
	Revision     int64     `gorm:"column:revision;not null" json:"revision"`
	ConfigJSON   string    `gorm:"column:config_json;not null" json:"configJson"`
	ConfigHash   string    `gorm:"column:config_hash;not null" json:"configHash"`
	Status       string    `gorm:"column:status;not null;default:'draft'" json:"status"`
	ChangeNote   string    `gorm:"column:change_note;not null;default:''" json:"changeNote"`
	CreatedBy    string    `gorm:"column:created_by;not null;default:''" json:"createdBy"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	PublishedAt  time.Time `gorm:"column:published_at" json:"publishedAt,omitempty"`
	TestRunID    string    `gorm:"column:test_run_id;not null;default:''" json:"testRunId"`
	TestStatus   string    `gorm:"column:test_status;not null;default:'not_run'" json:"testStatus"`
	TestEvidence string    `gorm:"column:test_evidence;not null;default:''" json:"testEvidence"`
}

type AgentRelease struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID         string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID         string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	RevisionID      string     `gorm:"column:revision_id;not null;index" json:"revisionId"`
	Version         string     `gorm:"column:version;not null" json:"version"`
	ReleaseNotes    string     `gorm:"column:release_notes;not null;default:''" json:"releaseNotes"`
	Status          string     `gorm:"column:status;not null;default:'published'" json:"status"`
	EvaluationRunID string     `gorm:"column:evaluation_run_id;not null;default:''" json:"evaluationRunId"`
	CreatedBy       string     `gorm:"column:created_by;not null;default:''" json:"createdBy"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	SupersededAt    *time.Time `gorm:"column:superseded_at" json:"supersededAt,omitempty"`
}

type AgentTemplate struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	Name          string    `gorm:"column:name;not null" json:"name"`
	Category      string    `gorm:"column:category;not null;default:'general';index" json:"category"`
	Archetype     string    `gorm:"column:archetype;not null;default:'companion'" json:"archetype"`
	Description   string    `gorm:"column:description;not null;default:''" json:"description"`
	ConfigJSON    string    `gorm:"column:config_json;not null" json:"configJson"`
	PublisherID   string    `gorm:"column:publisher_id;not null;default:'com.amitia'" json:"publisherId"`
	TrustLevel    string    `gorm:"column:trust_level;not null;default:'official'" json:"trustLevel"`
	Builtin       bool      `gorm:"column:builtin;not null;default:false" json:"builtin"`
	SchemaVersion int       `gorm:"column:schema_version;not null;default:1" json:"schemaVersion"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type PlatformUser struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	DisplayName  string     `gorm:"column:display_name;not null" json:"displayName"`
	Email        string     `gorm:"column:email;not null;default:'';index" json:"email"`
	AuthHash     string     `gorm:"column:auth_hash;not null;default:''" json:"-"`
	Avatar       string     `gorm:"column:avatar;not null;default:''" json:"avatar"`
	Status       string     `gorm:"column:status;not null;default:'active'" json:"status"`
	IsLocalOwner bool       `gorm:"column:is_local_owner;not null;default:false" json:"isLocalOwner"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	RevokedAt    *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
}

type PlatformSession struct {
	ID        string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID   string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	UserID    string     `gorm:"column:user_id;not null;index" json:"userId"`
	TokenHash string     `gorm:"column:token_hash;not null;uniqueIndex" json:"-"`
	Status    string     `gorm:"column:status;not null;default:'active';index" json:"status"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null;index" json:"expiresAt"`
	CreatedAt time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
}

type PlatformMembership struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID     string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	UserID      string    `gorm:"column:user_id;not null;index" json:"userId"`
	TeamID      string    `gorm:"column:team_id;not null;default:'';index" json:"teamId"`
	Role        string    `gorm:"column:role;not null;default:'member'" json:"role"`
	Status      string    `gorm:"column:status;not null;default:'active'" json:"status"`
	Permissions string    `gorm:"column:permissions;not null;default:'[]'" json:"permissions"`
	Revision    int64     `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type PlatformInvitation struct {
	ID         string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID    string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	TeamID     string     `gorm:"column:team_id;not null;default:'';index" json:"teamId"`
	InviterID  string     `gorm:"column:inviter_id;not null" json:"inviterId"`
	Email      string     `gorm:"column:email;not null;index" json:"email"`
	Role       string     `gorm:"column:role;not null;default:'member'" json:"role"`
	TokenHash  string     `gorm:"column:token_hash;not null;uniqueIndex" json:"-"`
	Status     string     `gorm:"column:status;not null;default:'pending'" json:"status"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	AcceptedAt *time.Time `gorm:"column:accepted_at" json:"acceptedAt,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
}

type Team struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID          string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	Name             string     `gorm:"column:name;not null" json:"name"`
	Description      string     `gorm:"column:description;not null;default:''" json:"description"`
	OwnerID          string     `gorm:"column:owner_id;not null;default:'';index" json:"ownerId"`
	LeadAgentID      string     `gorm:"column:lead_agent_id;not null;default:''" json:"leadAgentId"`
	WorkspaceID      string     `gorm:"column:workspace_id;not null;default:''" json:"workspaceId"`
	WorkspaceRootURI string     `gorm:"column:workspace_root_uri;not null;default:''" json:"workspaceRootUri"`
	SharedMemory     bool       `gorm:"column:shared_memory;not null;default:false" json:"sharedMemory"`
	SharedWorkspace  bool       `gorm:"column:shared_workspace;not null;default:false" json:"sharedWorkspace"`
	Status           string     `gorm:"column:status;not null;default:'active'" json:"status"`
	Revision         int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt        *time.Time `gorm:"column:deleted_at;index" json:"deletedAt,omitempty"`
}

type TeamTemplate struct {
	ID                string    `gorm:"column:id;primaryKey" json:"id"`
	Name              string    `gorm:"column:name;not null" json:"name"`
	Description       string    `gorm:"column:description;not null;default:''" json:"description"`
	SharedMemory      bool      `gorm:"column:shared_memory;not null;default:true" json:"sharedMemory"`
	SharedWorkspace   bool      `gorm:"column:shared_workspace;not null;default:true" json:"sharedWorkspace"`
	RoutingPolicyJSON string    `gorm:"column:routing_policy_json;not null;default:'{}'" json:"routingPolicyJson"`
	Builtin           bool      `gorm:"column:builtin;not null;default:false" json:"builtin"`
	CreatedAt         time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type TeamMember struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID       string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	TeamID        string    `gorm:"column:team_id;not null;index" json:"teamId"`
	MemberType    string    `gorm:"column:member_type;not null" json:"memberType"`
	MemberID      string    `gorm:"column:member_id;not null;index" json:"memberId"`
	Role          string    `gorm:"column:role;not null;default:'member'" json:"role"`
	ResponderMode string    `gorm:"column:responder_mode;not null;default:'mention_or_lead'" json:"responderMode"`
	Capabilities  string    `gorm:"column:capabilities;not null;default:'[]'" json:"capabilities"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type TeamContext struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID     string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	TeamID      string    `gorm:"column:team_id;not null;index" json:"teamId"`
	Kind        string    `gorm:"column:kind;not null;index" json:"kind"`
	Key         string    `gorm:"column:key;not null;default:''" json:"key"`
	Content     string    `gorm:"column:content;not null" json:"content"`
	ContentHash string    `gorm:"column:content_hash;not null" json:"contentHash"`
	Visibility  string    `gorm:"column:visibility;not null;default:'team'" json:"visibility"`
	Revision    int64     `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedBy   string    `gorm:"column:created_by;not null;default:''" json:"createdBy"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type TeamHandoff struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID     string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	TeamID      string     `gorm:"column:team_id;not null;index" json:"teamId"`
	TaskID      string     `gorm:"column:task_id;not null;index" json:"taskId"`
	FromAgentID string     `gorm:"column:from_agent_id;not null;index" json:"fromAgentId"`
	ToAgentID   string     `gorm:"column:to_agent_id;not null;index" json:"toAgentId"`
	Status      string     `gorm:"column:status;not null;default:'pending'" json:"status"`
	Summary     string     `gorm:"column:summary;not null;default:''" json:"summary"`
	ContextJSON string     `gorm:"column:context_json;not null;default:'{}'" json:"contextJson"`
	EvidenceRef string     `gorm:"column:evidence_ref;not null;default:''" json:"evidenceRef"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	AcceptedAt  *time.Time `gorm:"column:accepted_at" json:"acceptedAt,omitempty"`
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

type ApprovalRecord struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID         string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID         string     `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	TeamID          string     `gorm:"column:team_id;not null;default:'';index" json:"teamId"`
	ConversationID  string     `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	TurnID          string     `gorm:"column:turn_id;not null;default:''" json:"turnId"`
	ToolCallID      string     `gorm:"column:tool_call_id;not null;default:'';index" json:"toolCallId"`
	ToolName        string     `gorm:"column:tool_name;not null" json:"toolName"`
	ActionSummary   string     `gorm:"column:action_summary;not null;default:''" json:"actionSummary"`
	ResourceSummary string     `gorm:"column:resource_summary;not null;default:''" json:"resourceSummary"`
	ArgumentsJSON   string     `gorm:"column:arguments_json;not null;default:'{}'" json:"argumentsJson"`
	EvidenceJSON    string     `gorm:"column:evidence_json;not null;default:'{}'" json:"evidenceJson"`
	ScopeJSON       string     `gorm:"column:scope_json;not null;default:'{}'" json:"scopeJson"`
	RiskLevel       string     `gorm:"column:risk_level;not null;default:'medium'" json:"riskLevel"`
	Reversible      bool       `gorm:"column:reversible;not null;default:false" json:"reversible"`
	Status          string     `gorm:"column:status;not null;default:'pending';index" json:"status"`
	AllowOnce       bool       `gorm:"column:allow_once;not null;default:true" json:"allowOnce"`
	SessionGrant    bool       `gorm:"column:session_grant;not null;default:false" json:"sessionGrant"`
	TimedGrantUntil *time.Time `gorm:"column:timed_grant_until" json:"timedGrantUntil,omitempty"`
	RequestHash     string     `gorm:"column:request_hash;not null" json:"requestHash"`
	RequestedBy     string     `gorm:"column:requested_by;not null;default:''" json:"requestedBy"`
	DecidedBy       string     `gorm:"column:decided_by;not null;default:''" json:"decidedBy"`
	DecisionReason  string     `gorm:"column:decision_reason;not null;default:''" json:"decisionReason"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null;index" json:"expiresAt"`
	DecidedAt       *time.Time `gorm:"column:decided_at" json:"decidedAt,omitempty"`
	ResumedAt       *time.Time `gorm:"column:resumed_at" json:"resumedAt,omitempty"`
	Revision        int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type ApprovalDecision struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	ApprovalID string    `gorm:"column:approval_id;not null;index" json:"approvalId"`
	Decision   string    `gorm:"column:decision;not null" json:"decision"`
	GrantMode  string    `gorm:"column:grant_mode;not null;default:'once'" json:"grantMode"`
	Reason     string    `gorm:"column:reason;not null;default:''" json:"reason"`
	ActorID    string    `gorm:"column:actor_id;not null;default:''" json:"actorId"`
	CreatedAt  time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type ReconciliationCase struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID       string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID       string     `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	ApprovalID    string     `gorm:"column:approval_id;not null;default:'';index" json:"approvalId"`
	ToolCallID    string     `gorm:"column:tool_call_id;not null;default:'';index" json:"toolCallId"`
	Status        string     `gorm:"column:status;not null;default:'open';index" json:"status"`
	Reason        string     `gorm:"column:reason;not null" json:"reason"`
	OriginalError string     `gorm:"column:original_error;not null;default:''" json:"originalError"`
	ResultHash    string     `gorm:"column:result_hash;not null;default:''" json:"resultHash"`
	ActionJSON    string     `gorm:"column:action_json;not null;default:'[]'" json:"actionJson"`
	AssignedTo    string     `gorm:"column:assigned_to;not null;default:''" json:"assignedTo"`
	ResolvedBy    string     `gorm:"column:resolved_by;not null;default:''" json:"resolvedBy"`
	Resolution    string     `gorm:"column:resolution;not null;default:''" json:"resolution"`
	Revision      int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	ResolvedAt    *time.Time `gorm:"column:resolved_at" json:"resolvedAt,omitempty"`
}

type CredentialGrant struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID      string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	ConnectionID string     `gorm:"column:connection_id;not null;index" json:"connectionId"`
	Provider     string     `gorm:"column:provider;not null" json:"provider"`
	AccountID    string     `gorm:"column:account_id;not null;default:''" json:"accountId"`
	ScopeJSON    string     `gorm:"column:scope_json;not null;default:'[]'" json:"scopeJson"`
	SecretRef    string     `gorm:"column:secret_ref;not null;default:''" json:"secretRef"`
	Status       string     `gorm:"column:status;not null;default:'active';index" json:"status"`
	ExpiresAt    *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	RevokedAt    *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
}

type CapabilityLease struct {
	ID         string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID    string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID    string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	GrantID    string     `gorm:"column:grant_id;not null;index" json:"grantId"`
	TokenHash  string     `gorm:"column:token_hash;not null;uniqueIndex" json:"-"`
	Capability string     `gorm:"column:capability;not null;index" json:"capability"`
	Resource   string     `gorm:"column:resource;not null;default:''" json:"resource"`
	MaxUses    int        `gorm:"column:max_uses;not null;default:1" json:"maxUses"`
	UsedCount  int        `gorm:"column:used_count;not null;default:0" json:"usedCount"`
	Status     string     `gorm:"column:status;not null;default:'active';index" json:"status"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	RevokedAt  *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
}

type NetworkPolicy struct {
	ID                  string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID             string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID             string    `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	Name                string    `gorm:"column:name;not null" json:"name"`
	DefaultAction       string    `gorm:"column:default_action;not null;default:'deny'" json:"defaultAction"`
	AllowDomains        string    `gorm:"column:allow_domains;not null;default:'[]'" json:"allowDomains"`
	DenyDomains         string    `gorm:"column:deny_domains;not null;default:'[]'" json:"denyDomains"`
	AllowedPorts        string    `gorm:"column:allowed_ports;not null;default:'[443]'" json:"allowedPorts"`
	UploadPolicy        string    `gorm:"column:upload_policy;not null;default:'approval'" json:"uploadPolicy"`
	SensitiveDataPolicy string    `gorm:"column:sensitive_data_policy;not null;default:'block'" json:"sensitiveDataPolicy"`
	Revision            int64     `gorm:"column:revision;not null;default:1" json:"revision"`
	Enabled             bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt           time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type EgressDecision struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID          string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID          string    `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	PolicyID         string    `gorm:"column:policy_id;not null;default:''" json:"policyId"`
	Host             string    `gorm:"column:host;not null;index" json:"host"`
	Port             int       `gorm:"column:port;not null" json:"port"`
	Method           string    `gorm:"column:method;not null;default:''" json:"method"`
	PayloadHash      string    `gorm:"column:payload_hash;not null;default:''" json:"payloadHash"`
	Allowed          bool      `gorm:"column:allowed;not null;default:false" json:"allowed"`
	RequiresApproval bool      `gorm:"column:requires_approval;not null;default:false" json:"requiresApproval"`
	Reason           string    `gorm:"column:reason;not null;default:''" json:"reason"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type ResourceLease struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID       string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	ResourceType  string     `gorm:"column:resource_type;not null;index" json:"resourceType"`
	ResourceID    string     `gorm:"column:resource_id;not null;index" json:"resourceId"`
	HolderAgentID string     `gorm:"column:holder_agent_id;not null;index" json:"holderAgentId"`
	TaskID        string     `gorm:"column:task_id;not null;default:''" json:"taskId"`
	Mode          string     `gorm:"column:mode;not null;default:'exclusive'" json:"mode"`
	Fingerprint   string     `gorm:"column:fingerprint;not null;default:''" json:"fingerprint"`
	Status        string     `gorm:"column:status;not null;default:'active';index" json:"status"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	ReleasedAt    *time.Time `gorm:"column:released_at" json:"releasedAt,omitempty"`
}

type IdempotentOperation struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID        string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	IdempotencyKey string     `gorm:"column:idempotency_key;not null;uniqueIndex:idx_idempotent_scope" json:"idempotencyKey"`
	ScopeKey       string     `gorm:"column:scope_key;not null;uniqueIndex:idx_idempotent_scope" json:"scopeKey"`
	Fingerprint    string     `gorm:"column:fingerprint;not null" json:"fingerprint"`
	Status         string     `gorm:"column:status;not null;default:'running';index" json:"status"`
	ResultJSON     string     `gorm:"column:result_json;not null;default:''" json:"resultJson"`
	OriginalError  string     `gorm:"column:original_error;not null;default:''" json:"originalError"`
	Revision       int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

type BudgetPolicy struct {
	ID                string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID           string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	ScopeType         string    `gorm:"column:scope_type;not null;index" json:"scopeType"`
	ScopeID           string    `gorm:"column:scope_id;not null;index" json:"scopeId"`
	Period            string    `gorm:"column:period;not null;default:'monthly'" json:"period"`
	TokenLimit        int64     `gorm:"column:token_limit;not null;default:0" json:"tokenLimit"`
	CostLimitMicroUSD int64     `gorm:"column:cost_limit_micro_usd;not null;default:0" json:"costLimitMicroUsd"`
	ToolCallLimit     int64     `gorm:"column:tool_call_limit;not null;default:0" json:"toolCallLimit"`
	ConcurrencyLimit  int       `gorm:"column:concurrency_limit;not null;default:1" json:"concurrencyLimit"`
	WarningPercent    int       `gorm:"column:warning_percent;not null;default:80" json:"warningPercent"`
	HardStop          bool      `gorm:"column:hard_stop;not null;default:true" json:"hardStop"`
	RevisedAt         time.Time `gorm:"column:revised_at;not null" json:"revisedAt"`
}

type BudgetUsage struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	PolicyID     string    `gorm:"column:policy_id;not null;index" json:"policyId"`
	RequestID    string    `gorm:"column:request_id;not null;index" json:"requestId"`
	InputTokens  int64     `gorm:"column:input_tokens;not null;default:0" json:"inputTokens"`
	OutputTokens int64     `gorm:"column:output_tokens;not null;default:0" json:"outputTokens"`
	CostMicroUSD int64     `gorm:"column:cost_micro_usd;not null;default:0" json:"costMicroUsd"`
	ToolCalls    int64     `gorm:"column:tool_calls;not null;default:0" json:"toolCalls"`
	Provider     string    `gorm:"column:provider;not null;default:''" json:"provider"`
	Model        string    `gorm:"column:model;not null;default:''" json:"model"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type BudgetLease struct {
	ID         string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID    string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	PolicyID   string     `gorm:"column:policy_id;not null;index" json:"policyId"`
	ScopeType  string     `gorm:"column:scope_type;not null" json:"scopeType"`
	ScopeID    string     `gorm:"column:scope_id;not null" json:"scopeId"`
	HolderID   string     `gorm:"column:holder_id;not null;index" json:"holderId"`
	RequestID  string     `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	Status     string     `gorm:"column:status;not null;default:'active';index" json:"status"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	ReleasedAt *time.Time `gorm:"column:released_at" json:"releasedAt,omitempty"`
}

type RuntimeInstance struct {
	ID                 string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID            string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID            string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	Provider           string     `gorm:"column:provider;not null" json:"provider"`
	ProviderRef        string     `gorm:"column:provider_ref;not null;default:''" json:"providerRef"`
	Image              string     `gorm:"column:image;not null;default:''" json:"image"`
	State              string     `gorm:"column:state;not null;default:'created';index" json:"state"`
	RootPath           string     `gorm:"column:root_path;not null;default:''" json:"rootPath"`
	WorkspaceID        string     `gorm:"column:workspace_id;not null;default:'';index" json:"workspaceId"`
	WorkspaceRootURI   string     `gorm:"column:workspace_root_uri;not null;default:''" json:"workspaceRootUri"`
	Endpoint           string     `gorm:"column:endpoint;not null;default:''" json:"endpoint"`
	NetworkPolicyID    string     `gorm:"column:network_policy_id;not null;default:''" json:"networkPolicyId"`
	CPUQuota           float64    `gorm:"column:cpu_quota;not null;default:1" json:"cpuQuota"`
	MemoryBytes        int64      `gorm:"column:memory_bytes;not null;default:1073741824" json:"memoryBytes"`
	DiskBytes          int64      `gorm:"column:disk_bytes;not null;default:10737418240" json:"diskBytes"`
	ProcessLimit       int        `gorm:"column:process_limit;not null;default:64" json:"processLimit"`
	IdleTimeoutSeconds int        `gorm:"column:idle_timeout_seconds;not null;default:1800" json:"idleTimeoutSeconds"`
	SnapshotID         string     `gorm:"column:snapshot_id;not null;default:''" json:"snapshotId"`
	LastError          string     `gorm:"column:last_error;not null;default:''" json:"lastError"`
	Revision           int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	StartedAt          *time.Time `gorm:"column:started_at" json:"startedAt,omitempty"`
	StoppedAt          *time.Time `gorm:"column:stopped_at" json:"stoppedAt,omitempty"`
}

type RuntimeSnapshot struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID      string    `gorm:"column:agent_id;not null;index" json:"agentId"`
	RuntimeID    string    `gorm:"column:runtime_id;not null;index" json:"runtimeId"`
	Name         string    `gorm:"column:name;not null" json:"name"`
	Path         string    `gorm:"column:path;not null;default:''" json:"path"`
	ManifestJSON string    `gorm:"column:manifest_json;not null;default:'{}'" json:"manifestJson"`
	ContentHash  string    `gorm:"column:content_hash;not null;default:''" json:"contentHash"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type HostControlSession struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID        string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	Platform       string     `gorm:"column:platform;not null" json:"platform"`
	Capabilities   string     `gorm:"column:capabilities;not null;default:'[]'" json:"capabilities"`
	PermissionJSON string     `gorm:"column:permission_json;not null;default:'{}'" json:"permissionJson"`
	Status         string     `gorm:"column:status;not null;default:'pending';index" json:"status"`
	ApprovedBy     string     `gorm:"column:approved_by;not null;default:''" json:"approvedBy"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	EndedAt        *time.Time `gorm:"column:ended_at" json:"endedAt,omitempty"`
}

type EvaluationSuite struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID        string    `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	Name           string    `gorm:"column:name;not null" json:"name"`
	Description    string    `gorm:"column:description;not null;default:''" json:"description"`
	ConfigJSON     string    `gorm:"column:config_json;not null;default:'{}'" json:"configJson"`
	GatePolicyJSON string    `gorm:"column:gate_policy_json;not null;default:'{}'" json:"gatePolicyJson"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type EvaluationCase struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	SuiteID        string    `gorm:"column:suite_id;not null;index" json:"suiteId"`
	Name           string    `gorm:"column:name;not null" json:"name"`
	InputJSON      string    `gorm:"column:input_json;not null;default:'{}'" json:"inputJson"`
	ExpectedJSON   string    `gorm:"column:expected_json;not null;default:'{}'" json:"expectedJson"`
	Category       string    `gorm:"column:category;not null;default:'general'" json:"category"`
	SafetyCritical bool      `gorm:"column:safety_critical;not null;default:false" json:"safetyCritical"`
	Weight         float64   `gorm:"column:weight;not null;default:1" json:"weight"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type EvaluationRun struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID           string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID           string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	RevisionID        string     `gorm:"column:revision_id;not null;index" json:"revisionId"`
	SuiteID           string     `gorm:"column:suite_id;not null;index" json:"suiteId"`
	Status            string     `gorm:"column:status;not null;default:'running';index" json:"status"`
	TaskSuccessRate   float64    `gorm:"column:task_success_rate;not null;default:0" json:"taskSuccessRate"`
	ToolAccuracy      float64    `gorm:"column:tool_accuracy;not null;default:0" json:"toolAccuracy"`
	SafetyViolations  int64      `gorm:"column:safety_violations;not null;default:0" json:"safetyViolations"`
	FalseSuccessRate  float64    `gorm:"column:false_success_rate;not null;default:0" json:"falseSuccessRate"`
	RecoveryRate      float64    `gorm:"column:recovery_rate;not null;default:0" json:"recoveryRate"`
	MemoryLeakCount   int64      `gorm:"column:memory_leak_count;not null;default:0" json:"memoryLeakCount"`
	TotalTokens       int64      `gorm:"column:total_tokens;not null;default:0" json:"totalTokens"`
	TotalCostMicroUSD int64      `gorm:"column:total_cost_micro_usd;not null;default:0" json:"totalCostMicroUsd"`
	AverageLatencyMS  int64      `gorm:"column:average_latency_ms;not null;default:0" json:"averageLatencyMs"`
	GateStatus        string     `gorm:"column:gate_status;not null;default:'pending'" json:"gateStatus"`
	GateReason        string     `gorm:"column:gate_reason;not null;default:''" json:"gateReason"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	CompletedAt       *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

type EvaluationResult struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID       string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	RunID         string    `gorm:"column:run_id;not null;index" json:"runId"`
	CaseID        string    `gorm:"column:case_id;not null;index" json:"caseId"`
	Passed        bool      `gorm:"column:passed;not null;default:false" json:"passed"`
	Score         float64   `gorm:"column:score;not null;default:0" json:"score"`
	LatencyMS     int64     `gorm:"column:latency_ms;not null;default:0" json:"latencyMs"`
	Tokens        int64     `gorm:"column:tokens;not null;default:0" json:"tokens"`
	CostMicroUSD  int64     `gorm:"column:cost_micro_usd;not null;default:0" json:"costMicroUsd"`
	ToolTraceJSON string    `gorm:"column:tool_trace_json;not null;default:'[]'" json:"toolTraceJson"`
	FailureJSON   string    `gorm:"column:failure_json;not null;default:''" json:"failureJson"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type ActivityEvent struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	ActorID        string    `gorm:"column:actor_id;not null;default:'';index" json:"actorId"`
	ActorType      string    `gorm:"column:actor_type;not null;default:''" json:"actorType"`
	AgentID        string    `gorm:"column:agent_id;not null;default:'';index" json:"agentId"`
	TeamID         string    `gorm:"column:team_id;not null;default:'';index" json:"teamId"`
	RuntimeID      string    `gorm:"column:runtime_id;not null;default:'';index" json:"runtimeId"`
	ConversationID string    `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	Kind           string    `gorm:"column:kind;not null;index" json:"kind"`
	State          string    `gorm:"column:state;not null;index" json:"state"`
	Title          string    `gorm:"column:title;not null" json:"title"`
	Detail         string    `gorm:"column:detail;not null;default:''" json:"detail"`
	WaitingReason  string    `gorm:"column:waiting_reason;not null;default:''" json:"waitingReason"`
	CostMicroUSD   int64     `gorm:"column:cost_micro_usd;not null;default:0" json:"costMicroUsd"`
	DurationMS     int64     `gorm:"column:duration_ms;not null;default:0" json:"durationMs"`
	PayloadJSON    string    `gorm:"column:payload_json;not null;default:'{}'" json:"payloadJson"`
	OccurredAt     time.Time `gorm:"column:occurred_at;not null;index" json:"occurredAt"`
}

type AgentPackage struct {
	ID              string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID         string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID         string    `gorm:"column:agent_id;not null;index" json:"agentId"`
	Name            string    `gorm:"column:name;not null" json:"name"`
	Version         string    `gorm:"column:version;not null" json:"version"`
	ManifestJSON    string    `gorm:"column:manifest_json;not null" json:"manifestJson"`
	BundleJSON      string    `gorm:"column:bundle_json;not null" json:"bundleJson"`
	ContentHash     string    `gorm:"column:content_hash;not null" json:"contentHash"`
	ArchivePath     string    `gorm:"column:archive_path;not null;default:''" json:"archivePath,omitempty"`
	ArchiveHash     string    `gorm:"column:archive_hash;not null;default:''" json:"archiveHash,omitempty"`
	ArchiveBytes    int64     `gorm:"column:archive_bytes;not null;default:0" json:"archiveBytes,omitempty"`
	IncludesSecrets bool      `gorm:"column:includes_secrets;not null;default:false" json:"includesSecrets"`
	Encrypted       bool      `gorm:"column:encrypted;not null;default:false" json:"encrypted"`
	CreatedBy       string    `gorm:"column:created_by;not null;default:''" json:"createdBy"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type AgentAutonomyPolicy struct {
	ID               string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID          string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID          string    `gorm:"column:agent_id;not null;uniqueIndex" json:"agentId"`
	Level            int       `gorm:"column:level;not null;default:1" json:"level"`
	WatchOnly        bool      `gorm:"column:watch_only;not null;default:true" json:"watchOnly"`
	AutoLowRisk      bool      `gorm:"column:auto_low_risk;not null;default:false" json:"autoLowRisk"`
	ExternalApproval bool      `gorm:"column:external_approval;not null;default:true" json:"externalApproval"`
	MaxActionsPerDay int       `gorm:"column:max_actions_per_day;not null;default:20" json:"maxActionsPerDay"`
	MaxCostMicroUSD  int64     `gorm:"column:max_cost_micro_usd;not null;default:0" json:"maxCostMicroUsd"`
	QuietHours       string    `gorm:"column:quiet_hours;not null;default:''" json:"quietHours"`
	EscalationTarget string    `gorm:"column:escalation_target;not null;default:''" json:"escalationTarget"`
	TriggersJSON     string    `gorm:"column:triggers_json;not null;default:'[]'" json:"triggersJson"`
	KillSwitch       bool      `gorm:"column:kill_switch;not null;default:false" json:"killSwitch"`
	Revision         int64     `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type MarketplacePublisher struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	DisplayName string    `gorm:"column:display_name;not null" json:"displayName"`
	TrustLevel  string    `gorm:"column:trust_level;not null;default:'community';index" json:"trustLevel"`
	Homepage    string    `gorm:"column:homepage;not null;default:''" json:"homepage"`
	Status      string    `gorm:"column:status;not null;default:'active';index" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type MarketplaceCategory struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	Name        string    `gorm:"column:name;not null" json:"name"`
	Description string    `gorm:"column:description;not null;default:''" json:"description"`
	Icon        string    `gorm:"column:icon;not null;default:''" json:"icon"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0" json:"sortOrder"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

type MarketplaceListing struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	ExtensionID      string     `gorm:"column:extension_id;not null;uniqueIndex" json:"extensionId"`
	PublisherID      string     `gorm:"column:publisher_id;not null;index" json:"publisherId"`
	CategoryID       string     `gorm:"column:category_id;not null;index" json:"categoryId"`
	TagsJSON         string     `gorm:"column:tags_json;not null;default:'[]'" json:"tagsJson"`
	DisplayName      string     `gorm:"column:display_name;not null" json:"displayName"`
	Summary          string     `gorm:"column:summary;not null;default:''" json:"summary"`
	Description      string     `gorm:"column:description;not null;default:''" json:"description"`
	Icon             string     `gorm:"column:icon;not null;default:''" json:"icon"`
	LatestVersion    string     `gorm:"column:latest_version;not null;default:''" json:"latestVersion"`
	TrustLevel       string     `gorm:"column:trust_level;not null;default:'community';index" json:"trustLevel"`
	SecurityScore    int        `gorm:"column:security_score;not null;default:0" json:"securityScore"`
	InstallCount     int64      `gorm:"column:install_count;not null;default:0" json:"installCount"`
	RatingAverage    float64    `gorm:"column:rating_average;not null;default:0" json:"ratingAverage"`
	RatingCount      int64      `gorm:"column:rating_count;not null;default:0" json:"ratingCount"`
	PlatformsJSON    string     `gorm:"column:platforms_json;not null;default:'[]'" json:"platformsJson"`
	CapabilitiesJSON string     `gorm:"column:capabilities_json;not null;default:'[]'" json:"capabilitiesJson"`
	PermissionsJSON  string     `gorm:"column:permissions_json;not null;default:'[]'" json:"permissionsJson"`
	SBOMURL          string     `gorm:"column:sbom_url;not null;default:''" json:"sbomUrl"`
	Status           string     `gorm:"column:status;not null;default:'draft';index" json:"status"`
	PublishedAt      *time.Time `gorm:"column:published_at" json:"publishedAt,omitempty"`
	WithdrawnAt      *time.Time `gorm:"column:withdrawn_at" json:"withdrawnAt,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type MarketplaceVersion struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	ListingID         string     `gorm:"column:listing_id;not null;index;uniqueIndex:idx_market_listing_version" json:"listingId"`
	Version           string     `gorm:"column:version;not null;index;uniqueIndex:idx_market_listing_version" json:"version"`
	OriginalName      string     `gorm:"column:original_name;not null;default:''" json:"originalName"`
	PackageURL        string     `gorm:"column:package_url;not null;default:''" json:"packageUrl"`
	PackageHash       string     `gorm:"column:package_hash;not null;default:''" json:"packageHash"`
	Signature         string     `gorm:"column:signature;not null;default:''" json:"signature"`
	SBOMJSON          string     `gorm:"column:sbom_json;not null;default:'{}'" json:"sbomJson"`
	CompatibilityJSON string     `gorm:"column:compatibility_json;not null;default:'{}'" json:"compatibilityJson"`
	Status            string     `gorm:"column:status;not null;default:'published';index" json:"status"`
	WithdrawnAt       *time.Time `gorm:"column:withdrawn_at" json:"withdrawnAt,omitempty"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type MarketplaceInstallation struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID     string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	ListingID   string    `gorm:"column:listing_id;not null;index" json:"listingId"`
	Version     string    `gorm:"column:version;not null" json:"version"`
	Status      string    `gorm:"column:status;not null;default:'requested'" json:"status"`
	OperationID string    `gorm:"column:operation_id;not null;default:''" json:"operationId"`
	RequestedBy string    `gorm:"column:requested_by;not null;default:''" json:"requestedBy"`
	CreatedAt   time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

type A2AAgentCard struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID      string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID      string    `gorm:"column:agent_id;not null;index" json:"agentId"`
	Name         string    `gorm:"column:name;not null" json:"name"`
	Description  string    `gorm:"column:description;not null;default:''" json:"description"`
	URL          string    `gorm:"column:url;not null" json:"url"`
	Version      string    `gorm:"column:version;not null;default:'1.0.0'" json:"version"`
	SkillsJSON   string    `gorm:"column:skills_json;not null;default:'[]'" json:"skillsJson"`
	SecurityJSON string    `gorm:"column:security_json;not null;default:'{}'" json:"securityJson"`
	Status       string    `gorm:"column:status;not null;default:'active';index" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (A2AAgentCard) TableName() string {
	return "a2a_agent_cards"
}

type A2ATask struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	AgentID        string     `gorm:"column:agent_id;not null;index" json:"agentId"`
	RemoteAgentURL string     `gorm:"column:remote_agent_url;not null;default:''" json:"remoteAgentUrl"`
	Status         string     `gorm:"column:status;not null;default:'submitted';index" json:"status"`
	InputJSON      string     `gorm:"column:input_json;not null;default:'{}'" json:"inputJson"`
	OutputJSON     string     `gorm:"column:output_json;not null;default:''" json:"outputJson"`
	OriginalError  string     `gorm:"column:original_error;not null;default:''" json:"originalError"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

func (A2ATask) TableName() string {
	return "a2a_tasks"
}

type A2AReplayRecord struct {
	ID            string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID       string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	SenderAgentID string    `gorm:"column:sender_agent_id;not null;index" json:"senderAgentId"`
	Nonce         string    `gorm:"column:nonce;not null;uniqueIndex" json:"nonce"`
	SignatureHash string    `gorm:"column:signature_hash;not null" json:"signatureHash"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null;index" json:"expiresAt"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

func platformModels() []any {
	return []any{
		&AgentProfile{}, &AgentRevision{}, &AgentRelease{}, &AgentTemplate{},
		&PlatformUser{}, &PlatformSession{}, &PlatformMembership{}, &PlatformInvitation{},
		&Team{}, &TeamTemplate{}, &TeamMember{}, &TeamContext{}, &TeamHandoff{},
		&ApprovalRecord{}, &ApprovalDecision{}, &ReconciliationCase{},
		&CredentialGrant{}, &CapabilityLease{}, &NetworkPolicy{}, &EgressDecision{},
		&ResourceLease{}, &IdempotentOperation{}, &BudgetPolicy{}, &BudgetUsage{}, &BudgetLease{},
		&RuntimeInstance{}, &RuntimeSnapshot{}, &HostControlSession{},
		&EvaluationSuite{}, &EvaluationCase{}, &EvaluationRun{}, &EvaluationResult{},
		&ActivityEvent{}, &AgentPackage{},
		&AgentAutonomyPolicy{},
		&MarketplacePublisher{}, &MarketplaceCategory{}, &MarketplaceListing{},
		&MarketplaceVersion{}, &MarketplaceInstallation{},
		&A2AAgentCard{}, &A2ATask{}, &A2AReplayRecord{},
	}
}
