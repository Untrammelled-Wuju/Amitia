package execution

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	amitialog "log"
)

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusDenied   ApprovalStatus = "denied"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

type ApprovalRequest struct {
	RuntimeKey     string         `json:"-"`
	ID             string         `json:"id"`
	SpaceID        string         `json:"-"`
	AgentID        string         `json:"agentId,omitempty"`
	TeamID         string         `json:"teamId,omitempty"`
	CharacterID    string         `json:"characterId,omitempty"`
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
	ResolvedBy     string         `json:"resolvedBy,omitempty"`
	ResolvedByType string         `json:"resolvedByType,omitempty"`
	ResolvedAt     string         `json:"resolvedAt,omitempty"`
	AuthMethod     string         `json:"authMethod,omitempty"`
	SessionID      string         `json:"sessionId,omitempty"`
}

type ApprovalActor struct {
	ID            string
	PrincipalType string
	AuthMethod    string
	SessionID     string
}

type pendingApproval struct {
	value    ApprovalRequest
	decision chan bool
	resolved bool
}

type ApprovalBroker struct {
	mu                 sync.Mutex
	pending            map[string]*pendingApproval
	persistence        ApprovalPersistence
	onRequested        func(ApprovalRequest) error
	onResolved         func(ApprovalRequest) error
	requestedObservers []func(ApprovalRequest) error
	resolvedObservers  []func(ApprovalRequest) error
}

func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{pending: make(map[string]*pendingApproval)}
}

func (b *ApprovalBroker) SetPersistence(persistence ApprovalPersistence) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.persistence = persistence
	b.mu.Unlock()
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
	b.mu.Lock()
	persistence := b.persistence
	b.mu.Unlock()
	if persistence != nil {
		_, found, err := persistence.FindApprovedApproval(ctx, request)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
		if err := persistence.SaveApprovalRequest(ctx, request); err != nil {
			return false, err
		}
	}
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
			if err := observer(request); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
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
	return b.ResolveWithActor(id, approved, ApprovalActor{})
}

func (b *ApprovalBroker) ResolveWithActor(id string, approved bool, actor ApprovalActor) error {
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
	request.ResolvedBy = strings.TrimSpace(actor.ID)
	request.ResolvedByType = strings.TrimSpace(actor.PrincipalType)
	request.AuthMethod = strings.TrimSpace(actor.AuthMethod)
	request.SessionID = strings.TrimSpace(actor.SessionID)
	request.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	entry.resolved = true
	entry.value = request
	onResolved := b.onResolved
	resolvedObservers := append([]func(ApprovalRequest) error(nil), b.resolvedObservers...)
	persistence := b.persistence
	b.mu.Unlock()
	if persistence != nil {
		if err := persistence.SaveApprovalResolution(context.Background(), request, actor); err != nil {
			return err
		}
	}
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
			if err := observer(request); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}
	entry.decision <- approved
	return nil
}

func (b *ApprovalBroker) HasPending(id string) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.pending[strings.TrimSpace(id)]
	return ok && entry != nil && !entry.resolved
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
	request.ResolvedByType = "system"
	request.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	entry.resolved = true
	entry.value = request
	onResolved := b.onResolved
	resolvedObservers := append([]func(ApprovalRequest) error(nil), b.resolvedObservers...)
	persistence := b.persistence
	b.mu.Unlock()
	if persistence != nil {
		persisted := request
		persisted.Status = ApprovalStatusExpired
		if err := persistence.SaveApprovalResolution(context.Background(), persisted, ApprovalActor{PrincipalType: "system"}); err != nil {
			return err
		}
	}
	if onResolved != nil {
		if err := onResolved(request); err != nil {
			return err
		}
	}
	for _, observer := range resolvedObservers {
		if observer != nil {
			if err := observer(request); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
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
