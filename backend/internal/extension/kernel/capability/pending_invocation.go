package capability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type PendingInvocation struct {
	InvocationID   string
	CommandID      string
	UserID         string
	DeviceID       string
	RuntimeID      string
	SessionID      string
	Generation     int64
	HandlerName    string
	IdempotencyKey string
	FencingToken   int64
	CreatedAt      time.Time
	DeadlineAt     time.Time
	ResultCh       chan UnifiedToolResult
	CancelFunc     context.CancelFunc
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
	deadlineCtx, cancel := context.WithTimeout(context.Background(), deadline)
	pi := &PendingInvocation{
		InvocationID:   invocationID,
		CommandID:      commandID,
		UserID:         string(req.Route.UserID),
		DeviceID:       string(req.Route.DeviceID),
		RuntimeID:      string(req.Route.RuntimeID),
		SessionID:      sessionID,
		Generation:     generation,
		HandlerName:    req.Route.Binding.HandlerName,
		IdempotencyKey: req.Invocation.IdempotencyKey,
		FencingToken:   req.Invocation.FencingToken,
		CreatedAt:      time.Now().UTC(),
		DeadlineAt:     time.Now().UTC().Add(deadline),
		ResultCh:       make(chan UnifiedToolResult, 1),
		CancelFunc:     cancel,
	}
	m.mu.Lock()
	if existing, exists := m.pending[invocationID]; exists && existing != nil {
		m.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("invocation already pending: %s", invocationID)
	}
	m.pending[invocationID] = pi
	m.mu.Unlock()
	go func() {
		<-deadlineCtx.Done()
		if deadlineCtx.Err() == context.DeadlineExceeded {
			m.Cancel(invocationID, "invocation deadline exceeded")
		}
	}()
	return pi, nil
}

func (m *PendingInvocationManager) Complete(invocationID string, result UnifiedToolResult) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pi, ok := m.pending[invocationID]
	if !ok {
		return false
	}
	if result.Generation != 0 && pi.Generation != 0 && result.Generation != pi.Generation {
		return false
	}
	if result.RuntimeSessionID != "" && pi.SessionID != "" && result.RuntimeSessionID != pi.SessionID {
		return false
	}
	if result.DeviceID != "" && pi.DeviceID != "" && result.DeviceID != pi.DeviceID {
		return false
	}
	if result.RuntimeID != "" && pi.RuntimeID != "" && result.RuntimeID != pi.RuntimeID {
		return false
	}
	if !pendingReliabilityMatches(pi, result) {
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

func (m *PendingInvocationManager) Fail(invocationID string, errResult UnifiedToolResult) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pi, ok := m.pending[invocationID]
	if !ok {
		return false
	}
	if errResult.Generation != 0 && pi.Generation != 0 && errResult.Generation != pi.Generation {
		return false
	}
	if errResult.RuntimeSessionID != "" && pi.SessionID != "" && errResult.RuntimeSessionID != pi.SessionID {
		return false
	}
	if errResult.DeviceID != "" && pi.DeviceID != "" && errResult.DeviceID != pi.DeviceID {
		return false
	}
	if errResult.RuntimeID != "" && pi.RuntimeID != "" && errResult.RuntimeID != pi.RuntimeID {
		return false
	}
	if !pendingReliabilityMatches(pi, errResult) {
		return false
	}
	delete(m.pending, invocationID)
	pi.CancelFunc()
	select {
	case pi.ResultCh <- errResult:
	default:
	}
	return true
}

func pendingReliabilityMatches(pi *PendingInvocation, result UnifiedToolResult) bool {
	if pi == nil {
		return false
	}
	if pi.IdempotencyKey != "" {
		if result.Metadata == nil || fmt.Sprint(result.Metadata["idempotencyKey"]) != pi.IdempotencyKey {
			return false
		}
	}
	if pi.FencingToken != 0 {
		if result.Metadata == nil {
			return false
		}
		value, ok := result.Metadata["fencingToken"]
		if !ok {
			return false
		}
		var received int64
		switch typed := value.(type) {
		case int64:
			received = typed
		case int:
			received = int64(typed)
		case float64:
			received = int64(typed)
		default:
			_, _ = fmt.Sscan(fmt.Sprint(value), &received)
		}
		if received != pi.FencingToken {
			return false
		}
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

func (m *PendingInvocationManager) SupersedeSession(oldSessionID string, newGeneration int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pi := range m.pending {
		if pi.SessionID == oldSessionID && pi.Generation != newGeneration {
			delete(m.pending, id)
			pi.CancelFunc()
			result := NewToolFailureResult(id, "", &ToolError{
				Code:      ErrorCodeSessionSuperseded,
				Message:   "session superseded by newer connection",
				Retryable: true,
			})
			select {
			case pi.ResultCh <- result:
			default:
			}
		}
	}
}

func (m *PendingInvocationManager) CancelByDevice(deviceID string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pi := range m.pending {
		if pi.DeviceID == deviceID {
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
