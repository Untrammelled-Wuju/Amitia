package extensioncontext

import (
	"context"
	"encoding/json"
	"time"
)

type Request struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	At             time.Time
}

type Contribution struct {
	Source      string          `json:"source"`
	ExtensionID string          `json:"extensionId,omitempty"`
	ModuleID    string          `json:"moduleId,omitempty"`
	ToolID      string          `json:"toolId,omitempty"`
	Priority    int             `json:"priority"`
	Data        json.RawMessage `json:"data,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type Snapshot struct {
	Slot          string         `json:"slot"`
	Contributions []Contribution `json:"contributions"`
}

type Provider interface {
	Resolve(ctx context.Context, slot string, request Request) (json.RawMessage, error)
}

func Decode(raw json.RawMessage) (Snapshot, error) {
	var snapshot Snapshot
	if len(raw) == 0 {
		return snapshot, nil
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
