package task_runtime

import (
	"context"
	"sync"
	"time"
)

type PendingTask struct {
	TaskRunID    string
	AttemptID    string
	TaskDefID    string
	UserID       string
	DeviceID     string
	RuntimeID    string
	SessionID    string
	Generation   int64
	LeaseID      string
	CreatedAt    time.Time
	DeadlineAt   time.Time
	ClaimCh      chan TaskClaimResult
	CancelFunc   context.CancelFunc
	CancelAckCh  chan struct{}
}

type TaskClaimResult struct {
	Success   bool
	WorkerID  string
	LeaseID   string
	LeaseExp  time.Time
	Error     string
}

type PendingTaskManager struct {
	mu         sync.RWMutex
	pending    map[string]*PendingTask
	defaultTTL time.Duration
}

func NewPendingTaskManager() *PendingTaskManager {
	return &PendingTaskManager{
		pending:    make(map[string]*PendingTask),
		defaultTTL: 60 * time.Second,
	}
}

func (m *PendingTaskManager) Register(request TaskExecutionRequest, sessionID string, generation int64, deadline time.Duration) (*PendingTask, error) {
	if deadline <= 0 {
		deadline = m.defaultTTL
	}
	taskRunID := request.Run.TaskRunID
	_, cancel := context.WithTimeout(context.Background(), deadline)
	target := request.Target.Normalize()
	pt := &PendingTask{
		TaskRunID:   taskRunID,
		AttemptID:   request.AttemptID.String(),
		TaskDefID:   request.Run.TaskDefinitionID,
		UserID:      string(target.UserID),
		DeviceID:    string(target.DeviceID),
		RuntimeID:   string(target.RuntimeID),
		SessionID:   sessionID,
		Generation:  generation,
		LeaseID:     generateLeaseID(),
		CreatedAt:   time.Now().UTC(),
		DeadlineAt:  time.Now().UTC().Add(deadline),
		ClaimCh:     make(chan TaskClaimResult, 1),
		CancelFunc:  cancel,
		CancelAckCh: make(chan struct{}, 1),
	}
	m.mu.Lock()
	m.pending[taskRunID] = pt
	m.mu.Unlock()
	return pt, nil
}

func (m *PendingTaskManager) Claim(taskRunID string, workerID string, leaseDuration time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok {
		return false
	}
	leaseExp := time.Now().UTC().Add(leaseDuration)
	pt.LeaseID = generateLeaseID()
	result := TaskClaimResult{
		Success:  true,
		WorkerID: workerID,
		LeaseID:  pt.LeaseID,
		LeaseExp: leaseExp,
	}
	select {
	case pt.ClaimCh <- result:
	default:
	}
	return true
}

func (m *PendingTaskManager) Complete(taskRunID string, success bool, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok {
		return
	}
	delete(m.pending, taskRunID)
	pt.CancelFunc()
	result := TaskClaimResult{
		Success: success,
		Error:   errMsg,
	}
	select {
	case pt.ClaimCh <- result:
	default:
	}
	if !success {
		select {
		case pt.CancelAckCh <- struct{}{}:
		default:
		}
	}
}

func (m *PendingTaskManager) Get(taskRunID string) (*PendingTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pt, ok := m.pending[taskRunID]
	return pt, ok
}

func (m *PendingTaskManager) Cancel(taskRunID string, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok {
		return false
	}
	delete(m.pending, taskRunID)
	pt.CancelFunc()
	result := TaskClaimResult{
		Success: false,
		Error:   reason,
	}
	select {
	case pt.ClaimCh <- result:
	default:
	}
	return true
}

func (m *PendingTaskManager) CancelAll(sessionID string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pt := range m.pending {
		if pt.SessionID == sessionID {
			delete(m.pending, id)
			pt.CancelFunc()
			result := TaskClaimResult{
				Success: false,
				Error:   reason,
			}
			select {
			case pt.ClaimCh <- result:
			default:
			}
		}
	}
}

func (m *PendingTaskManager) WaitForClaim(ctx context.Context, taskRunID string) (TaskClaimResult, error) {
	m.mu.RLock()
	pt, ok := m.pending[taskRunID]
	m.mu.RUnlock()
	if !ok {
		return TaskClaimResult{Success: false, Error: "task not registered"}, nil
	}
	select {
	case result := <-pt.ClaimCh:
		return result, nil
	case <-ctx.Done():
		return TaskClaimResult{Success: false, Error: ctx.Err().Error()}, ctx.Err()
	}
}

func (m *PendingTaskManager) WaitForCancelAck(ctx context.Context, taskRunID string) bool {
	m.mu.RLock()
	pt, ok := m.pending[taskRunID]
	m.mu.RUnlock()
	if !ok {
		return true
	}
	select {
	case <-pt.CancelAckCh:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *PendingTaskManager) PendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}

func generateLeaseID() string {
	return "lease_" + time.Now().Format("20060102150405") + "_" + randomSuffix(8)
}

func randomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
