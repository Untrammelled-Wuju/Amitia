// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

type EntityType string

const (
	EntityGenerationTask   EntityType = "generation_task"
	EntityGenerationAction EntityType = "generation_action"
	EntityGenerationFrame  EntityType = "generation_frame"
	EntityProcessingTask   EntityType = "processing_task"
	EntityProcessingAction EntityType = "processing_action"
	EntityProcessedFrame   EntityType = "processed_frame"
	EntityPackage          EntityType = "package"
)

func (e EntityType) String() string { return string(e) }

func (e EntityType) IsGeneration() bool {
	switch e {
	case EntityGenerationTask, EntityGenerationAction, EntityGenerationFrame:
		return true
	default:
		return false
	}
}

func (e EntityType) IsProcessing() bool {
	switch e {
	case EntityProcessingTask, EntityProcessingAction, EntityProcessedFrame:
		return true
	default:
		return false
	}
}

func (e EntityType) SupportsPartialSuccess() bool {
	switch e {
	case EntityGenerationTask, EntityGenerationAction, EntityProcessingTask:
		return true
	default:
		return false
	}
}

type ActorType string

const (
	ActorService      ActorType = "service"
	ActorWorker       ActorType = "worker"
	ActorRecovery     ActorType = "recovery"
	ActorMigration    ActorType = "migration"
	ActorOperator     ActorType = "operator"
	ActorFinalizer    ActorType = "finalizer"
	ActorRetryService ActorType = "retry_service"
)

type TransitionReason string

const (
	ReasonUserSubmit     TransitionReason = "user.submit"
	ReasonUserCancel     TransitionReason = "user.cancel"
	ReasonUserRetry      TransitionReason = "user.retry"
	ReasonUserRestart    TransitionReason = "user.restart"
	ReasonUserRegenerate TransitionReason = "user.regenerate"

	ReasonWorkerClaim         TransitionReason = "worker.claim"
	ReasonWorkerStageAdvanced TransitionReason = "worker.stage_advanced"
	ReasonWorkerCompleted     TransitionReason = "worker.completed"
	ReasonWorkerActionFailed  TransitionReason = "worker.action_failed"
	ReasonWorkerFrameFailed   TransitionReason = "worker.frame_failed"
	ReasonWorkerLeaseLost     TransitionReason = "worker.lease_lost"

	ReasonSystemServiceShutdown    TransitionReason = "system.service_shutdown"
	ReasonSystemLeaseExpired       TransitionReason = "system.lease_expired"
	ReasonSystemRecovered          TransitionReason = "system.recovered"
	ReasonSystemPersistenceFailed  TransitionReason = "system.persistence_failed"
	ReasonSystemPackageBuildFailed TransitionReason = "system.package_build_failed"
	ReasonSystemManifestInvalid    TransitionReason = "system.manifest_invalid"
	ReasonSystemHashFailed         TransitionReason = "system.hash_failed"
	ReasonSystemArtifactMissing    TransitionReason = "system.artifact_missing"
	ReasonSystemTransitionConflict TransitionReason = "system.transition_conflict"
	ReasonSystemUnknownState       TransitionReason = "system.unknown_state"
	ReasonSystemDependencyFailed   TransitionReason = "system.dependency_failed"
)

const (
	ReasonGenerationTaskSubmit            TransitionReason = "generation_task.submit"
	ReasonGenerationTaskClaim             TransitionReason = "generation_task.claim"
	ReasonGenerationTaskCancelBeforeClaim TransitionReason = "generation_task.cancel_before_claim"
	ReasonGenerationTaskCancelRequested   TransitionReason = "generation_task.cancel_requested"
	ReasonGenerationTaskFinalizeSuccess   TransitionReason = "generation_task.finalize_success"
	ReasonGenerationTaskFinalizePartial   TransitionReason = "generation_task.finalize_partial"
	ReasonGenerationTaskFinalizeFailure   TransitionReason = "generation_task.finalize_failure"
	ReasonGenerationTaskCancelConverged   TransitionReason = "generation_task.cancel_converged"
	ReasonGenerationTaskRetry             TransitionReason = "generation_task.retry"
	ReasonGenerationTaskRetryFailedSubset TransitionReason = "generation_task.retry_failed_subset"
	ReasonGenerationTaskRestart           TransitionReason = "generation_task.restart"
	ReasonGenerationTaskRegenerate        TransitionReason = "generation_task.regenerate"

	ReasonGenerationActionSubmit            TransitionReason = "generation_action.submit"
	ReasonGenerationActionClaim             TransitionReason = "generation_action.claim"
	ReasonGenerationActionCancelBeforeClaim TransitionReason = "generation_action.cancel_before_claim"
	ReasonGenerationActionCancelRequested   TransitionReason = "generation_action.cancel_requested"
	ReasonGenerationActionFinalizeSuccess   TransitionReason = "generation_action.finalize_success"
	ReasonGenerationActionFinalizePartial   TransitionReason = "generation_action.finalize_partial"
	ReasonGenerationActionFinalizeFailure   TransitionReason = "generation_action.finalize_failure"
	ReasonGenerationActionCancelConverged   TransitionReason = "generation_action.cancel_converged"
	ReasonGenerationActionRetry             TransitionReason = "generation_action.retry"
	ReasonGenerationActionRetryFailedSubset TransitionReason = "generation_action.retry_failed_subset"
	ReasonGenerationActionRestart           TransitionReason = "generation_action.restart"
	ReasonGenerationActionRegenerate        TransitionReason = "generation_action.regenerate"

	ReasonGenerationFrameSubmit            TransitionReason = "generation_frame.submit"
	ReasonGenerationFrameClaim             TransitionReason = "generation_frame.claim"
	ReasonGenerationFrameCancelBeforeClaim TransitionReason = "generation_frame.cancel_before_claim"
	ReasonGenerationFrameCancelRequested   TransitionReason = "generation_frame.cancel_requested"
	ReasonGenerationFrameFinalizeSuccess   TransitionReason = "generation_frame.finalize_success"
	ReasonGenerationFrameFinalizeFailure   TransitionReason = "generation_frame.finalize_failure"
	ReasonGenerationFrameCancelConverged   TransitionReason = "generation_frame.cancel_converged"
	ReasonGenerationFrameRetry             TransitionReason = "generation_frame.retry"
	ReasonGenerationFrameRestart           TransitionReason = "generation_frame.restart"
	ReasonGenerationFrameRegenerate        TransitionReason = "generation_frame.regenerate"

	ReasonProcessingTaskSubmit            TransitionReason = "processing_task.submit"
	ReasonProcessingTaskClaim             TransitionReason = "processing_task.claim"
	ReasonProcessingTaskCancelBeforeClaim TransitionReason = "processing_task.cancel_before_claim"
	ReasonProcessingTaskCancelRequested   TransitionReason = "processing_task.cancel_requested"
	ReasonProcessingTaskFinalizeSuccess   TransitionReason = "processing_task.finalize_success"
	ReasonProcessingTaskFinalizePartial   TransitionReason = "processing_task.finalize_partial"
	ReasonProcessingTaskFinalizeFailure   TransitionReason = "processing_task.finalize_failure"
	ReasonProcessingTaskCancelConverged   TransitionReason = "processing_task.cancel_converged"
	ReasonProcessingTaskRetry             TransitionReason = "processing_task.retry"
	ReasonProcessingTaskRetryFailedSubset TransitionReason = "processing_task.retry_failed_subset"
	ReasonProcessingTaskRestart           TransitionReason = "processing_task.restart"
	ReasonProcessingTaskRegenerate        TransitionReason = "processing_task.regenerate"

	ReasonProcessingActionSubmit            TransitionReason = "processing_action.submit"
	ReasonProcessingActionClaim             TransitionReason = "processing_action.claim"
	ReasonProcessingActionCancelBeforeClaim TransitionReason = "processing_action.cancel_before_claim"
	ReasonProcessingActionCancelRequested   TransitionReason = "processing_action.cancel_requested"
	ReasonProcessingActionFinalizeSuccess   TransitionReason = "processing_action.finalize_success"
	ReasonProcessingActionFinalizeFailure   TransitionReason = "processing_action.finalize_failure"
	ReasonProcessingActionCancelConverged   TransitionReason = "processing_action.cancel_converged"
	ReasonProcessingActionRetry             TransitionReason = "processing_action.retry"
	ReasonProcessingActionRestart           TransitionReason = "processing_action.restart"
	ReasonProcessingActionRegenerate        TransitionReason = "processing_action.regenerate"

	ReasonProcessedFrameSubmit            TransitionReason = "processed_frame.submit"
	ReasonProcessedFrameClaim             TransitionReason = "processed_frame.claim"
	ReasonProcessedFrameCancelBeforeClaim TransitionReason = "processed_frame.cancel_before_claim"
	ReasonProcessedFrameCancelRequested   TransitionReason = "processed_frame.cancel_requested"
	ReasonProcessedFrameFinalizeSuccess   TransitionReason = "processed_frame.finalize_success"
	ReasonProcessedFrameFinalizeFailure   TransitionReason = "processed_frame.finalize_failure"
	ReasonProcessedFrameCancelConverged   TransitionReason = "processed_frame.cancel_converged"
	ReasonProcessedFrameRetry             TransitionReason = "processed_frame.retry"
	ReasonProcessedFrameRestart           TransitionReason = "processed_frame.restart"
	ReasonProcessedFrameRegenerate        TransitionReason = "processed_frame.regenerate"

	ReasonPackageFinalizeSuccess TransitionReason = "package.finalize_success"
	ReasonPackageFinalizeFailure TransitionReason = "package.finalize_failure"
)

func (r TransitionReason) String() string { return string(r) }

type StopCause string

const (
	StopCauseUserCancel    StopCause = "user_cancel"
	StopCauseServiceStop   StopCause = "service_stop"
	StopCauseLeaseLost     StopCause = "lease_lost"
	StopCauseParentFailed  StopCause = "parent_failed"
	StopCauseExecutionSwap StopCause = "execution_replaced"
)

func (s StopCause) String() string { return string(s) }
