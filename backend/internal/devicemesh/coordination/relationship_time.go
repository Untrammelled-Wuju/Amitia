package coordination

import (
	"context"
	"encoding/json"
)

type RelationshipTimeRequest struct {
	EpisodeID string          `json:"episodeId,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}

type RelationshipTimePort interface {
	RelationshipTime(context.Context, ExecutionScope, RelationshipTimeRequest) (json.RawMessage, error)
}
