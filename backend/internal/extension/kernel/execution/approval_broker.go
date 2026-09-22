package execution

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusDenied   ApprovalStatus = "denied"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

type ApprovalRequest struct {
	ID             string         `json:"id"`
	SpaceID        string         `json:"-"`
	ConversationID string         `json:"conversationId"`
	TurnID         string         `json:"turnId,omitempty"`
	RequestID      string         `json:"requestId,omitempty"`
	ToolCallID     string         `json:"toolCallId,omitempty"`
	ToolName       string         `json:"toolName"`
	Arguments      string         `json:"arguments,omitempty"`
	RiskLevel      string         `json:"riskLevel,omitempty"`
	Status         ApprovalStatus `json:"status"`
	CreatedAt      string         `json:"createdAt"`
	ExpiresAt      string         `json:"expiresAt"`
}

type pendingApproval struct {
	value    ApprovalRequest
	decision chan bool
	resolved bool
}

type ApprovalBroker struct {
	mu                 sync.Mutex
	pending            map[string]*pendingApproval
	onRequested        func(ApprovalRequest) error
	onResolved         func(ApprovalRequest) error
	requestedObservers []func(ApprovalRequest) error
	resolvedObservers  []func(ApprovalRequest) error
}

func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{pending: make(map[string]*pendingApproval)}
}

func (b *ApprovalBroker) SetObservers(onRequested func(ApprovalRequest) error, onResolved func(ApprovalRequest) error) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.onRequested = onRequested
	b.onResolved = onResolved
	b.mu.Unlock()
}

// AddObservers adds best-effort observers without replacing the primary UI/event
// callbacks. Observer failures never block approval execution.
func (b *ApprovalBroker) AddObservers(onRequested func(ApprovalRequest) error, onResolved func(ApprovalRequest) error) {
	if b == nil {
		return
	}
	b.mu.Lock()
	if onRequested != nil {
		b.requestedObservers = append(b.requestedObservers, onRequested)
	}
	if onResolved != nil {
		b.resolvedObservers = append(b.resolvedObservers, onResolved)
	}
	b.mu.Unlock()
}

func (b *ApprovalBroker) Await(ctx context.Context, request ApprovalRequest, timeout time.Duration) (bool, error) {
	if b == nil {
		return false, fmt.Errorf("approval broker unavailable")
	}
	if strings.TrimSpace(request.ConversationID) == "" {
		return false, fmt.Errorf("approval requires a conversation")
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	expiresAt := time.Now().Add(timeout)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(expiresAt) {
		expiresAt = deadline
	}
	request.ID = uuid.NewString()
	request.Status = ApprovalStatusPending
	request.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	request.ExpiresAt = expiresAt.UTC().Format(time.RFC3339Nano)
	request.Arguments = truncateApprovalText(request.Arguments, 16*1024)
	entry := &pendingApproval{value: request, decision: make(chan bool, 1)}
	b.mu.Lock()
	b.pending[request.ID] = entry
	onRequested := b.onRequested
	requestedObservers := append([]func(ApprovalRequest) error(nil), b.requestedObservers...)
	b.mu.Unlock()
	if onRequested != nil {
		if err := onRequested(request); err != nil {
			b.mu.Lock()
			delete(b.pending, request.ID)
			b.mu.Unlock()
			return false, err
		}
	}
	for _, observer := range requestedObservers {
		if observer != nil {
			_ = observer(request)
		}
	}
	defer func() {
		b.mu.Lock()
		delete(b.pending, request.ID)
		b.mu.Unlock()
	}()

	timer := time.NewTimer(time.Until(expiresAt))
	defer timer.Stop()
	select {
	case approved := <-entry.decision:
		return approved, nil
	case <-ctx.Done():
		if err := b.expire(request.ID); err != nil {
			return false, err
		}
		return false, ctx.Err()
	case <-timer.C:
		if err := b.expire(request.ID); err != nil {
			return false, err
		}
		return false, context.DeadlineExceeded
	}
}

func (b *ApprovalBroker) List(spaceID, conversationID string) []ApprovalRequest {
	if b == nil {
		return []ApprovalRequest{}
	}
	conversationID = strings.TrimSpace(conversationID)
	spaceID = strings.TrimSpace(spaceID)
	now := time.Now()
	b.mu.Lock()
	items := make([]ApprovalRequest, 0, len(b.pending))
	for id, entry := range b.pending {
		expiresAt, err := time.Parse(time.RFC3339Nano, entry.value.ExpiresAt)
		if err != nil || !expiresAt.After(now) {
			delete(b.pending, id)
			continue
		}
		if spaceID != "" && entry.value.SpaceID != "" && entry.value.SpaceID != spaceID {
			continue
		}
		if conversationID != "" && entry.value.ConversationID != conversationID {
			continue
		}
		item := entry.value
		items = append(items, item)
	}
	b.mu.Unlock()
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt < items[j].CreatedAt
	})
	return items
}

func (b *ApprovalBroker) Resolve(id string, approved bool) error {
	if b == nil {
		return fmt.Errorf("approval broker unavailable")
	}
	id = strings.TrimSpace(id)
	b.mu.Lock()
	entry, ok := b.pending[id]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("approval not found")
	}
	if entry.resolved {
		b.mu.Unlock()
		return fmt.Errorf("approval already resolved")
	}
	request := entry.value
	if approved {
		request.Status = ApprovalStatusApproved
	} else {
		request.Status = ApprovalStatusDenied
	}
	entry.resolved = true
	entry.value = request
	onResolved := b.onResolved
	resolvedObservers := append([]func(ApprovalRequest) error(nil), b.resolvedObservers...)
	b.mu.Unlock()
	if onResolved != nil {
		if err := onResolved(request); err != nil {
			b.mu.Lock()
			if current, exists := b.pending[id]; exists && current == entry {
				current.resolved = false
				current.value.Status = ApprovalStatusPending
			}
			b.mu.Unlock()
			return err
		}
	}
	for _, observer := range resolvedObservers {
		if observer != nil {
			_ = observer(request)
		}
	}
	entry.decision <- approved
	return nil
}

func (b *ApprovalBroker) expire(id string) error {
	if b == nil {
		return fmt.Errorf("approval broker unavailable")
	}
	b.mu.Lock()
	entry, ok := b.pending[strings.TrimSpace(id)]
	if !ok || entry.resolved {
		b.mu.Unlock()
		return nil
	}
	request := entry.value
	request.Status = ApprovalStatusExpired
	entry.resolved = true
	entry.value = request
	onResolved := b.onResolved
	resolvedObservers := append([]func(ApprovalRequest) error(nil), b.resolvedObservers...)
	b.mu.Unlock()
	if onResolved != nil {
		if err := onResolved(request); err != nil {
			return err
		}
	}
	for _, observer := range resolvedObservers {
		if observer != nil {
			_ = observer(request)
		}
	}
	return nil
}

func truncateApprovalText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
