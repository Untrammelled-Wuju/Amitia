package protocol

import (
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

type GameSessionStatus string

const (
	GameSessionCreated    GameSessionStatus = "created"
	GameSessionConnecting GameSessionStatus = "connecting"
	GameSessionReady      GameSessionStatus = "ready"
	GameSessionPaused     GameSessionStatus = "paused"
	GameSessionClosed     GameSessionStatus = "closed"
	GameSessionFailed     GameSessionStatus = "failed"
)

type GameSessionOpenRequest struct {
	GameRoot              string          `json:"gameRoot,omitempty"`
	GameVersion           string          `json:"gameVersion,omitempty"`
	CharacterID           string          `json:"characterId,omitempty"`
	AutoInstallCompanions *bool           `json:"autoInstallCompanions,omitempty"`
	Payload               json.RawMessage `json:"payload,omitempty"`
}

func (r GameSessionOpenRequest) ShouldAutoInstallCompanions() bool {
	return r.AutoInstallCompanions == nil || *r.AutoInstallCompanions
}

type GameSession struct {
	ID             string            `json:"id"`
	GameID         string            `json:"gameId"`
	GameVersion    string            `json:"gameVersion,omitempty"`
	Edition        string            `json:"edition,omitempty"`
	Status         GameSessionStatus `json:"status"`
	CharacterID    string            `json:"characterId,omitempty"`
	WorldID        string            `json:"worldId,omitempty"`
	ConnectionMode string            `json:"connectionMode,omitempty"`
	StartedAt      time.Time         `json:"startedAt,omitempty"`
	UpdatedAt      time.Time         `json:"updatedAt,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
	Payload        json.RawMessage   `json:"payload,omitempty"`
}

type GameLocation struct {
	World       string             `json:"world,omitempty"`
	Region      string             `json:"region,omitempty"`
	Coordinates map[string]float64 `json:"coordinates,omitempty"`
	Payload     json.RawMessage    `json:"payload,omitempty"`
}

type GameEntity struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Name       string          `json:"name,omitempty"`
	Location   *GameLocation   `json:"location,omitempty"`
	Attributes map[string]any  `json:"attributes,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type GameInventoryItem struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Name       string          `json:"name,omitempty"`
	Quantity   float64         `json:"quantity,omitempty"`
	Slot       string          `json:"slot,omitempty"`
	Attributes map[string]any  `json:"attributes,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type GameInventory struct {
	Items    []GameInventoryItem `json:"items,omitempty"`
	Capacity *int                `json:"capacity,omitempty"`
	Payload  json.RawMessage     `json:"payload,omitempty"`
}

type GameObservation struct {
	SessionID  string          `json:"sessionId"`
	Sequence   uint64          `json:"sequence"`
	ObservedAt time.Time       `json:"observedAt"`
	Location   *GameLocation   `json:"location,omitempty"`
	Entities   []GameEntity    `json:"entities,omitempty"`
	Inventory  *GameInventory  `json:"inventory,omitempty"`
	State      map[string]any  `json:"state,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type GameEvent struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	EntityIDs  []string        `json:"entityIds,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type GameAction struct {
	ID             string          `json:"id"`
	SessionID      string          `json:"sessionId"`
	Type           string          `json:"type"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	DeadlineMS     int64           `json:"deadlineMs,omitempty"`
}

type GameActionStatus string

const (
	GameActionSucceeded GameActionStatus = "succeeded"
	GameActionFailed    GameActionStatus = "failed"
	GameActionCancelled GameActionStatus = "cancelled"
	GameActionRejected  GameActionStatus = "rejected"
)

type GameActionResult struct {
	ActionID     string           `json:"actionId"`
	SessionID    string           `json:"sessionId"`
	Status       GameActionStatus `json:"status"`
	Observation  *GameObservation `json:"observation,omitempty"`
	Output       json.RawMessage  `json:"output,omitempty"`
	ErrorCode    string           `json:"errorCode,omitempty"`
	ErrorMessage string           `json:"errorMessage,omitempty"`
	Retryable    bool             `json:"retryable,omitempty"`
}

type GameGoal struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"sessionId"`
	Description string          `json:"description"`
	Priority    int             `json:"priority,omitempty"`
	State       string          `json:"state,omitempty"`
	Constraints json.RawMessage `json:"constraints,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type GameCapability struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

type GameCompanionArtifact struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Platforms     []string `json:"platforms,omitempty"`
	GameVersions  []string `json:"gameVersions,omitempty"`
	Source        string   `json:"source"`
	InstallTarget string   `json:"installTarget,omitempty"`
	Required      bool     `json:"required,omitempty"`
	SHA256        string   `json:"sha256,omitempty"`
}

type GameNetworkPolicy struct {
	Mode           string   `json:"mode,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	AllowedPorts   []int    `json:"allowedPorts,omitempty"`
	RequireProxy   bool     `json:"requireProxy,omitempty"`
	AuditAll       bool     `json:"auditAll,omitempty"`
}

type GamePluginSpec struct {
	ProtocolVersion     string                  `json:"protocolVersion"`
	GameProtocolVersion string                  `json:"gameProtocolVersion,omitempty"`
	RuntimeModuleID     string                  `json:"runtimeModuleId"`
	GameID              string                  `json:"gameId,omitempty"`
	GameFamily          string                  `json:"gameFamily,omitempty"`
	Editions            []string                `json:"editions,omitempty"`
	SupportedVersions   []string                `json:"supportedVersions,omitempty"`
	ConnectionModes     []string                `json:"connectionModes,omitempty"`
	Services            []string                `json:"services,omitempty"`
	Actions             []GameCapability        `json:"actions,omitempty"`
	Observations        []GameCapability        `json:"observations,omitempty"`
	CompanionArtifacts  []GameCompanionArtifact `json:"companionArtifacts,omitempty"`
	Network             *GameNetworkPolicy      `json:"network,omitempty"`
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
	if err := json.Unmarshal(data, &parsed); err != nil {
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
	seenArtifacts := make(map[string]struct{}, len(s.CompanionArtifacts))
	for _, artifact := range s.CompanionArtifacts {
		id := strings.TrimSpace(artifact.ID)
		if id == "" || strings.TrimSpace(artifact.Type) == "" || strings.TrimSpace(artifact.Source) == "" || strings.TrimSpace(artifact.InstallTarget) == "" {
			return fmt.Errorf("companion artifact id, type, source and installTarget are required")
		}
		if _, exists := seenArtifacts[id]; exists {
			return fmt.Errorf("duplicate companion artifact id %q", id)
		}
		seenArtifacts[id] = struct{}{}
		if !safePackageRelativePath(artifact.Source) {
			return fmt.Errorf("companion artifact %q source must be a safe package-relative path", id)
		}
		if !safePackageRelativePath(artifact.InstallTarget) {
			return fmt.Errorf("companion artifact %q installTarget must be a safe game-root-relative path", id)
		}
	}
	return nil
}

func safePackageRelativePath(raw string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return false
	}
	// Reject Windows drive/ADS syntax even when validation runs on Unix.
	if strings.Contains(normalized, ":") {
		return false
	}
	clean := path.Clean(normalized)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
