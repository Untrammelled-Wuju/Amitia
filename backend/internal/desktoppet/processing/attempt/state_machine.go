package attempt

import (
	"fmt"
	"time"
)

const (
	StateCreated           = "created"
	StateQueued            = "queued"
	StateRunning           = "running"
	StatePipelineCompleted = "pipeline_completed"
	StateCommitting        = "committing"
	StateCommitted         = "committed"
	StateFailedRetryable   = "failed_retryable"
	StateFailedTerminal    = "failed_terminal"
	StateCancelRequested   = "cancel_requested"
	StateCancelled         = "cancelled"
)

var terminalStates = map[string]bool{
	StateCommitted:      true,
	StateCancelled:      true,
	StateFailedTerminal: true,
}

var allowedTransitions = map[string]map[string]bool{
	StateCreated: {
		StateQueued: true,
	},
	StateQueued: {
		StateRunning: true,
	},
	StateRunning: {
		StatePipelineCompleted: true,
		StateFailedRetryable:   true,
		StateCancelRequested:   true,
	},
	StatePipelineCompleted: {
		StateCommitting:     true,
		StateCancelRequested: true,
	},
	StateCommitting: {
		StateCommitted:      true,
		StateFailedRetryable: true,
	},
	StateCancelRequested: {
		StateCancelled: true,
	},
	StateFailedRetryable: {
		StateQueued: true,
	},
}

type StateMachine struct{}

func NewStateMachine() *StateMachine {
	return &StateMachine{}
}

func (sm *StateMachine) CanTransition(from, to string) bool {
	if from == "" || to == "" {
		return false
	}
	if from == to {
		return false
	}
	if to == StateFailedTerminal {
		return !terminalStates[from]
	}
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

func (sm *StateMachine) ValidateTransition(from, to string) error {
	if sm.CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("attempt: invalid state transition from %s to %s", from, to)
}

type LeaseRepo interface {
	AcquireAttemptLease(attemptID, executionID, leaseOwner string, leaseDuration time.Duration) error
	RenewAttemptLease(attemptID, executionID string, leaseDuration time.Duration) error
	ReleaseAttemptLease(attemptID string) error
	GetAttemptLeaseInfo(attemptID string) (leaseOwner, leaseExpiresAt string, err error)
}

type LeaseManager struct {
	repo  LeaseRepo
	clock func() time.Time
}

func NewLeaseManager(repo LeaseRepo) *LeaseManager {
	return &LeaseManager{
		repo:  repo,
		clock: time.Now,
	}
}

func (lm *LeaseManager) Acquire(attemptID, executionID, leaseOwner string, leaseDuration time.Duration) error {
	if attemptID == "" {
		return fmt.Errorf("attempt: acquire lease: attempt id is empty")
	}
	if executionID == "" {
		return fmt.Errorf("attempt: acquire lease: execution id is empty")
	}
	return lm.repo.AcquireAttemptLease(attemptID, executionID, leaseOwner, leaseDuration)
}

func (lm *LeaseManager) Renew(attemptID, executionID string, leaseDuration time.Duration) error {
	if attemptID == "" {
		return fmt.Errorf("attempt: renew lease: attempt id is empty")
	}
	if executionID == "" {
		return fmt.Errorf("attempt: renew lease: execution id is empty")
	}
	return lm.repo.RenewAttemptLease(attemptID, executionID, leaseDuration)
}

func (lm *LeaseManager) Release(attemptID string) error {
	if attemptID == "" {
		return fmt.Errorf("attempt: release lease: attempt id is empty")
	}
	return lm.repo.ReleaseAttemptLease(attemptID)
}

func (lm *LeaseManager) IsLeaseValid(attemptID, executionID string) bool {
	if attemptID == "" || executionID == "" {
		return false
	}
	leaseOwner, leaseExpiresAt, err := lm.repo.GetAttemptLeaseInfo(attemptID)
	if err != nil || leaseOwner == "" {
		return false
	}
	if leaseOwner != executionID {
		return false
	}
	if leaseExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse("2006-01-02 15:04:05", leaseExpiresAt)
	if err != nil {
		return false
	}
	return lm.clock().Before(expiresAt)
}
