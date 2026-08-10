package execution

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CircuitResultClassifier interface {
	Classify(result capability.UnifiedToolResult, dispatched bool) CircuitOutcome
}

type DefaultCircuitResultClassifier struct{}

func NewCircuitResultClassifier() CircuitResultClassifier {
	return &DefaultCircuitResultClassifier{}
}

func (c *DefaultCircuitResultClassifier) Classify(result capability.UnifiedToolResult, dispatched bool) CircuitOutcome {
	if result.Status == capability.ToolResultStatusSuccess {
		return CircuitOutcomeSuccess
	}

	if result.Error == nil {
		return CircuitOutcomeNeutral
	}

	code := result.Error.Code
	category := string(result.Error.Category)

	switch code {
	case capability.ErrorCodeRuntimeUnavailable,
		capability.ErrorCodeConnectionLost:
		if dispatched {
			return CircuitOutcomeFailure
		}
		return CircuitOutcomeNeutral

	case capability.ErrorCodeExecutionFailed:
		if dispatched && category == string(capability.ToolErrorCategoryRuntime) && result.Error.Retryable {
			return CircuitOutcomeFailure
		}
		return CircuitOutcomeNeutral

	case capability.ErrorCodeTimeout:
		if dispatched {
			return CircuitOutcomeFailure
		}
		return CircuitOutcomeNeutral

	case capability.ErrorCodePermissionDenied,
		capability.ErrorCodeScopeDenied,
		capability.ErrorCodeInvalidInput,
		capability.ErrorCodeNotAvailable,
		capability.ErrorCodeRateLimited,
		capability.ErrorCodeConflict,
		capability.ErrorCodeDependencyMissing,
		capability.ErrorCodeCancelled,
		capability.ErrorCodeInvalidResult,
		capability.ErrorCodeInternalError,
		capability.ErrorCodeStreamProtocol,
		capability.ErrorCodeStreamLimitExceeded,
		capability.ErrorCodeStreamDeliveryFailed,
		capability.ErrorCodeResourceLimitInvalid,
		capability.ErrorCodeResourceLimitUnavailable,
		capability.ErrorCodeResourceLimitExceeded,
		capability.ErrorCodeResourceUsageUnavailable:
		return CircuitOutcomeNeutral
	}

	return CircuitOutcomeNeutral
}
