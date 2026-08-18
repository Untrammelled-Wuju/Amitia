package task_runtime

import (
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

type PendingTask struct {
	TaskRunID        string
	AttemptID        string
	TaskDefID        string
	UserID           string
	DeviceID         string
	RuntimeID        string
	SessionID        string
	Generation       int64
	LeaseID          string
	CreatedAt        time.Time
	DeadlineAt       time.Time
	ClaimCh          chan TaskClaimResult
	CancelFunc       context.CancelFunc
	CancelAckCh      chan struct{}
	LastHeartbeatSeq int64
	LeaseExpiresAt   time.Time
}

type TaskClaimResult struct {
	Success  bool
	WorkerID string
	LeaseID  string
	LeaseExp time.Time
	Error    string
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
	deadlineCtx, cancel := context.WithCancel(context.Background())
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
	if existing, exists := m.pending[taskRunID]; exists && existing != nil {
		m.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("task already pending: %s", taskRunID)
	}
	m.pending[taskRunID] = pt
	m.mu.Unlock()
	go m.watchDeadline(deadlineCtx, taskRunID)
	return pt, nil
}

func (m *PendingTaskManager) watchDeadline(ctx context.Context, taskRunID string) {
	for {
		m.mu.RLock()
		pt, ok := m.pending[taskRunID]
		if !ok {
			m.mu.RUnlock()
			return
		}
		deadline := pt.DeadlineAt
		m.mu.RUnlock()

		wait := time.Until(deadline)
		if wait <= 0 {
			m.mu.Lock()
			current, exists := m.pending[taskRunID]
			expired := exists && !current.DeadlineAt.After(time.Now().UTC())
			if expired {
				delete(m.pending, taskRunID)
				current.CancelFunc()
				select {
				case current.ClaimCh <- TaskClaimResult{Success: false, Error: "task lease/deadline expired"}:
				default:
				}
				select {
				case current.CancelAckCh <- struct{}{}:
				default:
				}
			}
			m.mu.Unlock()
			if !exists || expired {
				return
			}
			// The deadline was renewed after the previous read. Re-evaluate it.
			continue
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			// Re-read DeadlineAt because heartbeat/claim may have renewed it.
		}
	}
}

func (m *PendingTaskManager) Claim(taskRunID string, workerID string, leaseDuration time.Duration) bool {
	return m.ClaimBound(taskRunID, "", "", "", 0, workerID, leaseDuration)
}

func (m *PendingTaskManager) ClaimBound(taskRunID, attemptID, leaseID, sessionID string, generation int64, workerID string, leaseDuration time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok || !pendingTaskBindingMatches(pt, attemptID, leaseID, sessionID, generation) {
		return false
	}
	if leaseDuration <= 0 {
		leaseDuration = m.defaultTTL
	}
	leaseExp := time.Now().UTC().Add(leaseDuration)
	pt.LeaseExpiresAt = leaseExp
	pt.DeadlineAt = leaseExp
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

func (m *PendingTaskManager) ValidateBound(taskRunID, attemptID, leaseID, sessionID string, generation int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pt, ok := m.pending[taskRunID]
	return ok && pendingTaskBindingMatches(pt, attemptID, leaseID, sessionID, generation)
}

func (m *PendingTaskManager) HeartbeatBound(taskRunID, attemptID, leaseID, sessionID string, generation, sequence int64, leaseDuration time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok || !pendingTaskBindingMatches(pt, attemptID, leaseID, sessionID, generation) {
		return false
	}
	if sequence > 0 && sequence <= pt.LastHeartbeatSeq {
		return false
	}
	if sequence > 0 {
		pt.LastHeartbeatSeq = sequence
	}
	if leaseDuration <= 0 {
		leaseDuration = m.defaultTTL
	}
	pt.LeaseExpiresAt = time.Now().UTC().Add(leaseDuration)
	pt.DeadlineAt = pt.LeaseExpiresAt
	return true
}

func pendingTaskBindingMatches(pt *PendingTask, attemptID, leaseID, sessionID string, generation int64) bool {
	if pt == nil {
		return false
	}
	if attemptID != "" && pt.AttemptID != "" && pt.AttemptID != attemptID {
		return false
	}
	if leaseID != "" && pt.LeaseID != "" && pt.LeaseID != leaseID {
		return false
	}
	if sessionID != "" && pt.SessionID != "" && pt.SessionID != sessionID {
		return false
	}
	if generation > 0 && pt.Generation > 0 && pt.Generation != generation {
		return false
	}
	return true
}

func (m *PendingTaskManager) Complete(taskRunID string, success bool, errMsg string) {
	m.CompleteBound(taskRunID, "", "", "", 0, success, errMsg)
}

func (m *PendingTaskManager) CompleteBound(taskRunID, attemptID, leaseID, sessionID string, generation int64, success bool, errMsg string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok || !pendingTaskBindingMatches(pt, attemptID, leaseID, sessionID, generation) {
		return false
	}
	delete(m.pending, taskRunID)
	pt.CancelFunc()
	result := TaskClaimResult{Success: success, Error: errMsg}
	select {
	case pt.ClaimCh <- result:
	default:
	}
	select {
	case pt.CancelAckCh <- struct{}{}:
	default:
	}
	return true
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
			select {
			case pt.CancelAckCh <- struct{}{}:
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
		idx, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[idx.Int64()]
	}
	return string(b)
}
