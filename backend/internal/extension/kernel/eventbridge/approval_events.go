package eventbridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
)

const approvalProducerID = "approval-broker"

const approvalDedupLimit = 4096

// ApprovalEventPublisher delivers approval lifecycle changes into the durable
// event stream so hosts, workflows and devices observe them after reconnects.
// It remembers recently published (approval, state) pairs so repeated broker
// notifications cannot fan out duplicate events.
type ApprovalEventPublisher struct {
	publisher *Publisher

	mu   sync.Mutex
	seen map[string]struct{}
}

type ApprovalEventPayload struct {
	ApprovalID     string `json:"approvalId"`
	SpaceID        string `json:"spaceId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	TurnID         string `json:"turnId,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
	ToolCallID     string `json:"toolCallId,omitempty"`
	ToolName       string `json:"toolName,omitempty"`
	RiskLevel      string `json:"riskLevel,omitempty"`
	Status         string `json:"status"`
	ResolvedBy     string `json:"resolvedBy,omitempty"`
	ResolvedByType string `json:"resolvedByType,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
	ResolvedAt     string `json:"resolvedAt,omitempty"`
}

func NewApprovalEventPublisher(publisher *Publisher) *ApprovalEventPublisher {
	return &ApprovalEventPublisher{publisher: publisher, seen: make(map[string]struct{})}
}

func (p *ApprovalEventPublisher) Requested(ctx context.Context, request execution.ApprovalRequest) error {
	return p.publish(ctx, approvalRequested, "requested", request)
}

func (p *ApprovalEventPublisher) Resolved(ctx context.Context, request execution.ApprovalRequest) error {
	return p.publish(ctx, approvalResolved, "resolved", request)
}

func (p *ApprovalEventPublisher) publish(ctx context.Context, typeID event.EventTypeID, state string, request execution.ApprovalRequest) error {
	if p == nil || p.publisher == nil {
		return nil
	}
	if request.ID == "" {
		return fmt.Errorf("eventbridge: approval id required")
	}
	key := request.ID + "|" + state
	if !p.markOnce(key) {
		return nil
	}
	payload := ApprovalEventPayload{
		ApprovalID:     request.ID,
		SpaceID:        request.SpaceID,
		ConversationID: request.ConversationID,
		TurnID:         request.TurnID,
		RequestID:      request.RequestID,
		ToolCallID:     request.ToolCallID,
		ToolName:       request.ToolName,
		RiskLevel:      request.RiskLevel,
		Status:         string(request.Status),
		ResolvedBy:     request.ResolvedBy,
		ResolvedByType: request.ResolvedByType,
		SessionID:      request.SessionID,
		CreatedAt:      request.CreatedAt,
		ExpiresAt:      request.ExpiresAt,
		ResolvedAt:     request.ResolvedAt,
	}
	opts := event.PublishOptions{
		Domain:        event.EventDomainInteraction,
		ProducerType:  event.EventProducerTypeSystem,
		ProducerID:    approvalProducerID,
		AggregateType: "approval",
		AggregateID:   request.ID,
		PartitionKey:  firstNonEmpty(request.SpaceID, request.ConversationID, request.ID),
		OrderingKey:   request.ID,
		OperationID:   request.RequestID,
		CausationID:   request.ID,
	}
	if _, err := p.publisher.Publish(ctx, typeID, eventVersion, payload, opts); err != nil {
		p.forget(key)
		return err
	}
	return nil
}

func (p *ApprovalEventPublisher) markOnce(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen == nil {
		p.seen = make(map[string]struct{})
	}
	if _, exists := p.seen[key]; exists {
		return false
	}
	if len(p.seen) >= approvalDedupLimit {
		for existing := range p.seen {
			delete(p.seen, existing)
			break
		}
	}
	p.seen[key] = struct{}{}
	return true
}

func (p *ApprovalEventPublisher) forget(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.seen, key)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
