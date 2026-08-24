package protocol

import (
	"encoding/json"
	"time"
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
