package health

import "time"

type MCPHealthState string

const (
	MCPHealthUnknown               MCPHealthState = "unknown"
	MCPHealthDisabled              MCPHealthState = "disabled"
	MCPHealthInstalling            MCPHealthState = "installing"
	MCPHealthAuthorizationRequired MCPHealthState = "authorization_required"
	MCPHealthStarting              MCPHealthState = "starting"
	MCPHealthReady                 MCPHealthState = "ready"
	MCPHealthDegraded              MCPHealthState = "degraded"
	MCPHealthUnreachable           MCPHealthState = "unreachable"
	MCPHealthIncompatible          MCPHealthState = "incompatible"
	MCPHealthFailed                MCPHealthState = "failed"
	MCPHealthStopped               MCPHealthState = "stopped"
)

type MCPServerInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Vendor       string `json:"vendor,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

type MCPHealthSnapshot struct {
	ServerID            string         `json:"server_id"`
	State               MCPHealthState `json:"state"`
	Installed           bool           `json:"installed"`
	Enabled             bool           `json:"enabled"`
	AuthorizationState  string         `json:"authorization_state"`
	Reachability        string         `json:"reachability"`
	ProtocolState       string         `json:"protocol_state"`
	CapabilityState     string         `json:"capability_state"`
	LastProbeAt         time.Time      `json:"last_probe_at"`
	LastSuccessAt       time.Time      `json:"last_success_at"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	LatencyMS           int64          `json:"latency_ms"`
	ErrorCode           string         `json:"error_code,omitempty"`
	ErrorMessage        string         `json:"error_message,omitempty"`
	RetryAt             *time.Time     `json:"retry_at,omitempty"`
	ProtocolVersion     string         `json:"protocol_version,omitempty"`
	ServerInfo          MCPServerInfo  `json:"server_info,omitempty"`
}

func (s MCPHealthSnapshot) IsReady() bool {
	return s.State == MCPHealthReady
}

func (s MCPHealthSnapshot) IsDegraded() bool {
	return s.State == MCPHealthDegraded
}

func (s MCPHealthSnapshot) IsUnavailable() bool {
	switch s.State {
	case MCPHealthUnreachable, MCPHealthIncompatible, MCPHealthFailed:
		return true
	}
	return false
}

type HealthReachability string

const (
	MCPReachUnknown     HealthReachability = "unknown"
	MCPReachReachable   HealthReachability = "reachable"
	MCPReachUnreachable HealthReachability = "unreachable"
)

type ProtocolEra string

const (
	MCPProtocolEraUnknown ProtocolEra = "unknown"
	MCPProtocolEraModern  ProtocolEra = "modern_2026"
	MCPProtocolEraLegacy  ProtocolEra = "legacy_2025"
)
