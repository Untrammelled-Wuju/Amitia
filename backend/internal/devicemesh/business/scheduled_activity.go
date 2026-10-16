package business

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ScheduledActivity struct {
	LastUserAt time.Time
	Unreplied  int
}

func (e *Engine) ScheduledConversationActivity(ctx context.Context, request Request, binding json.RawMessage) (ScheduledActivity, error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ScheduledActivity{}, err
	}
	defer finish()
	var saved coordination.ExecutionScope
	if json.Unmarshal(binding, &saved) != nil || !sameScheduledOwner(saved, scope) {
		return ScheduledActivity{}, coordination.ErrWrongOwner
	}
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{RequestID: request.RequestID, ConversationID: request.ConversationID, ResourceKind: "message", Limit: 128})
	if err != nil {
		return ScheduledActivity{}, err
	}
	activity := ScheduledActivity{}
	proactive := []time.Time{}
	for _, resource := range snapshot.Resources {
		if resource.Kind != "message" || resource.Deleted {
			continue
		}
		var message struct {
			ConversationID string `json:"conversationId"`
			Role           string `json:"role"`
			Source         string `json:"source"`
			CreatedAt      string `json:"createdAt"`
		}
		if json.Unmarshal(resource.Body, &message) != nil || message.ConversationID != request.ConversationID {
			continue
		}
		created, err := time.Parse(time.RFC3339Nano, message.CreatedAt)
		if err != nil {
			continue
		}
		if message.Role == "user" && created.After(activity.LastUserAt) {
			activity.LastUserAt = created
		}
		if message.Role == "assistant" && message.Source == "proactive" {
			proactive = append(proactive, created)
		}
	}
	for _, created := range proactive {
		if created.After(activity.LastUserAt) {
			activity.Unreplied++
		}
	}
	return activity, nil
}
