package canary

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type StageManager struct {
	states map[string]*CanaryState
	repo   *CanaryRepository
	mu     sync.RWMutex
}

func NewStageManager(repo *CanaryRepository) *StageManager {
	return &StageManager{
		states: make(map[string]*CanaryState),
		repo:   repo,
	}
}

func (m *StageManager) StartCanary(ctx context.Context, state *CanaryState, policy *CanaryPolicy) error {
	m.mu.Lock()
	now := time.Now().UTC()
	state.StartedAt = now
	state.UpdatedAt = now
	if state.Status == "" {
		state.Status = CanaryStatusCreated
	}
	m.states[state.CanaryID] = state
	snapshot := *state
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.SaveCanaryState(ctx, snapshot); err != nil {
			return fmt.Errorf("canary: persist canary state: %w", err)
		}
	}
	return nil
}

func (m *StageManager) AdvanceStage(ctx context.Context, state *CanaryState, policy *CanaryPolicy, healthEval HealthEvaluation, observations int, elapsed time.Duration) (*AutoAdvanceResult, error) {
	m.mu.Lock()

	result := &AutoAdvanceResult{}

	if healthEval.ShouldAbort {
		result.CanAdvance = false
		result.Reason = "health_check_failed"
		result.Blockers = append(result.Blockers, healthEval.AbortReason)
		m.mu.Unlock()
		return result, nil
	}

	stage := m.currentStage(state, policy)
	if stage == nil {
		result.CanAdvance = false
		result.Reason = "no_current_stage"
		m.mu.Unlock()
		return result, nil
	}

	if !stage.AutoAdvance {
		result.CanAdvance = false
		result.Reason = "auto_advance_disabled"
		m.mu.Unlock()
		return result, nil
	}

	if elapsed < stage.MinDuration {
		result.CanAdvance = false
		result.Reason = "min_duration_not_met"
		m.mu.Unlock()
		return result, nil
	}

	if observations < stage.MinInvocations {
		result.CanAdvance = false
		result.Reason = "canary_waiting_for_observations"
		m.mu.Unlock()
		return result, nil
	}

	next := m.nextStage(state, policy)
	if next == nil {
		result.CanAdvance = false
		result.Reason = "already_at_final_stage"
		m.mu.Unlock()
		return result, nil
	}

	state.CurrentStage++
	state.UpdatedAt = time.Now().UTC()
	canaryID := state.CanaryID
	status := state.Status
	result.CanAdvance = true
	result.NextStage = string(next.Mode)
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, status, ""); err != nil {
			return result, fmt.Errorf("canary: persist stage advance: %w", err)
		}
	}
	return result, nil
}

func (m *StageManager) PauseCanary(ctx context.Context, state *CanaryState) error {
	m.mu.Lock()
	now := time.Now().UTC()
	state.PausedAt = &now
	state.Status = CanaryStatusPaused
	state.UpdatedAt = now
	canaryID := state.CanaryID
	status := state.Status
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, status, ""); err != nil {
			return fmt.Errorf("canary: persist pause: %w", err)
		}
	}
	return nil
}

func (m *StageManager) ResumeCanary(ctx context.Context, state *CanaryState) error {
	m.mu.Lock()
	state.PausedAt = nil
	if state.Status == CanaryStatusPaused {
		state.Status = CanaryStatusCanary
	}
	state.UpdatedAt = time.Now().UTC()
	canaryID := state.CanaryID
	status := state.Status
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, status, ""); err != nil {
			return fmt.Errorf("canary: persist resume: %w", err)
		}
	}
	return nil
}

func (m *StageManager) AbortCanary(ctx context.Context, state *CanaryState, reason string) error {
	m.mu.Lock()
	now := time.Now().UTC()
	state.Status = CanaryStatusAborting
	state.AbortReason = reason
	state.UpdatedAt = now
	canaryID := state.CanaryID
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, CanaryStatusAborting, reason); err != nil {
			return fmt.Errorf("canary: persist aborting: %w", err)
		}
	}

	m.mu.Lock()
	finishedAt := time.Now().UTC()
	state.Status = CanaryStatusAborted
	state.FinishedAt = &finishedAt
	state.UpdatedAt = finishedAt
	snapshot := *state
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, CanaryStatusAborted, reason); err != nil {
			return fmt.Errorf("canary: persist aborted: %w", err)
		}
		if err := m.repo.SaveCanaryState(ctx, snapshot); err != nil {
			return fmt.Errorf("canary: persist aborted state: %w", err)
		}
	}
	return nil
}

func (m *StageManager) CheckAutoAbort(ctx context.Context, state *CanaryState, policy *CanaryPolicy, healthEval HealthEvaluation) (*AutoAbortResult, error) {
	if healthEval.ShouldAbort {
		return &AutoAbortResult{
			ShouldAbort: true,
			Reason:      healthEval.AbortReason,
			Trigger:     "health_check",
		}, nil
	}
	return &AutoAbortResult{
		ShouldAbort: false,
	}, nil
}

func (m *StageManager) CommitCanary(ctx context.Context, state *CanaryState) error {
	m.mu.Lock()
	now := time.Now().UTC()
	state.Status = CanaryStatusCommitting
	state.UpdatedAt = now
	canaryID := state.CanaryID
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, CanaryStatusCommitting, ""); err != nil {
			return fmt.Errorf("canary: persist committing: %w", err)
		}
	}

	m.mu.Lock()
	finishedAt := time.Now().UTC()
	state.Status = CanaryStatusCompleted
	state.FinishedAt = &finishedAt
	state.UpdatedAt = finishedAt
	snapshot := *state
	m.mu.Unlock()

	if m.repo != nil {
		if err := m.repo.UpdateCanaryStatus(ctx, canaryID, CanaryStatusCompleted, ""); err != nil {
			return fmt.Errorf("canary: persist completed: %w", err)
		}
		if err := m.repo.SaveCanaryState(ctx, snapshot); err != nil {
			return fmt.Errorf("canary: persist completed state: %w", err)
		}
	}
	return nil
}

func (m *StageManager) currentStage(state *CanaryState, policy *CanaryPolicy) *CanaryStage {
	if state.CurrentStage < 0 || state.CurrentStage >= len(policy.Stages) {
		return nil
	}
	return &policy.Stages[state.CurrentStage]
}

func (m *StageManager) nextStage(state *CanaryState, policy *CanaryPolicy) *CanaryStage {
	next := state.CurrentStage + 1
	if next >= len(policy.Stages) {
		return nil
	}
	return &policy.Stages[next]
}
