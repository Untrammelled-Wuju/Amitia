package schedule

import "errors"

var (
	ErrScheduleNotFound          = errors.New("schedule: not found")
	ErrScheduleAlreadyExists     = errors.New("schedule: already exists")
	ErrTriggerNotFound           = errors.New("schedule: trigger not found")
	ErrInvalidCronExpression     = errors.New("schedule: invalid cron expression")
	ErrInvalidInterval           = errors.New("schedule: invalid interval")
	ErrInvalidOneShotTime        = errors.New("schedule: invalid one-shot time")
	ErrInvalidTimezone           = errors.New("schedule: invalid timezone")
	ErrInvalidTriggerType        = errors.New("schedule: invalid trigger type")
	ErrInvalidTargetType         = errors.New("schedule: invalid target type")
	ErrFrequencyTooHigh          = errors.New("schedule: frequency below minimum (1 minute)")
	ErrIntervalTooLarge          = errors.New("schedule: interval exceeds maximum (365 days)")
	ErrLeaseAcquisitionFailed    = errors.New("schedule: lease acquisition failed")
	ErrLeaseExpired              = errors.New("schedule: lease expired")
	ErrIdempotencyConflict       = errors.New("schedule: idempotency conflict")
	ErrOverlapForbidden          = errors.New("schedule: overlap forbidden")
	ErrCircuitOpen               = errors.New("schedule: circuit breaker open")
	ErrScheduleBlocked           = errors.New("schedule: blocked by dependency or permission")
	ErrScheduleQuarantined       = errors.New("schedule: quarantined")
	ErrPermissionDenied          = errors.New("schedule: permission denied")
	ErrScopeDenied               = errors.New("schedule: scope denied")
	ErrDependencyMissing         = errors.New("schedule: dependency missing")
	ErrScheduleNotEnabled        = errors.New("schedule: not enabled")
	ErrSchedulePaused            = errors.New("schedule: paused")
	ErrScheduleExpired           = errors.New("schedule: expired")
	ErrInvalidStateTransition    = errors.New("schedule: invalid state transition")
	ErrMaxRetriesExceeded        = errors.New("schedule: max retries exceeded")
	ErrNonIdempotentUnknownResult = errors.New("schedule: non-idempotent target returned unknown result")
	ErrRuntimeHandlerNotFound    = errors.New("schedule: runtime handler not found")
	ErrTargetExecutionFailed     = errors.New("schedule: target execution failed")
	ErrDefinitionHashMismatch    = errors.New("schedule: definition hash mismatch")
	ErrGenerationMismatch        = errors.New("schedule: generation mismatch")
	ErrSchedulerClosed           = errors.New("schedule: scheduler closed")
	ErrInvalidDefinitionHash     = errors.New("schedule: invalid definition hash")
)

type ScheduleError struct {
	Code    string
	Message string
	Err     error
}

func (e *ScheduleError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ScheduleError) Unwrap() error {
	return e.Err
}

func NewScheduleError(code, message string, err error) *ScheduleError {
	return &ScheduleError{Code: code, Message: message, Err: err}
}

const (
	ErrCodeCronParseFailed        = "SCHEDULE_CRON_PARSE_FAILED"
	ErrCodeIntervalInvalid        = "SCHEDULE_INTERVAL_INVALID"
	ErrCodeOneShotTimeInvalid     = "SCHEDULE_ONESHOT_TIME_INVALID"
	ErrCodeTimezoneInvalid        = "SCHEDULE_TIMEZONE_INVALID"
	ErrCodeFrequencyTooHigh       = "SCHEDULE_FREQUENCY_TOO_HIGH"
	ErrCodeLeaseFailed            = "SCHEDULE_LEASE_FAILED"
	ErrCodeIdempotencyConflict    = "SCHEDULE_IDEMPOTENCY_CONFLICT"
	ErrCodeOverlapForbidden       = "SCHEDULE_OVERLAP_FORBIDDEN"
	ErrCodeCircuitOpen            = "SCHEDULE_CIRCUIT_OPEN"
	ErrCodePermissionDenied       = "SCHEDULE_PERMISSION_DENIED"
	ErrCodeScopeDenied            = "SCHEDULE_SCOPE_DENIED"
	ErrCodeDependencyMissing      = "SCHEDULE_DEPENDENCY_MISSING"
	ErrCodeTargetNotFound         = "SCHEDULE_TARGET_NOT_FOUND"
	ErrCodeTargetExecutionFailed  = "SCHEDULE_TARGET_EXECUTION_FAILED"
	ErrCodeNonIdempotentUnknown   = "SCHEDULE_NON_IDEMPOTENT_UNKNOWN"
	ErrCodeMaxRetriesExceeded     = "SCHEDULE_MAX_RETRIES_EXCEEDED"
	ErrCodeGenerationMismatch     = "SCHEDULE_GENERATION_MISMATCH"
	ErrCodeDefinitionHashMismatch = "SCHEDULE_DEFINITION_HASH_MISMATCH"
	ErrCodeQuarantined            = "SCHEDULE_QUARANTINED"
	ErrCodeInvalidStateTransition = "SCHEDULE_INVALID_STATE_TRANSITION"
	ErrCodeRuntimeHandlerMissing  = "SCHEDULE_RUNTIME_HANDLER_MISSING"
)
