package hook

import "errors"

type HookErrorCode string

const (
	ErrCodeHookPointNotFound       HookErrorCode = "hook_point_not_found"
	ErrCodeHookPointExists         HookErrorCode = "hook_point_exists"
	ErrCodeContractVersionMismatch HookErrorCode = "contract_version_mismatch"
	ErrCodePhaseNotSupported       HookErrorCode = "phase_not_supported"
	ErrCodePriorityOutOfRange      HookErrorCode = "priority_out_of_range"
	ErrCodeMutationClaimDenied     HookErrorCode = "mutation_claim_denied"
	ErrCodeTimeoutExceedsMax       HookErrorCode = "timeout_exceeds_max"
	ErrCodeFailurePolicyTooLoose   HookErrorCode = "failure_policy_too_loose"
	ErrCodeEntryNotDeclared        HookErrorCode = "entry_not_declared"
	ErrCodeRuntimeNotReady         HookErrorCode = "runtime_not_ready"
	ErrCodeContributionDisabled    HookErrorCode = "contribution_disabled"
	ErrCodeHookRecursionDetected   HookErrorCode = "hook_recursion_detected"
	ErrCodeHookDepthExceeded       HookErrorCode = "hook_depth_exceeded"
	ErrCodeHookTimeout             HookErrorCode = "hook_timeout"
	ErrCodeHookCancelled           HookErrorCode = "hook_cancelled"
	ErrCodeHookRuntimeError        HookErrorCode = "hook_runtime_error"
	ErrCodeHookResultInvalid       HookErrorCode = "hook_result_invalid"
	ErrCodeHookMutationConflict    HookErrorCode = "hook_mutation_conflict"
	ErrCodeHookPayloadTooLarge     HookErrorCode = "hook_payload_too_large"
	ErrCodeHookResultTooLarge      HookErrorCode = "hook_result_too_large"
	ErrCodeHookTooManyOps          HookErrorCode = "hook_too_many_operations"
	ErrCodeHookPathTooLong         HookErrorCode = "hook_path_too_long"
	ErrCodeHookPathNotWhitelisted  HookErrorCode = "hook_path_not_whitelisted"
	ErrCodeHookSensitiveField      HookErrorCode = "hook_sensitive_field"
	ErrCodeHookBusinessInvariant   HookErrorCode = "hook_business_invariant"
	ErrCodePermissionDenied        HookErrorCode = "hook_permission_denied"
	ErrCodeScopeDenied             HookErrorCode = "hook_scope_denied"
	ErrCodeDependencyUnavailable   HookErrorCode = "dependency_unavailable"
	ErrCodeCircuitOpen             HookErrorCode = "circuit_open"
	ErrCodeHookNotFound            HookErrorCode = "hook_not_found"
	ErrCodeInvalidDecision         HookErrorCode = "invalid_decision_for_phase"
	ErrCodeMaxHandlersExceeded     HookErrorCode = "max_handlers_exceeded"
)

type HookError struct {
	Code    HookErrorCode
	Message string
	Cause   error
}

func (e *HookError) Error() string {
	if e.Cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *HookError) Unwrap() error {
	return e.Cause
}

func NewHookError(code HookErrorCode, message string) *HookError {
	return &HookError{Code: code, Message: message}
}

func WrapHookError(code HookErrorCode, message string, cause error) *HookError {
	return &HookError{Code: code, Message: message, Cause: cause}
}

var (
	ErrPointNotFound       = errors.New("hook: point not found")
	ErrPointExists         = errors.New("hook: point already exists")
	ErrContributionExists  = errors.New("hook: contribution already exists")
	ErrContributionMissing = errors.New("hook: contribution not found")
	ErrMaxHandlers         = errors.New("hook: max handlers exceeded")
	ErrRecursion           = errors.New("hook: recursion detected")
	ErrDepthExceeded       = errors.New("hook: depth exceeded")
	ErrCircuitOpen         = errors.New("hook: circuit open")
)
