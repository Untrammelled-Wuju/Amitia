package desktop_update

import (
	"fmt"
	"sync"
)

type UpdateState string

const (
	StateCreated             UpdateState = "created"
	StateChecking            UpdateState = "checking"
	StateAvailable           UpdateState = "available"
	StateDownloading         UpdateState = "downloading"
	StateDownloaded          UpdateState = "downloaded"
	StateVerifying           UpdateState = "verifying"
	StateStaging             UpdateState = "staging"
	StatePreflight           UpdateState = "preflight"
	StateWaitingConfirmation UpdateState = "waiting_confirmation"
	StateDraining            UpdateState = "draining"
	StateMigrating           UpdateState = "migrating"
	StateActivating          UpdateState = "activating"
	StateVerifyingHealth     UpdateState = "verifying_health"
	StateCommitting          UpdateState = "committing"
	StateCompleted           UpdateState = "completed"
	StateRollbackPending     UpdateState = "rollback_pending"
	StateRollingBack         UpdateState = "rolling_back"
	StateRolledBack          UpdateState = "rolled_back"
	StateFailed              UpdateState = "failed"
	StateCancelled           UpdateState = "cancelled"
	StateRecoveryRequired    UpdateState = "recovery_required"
	StateManualIntervention  UpdateState = "manual_intervention"
)

type StateMachine struct {
	mu          sync.RWMutex
	transitions map[UpdateState][]UpdateState
}

func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		transitions: map[UpdateState][]UpdateState{
			StateCreated: {
				StateChecking,
				StateCancelled,
				StateFailed,
			},
			StateChecking: {
				StateAvailable,
				StateCompleted,
				StateFailed,
				StateCancelled,
			},
			StateAvailable: {
				StateDownloading,
				StateCancelled,
				StateFailed,
			},
			StateDownloading: {
				StateDownloaded,
				StateFailed,
				StateCancelled,
				StateRecoveryRequired,
			},
			StateDownloaded: {
				StateVerifying,
				StateFailed,
				StateCancelled,
			},
			StateVerifying: {
				StateStaging,
				StateFailed,
				StateCancelled,
			},
			StateStaging: {
				StatePreflight,
				StateFailed,
				StateCancelled,
				StateRecoveryRequired,
			},
			StatePreflight: {
				StateWaitingConfirmation,
				StateDraining,
				StateFailed,
				StateCancelled,
			},
			StateWaitingConfirmation: {
				StateDraining,
				StateCancelled,
				StateFailed,
			},
			StateDraining: {
				StateMigrating,
				StateFailed,
				StateCancelled,
				StateRecoveryRequired,
			},
			StateMigrating: {
				StateActivating,
				StateRollbackPending,
				StateFailed,
				StateRecoveryRequired,
			},
			StateActivating: {
				StateVerifyingHealth,
				StateRollbackPending,
				StateFailed,
				StateRecoveryRequired,
			},
			StateVerifyingHealth: {
				StateCommitting,
				StateRollbackPending,
				StateFailed,
				StateRecoveryRequired,
			},
			StateCommitting: {
				StateCompleted,
				StateRollbackPending,
				StateFailed,
				StateRecoveryRequired,
			},
			StateCompleted: {},
			StateRollbackPending: {
				StateRollingBack,
				StateManualIntervention,
				StateFailed,
			},
			StateRollingBack: {
				StateRolledBack,
				StateManualIntervention,
				StateFailed,
			},
			StateRolledBack: {},
			StateFailed: {
				StateCreated,
			},
			StateCancelled: {},
			StateRecoveryRequired: {
				StateCreated,
				StateManualIntervention,
			},
			StateManualIntervention: {
				StateCreated,
			},
		},
	}
	return sm
}

func (sm *StateMachine) CanTransition(current, target UpdateState) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	allowed, ok := sm.transitions[current]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

func (sm *StateMachine) Transition(current, target UpdateState) error {
	if !sm.CanTransition(current, target) {
		return fmt.Errorf("desktop_update: invalid state transition %s -> %s", current, target)
	}
	return nil
}

func (sm *StateMachine) AllowedTransitions(from UpdateState) []UpdateState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	allowed, ok := sm.transitions[from]
	if !ok {
		return nil
	}
	out := make([]UpdateState, len(allowed))
	copy(out, allowed)
	return out
}

func (sm *StateMachine) IsTerminal(state UpdateState) bool {
	switch state {
	case StateCompleted, StateRolledBack, StateCancelled, StateManualIntervention:
		return true
	default:
		return false
	}
}

func (sm *StateMachine) IsRecoverable(state UpdateState) bool {
	switch state {
	case StateDownloading, StateVerifying, StateStaging, StateDraining,
		StateMigrating, StateActivating, StateVerifyingHealth, StateCommitting,
		StateRollingBack:
		return true
	default:
		return false
	}
}
