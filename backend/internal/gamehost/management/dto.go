package management

import "time"

type GamePluginSummaryDTO struct {
	ExtensionID      string `json:"extensionId"`
	PluginID         string `json:"pluginId"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Description      string `json:"description"`
	Enabled          bool   `json:"enabled"`
	InstallState     string `json:"installState"`
	Health           string `json:"health"`
	RuntimeCount     int    `json:"runtimeCount"`
	ManagementTarget string `json:"managementTarget"`
}

type GamePluginDetailDTO struct {
	ExtensionID        string                  `json:"extensionId"`
	PluginID           string                  `json:"pluginId"`
	Name               string                  `json:"name"`
	Version            string                  `json:"version"`
	Description        string                  `json:"description"`
	Enabled            bool                    `json:"enabled"`
	InstallState       string                  `json:"installState"`
	PackageRevision    string                  `json:"packageRevision,omitempty"`
	DescriptorRevision string                  `json:"descriptorRevision,omitempty"`
	ManagementTarget   string                  `json:"managementTarget"`
	Capabilities       []string                `json:"capabilities,omitempty"`
	Permissions        []string                `json:"permissions,omitempty"`
	Provider           string                  `json:"provider,omitempty"`
	Runtimes           []GameRuntimeSummaryDTO `json:"runtimes,omitempty"`
	HealthSummary      *HealthSummaryDTO       `json:"healthSummary,omitempty"`
}

type GameRuntimeSummaryDTO struct {
	RuntimeID      string `json:"runtimeId"`
	PluginID       string `json:"pluginId"`
	ExtensionID    string `json:"extensionId"`
	State          string `json:"state"`
	Health         string `json:"health"`
	ServiceCount   int    `json:"serviceCount"`
	Connected      bool   `json:"connected"`
	Ready          bool   `json:"ready"`
	ControlMode    string `json:"controlMode"`
	AuthorityEpoch uint64 `json:"authorityEpoch"`
}

type GameRuntimeDetailDTO struct {
	RuntimeID        string                `json:"runtimeId"`
	PluginID         string                `json:"pluginId"`
	ExtensionID      string                `json:"extensionId"`
	RuntimeState     string                `json:"runtimeState"`
	DesiredState     string                `json:"desiredState,omitempty"`
	CreatedAt        *time.Time            `json:"createdAt,omitempty"`
	UpdatedAt        *time.Time            `json:"updatedAt,omitempty"`
	Process          *ProcessSummaryDTO    `json:"process,omitempty"`
	Connection       *ConnectionSummaryDTO `json:"connection,omitempty"`
	Handshake        *HandshakeSummaryDTO  `json:"handshake,omitempty"`
	Services         []GameServiceDTO      `json:"services,omitempty"`
	ControlAuthority *ControlAuthorityDTO  `json:"controlAuthority,omitempty"`
	HealthSummary    *HealthSummaryDTO     `json:"healthSummary,omitempty"`
}

type GameServiceDTO struct {
	ServiceID    string `json:"serviceId"`
	RuntimeID    string `json:"runtimeId"`
	DefinitionID string `json:"definitionId"`
	ModuleID     string `json:"moduleId"`
	State        string `json:"state"`
	Health       string `json:"health"`
	Connected    bool   `json:"connected"`
	Ready        bool   `json:"ready"`
}

type HealthSummaryDTO struct {
	Status    string     `json:"status"`
	Message   string     `json:"message,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

type ProcessSummaryDTO struct {
	Managed           bool   `json:"managed"`
	Running           bool   `json:"running"`
	ProcessGeneration uint64 `json:"processGeneration"`
	RestartCount      int    `json:"restartCount"`
}

type ConnectionSummaryDTO struct {
	Connected       bool       `json:"connected"`
	ProtocolVersion string     `json:"protocolVersion,omitempty"`
	PeerGeneration  uint64     `json:"peerGeneration,omitempty"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt,omitempty"`
}

type HandshakeSummaryDTO struct {
	HandshakeState string `json:"handshakeState"`
	Ready          bool   `json:"ready"`
	Protocol       string `json:"protocol,omitempty"`
	SDKName        string `json:"sdkName,omitempty"`
	SDKVersion     string `json:"sdkVersion,omitempty"`
}

type ControlAuthorityDTO struct {
	RuntimeID string     `json:"runtimeId"`
	Mode      string     `json:"mode"`
	Epoch     uint64     `json:"epoch"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

type GameCenterPluginList struct {
	Items    []GamePluginSummaryDTO `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

type GameCenterRuntimeList struct {
	Items []GameRuntimeSummaryDTO `json:"items"`
	Total int                     `json:"total"`
}

type GameCenterServiceList struct {
	Items []GameServiceDTO `json:"items"`
	Total int              `json:"total"`
}

type PluginFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	Status   string `form:"status"`
	Enabled  *bool  `form:"enabled"`
}

type RuntimeFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	PluginID string `form:"pluginId"`
	Status   string `form:"status"`
}

func (f PluginFilter) Normalize() PluginFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	return f
}

func (f RuntimeFilter) Normalize() RuntimeFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	return f
}

type AgentContextBindRequest struct {
	ServiceID      string `json:"serviceId"`
	UserID         string `json:"userId,omitempty"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Channel        string `json:"channel,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
}
