package capability

import (
	"context"
	"sync"
	"time"
)

type PendingInvocation struct {
	InvocationID  string
	CommandID     string
	UserID        string
	DeviceID      string
	RuntimeID     string
	SessionID     string
	Generation    int64
	HandlerName   string
	CreatedAt     time.Time
	DeadlineAt    time.Time
	ResultCh      chan UnifiedToolResult
	CancelFunc    context.CancelFunc
}

type PendingInvocationManager struct {
	mu         sync.RWMutex
	pending    map[string]*PendingInvocation
	defaultTTL time.Duration
}

func NewPendingInvocationManager() *PendingInvocationManager {
	return &PendingInvocationManager{
		pending:    make(map[string]*PendingInvocation),
		defaultTTL: 30 * time.Second,
	}
}

func (m *PendingInvocationManager) Register(req DeviceRuntimeInvocationRequest, commandID string, sessionID string, generation int64, deadline time.Duration) (*PendingInvocation, error) {
	if deadline <= 0 {
		deadline = m.defaultTTL
	}
	invocationID := req.Invocation.InvocationID
	_, cancel := context.WithTimeout(context.Background(), deadline)
	pi := &PendingInvocation{
		InvocationID: invocationID,
		CommandID:    commandID,
		UserID:       string(req.Route.UserID),
		DeviceID:     string(req.Route.DeviceID),
		RuntimeID:    string(req.Route.RuntimeID),
		SessionID:    sessionID,
		Generation:   generation,
		HandlerName:  req.Route.Binding.HandlerName,
		CreatedAt:    time.Now().UTC(),
		DeadlineAt:   time.Now().UTC().Add(deadline),
		ResultCh:     make(chan UnifiedToolResult, 1),
		CancelFunc:   cancel,
	}
	m.mu.Lock()
	m.pending[invocationID] = pi
	m.mu.Unlock()
	return pi, nil
}

func (m *PendingInvocationManager) Complete(invocationID string, result UnifiedToolResult) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pi, ok := m.pending[invocationID]
	if !ok {
		return false
	}
	delete(m.pending, invocationID)
	pi.CancelFunc()
	select {
	case pi.ResultCh <- result:
	default:
	}
	return true
}

func (m *PendingInvocationManager) Get(invocationID string) (*PendingInvocation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pi, ok := m.pending[invocationID]
	return pi, ok
}

func (m *PendingInvocationManager) Cancel(invocationID string, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pi, ok := m.pending[invocationID]
	if !ok {
		return false
	}
	delete(m.pending, invocationID)
	pi.CancelFunc()
	result := NewToolFailureResult(invocationID, "", &ToolError{
		Code:      ErrorCodeCancelled,
		Message:   reason,
		Retryable: false,
	})
	select {
	case pi.ResultCh <- result:
	default:
	}
	return true
}

func (m *PendingInvocationManager) CancelAll(sessionID string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pi := range m.pending {
		if pi.SessionID == sessionID {
			delete(m.pending, id)
			pi.CancelFunc()
			result := NewToolFailureResult(id, "", &ToolError{
				Code:      ErrorCodeConnectionLost,
				Message:   reason,
				Retryable: true,
			})
			select {
			case pi.ResultCh <- result:
			default:
			}
		}
	}
}

func (m *PendingInvocationManager) WaitForResult(ctx context.Context, invocationID string) (UnifiedToolResult, error) {
	m.mu.RLock()
	pi, ok := m.pending[invocationID]
	m.mu.RUnlock()
	if !ok {
		return NewToolFailureResult(invocationID, "", &ToolError{
			Code:    ErrorCodeRuntimeUnavailable,
			Message: "invocation not registered",
		}), nil
	}
	select {
	case result := <-pi.ResultCh:
		return result, nil
	case <-ctx.Done():
		return ResultFromContextError(invocationID, ctx.Err()), ctx.Err()
	}
}

func (m *PendingInvocationManager) PendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}
