package hook

import (
	"context"
	"encoding/json"
	"time"
)

type HookContextSnapshot struct {
	TraceID              string    `json:"traceId"`
	OperationID          string    `json:"operationId"`
	InvocationID         string    `json:"invocationId"`
	ExtensionID          string    `json:"extensionId"`
	CharacterID          *string   `json:"characterId,omitempty"`
	ConversationID       *string   `json:"conversationId,omitempty"`
	MessageID            *string   `json:"messageId,omitempty"`
	ScopeSnapshotID      string    `json:"scopeSnapshotId"`
	PermissionSnapshotID string    `json:"permissionSnapshotId"`
	Platform             string    `json:"platform"`
	Timestamp            time.Time `json:"timestamp"`
	Depth                int       `json:"depth"`
	ParentHookID         *string   `json:"parentHookId,omitempty"`
}

type HookInvocationInput struct {
	HookPointID     string              `json:"hookPointId"`
	ContractVersion int                 `json:"contractVersion"`
	Payload         json.RawMessage     `json:"payload"`
	Context         HookContextSnapshot `json:"context"`
}

type InvocationContext struct {
	Input         HookInvocationInput
	Point         HookPointDefinition
	Contribution  HookContributionDefinition
	Ctx           context.Context
	Cancel        context.CancelFunc
	Deadline      time.Time
	Depth         int
	ParentStack   []string
	OriginContrib string
}

func NewInvocationContext(parent context.Context, input HookInvocationInput, point HookPointDefinition, contrib HookContributionDefinition, depth int, parentStack []string) InvocationContext {
	timeout := contrib.Timeout
	if timeout <= 0 || timeout > point.MaxTimeout {
		timeout = point.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return InvocationContext{
		Input:         input,
		Point:         point,
		Contribution:  contrib,
		Ctx:           ctx,
		Cancel:        cancel,
		Deadline:      time.Now().Add(timeout),
		Depth:         depth,
		ParentStack:   parentStack,
		OriginContrib: contrib.ContributionID,
	}
}

func (ic *InvocationContext) IsInStack(contributionID string) bool {
	for _, id := range ic.ParentStack {
		if id == contributionID {
			return true
		}
	}
	return false
}

func (ic *InvocationContext) ExtendedStack() []string {
	out := make([]string, 0, len(ic.ParentStack)+1)
	out = append(out, ic.ParentStack...)
	out = append(out, ic.Contribution.ContributionID)
	return out
}
