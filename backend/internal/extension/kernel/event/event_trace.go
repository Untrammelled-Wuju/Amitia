package event

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventOperation struct {
	OperationID   string
	OperationType string
	ExtensionID   string
	StartedAt     time.Time
	FinishedAt    *time.Time
	Status        string
}

type EventInvocation struct {
	InvocationID         string
	OperationID          string
	EventID              string
	DeliveryID           string
	SubscriptionID       string
	Attempt             int
	RuntimeInstanceID    string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	TraceID             string
	FilterResult        string
	ProjectionResult    string
	OrderingResult      string
	PermissionResult    string
	ScopeResult         string
	Status              string
	StartedAt           time.Time
	FinishedAt          *time.Time
	ErrorCode           string
	ErrorMessage        string
}

type EventSideEffect struct {
	InvocationID string
	Kind         string
	Target       string
	Hash         string
	OccurredAt   time.Time
}

type EventAuditEntry struct {
	OperationID    string
	InvocationID   string
	EventID        string
	DeliveryID     string
	Action         string
	Actor          string
	ExtensionID    string
	Timestamp      time.Time
	PayloadHash    string
	ErrorCode      string
	Success        bool
	Detail         json.RawMessage
}

type EventTraceRecorder struct {
	mu          sync.Mutex
	operations  map[string]*EventOperation
	invocations map[string]*EventInvocation
	sideEffects map[string][]EventSideEffect
	audit       []EventAuditEntry
	maxEntries  int
}

func NewEventTraceRecorder(maxEntries int) *EventTraceRecorder {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &EventTraceRecorder{
		operations:  make(map[string]*EventOperation),
		invocations: make(map[string]*EventInvocation),
		sideEffects: make(map[string][]EventSideEffect),
		maxEntries:  maxEntries,
	}
}

func (r *EventTraceRecorder) StartOperation(ctx context.Context, opType, extensionID string) *EventOperation {
	op := &EventOperation{
		OperationID:   newOperationID(),
		OperationType: opType,
		ExtensionID:   extensionID,
		StartedAt:     time.Now().UTC(),
		Status:        "running",
	}
	r.mu.Lock()
	r.operations[op.OperationID] = op
	r.mu.Unlock()
	return op
}

func (r *EventTraceRecorder) FinishOperation(operationID, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	op, ok := r.operations[operationID]
	if !ok {
		return
	}
	now := time.Now().UTC()
	op.FinishedAt = &now
	op.Status = status
}

func (r *EventTraceRecorder) StartInvocation(operationID, eventID, deliveryID, subscriptionID string, attempt int) *EventInvocation {
	inv := &EventInvocation{
		InvocationID:   newInvocationID(),
		OperationID:    operationID,
		EventID:        eventID,
		DeliveryID:     deliveryID,
		SubscriptionID: subscriptionID,
		Attempt:        attempt,
		StartedAt:      time.Now().UTC(),
		Status:         "running",
	}
	r.mu.Lock()
	r.invocations[inv.InvocationID] = inv
	r.mu.Unlock()
	return inv
}

func (r *EventTraceRecorder) FinishInvocation(invocationID, status, errorCode, errorMessage string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.invocations[invocationID]
	if !ok {
		return
	}
	now := time.Now().UTC()
	inv.FinishedAt = &now
	inv.Status = status
	inv.ErrorCode = errorCode
	inv.ErrorMessage = errorMessage
}

func (r *EventTraceRecorder) RecordSideEffect(invocationID, kind, target, hash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sideEffects[invocationID] = append(r.sideEffects[invocationID], EventSideEffect{
		InvocationID: invocationID,
		Kind:         kind,
		Target:       target,
		Hash:         hash,
		OccurredAt:   time.Now().UTC(),
	})
}

func (r *EventTraceRecorder) RecordAudit(entry EventAuditEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.audit) >= r.maxEntries {
		r.audit = r.audit[1:]
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	r.audit = append(r.audit, entry)
}

func (r *EventTraceRecorder) GetOperation(operationID string) (*EventOperation, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	op, ok := r.operations[operationID]
	return op, ok
}

func (r *EventTraceRecorder) GetInvocation(invocationID string) (*EventInvocation, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.invocations[invocationID]
	return inv, ok
}

func (r *EventTraceRecorder) GetSideEffects(invocationID string) []EventSideEffect {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sideEffects[invocationID]
}

func (r *EventTraceRecorder) QueryAudit(filter AuditFilter) []EventAuditEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []EventAuditEntry
	for _, e := range r.audit {
		if filter.EventID != "" && e.EventID != filter.EventID {
			continue
		}
		if filter.DeliveryID != "" && e.DeliveryID != filter.DeliveryID {
			continue
		}
		if filter.ExtensionID != "" && e.ExtensionID != filter.ExtensionID {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		result = append(result, e)
	}
	return result
}

type AuditFilter struct {
	EventID     string
	DeliveryID  string
	ExtensionID string
	Action      string
}

func newOperationID() string {
	return fmt.Sprintf("op-%s", uuid.NewString())
}

func newInvocationID() string {
	return fmt.Sprintf("inv-%s", uuid.NewString())
}

func SanitizeForAudit(payload json.RawMessage, sensitiveFields []SensitiveFieldRule) json.RawMessage {
	if len(payload) == 0 {
		return payload
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return json.RawMessage(`{}`)
	}
	for _, rule := range sensitiveFields {
		if _, ok := raw[rule.Path]; ok {
			switch rule.DefaultAction {
			case SensitiveOmit:
				delete(raw, rule.Path)
			case SensitiveMask:
				raw[rule.Path] = "***"
			case SensitiveHash:
				raw[rule.Path] = "<hashed>"
			case SensitiveSummary:
				raw[rule.Path] = "<summary>"
			}
		}
	}
	out, _ := json.Marshal(raw)
	return out
}
