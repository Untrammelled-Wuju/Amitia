package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"
)

const GameProtocolVersion = "amitia-game/2"

const EventIDGameEvent = "game.event"

const (
	MethodGameSessionOpen     = "game.session.open"
	MethodGameSessionClose    = "game.session.close"
	MethodGameSessionSnapshot = "game.session.snapshot"
	MethodGameObservationGet  = "game.observation.get"
	MethodGameActionExecute   = "game.action.execute"
	MethodGameGoalSet         = "game.goal.set"
	MethodGameCapabilitiesGet = "game.capabilities.get"
)

type PluginSessionStatus string

const (
	PluginSessionCreated    PluginSessionStatus = "created"
	PluginSessionConnecting PluginSessionStatus = "connecting"
	PluginSessionReady      PluginSessionStatus = "ready"
	PluginSessionPaused     PluginSessionStatus = "paused"
	PluginSessionClosed     PluginSessionStatus = "closed"
	PluginSessionFailed     PluginSessionStatus = "failed"
)

type PluginSessionOpenRequest struct {
	Context  map[string]any  `json:"context,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type PluginSession struct {
	ID        string              `json:"id"`
	Status    PluginSessionStatus `json:"status"`
	StartedAt time.Time           `json:"startedAt,omitempty"`
	UpdatedAt time.Time           `json:"updatedAt,omitempty"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
	Payload   json.RawMessage     `json:"payload,omitempty"`
}

type PluginEvent struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId,omitempty"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type PluginOperation struct {
	ID             string          `json:"id"`
	SessionID      string          `json:"sessionId,omitempty"`
	Type           string          `json:"type"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	DeadlineMS     int64           `json:"deadlineMs,omitempty"`
}

type PluginOperationStatus string

const (
	PluginOperationSucceeded PluginOperationStatus = "succeeded"
	PluginOperationFailed    PluginOperationStatus = "failed"
	PluginOperationCancelled PluginOperationStatus = "cancelled"
	PluginOperationRejected  PluginOperationStatus = "rejected"
)

type PluginOperationResult struct {
	OperationID  string                `json:"operationId"`
	SessionID    string                `json:"sessionId,omitempty"`
	Status       PluginOperationStatus `json:"status"`
	Output       json.RawMessage       `json:"output,omitempty"`
	ErrorCode    string                `json:"errorCode,omitempty"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
	Retryable    bool                  `json:"retryable,omitempty"`
}

type PluginCapability struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

type PluginArtifact struct {
	ID                    string   `json:"id"`
	Type                  string   `json:"type"`
	Platforms             []string `json:"platforms,omitempty"`
	Architectures         []string `json:"architectures,omitempty"`
	CompatibilityVersions []string `json:"compatibilityVersions,omitempty"`
	Source                string   `json:"source"`
	Target                string   `json:"target,omitempty"`
	Required              bool     `json:"required,omitempty"`
	SHA256                string   `json:"sha256,omitempty"`
	GameVersions          []string `json:"gameVersions,omitempty"`
	InstallTarget         string   `json:"installTarget,omitempty"`
}

func (a PluginArtifact) EffectiveCompatibilityVersions() []string {
	if len(a.CompatibilityVersions) > 0 {
		return a.CompatibilityVersions
	}
	return a.GameVersions
}

func (a PluginArtifact) EffectiveTarget() string {
	if strings.TrimSpace(a.Target) != "" {
		return a.Target
	}
	return a.InstallTarget
}

type PluginNetworkPolicy struct {
	Mode           string   `json:"mode,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	AllowedPorts   []int    `json:"allowedPorts,omitempty"`
	RequireProxy   bool     `json:"requireProxy,omitempty"`
	AuditAll       bool     `json:"auditAll,omitempty"`
}

type GameCapability = PluginCapability
type GameCompanionArtifact = PluginArtifact
type GameNetworkPolicy = PluginNetworkPolicy

type GamePluginSpec struct {
	ProtocolVersion     string               `json:"protocolVersion"`
	GameProtocolVersion string               `json:"gameProtocolVersion,omitempty"`
	RuntimeModuleID     string               `json:"runtimeModuleId"`
	Capabilities        []PluginCapability   `json:"capabilities,omitempty"`
	Artifacts           []PluginArtifact     `json:"artifacts,omitempty"`
	Network             *PluginNetworkPolicy `json:"network,omitempty"`
	Metadata            map[string]any       `json:"metadata,omitempty"`

	GameID             string           `json:"gameId,omitempty"`
	GameFamily         string           `json:"gameFamily,omitempty"`
	Editions           []string         `json:"editions,omitempty"`
	SupportedVersions  []string         `json:"supportedVersions,omitempty"`
	ConnectionModes    []string         `json:"connectionModes,omitempty"`
	Services           []string         `json:"services,omitempty"`
	Actions            []GameCapability `json:"actions,omitempty"`
	Observations       []GameCapability `json:"observations,omitempty"`
	CompanionArtifacts []PluginArtifact `json:"companionArtifacts,omitempty"`
}

func ParseGamePluginSpec(spec map[string]any) (GamePluginSpec, error) {
	if spec == nil {
		return GamePluginSpec{}, fmt.Errorf("game plugin spec is required")
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return GamePluginSpec{}, fmt.Errorf("marshal game plugin spec: %w", err)
	}
	var parsed GamePluginSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return GamePluginSpec{}, fmt.Errorf("decode game plugin spec: %w", err)
	}
	parsed.ProtocolVersion = strings.TrimSpace(parsed.ProtocolVersion)
	parsed.GameProtocolVersion = strings.TrimSpace(parsed.GameProtocolVersion)
	parsed.RuntimeModuleID = strings.TrimSpace(parsed.RuntimeModuleID)
	parsed.GameID = strings.TrimSpace(parsed.GameID)
	if parsed.GameProtocolVersion == "" {
		parsed.GameProtocolVersion = GameProtocolVersion
	}
	return parsed, nil
}

func (s GamePluginSpec) EffectiveArtifacts() []PluginArtifact {
	if len(s.Artifacts) > 0 {
		return s.Artifacts
	}
	return s.CompanionArtifacts
}

func (s GamePluginSpec) Validate() error {
	if s.ProtocolVersion == "" {
		return fmt.Errorf("protocolVersion is required")
	}
	if s.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocolVersion %q", s.ProtocolVersion)
	}
	if s.GameProtocolVersion != "" && s.GameProtocolVersion != GameProtocolVersion {
		return fmt.Errorf("unsupported gameProtocolVersion %q", s.GameProtocolVersion)
	}
	if s.RuntimeModuleID == "" {
		return fmt.Errorf("runtimeModuleId is required")
	}
	if s.Network != nil {
		mode := strings.ToLower(strings.TrimSpace(s.Network.Mode))
		switch mode {
		case "none", "loopback", "unrestricted", "restricted":
		default:
			return fmt.Errorf("unsupported network mode %q", s.Network.Mode)
		}
		if mode != "restricted" && (len(s.Network.AllowedDomains) > 0 || len(s.Network.AllowedPorts) > 0) {
			return fmt.Errorf("allowedDomains/allowedPorts require network mode restricted")
		}
		if s.Network.RequireProxy && mode != "restricted" {
			return fmt.Errorf("requireProxy requires network mode restricted")
		}
		if mode == "restricted" && len(s.Network.AllowedDomains) == 0 && len(s.Network.AllowedPorts) == 0 && !s.Network.RequireProxy {
			return fmt.Errorf("restricted network mode requires an allowlist or requireProxy")
		}
		for _, port := range s.Network.AllowedPorts {
			if port < 1 || port > 65535 {
				return fmt.Errorf("network port %d is out of range", port)
			}
		}
	}
	artifacts := s.EffectiveArtifacts()
	seenArtifacts := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		id := strings.TrimSpace(artifact.ID)
		target := strings.TrimSpace(artifact.EffectiveTarget())
		if id == "" || strings.TrimSpace(artifact.Type) == "" || strings.TrimSpace(artifact.Source) == "" || target == "" {
			return fmt.Errorf("artifact id, type, source and target are required")
		}
		if _, exists := seenArtifacts[id]; exists {
			return fmt.Errorf("duplicate artifact id %q", id)
		}
		seenArtifacts[id] = struct{}{}
		if !safePackageRelativePath(artifact.Source) {
			return fmt.Errorf("artifact %q source must be a safe package-relative path", id)
		}
		if !safePackageRelativePath(target) {
			return fmt.Errorf("artifact %q target must be a safe target-relative path", id)
		}
	}
	return nil
}

func safePackageRelativePath(raw string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return false
	}
	if strings.Contains(normalized, ":") {
		return false
	}
	clean := path.Clean(normalized)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
