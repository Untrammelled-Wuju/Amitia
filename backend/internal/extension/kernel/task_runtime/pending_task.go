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
	SpaceID          string
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
	ConfirmAuthority func(context.Context) error
	AuthorityCallID  string
}

type TaskClaimResult struct {
	Success  bool
	WorkerID string
	LeaseID  string
	LeaseExp time.Time
	Error    string
}

type confirmedTaskStop struct {
	AttemptID   string
	LeaseID     string
	SessionID   string
	SpaceID     string
	DeviceID    string
	RuntimeID   string
	Generation  int64
	ConfirmedAt time.Time
}

type PendingTaskManager struct {
	mu         sync.RWMutex
	pending    map[string]*PendingTask
	confirmed  map[string]confirmedTaskStop
	defaultTTL time.Duration
}

func NewPendingTaskManager() *PendingTaskManager {
	return &PendingTaskManager{
		pending:    make(map[string]*PendingTask),
		confirmed:  make(map[string]confirmedTaskStop),
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
		SpaceID:     string(target.SpaceID),
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
	delete(m.confirmed, taskRunID)
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

	if pt == nil || attemptID == "" || leaseID == "" || sessionID == "" || generation < 1 {
		return false
	}
	return pt.AttemptID == attemptID && pt.LeaseID == leaseID && pt.SessionID == sessionID && pt.Generation == generation
}

func (m *PendingTaskManager) CompleteBound(taskRunID, attemptID, leaseID, sessionID string, generation int64, success bool, errMsg string) bool {
	if attemptID == "" || leaseID == "" || sessionID == "" || generation < 1 {
		return false
	}
	m.mu.Lock()
	pt, ok := m.pending[taskRunID]
	if !ok || !pendingTaskBindingMatches(pt, attemptID, leaseID, sessionID, generation) {
		m.mu.Unlock()
		return false
	}
	confirm := pt.ConfirmAuthority
	m.mu.Unlock()
	if confirm != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := confirm(ctx)
		cancel()
		if err != nil {
			return false
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, exists := m.pending[taskRunID]; !exists || current != pt {
		return false
	}
	delete(m.pending, taskRunID)
	if m.confirmed == nil {
		m.confirmed = make(map[string]confirmedTaskStop)
	}
	now := time.Now()
	for id, confirmedAt := range m.confirmed {
		if now.Sub(confirmedAt.ConfirmedAt) > 10*time.Minute {
			delete(m.confirmed, id)
		}
	}
	if len(m.confirmed) >= 256 {
		oldestID := ""
		var oldest time.Time
		for id, confirmedAt := range m.confirmed {
			if oldestID == "" || confirmedAt.ConfirmedAt.Before(oldest) {
				oldestID, oldest = id, confirmedAt.ConfirmedAt
			}
		}
		delete(m.confirmed, oldestID)
	}
	m.confirmed[taskRunID] = confirmedTaskStop{AttemptID: pt.AttemptID, LeaseID: pt.LeaseID, SessionID: pt.SessionID, SpaceID: pt.SpaceID, DeviceID: pt.DeviceID, RuntimeID: pt.RuntimeID, Generation: pt.Generation, ConfirmedAt: now}
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

func (m *PendingTaskManager) BindAuthority(taskRunID, attemptID, sessionID string, generation int64, confirm func(context.Context) error, callIDs ...string) error {
	if len(callIDs) > 0 && (len(callIDs) != 1 || callIDs[0] == "" || len(callIDs[0]) > 512) {
		return fmt.Errorf("任务授权调用编号无效")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pt, ok := m.pending[taskRunID]
	if !ok || pt.AttemptID != attemptID || pt.SessionID != sessionID || pt.Generation != generation || confirm == nil || pt.ConfirmAuthority != nil {
		return NewTaskError(ErrTaskScopeDenied, "远端任务授权与等待执行身份不一致")
	}
	pt.ConfirmAuthority = confirm
	if len(callIDs) > 0 {
		pt.AuthorityCallID = callIDs[0]
	}
	return nil
}

func (m *PendingTaskManager) ValidateOwnerBound(taskRunID, attemptID, leaseID, sessionID string, generation int64, callID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pt, ok := m.pending[taskRunID]
	return ok && attemptID != "" && leaseID != "" && sessionID != "" && generation > 0 && pt.AttemptID == attemptID && pt.LeaseID == leaseID && pt.SessionID == sessionID && pt.Generation == generation && callID != "" && pt.AuthorityCallID == callID && pt.ConfirmAuthority != nil && pt.LeaseExpiresAt.After(time.Now())
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

func (m *PendingTaskManager) WaitForCancelAckBound(ctx context.Context, run *TaskRun) bool {
	if m == nil || run == nil || run.ExecutionAttemptID == "" || run.LeaseID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
		return false
	}
	if m.ConfirmStoppedAttempt(run) {
		return true
	}
	m.mu.RLock()
	pending, exists := m.pending[run.TaskRunID]
	bound := exists && pending.AttemptID == run.ExecutionAttemptID.String() && pending.LeaseID == run.LeaseID && pending.SessionID == run.ExecutionTarget.RuntimeSessionID.String() && pending.SpaceID == run.ExecutionTarget.SpaceID.String() && pending.DeviceID == run.ExecutionTarget.DeviceID.String() && pending.RuntimeID == run.ExecutionTarget.RuntimeID.String() && pending.Generation == run.ExecutionTarget.ConnectionGeneration
	var stopped <-chan struct{}
	if bound {
		stopped = pending.CancelAckCh
	}
	m.mu.RUnlock()
	if !bound {
		return false
	}
	select {
	case <-stopped:
		return m.ConfirmStoppedAttempt(run)
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

func (m *PendingTaskManager) ConfirmStoppedAttempt(run *TaskRun) bool {
	if m == nil || run == nil || run.ExecutionAttemptID == "" || run.LeaseID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, active := m.pending[run.TaskRunID]; active {
		return false
	}
	ack, ok := m.confirmed[run.TaskRunID]
	return ok && time.Since(ack.ConfirmedAt) <= 10*time.Minute && ack.AttemptID == run.ExecutionAttemptID.String() && ack.LeaseID == run.LeaseID && ack.SessionID == run.ExecutionTarget.RuntimeSessionID.String() && ack.SpaceID == run.ExecutionTarget.SpaceID.String() && ack.DeviceID == run.ExecutionTarget.DeviceID.String() && ack.RuntimeID == run.ExecutionTarget.RuntimeID.String() && ack.Generation == run.ExecutionTarget.ConnectionGeneration
}
