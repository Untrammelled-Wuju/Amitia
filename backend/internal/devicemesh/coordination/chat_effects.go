package coordination

import "context"

type OwnedChatEffectsRequest struct {
	ConversationID string `json:"conversationId"`
	RequestID      string `json:"requestId"`
	MessageID      string `json:"messageId"`
	ThreadID       string `json:"threadId,omitempty"`
	Source         string `json:"source,omitempty"`
	Channel        string `json:"channel,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	IsInternal     bool   `json:"isInternal,omitempty"`
}

type OwnerPostCommitPort interface {
	CommitOwnedChatEffects(context.Context, ExecutionScope, OwnedChatEffectsRequest) error
}
