package observability

import (
	"context"
	"sync"
	"time"
)

type StorageBackend interface {
	TraceStore
	OperationStore
	InvocationStore
	AttemptStore
	RuntimeEventStore
	AuditEventStore
	ErrorRecordStore
	Ping(ctx context.Context) error
	Close() error
}

type TraceStore interface {
	SaveTrace(ctx context.Context, t Trace) error
	GetTrace(ctx context.Context, traceID string) (*Trace, error)
	DeleteTrace(ctx context.Context, traceID string) error
}

type OperationStore interface {
	SaveOperation(ctx context.Context, op OperationRecord) error
	GetOperation(ctx context.Context, operationID string) (*OperationRecord, error)
	ListOperations(ctx context.Context, filter OperationFilter) ([]OperationRecord, string, error)
	UpdateOperationStatus(ctx context.Context, operationID string, status ExecutionStatus) error
}

type InvocationStore interface {
	SaveInvocation(ctx context.Context, inv InvocationRecord) error
	GetInvocation(ctx context.Context, invocationID string) (*InvocationRecord, error)
	ListInvocations(ctx context.Context, filter InvocationFilter) ([]InvocationRecord, string, error)
	UpdateInvocationStatus(ctx context.Context, invocationID string, status ExecutionStatus) error
	GetInvocationChildren(ctx context.Context, parentID string) ([]InvocationRecord, error)
	IncrementSideEffectCount(ctx context.Context, invocationID string, delta int) error
}

type AttemptStore interface {
	SaveAttempt(ctx context.Context, att ExecutionAttempt) error
	GetAttempt(ctx context.Context, attemptID string) (*ExecutionAttempt, error)
	ListAttemptsByInvocation(ctx context.Context, invocationID string) ([]ExecutionAttempt, error)
}

type RuntimeEventStore interface {
	SaveRuntimeEvent(ctx context.Context, evt RuntimeEventRecord) error
	ListRuntimeEvents(ctx context.Context, filter EventFilter) ([]RuntimeEventRecord, string, error)
}

type AuditEventStore interface {
	SaveAuditEvent(ctx context.Context, evt AuditEvent) error
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, string, error)
	GetAuditEvent(ctx context.Context, auditID string) (*AuditEvent, error)
}

type ErrorRecordStore interface {
	SaveErrorRecord(ctx context.Context, rec ErrorRecord) error
	GetErrorRecord(ctx context.Context, errorID string) (*ErrorRecord, error)
	ListErrorRecords(ctx context.Context, invocationID string) ([]ErrorRecord, error)
}

type ListOptions struct {
	Limit  int
	Cursor string
}

type OperationFilter struct {
	TraceID     string
	Type        OperationType
	ActorType   ActorType
	ActorID     string
	SubjectType SubjectType
	SubjectID   string
	Status      ExecutionStatus
	ExtensionID string
	Since       *time.Time
	Until       *time.Time
	ListOptions
}

type InvocationFilter struct {
	TraceID        string
	OperationID    string
	ParentID       string
	ExtensionID    string
	ModuleID       string
	CapabilityID   string
	CharacterID    string
	ConversationID string
	UserID         string
	RuntimeID      string
	Status         ExecutionStatus
	RiskLevel      RiskLevel
	ErrorCode      string
	Since          *time.Time
	Until          *time.Time
	ListOptions
}

type EventFilter struct {
	TraceID      string
	InvocationID string
	AttemptID    string
	EventType    string
	Severity     string
	Since        *time.Time
	Until        *time.Time
	ListOptions
}

type AuditFilter struct {
	TraceID     string
	OperationID string
	ActorType   ActorType
	ActorID     string
	Action      string
	Decision    string
	Result      string
	Since       *time.Time
	Until       *time.Time
	ListOptions
}

type MemoryStore struct {
	traces       map[string]Trace
	operations   map[string]OperationRecord
	invocations  map[string]InvocationRecord
	attempts     map[string][]ExecutionAttempt
	events       []RuntimeEventRecord
	auditEvents  []AuditEvent
	errorRecords map[string][]ErrorRecord
	mu           sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		traces:       make(map[string]Trace),
		operations:   make(map[string]OperationRecord),
		invocations:  make(map[string]InvocationRecord),
		attempts:     make(map[string][]ExecutionAttempt),
		events:       make([]RuntimeEventRecord, 0),
		auditEvents:  make([]AuditEvent, 0),
		errorRecords: make(map[string][]ErrorRecord),
	}
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) SaveTrace(ctx context.Context, t Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces[t.TraceID] = t
	return nil
}

func (s *MemoryStore) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.traces[traceID]
	if !ok {
		return nil, ErrTraceNotFound
	}
	return &t, nil
}

func (s *MemoryStore) DeleteTrace(ctx context.Context, traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.traces, traceID)
	return nil
}

func (s *MemoryStore) SaveOperation(ctx context.Context, op OperationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[op.OperationID] = op
	return nil
}

func (s *MemoryStore) GetOperation(ctx context.Context, operationID string) (*OperationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.operations[operationID]
	if !ok {
		return nil, ErrOperationNotFound
	}
	return &op, nil
}

func (s *MemoryStore) ListOperations(ctx context.Context, filter OperationFilter) ([]OperationRecord, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]OperationRecord, 0)
	for _, op := range s.operations {
		if !matchOperation(op, filter) {
			continue
		}
		results = append(results, op)
	}

	return paginateOps(results, filter.Limit, filter.Cursor)
}

func (s *MemoryStore) UpdateOperationStatus(ctx context.Context, operationID string, status ExecutionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.operations[operationID]
	if !ok {
		return ErrOperationNotFound
	}
	op.Status = status
	s.operations[operationID] = op
	return nil
}

func (s *MemoryStore) SaveInvocation(ctx context.Context, inv InvocationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invocations[inv.InvocationID] = inv
	return nil
}

func (s *MemoryStore) GetInvocation(ctx context.Context, invocationID string) (*InvocationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.invocations[invocationID]
	if !ok {
		return nil, ErrInvocationNotFound
	}
	return &inv, nil
}

func (s *MemoryStore) ListInvocations(ctx context.Context, filter InvocationFilter) ([]InvocationRecord, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]InvocationRecord, 0)
	for _, inv := range s.invocations {
		if !matchInvocation(inv, filter) {
			continue
		}
		results = append(results, inv)
	}

	return paginateInvs(results, filter.Limit, filter.Cursor)
}

func (s *MemoryStore) UpdateInvocationStatus(ctx context.Context, invocationID string, status ExecutionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invocations[invocationID]
	if !ok {
		return ErrInvocationNotFound
	}
	if err := IsTransitionValid(inv.Status, status); err != nil {
		return err
	}
	inv.Status = status
	s.invocations[invocationID] = inv
	return nil
}

func (s *MemoryStore) GetInvocationChildren(ctx context.Context, parentID string) ([]InvocationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	children := make([]InvocationRecord, 0)
	for _, inv := range s.invocations {
		if inv.ParentID == parentID {
			children = append(children, inv)
		}
	}
	return children, nil
}

func (s *MemoryStore) IncrementSideEffectCount(ctx context.Context, invocationID string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invocations[invocationID]
	if !ok {
		return ErrInvocationNotFound
	}
	inv.SideEffectCount += delta
	s.invocations[invocationID] = inv
	return nil
}

func (s *MemoryStore) SaveAttempt(ctx context.Context, att ExecutionAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[att.InvocationID] = append(s.attempts[att.InvocationID], att)
	return nil
}

func (s *MemoryStore) GetAttempt(ctx context.Context, attemptID string) (*ExecutionAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, atts := range s.attempts {
		for _, a := range atts {
			if a.AttemptID == attemptID {
				return &a, nil
			}
		}
	}
	return nil, ErrAttemptNotFound
}

func (s *MemoryStore) ListAttemptsByInvocation(ctx context.Context, invocationID string) ([]ExecutionAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	atts := s.attempts[invocationID]
	result := make([]ExecutionAttempt, len(atts))
	copy(result, atts)
	return result, nil
}

func (s *MemoryStore) SaveRuntimeEvent(ctx context.Context, evt RuntimeEventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, evt)
	return nil
}

func (s *MemoryStore) ListRuntimeEvents(ctx context.Context, filter EventFilter) ([]RuntimeEventRecord, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]RuntimeEventRecord, 0)
	for _, evt := range s.events {
		if !matchEvent(evt, filter) {
			continue
		}
		results = append(results, evt)
	}

	return paginateEvents(results, filter.Limit, filter.Cursor)
}

func (s *MemoryStore) SaveAuditEvent(ctx context.Context, evt AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditEvents = append(s.auditEvents, evt)
	return nil
}

func (s *MemoryStore) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]AuditEvent, 0)
	for _, evt := range s.auditEvents {
		if !matchAudit(evt, filter) {
			continue
		}
		results = append(results, evt)
	}

	return paginateAudits(results, filter.Limit, filter.Cursor)
}

func (s *MemoryStore) GetAuditEvent(ctx context.Context, auditID string) (*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, evt := range s.auditEvents {
		if evt.AuditID == auditID {
			return &evt, nil
		}
	}
	return nil, ErrOperationNotFound
}

func (s *MemoryStore) SaveErrorRecord(ctx context.Context, rec ErrorRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorRecords[rec.InvocationID] = append(s.errorRecords[rec.InvocationID], rec)
	return nil
}

func (s *MemoryStore) GetErrorRecord(ctx context.Context, errorID string) (*ErrorRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, recs := range s.errorRecords {
		for _, r := range recs {
			if r.ErrorID == errorID {
				return &r, nil
			}
		}
	}
	return nil, ErrOperationNotFound
}

func (s *MemoryStore) ListErrorRecords(ctx context.Context, invocationID string) ([]ErrorRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	recs := s.errorRecords[invocationID]
	result := make([]ErrorRecord, len(recs))
	copy(result, recs)
	return result, nil
}

func matchOperation(op OperationRecord, f OperationFilter) bool {
	if f.TraceID != "" && op.TraceID != f.TraceID {
		return false
	}
	if f.Type != "" && op.Type != f.Type {
		return false
	}
	if f.ActorType != "" && op.ActorType != f.ActorType {
		return false
	}
	if f.ActorID != "" && op.ActorID != f.ActorID {
		return false
	}
	if f.SubjectType != "" && op.SubjectType != f.SubjectType {
		return false
	}
	if f.SubjectID != "" && op.SubjectID != f.SubjectID {
		return false
	}
	if f.Status != "" && op.Status != f.Status {
		return false
	}
	if f.Since != nil && op.StartedAt.Before(*f.Since) {
		return false
	}
	if f.Until != nil && op.StartedAt.After(*f.Until) {
		return false
	}
	return true
}

func matchInvocation(inv InvocationRecord, f InvocationFilter) bool {
	if f.TraceID != "" && inv.TraceID != f.TraceID {
		return false
	}
	if f.OperationID != "" && inv.OperationID != f.OperationID {
		return false
	}
	if f.ParentID != "" && inv.ParentID != f.ParentID {
		return false
	}
	if f.ExtensionID != "" && inv.ExtensionID != f.ExtensionID {
		return false
	}
	if f.ModuleID != "" && inv.ModuleID != f.ModuleID {
		return false
	}
	if f.CapabilityID != "" && inv.CapabilityID != f.CapabilityID {
		return false
	}
	if f.CharacterID != "" && inv.CharacterID != f.CharacterID {
		return false
	}
	if f.ConversationID != "" && inv.ConversationID != f.ConversationID {
		return false
	}
	if f.UserID != "" && inv.UserID != f.UserID {
		return false
	}
	if f.RuntimeID != "" && inv.RuntimeID != f.RuntimeID {
		return false
	}
	if f.Status != "" && inv.Status != f.Status {
		return false
	}
	if f.RiskLevel != "" && inv.RiskLevel != f.RiskLevel {
		return false
	}
	if f.ErrorCode != "" && inv.ErrorCode != f.ErrorCode {
		return false
	}
	if f.Since != nil && inv.CreatedAt.Before(*f.Since) {
		return false
	}
	if f.Until != nil && inv.CreatedAt.After(*f.Until) {
		return false
	}
	return true
}

func matchEvent(evt RuntimeEventRecord, f EventFilter) bool {
	if f.TraceID != "" && evt.TraceID != f.TraceID {
		return false
	}
	if f.InvocationID != "" && evt.InvocationID != f.InvocationID {
		return false
	}
	if f.AttemptID != "" && evt.AttemptID != f.AttemptID {
		return false
	}
	if f.EventType != "" && evt.EventType != f.EventType {
		return false
	}
	if f.Severity != "" && evt.Severity != f.Severity {
		return false
	}
	if f.Since != nil && evt.Timestamp.Before(*f.Since) {
		return false
	}
	if f.Until != nil && evt.Timestamp.After(*f.Until) {
		return false
	}
	return true
}

func matchAudit(evt AuditEvent, f AuditFilter) bool {
	if f.TraceID != "" && evt.TraceID != f.TraceID {
		return false
	}
	if f.OperationID != "" && evt.OperationID != f.OperationID {
		return false
	}
	if f.ActorType != "" && evt.ActorType != f.ActorType {
		return false
	}
	if f.ActorID != "" && evt.ActorID != f.ActorID {
		return false
	}
	if f.Action != "" && evt.Action != f.Action {
		return false
	}
	if f.Decision != "" && evt.Decision != f.Decision {
		return false
	}
	if f.Result != "" && evt.Result != f.Result {
		return false
	}
	if f.Since != nil && evt.CreatedAt.Before(*f.Since) {
		return false
	}
	if f.Until != nil && evt.CreatedAt.After(*f.Until) {
		return false
	}
	return true
}
