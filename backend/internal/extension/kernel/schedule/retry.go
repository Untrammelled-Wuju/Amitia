package schedule

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type RetryService struct {
	store  ScheduleStore
	clock  Clock
	config ScheduleConfig
}

func NewRetryService(store ScheduleStore, clock Clock, config ScheduleConfig) *RetryService {
	if clock == nil {
		clock = NewRealClock()
	}
	return &RetryService{store: store, clock: clock, config: config}
}

type RetryDecision struct {
	ShouldRetry        bool
	Backoff            time.Duration
	AvailableAt        time.Time
	Reason             string
	ManualIntervention bool
}

func (r *RetryService) ShouldRetry(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord, errorCode string) (*RetryDecision, error) {
	if def.Target.IdempotencyMode == IdempotencyModeNonIdempotent {
		return &RetryDecision{
			ShouldRetry:        false,
			Reason:             "non_idempotent_target",
			ManualIntervention: true,
		}, nil
	}

	policy := def.RetryPolicy
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = r.config.MaxRetryAttempts
	}

	if trigger.Attempt >= policy.MaxAttempts {
		return &RetryDecision{
			ShouldRetry: false,
			Reason:      "max_attempts_exceeded",
		}, nil
	}

	permanentErrorCodes := map[string]bool{
		ErrCodePermissionDenied:       true,
		ErrCodeScopeDenied:            true,
		ErrCodeDefinitionHashMismatch: true,
		ErrCodeGenerationMismatch:     true,
		ErrCodeQuarantined:            true,
		ErrCodeInvalidStateTransition: true,
		"permission_denied":           true,
		"scope_denied":                true,
		"invalid_input":               true,
	}
	if permanentErrorCodes[errorCode] {
		return &RetryDecision{
			ShouldRetry:        false,
			Reason:             "permanent_error",
			ManualIntervention: true,
		}, nil
	}

	if len(policy.RetryableErrorCodes) > 0 {
		retryable := false
		for _, code := range policy.RetryableErrorCodes {
			if code == errorCode || code == "*" {
				retryable = true
				break
			}
		}
		if !retryable {
			return &RetryDecision{
				ShouldRetry: false,
				Reason:      "error_not_retryable",
			}, nil
		}
	}

	state, err := r.store.GetState(ctx, def.ScheduleID)
	if err != nil || state == nil {
		return &RetryDecision{ShouldRetry: false, Reason: "state_not_found"}, nil
	}
	if !state.Enabled || state.Paused {
		return &RetryDecision{ShouldRetry: false, Reason: "schedule_not_active"}, nil
	}

	if def.EndAt != nil && r.clock.Now().After(*def.EndAt) {
		return &RetryDecision{ShouldRetry: false, Reason: "past_end_at"}, nil
	}

	backoff := r.CalculateBackoff(def, trigger.Attempt+1)
	now := r.clock.Now()

	return &RetryDecision{
		ShouldRetry: true,
		Backoff:     backoff,
		AvailableAt: now.Add(backoff),
		Reason:      "retry_scheduled",
	}, nil
}

func (r *RetryService) CalculateBackoff(def *ScheduleContributionDefinition, attempt int) time.Duration {
	policy := def.RetryPolicy
	if policy.InitialBackoff <= 0 {
		policy.InitialBackoff = 5 * time.Second
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 5 * time.Minute
	}
	if policy.Multiplier <= 0 {
		policy.Multiplier = 2.0
	}

	backoff := policy.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * policy.Multiplier)
		if backoff > policy.MaxBackoff {
			backoff = policy.MaxBackoff
			break
		}
	}

	if policy.Jitter > 0 {
		jitterRange := float64(backoff) * policy.Jitter
		offset := time.Duration(rand.Float64()*jitterRange*2 - jitterRange)
		backoff += offset
		if backoff < 0 {
			backoff = policy.InitialBackoff
		}
	}

	return backoff
}

func (r *RetryService) ScheduleRetry(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord, errorCode, errorMessage string) (*ScheduleRetryRecord, error) {
	decision, err := r.ShouldRetry(ctx, def, trigger, errorCode)
	if err != nil {
		return nil, err
	}
	if !decision.ShouldRetry {
		return nil, fmt.Errorf("retry not allowed: %s", decision.Reason)
	}

	now := r.clock.Now()
	policy := def.RetryPolicy
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = r.config.MaxRetryAttempts
	}

	record := &ScheduleRetryRecord{
		RetryID:     "retry-" + uuid.NewString(),
		TriggerID:   trigger.TriggerID,
		ScheduleID:  def.ScheduleID,
		Attempt:     trigger.Attempt + 1,
		MaxAttempts: policy.MaxAttempts,
		ErrorCode:   errorCode,
		Backoff:     decision.Backoff,
		AvailableAt: decision.AvailableAt.UTC(),
		CreatedAt:   now.UTC(),
	}

	if err := r.store.PutRetry(ctx, record); err != nil {
		return nil, err
	}

	updates := map[string]any{
		"status":        RunStatusRetryWait,
		"attempt":       trigger.Attempt + 1,
		"error_code":    errorCode,
		"error_message": errorMessage,
	}
	_ = r.store.UpdateTriggerStatus(ctx, trigger.TriggerID, RunStatusRetryWait, updates)

	return record, nil
}

func (r *RetryService) ProcessDueRetries(ctx context.Context, executor *ScheduleExecutor) error {
	now := r.clock.Now()
	retries, err := r.store.ListDueRetries(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, retry := range retries {
		if retry == nil {
			continue
		}
		trigger, err := r.store.GetTrigger(ctx, retry.TriggerID)
		if err != nil || trigger == nil {
			_ = r.store.DeleteRetry(ctx, retry.RetryID)
			continue
		}
		if executor != nil {
			_, _ = executor.Execute(ctx, trigger)
		}
		_ = r.store.DeleteRetry(ctx, retry.RetryID)
	}
	return nil
}
