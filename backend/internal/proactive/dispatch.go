package proactive

import (
	"context"
)

type ProactiveDispatchRequest struct {
	CharacterID    string
	ConversationID string
	Channel        string
	Prompt         string
	RequestID      string
}

type ProactiveDispatchResult struct {
	Success bool
	Content string
}

type ProactiveDispatch interface {
	DispatchProactive(ctx context.Context, req ProactiveDispatchRequest) (*ProactiveDispatchResult, error)
}
