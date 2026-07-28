package workflow

import "errors"

var (
	ErrWorkflowNotFound    = errors.New("workflow: not found")
	ErrCycleDetected       = errors.New("workflow: cycle detected")
	ErrInvalidNodeID       = errors.New("workflow: invalid node id")
	ErrDuplicateNodeID     = errors.New("workflow: duplicate node id")
	ErrHandlerNotFound     = errors.New("workflow: handler not found")
	ErrExecutionTimeout    = errors.New("workflow: execution timeout")
	ErrStepTimeout         = errors.New("workflow: step timeout")
	ErrMaxStepsExceeded    = errors.New("workflow: max steps exceeded")
	ErrOutputLimitExceeded = errors.New("workflow: output limit exceeded")
	ErrWorkflowDisabled    = errors.New("workflow: disabled")
	ErrCheckpointNotFound  = errors.New("workflow: checkpoint not found")
	ErrCompensationFailed  = errors.New("workflow: compensation failed")
	ErrPermissionDenied    = errors.New("workflow: permission denied")
	ErrScopeDenied         = errors.New("workflow: scope denied")
	ErrGenerationMismatch  = errors.New("workflow: generation mismatch")
	ErrDepthExceeded       = errors.New("workflow: depth exceeded")
	ErrConcurrencyExceeded = errors.New("workflow: concurrency exceeded")
	ErrWorkflowRunNotFound = errors.New("workflow: run not found")
)
