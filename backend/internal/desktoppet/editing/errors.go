package editing

import (
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet"
)

const (
	ErrCodeEditTaskNotFound              = "EDIT_TASK_NOT_FOUND"
	ErrCodeEditActionNotFound            = "EDIT_ACTION_NOT_FOUND"
	ErrCodeEditRevisionNotFound          = "EDIT_REVISION_NOT_FOUND"
	ErrCodeEditRevisionNotReady          = "EDIT_REVISION_NOT_READY"
	ErrCodeEditSessionNotFound           = "EDIT_SESSION_NOT_FOUND"
	ErrCodeEditSessionExpired            = "EDIT_SESSION_EXPIRED"
	ErrCodeEditSessionVersionConflict    = "EDIT_SESSION_VERSION_CONFLICT"
	ErrCodeEditSessionAlreadyCommitted   = "EDIT_SESSION_ALREADY_COMMITTED"
	ErrCodeEditSessionStale              = "EDIT_SESSION_STALE"
	ErrCodeEditOperationInvalid          = "EDIT_OPERATION_INVALID"
	ErrCodeEditOperationNotReversible    = "EDIT_OPERATION_NOT_REVERSIBLE"
	ErrCodeEditFrameNotFound             = "EDIT_FRAME_NOT_FOUND"
	ErrCodeEditFrameCountInvalid         = "EDIT_FRAME_COUNT_INVALID"
	ErrCodeEditFrameDurationInvalid      = "EDIT_FRAME_DURATION_INVALID"
	ErrCodeEditAssetNotFound             = "EDIT_ASSET_NOT_FOUND"
	ErrCodeEditAssetHashMismatch         = "EDIT_ASSET_HASH_MISMATCH"
	ErrCodeEditUploadInvalid             = "EDIT_UPLOAD_INVALID"
	ErrCodeEditUploadTooLarge            = "EDIT_UPLOAD_TOO_LARGE"
	ErrCodeEditPatchInvalid              = "EDIT_PATCH_INVALID"
	ErrCodeEditAnchorInvalid             = "EDIT_ANCHOR_INVALID"
	ErrCodeEditCandidateNotFound         = "EDIT_CANDIDATE_NOT_FOUND"
	ErrCodeEditCandidateNotReady         = "EDIT_CANDIDATE_NOT_READY"
	ErrCodeEditCandidateAlreadyDecided   = "EDIT_CANDIDATE_ALREADY_DECIDED"
	ErrCodeEditCandidateQualityNotReady  = "EDIT_CANDIDATE_QUALITY_NOT_READY"
	ErrCodeEditCandidateAcceptConflict   = "EDIT_CANDIDATE_ACCEPT_CONFLICT"
	ErrCodeEditJobNotCancelable          = "EDIT_JOB_NOT_CANCELABLE"
	ErrCodeEditProviderStatusUnknown     = "EDIT_PROVIDER_STATUS_UNKNOWN"
	ErrCodeEditCostConfirmationRequired  = "EDIT_COST_CONFIRMATION_REQUIRED"
	ErrCodeEditRevisionPublishFailed     = "EDIT_REVISION_PUBLISH_FAILED"
	ErrCodeEditQualityPending            = "EDIT_QUALITY_PENDING"
	ErrCodeEditQualityGateBlocked        = "EDIT_QUALITY_GATE_BLOCKED"
	ErrCodeEditActiveBindingConflict     = "EDIT_ACTIVE_BINDING_CONFLICT"
	ErrCodeEditPermissionDenied          = "EDIT_PERMISSION_DENIED"
	ErrCodeEditManualRebaseRequired      = "EDIT_MANUAL_REBASE_REQUIRED"
	ErrCodeEditRegenerationFailed        = "EDIT_REGENERATION_FAILED"
	ErrCodeEditProviderSubmitFailed      = "EDIT_PROVIDER_SUBMIT_FAILED"
	ErrCodeEditRevisionImmutable         = "EDIT_REVISION_IMMUTABLE"
	ErrCodeEditRevisionConflict          = "EDIT_REVISION_CONFLICT"
	ErrCodeEditBridgeFailed              = "EDIT_BRIDGE_FAILED"
	ErrCodeEditLegacyUnresolved          = "EDIT_LEGACY_UNRESOLVED"
	ErrCodeEditOwnershipDenied           = "EDIT_OWNERSHIP_DENIED"
	ErrCodeEditContentHashMismatch       = "EDIT_CONTENT_HASH_MISMATCH"
	ErrCodeActionRevisionSourceInvalid   = "ACTION_REVISION_SOURCE_INVALID"
	ErrCodeActionRevisionAnchorInvalid   = "ACTION_REVISION_ANCHOR_INVALID"
	ErrCodeActionRevisionActiveNotFound  = "ACTION_REVISION_ACTIVE_NOT_FOUND"
	ErrCodeActionRevisionBindingConflict = "ACTION_REVISION_BINDING_CONFLICT"
)

const (
	RevisionTypeProcessed    = "processed"
	RevisionTypeEdit         = "edit"
	RevisionTypeRegenerated  = "regenerated"
	RevisionTypeImported     = "imported"
	RevisionTypeRollback     = "rollback"
	RevisionTypeLegacyImport = "legacy_import"
)

const (
	RevisionStatusCommitting       = "committing"
	RevisionStatusReady            = "ready"
	RevisionStatusQualityPending   = "quality_pending"
	RevisionStatusQualityReady     = "quality_ready"
	RevisionStatusFailed           = "failed"
	RevisionStatusLegacyUnresolved = "legacy_unresolved"
	RevisionStatusArchived         = "archived"

	RevisionStatusBuilding = "committing"
)

const (
	SessionStatusOpen       = "open"
	SessionStatusCommitting = "committing"
	SessionStatusCommitted  = "committed"
	SessionStatusAbandoned  = "abandoned"
	SessionStatusExpired    = "expired"
	SessionStatusConflicted = "conflicted"
)

const (
	OperationStatusApplied    = "applied"
	OperationStatusReverted   = "reverted"
	OperationStatusFailed     = "failed"
	OperationStatusSuperseded = "superseded"
)

const (
	JobStatusCreated             = "created"
	JobStatusQueued              = "queued"
	JobStatusPreparing           = "preparing"
	JobStatusSubmitting          = "submitting"
	JobStatusUnknownSubmission   = "unknown_submission"
	JobStatusPolling             = "polling"
	JobStatusArtifactReady       = "artifact_ready"
	JobStatusProcessing          = "processing"
	JobStatusCandidateCommitting = "candidate_committing"
	JobStatusQualityPending      = "quality_pending"
	JobStatusQualityRunning      = "quality_running"
	JobStatusReadyForReview      = "ready_for_review"
	JobStatusAccepted            = "accepted"
	JobStatusRejected            = "rejected"
	JobStatusFailedRetryable     = "failed_retryable"
	JobStatusFailedTerminal      = "failed_terminal"
	JobStatusCancelRequested     = "cancel_requested"
	JobStatusCancelled           = "cancelled"

	JobStatusRunning           = "running"
	JobStatusProviderSucceeded = "provider_succeeded"
	JobStatusMaterializing     = "materializing"
	JobStatusCompleted         = "completed"
	JobStatusFailed            = "failed"
	JobStatusUnknown           = "unknown"
)

func IsTerminalJobStatus(status string) bool {
	switch status {
	case JobStatusAccepted, JobStatusRejected, JobStatusFailedTerminal, JobStatusCancelled:
		return true
	}
	return false
}

func IsRetryableJobStatus(status string) bool {
	return status == JobStatusFailedRetryable
}

const (
	JobTypeSingleFrame         = "single_frame"
	JobTypeFullAction          = "full_action"
	JobTypeBackgroundReprocess = "background_reprocess"
	JobTypeNormalizeUpload     = "normalize_upload"
)

const (
	CandidateStatusPending        = "pending"
	CandidateStatusAccepted       = "accepted"
	CandidateStatusRejected       = "rejected"
	CandidateStatusExpired        = "expired"
	CandidateStatusCommitting     = "candidate_committing"
	CandidateStatusQualityPending = "quality_pending"
	CandidateStatusQualityRunning = "quality_running"
	CandidateStatusReadyForReview = "ready_for_review"
	CandidateStatusFailed         = "failed"
	CandidateStatusArchived       = "archived"
	CandidateStatusStaleCandidate = "stale_candidate"
)

const (
	AssetStatusStaging     = "staging"
	AssetStatusReady       = "ready"
	AssetStatusQuarantined = "quarantined"
	AssetStatusDeleted     = "deleted"
)

const (
	AssetSourceProcessed   = "processed"
	AssetSourceRegenerated = "regenerated"
	AssetSourceUploaded    = "uploaded"
	AssetSourceMaskApplied = "mask_applied"
	AssetSourceLegacy      = "legacy"
)

const (
	PatchTypeErase   = "erase"
	PatchTypeRestore = "restore"
	PatchTypeSoften  = "soften"
)

const (
	AnchorSpaceNormalizedCanvas = "normalized_canvas"
	AnchorSpacePixel            = "pixel"
)

const (
	JournalActionPublish  = "publish"
	JournalActionActivate = "activate"
	JournalActionRecover  = "recover"
)

const (
	JournalStatusPending   = "pending"
	JournalStatusCompleted = "completed"
	JournalStatusFailed    = "failed"
)

const (
	ActivationPolicyAfterQualityPass = "after_quality_pass"
	ActivationPolicyManual           = "manual"
	ActivationPolicyKeepCurrent      = "keep_current"
)

const (
	MinFrameDurationMS     = 16
	MaxFrameDurationMS     = 60000
	MinFrameCount          = 1
	MaxCheckpointKeep      = 10
	SessionDefaultTTLHours = 24
	CandidateRetentionDays = 7
	SessionRetentionDays   = 7
)

type EditError struct {
	Code    string `json:"errorCode"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

func (e *EditError) Error() string {
	if e.Detail != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewEditError(code, message string) *EditError {
	return &EditError{Code: code, Message: message}
}

func NewEditErrorWithDetail(code, message string, detail any) *EditError {
	return &EditError{Code: code, Message: message, Detail: detail}
}

var (
	ErrTaskNotFound                  = NewEditError(ErrCodeEditTaskNotFound, "处理任务不存在")
	ErrActionNotFound                = NewEditError(ErrCodeEditActionNotFound, "动作不存在")
	ErrRevisionNotFound              = NewEditError(ErrCodeEditRevisionNotFound, "Revision不存在")
	ErrRevisionNotReady              = NewEditError(ErrCodeEditRevisionNotReady, "Revision尚未就绪")
	ErrSessionNotFound               = NewEditError(ErrCodeEditSessionNotFound, "编辑会话不存在")
	ErrSessionExpired                = NewEditError(ErrCodeEditSessionExpired, "编辑会话已过期")
	ErrSessionAlreadyCommitted       = NewEditError(ErrCodeEditSessionAlreadyCommitted, "编辑会话已提交")
	ErrSessionStale                  = NewEditError(ErrCodeEditSessionStale, "编辑会话基线已漂移")
	ErrOperationInvalid              = NewEditError(ErrCodeEditOperationInvalid, "操作无效")
	ErrOperationNotReversible        = NewEditError(ErrCodeEditOperationNotReversible, "操作不可逆")
	ErrFrameNotFound                 = NewEditError(ErrCodeEditFrameNotFound, "帧不存在")
	ErrFrameCountInvalid             = NewEditError(ErrCodeEditFrameCountInvalid, "帧数量无效")
	ErrFrameDurationInvalid          = NewEditError(ErrCodeEditFrameDurationInvalid, "帧时长无效")
	ErrAssetNotFound                 = NewEditError(ErrCodeEditAssetNotFound, "资产不存在")
	ErrAssetHashMismatch             = NewEditError(ErrCodeEditAssetHashMismatch, "资产哈希不匹配")
	ErrUploadInvalid                 = NewEditError(ErrCodeEditUploadInvalid, "上传文件无效")
	ErrUploadTooLarge                = NewEditError(ErrCodeEditUploadTooLarge, "上传文件过大")
	ErrPatchInvalid                  = NewEditError(ErrCodeEditPatchInvalid, "Patch无效")
	ErrAnchorInvalid                 = NewEditError(ErrCodeEditAnchorInvalid, "锚点无效")
	ErrCandidateNotFound             = NewEditError(ErrCodeEditCandidateNotFound, "候选不存在")
	ErrCandidateNotReady             = NewEditError(ErrCodeEditCandidateNotReady, "候选尚未就绪")
	ErrCandidateAlreadyDecided       = NewEditError(ErrCodeEditCandidateAlreadyDecided, "候选已被处理")
	ErrCandidateQualityNotReady      = NewEditError(ErrCodeEditCandidateQualityNotReady, "候选质量评估未完成")
	ErrCandidateAcceptConflict       = NewEditError(ErrCodeEditCandidateAcceptConflict, "候选接受冲突，已被其他候选抢先")
	ErrJobNotCancelable              = NewEditError(ErrCodeEditJobNotCancelable, "Job不可取消")
	ErrProviderStatusUnknown         = NewEditError(ErrCodeEditProviderStatusUnknown, "Provider状态未知")
	ErrCostConfirmationRequired      = NewEditError(ErrCodeEditCostConfirmationRequired, "需要成本确认")
	ErrRevisionPublishFailed         = NewEditError(ErrCodeEditRevisionPublishFailed, "Revision发布失败")
	ErrQualityPending                = NewEditError(ErrCodeEditQualityPending, "质量评估进行中")
	ErrQualityGateBlocked            = NewEditError(ErrCodeEditQualityGateBlocked, "质量门禁阻止")
	ErrActiveBindingConflict         = NewEditError(ErrCodeEditActiveBindingConflict, "Active绑定冲突")
	ErrPermissionDenied              = NewEditError(ErrCodeEditPermissionDenied, "权限不足")
	ErrManualRebaseRequired          = NewEditError(ErrCodeEditManualRebaseRequired, "需要手动Rebase")
	ErrRegenerationFailed            = NewEditError(ErrCodeEditRegenerationFailed, "重生成失败")
	ErrProviderSubmitFailed          = NewEditError(ErrCodeEditProviderSubmitFailed, "Provider提交失败")
	ErrRevisionImmutable             = NewEditError(ErrCodeEditRevisionImmutable, "Revision内容不可变")
	ErrRevisionConflict              = NewEditError(ErrCodeEditRevisionConflict, "Revision冲突")
	ErrBridgeFailed                  = NewEditError(ErrCodeEditBridgeFailed, "Revision桥接失败")
	ErrLegacyUnresolved              = NewEditError(ErrCodeEditLegacyUnresolved, "Legacy数据无法解析")
	ErrOwnershipDenied               = NewEditError(ErrCodeEditOwnershipDenied, "所有权校验失败")
	ErrContentHashMismatch           = NewEditError(ErrCodeEditContentHashMismatch, "ContentHash不匹配")
	ErrActionRevisionSourceInvalid   = NewEditError(ErrCodeActionRevisionSourceInvalid, "ProcessingRevision来源校验失败")
	ErrActionRevisionAnchorInvalid   = NewEditError(ErrCodeActionRevisionAnchorInvalid, "Anchor解析失败或非法")
	ErrActionRevisionActiveNotFound  = NewEditError(ErrCodeActionRevisionActiveNotFound, "Active Revision不存在")
	ErrActionRevisionBindingConflict = NewEditError(ErrCodeActionRevisionBindingConflict, "Binding绑定冲突")
	ErrLegacyWriteDisabled           = NewEditError(desktoppet.ErrCodeLegacyPackageWriteDisabled, "Legacy editing write disabled")
)

const (
	CandidateSourceSingleFrame = "single_frame_regeneration"
	CandidateSourceFullAction  = "full_action_regeneration"
	CandidateSourceBgReprocess = "background_reprocess"
	CandidateSourceNormalize   = "normalize_upload"

	RegenModeSingleFrame = "single_frame_regeneration"
	RegenModeFullAction  = "full_action_regeneration"
	RegenModeBgReprocess = "background_reprocess"
	RegenModeNormalize   = "normalize_upload"

	AttemptOriginGenerationTask = "generation_task"
	AttemptOriginEditingRegen   = "editing_regeneration"

	JournalStatePlanCreated         = "plan_created"
	JournalStateProviderSubmitted   = "provider_submitted"
	JournalStateArtifactPersisted   = "artifact_persisted"
	JournalStateProcessingStarted   = "processing_started"
	JournalStateProcessingPublished = "processing_published"
	JournalStateCandidateCreated    = "candidate_created"
	JournalStateQualityCreated      = "quality_created"
	JournalStateReadyForReview      = "ready_for_review"
	JournalStateAccepted            = "accepted"
	JournalStateRejected            = "rejected"
	JournalStateFailed              = "failed"

	GateReasonActiveRevisionChanged = "active_revision_changed"

	RetentionProviderArtifactDays  = 7
	RetentionProcessingFailedDays  = 3
	RetentionCandidateRejectedDays = 30
)
