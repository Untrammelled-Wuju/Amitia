package task_runtime

import "errors"

type TaskErrorCode string

const (
	ErrTaskPauseUnsupported              TaskErrorCode = "task_pause_unsupported"
	ErrTaskPauseTimeout                  TaskErrorCode = "task_pause_timeout"
	ErrTaskPauseInProgress               TaskErrorCode = "task_pause_in_progress"
	ErrTaskNotPaused                     TaskErrorCode = "task_not_paused"
	ErrTaskResumeIncompatible            TaskErrorCode = "task_resume_incompatible"
	ErrTaskResumeStaleGeneration         TaskErrorCode = "task_resume_stale_generation"
	ErrTaskResumeFailed                  TaskErrorCode = "task_resume_failed"
	ErrTaskDefinitionInvalid             TaskErrorCode = "task_definition_invalid"
	ErrTaskInputInvalid                  TaskErrorCode = "task_input_invalid"
	ErrTaskNotEnabled                    TaskErrorCode = "task_not_enabled"
	ErrTaskPermissionDenied              TaskErrorCode = "task_permission_denied"
	ErrTaskScopeDenied                   TaskErrorCode = "task_scope_denied"
	ErrTaskDependencyUnavailable         TaskErrorCode = "task_dependency_unavailable"
	ErrTaskQueueFull                     TaskErrorCode = "task_queue_full"
	ErrTaskRuntimeStartFailed            TaskErrorCode = "task_runtime_start_failed"
	ErrTaskRuntimeCrashed                TaskErrorCode = "task_runtime_crashed"
	ErrTaskCancelled                     TaskErrorCode = "task_cancelled"
	ErrTaskTimedOut                      TaskErrorCode = "task_timed_out"
	ErrTaskCheckpointInvalid             TaskErrorCode = "task_checkpoint_invalid"
	ErrTaskCheckpointIncompatible        TaskErrorCode = "task_checkpoint_incompatible"
	ErrTaskResultInvalid                 TaskErrorCode = "task_result_invalid"
	ErrTaskResultTooLarge                TaskErrorCode = "task_result_too_large"
	ErrTaskArtifactFailed                TaskErrorCode = "task_artifact_failed"
	ErrTaskNonIdempotentUnknown          TaskErrorCode = "task_non_idempotent_unknown"
	ErrTaskRecoveryRequired              TaskErrorCode = "task_recovery_required"
	ErrTaskManualIntervention            TaskErrorCode = "task_manual_intervention"
	ErrTaskStateTransitionInvalid        TaskErrorCode = "task_state_transition_invalid"
	ErrTaskNotFound                      TaskErrorCode = "task_not_found"
	ErrTaskAlreadyExists                 TaskErrorCode = "task_already_exists"
	ErrTaskNotCancelable                 TaskErrorCode = "task_not_cancelable"
	ErrTaskNotRetryable                  TaskErrorCode = "task_not_retryable"
	ErrTaskExecutionPlacementInvalid     TaskErrorCode = "task_execution_placement_invalid"
	ErrTaskExecutionTargetInvalid        TaskErrorCode = "task_execution_target_invalid"
	ErrTaskExecutionTargetUnresolved     TaskErrorCode = "task_execution_target_unresolved"
	ErrTaskExecutionTargetConflict       TaskErrorCode = "task_execution_target_conflict"
	ErrTaskProviderBindingInvalid        TaskErrorCode = "task_provider_binding_invalid"
	ErrTaskDeviceBindingInvalid          TaskErrorCode = "task_device_binding_invalid"
	ErrTaskRuntimeBindingInvalid         TaskErrorCode = "task_runtime_binding_invalid"
	ErrTaskRuntimeSessionBindingInvalid  TaskErrorCode = "task_runtime_session_binding_invalid"
	ErrRemoteTaskExecutorUnavailable     TaskErrorCode = "remote_task_executor_unavailable"
	ErrTaskExecutionAttemptInvalid       TaskErrorCode = "task_execution_attempt_invalid"
	ErrTaskExecutionUnsupported          TaskErrorCode = "task_execution_unsupported"
)

type TaskError struct {
	Code    TaskErrorCode
	Message string
	Cause   error
}

func (e *TaskError) Error() string {
	if e.Cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *TaskError) Unwrap() error { return e.Cause }

func NewTaskError(code TaskErrorCode, message string) *TaskError {
	return &TaskError{Code: code, Message: message}
}

func WrapTaskError(code TaskErrorCode, message string, cause error) *TaskError {
	return &TaskError{Code: code, Message: message, Cause: cause}
}

func IsTaskErrorCode(err error, code TaskErrorCode) bool {
	var te *TaskError
	if errors.As(err, &te) {
		return te.Code == code
	}
	return false
}

func IsRetryableErrorCode(code TaskErrorCode) bool {
	switch code {
	case ErrTaskRuntimeCrashed, ErrTaskRuntimeStartFailed:
		return true
	}
	return false
}

func HTTPStatusForErrorCode(code TaskErrorCode) int {
	switch code {
	case ErrTaskNotFound:
		return 404
	case ErrTaskAlreadyExists:
		return 409
	case ErrTaskPermissionDenied, ErrTaskScopeDenied:
		return 403
	case ErrTaskDefinitionInvalid, ErrTaskInputInvalid,
		ErrTaskCheckpointInvalid, ErrTaskCheckpointIncompatible,
		ErrTaskResultInvalid, ErrTaskResultTooLarge:
		return 400
	case ErrTaskQueueFull:
		return 429
	case ErrTaskNotEnabled, ErrTaskDependencyUnavailable:
		return 503
	case ErrTaskManualIntervention:
		return 422
	case ErrTaskExecutionPlacementInvalid, ErrTaskExecutionTargetInvalid,
		ErrTaskProviderBindingInvalid, ErrTaskDeviceBindingInvalid,
		ErrTaskRuntimeBindingInvalid, ErrTaskRuntimeSessionBindingInvalid,
		ErrTaskExecutionTargetConflict, ErrTaskExecutionAttemptInvalid:
		return 400
	case ErrTaskExecutionTargetUnresolved:
		return 422
	case ErrTaskExecutionUnsupported, ErrRemoteTaskExecutorUnavailable:
		return 503
	case ErrTaskPauseUnsupported, ErrTaskNotPaused:
		return 409
	case ErrTaskPauseInProgress:
		return 409
	default:
		return 500
	}
}
