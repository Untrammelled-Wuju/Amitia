package coordination

import (
	"context"
	"encoding/json"
)

type ChannelDeliveryRequest struct {
	ConversationID string          `json:"conversationId"`
	RequestID      string          `json:"requestId"`
	Channel        string          `json:"channel"`
	PeerID         string          `json:"peerId"`
	Text           string          `json:"text,omitempty"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
}

type PendingChannelDelivery struct {
	Pending  bool                   `json:"pending"`
	Scope    ExecutionScope         `json:"scope"`
	Request  ChannelDeliveryRequest `json:"request"`
	ID       string                 `json:"-"`
	Revision int64                  `json:"-"`
	Cursor   string                 `json:"-"`
}

type ChannelDeliveryPort interface {
	SubmitChannelDelivery(context.Context, ExecutionScope, ChannelDeliveryRequest) error
}

type ChannelDeliveryRecoveryPort interface {
	PendingChannelDeliveries(context.Context, string) ([]PendingChannelDelivery, error)
	CompleteChannelDelivery(context.Context, PendingChannelDelivery) error
}
