package editing

import "fmt"

const (
	ErrCodeEditTaskNotFound            = "EDIT_TASK_NOT_FOUND"
	ErrCodeEditActionNotFound          = "EDIT_ACTION_NOT_FOUND"
	ErrCodeEditRevisionNotFound        = "EDIT_REVISION_NOT_FOUND"
	ErrCodeEditRevisionNotReady        = "EDIT_REVISION_NOT_READY"
	ErrCodeEditSessionNotFound         = "EDIT_SESSION_NOT_FOUND"
	ErrCodeEditSessionExpired          = "EDIT_SESSION_EXPIRED"
	ErrCodeEditSessionVersionConflict  = "EDIT_SESSION_VERSION_CONFLICT"
	ErrCodeEditSessionAlreadyCommitted = "EDIT_SESSION_ALREADY_COMMITTED"
	ErrCodeEditOperationInvalid        = "EDIT_OPERATION_INVALID"
	ErrCodeEditOperationNotReversible  = "EDIT_OPERATION_NOT_REVERSIBLE"
	ErrCodeEditFrameNotFound           = "EDIT_FRAME_NOT_FOUND"
	ErrCodeEditFrameCountInvalid       = "EDIT_FRAME_COUNT_INVALID"
	ErrCodeEditFrameDurationInvalid    = "EDIT_FRAME_DURATION_INVALID"
	ErrCodeEditAssetNotFound           = "EDIT_ASSET_NOT_FOUND"
	ErrCodeEditAssetHashMismatch       = "EDIT_ASSET_HASH_MISMATCH"
	ErrCodeEditUploadInvalid           = "EDIT_UPLOAD_INVALID"
	ErrCodeEditUploadTooLarge          = "EDIT_UPLOAD_TOO_LARGE"
	ErrCodeEditPatchInvalid            = "EDIT_PATCH_INVALID"
	ErrCodeEditAnchorInvalid           = "EDIT_ANCHOR_INVALID"
	ErrCodeEditCandidateNotFound       = "EDIT_CANDIDATE_NOT_FOUND"
	ErrCodeEditCandidateNotReady       = "EDIT_CANDIDATE_NOT_READY"
	ErrCodeEditCandidateAlreadyDecided = "EDIT_CANDIDATE_ALREADY_DECIDED"
	ErrCodeEditJobNotCancelable        = "EDIT_JOB_NOT_CANCELABLE"
	ErrCodeEditProviderStatusUnknown   = "EDIT_PROVIDER_STATUS_UNKNOWN"
	ErrCodeEditCostConfirmationRequired = "EDIT_COST_CONFIRMATION_REQUIRED"
	ErrCodeEditRevisionPublishFailed   = "EDIT_REVISION_PUBLISH_FAILED"
	ErrCodeEditQualityPending          = "EDIT_QUALITY_PENDING"
	ErrCodeEditQualityGateBlocked      = "EDIT_QUALITY_GATE_BLOCKED"
	ErrCodeEditActiveBindingConflict   = "EDIT_ACTIVE_BINDING_CONFLICT"
	ErrCodeEditPermissionDenied        = "EDIT_PERMISSION_DENIED"
)

const (
	RevisionTypeProcessed   = "processed"
	RevisionTypeEdit        = "edit"
	RevisionTypeRegenerated = "regenerated"
	RevisionTypeImported    = "imported"
	RevisionTypeRollback    = "rollback"
	RevisionTypeLegacyImport = "legacy_import"
)

const (
	RevisionStatusBuilding       = "building"
	RevisionStatusReady          = "ready"
	RevisionStatusQualityPending = "quality_pending"
	RevisionStatusQualityReady  = "quality_ready"
	RevisionStatusFailed         = "failed"
	RevisionStatusArchived       = "archived"
)

const (
	SessionStatusOpen        = "open"
	SessionStatusCommitting  = "committing"
	SessionStatusCommitted   = "committed"
	SessionStatusAbandoned   = "abandoned"
	SessionStatusExpired     = "expired"
	SessionStatusConflicted  = "conflicted"
)

const (
	OperationStatusApplied    = "applied"
	OperationStatusReverted   = "reverted"
	OperationStatusFailed     = "failed"
	OperationStatusSuperseded = "superseded"
)

const (
	JobStatusCreated          = "created"
	JobStatusQueued           = "queued"
	JobStatusRunning          = "running"
	JobStatusProviderSucceeded = "provider_succeeded"
	JobStatusMaterializing    = "materializing"
	JobStatusCompleted        = "completed"
	JobStatusFailed           = "failed"
	JobStatusCancelled        = "cancelled"
	JobStatusUnknown          = "unknown"
)

const (
	JobTypeSingleFrame        = "single_frame"
	JobTypeFullAction         = "full_action"
	JobTypeBackgroundReprocess = "background_reprocess"
	JobTypeNormalizeUpload    = "normalize_upload"
)

const (
	CandidateStatusPending  = "pending"
	CandidateStatusAccepted = "accepted"
	CandidateStatusRejected = "rejected"
	CandidateStatusExpired  = "expired"
)

const (
	AssetStatusStaging    = "staging"
	AssetStatusReady      = "ready"
	AssetStatusQuarantined = "quarantined"
	AssetStatusDeleted    = "deleted"
)

const (
	AssetSourceProcessed    = "processed"
	AssetSourceRegenerated  = "regenerated"
	AssetSourceUploaded     = "uploaded"
	AssetSourceMaskApplied  = "mask_applied"
	AssetSourceLegacy       = "legacy"
)

const (
	PatchTypeErase  = "erase"
	PatchTypeRestore = "restore"
	PatchTypeSoften = "soften"
)

const (
	AnchorSpaceNormalizedCanvas = "normalized_canvas"
	AnchorSpacePixel            = "pixel"
)

const (
	JournalActionPublish = "publish"
	JournalActionActivate = "activate"
	JournalActionRecover = "recover"
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
	MinFrameDurationMS = 16
	MaxFrameDurationMS = 60000
	MinFrameCount      = 1
	MaxCheckpointKeep  = 10
	SessionDefaultTTLHours = 24
	CandidateRetentionDays  = 7
	SessionRetentionDays    = 7
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
	ErrTaskNotFound            = NewEditError(ErrCodeEditTaskNotFound, "处理任务不存在")
	ErrActionNotFound          = NewEditError(ErrCodeEditActionNotFound, "动作不存在")
	ErrRevisionNotFound        = NewEditError(ErrCodeEditRevisionNotFound, "Revision不存在")
	ErrRevisionNotReady        = NewEditError(ErrCodeEditRevisionNotReady, "Revision尚未就绪")
	ErrSessionNotFound         = NewEditError(ErrCodeEditSessionNotFound, "编辑会话不存在")
	ErrSessionExpired          = NewEditError(ErrCodeEditSessionExpired, "编辑会话已过期")
	ErrSessionAlreadyCommitted = NewEditError(ErrCodeEditSessionAlreadyCommitted, "编辑会话已提交")
	ErrOperationInvalid        = NewEditError(ErrCodeEditOperationInvalid, "操作无效")
	ErrOperationNotReversible  = NewEditError(ErrCodeEditOperationNotReversible, "操作不可逆")
	ErrFrameNotFound           = NewEditError(ErrCodeEditFrameNotFound, "帧不存在")
	ErrFrameCountInvalid       = NewEditError(ErrCodeEditFrameCountInvalid, "帧数量无效")
	ErrFrameDurationInvalid    = NewEditError(ErrCodeEditFrameDurationInvalid, "帧时长无效")
	ErrAssetNotFound           = NewEditError(ErrCodeEditAssetNotFound, "资产不存在")
	ErrAssetHashMismatch       = NewEditError(ErrCodeEditAssetHashMismatch, "资产哈希不匹配")
	ErrUploadInvalid           = NewEditError(ErrCodeEditUploadInvalid, "上传文件无效")
	ErrUploadTooLarge          = NewEditError(ErrCodeEditUploadTooLarge, "上传文件过大")
	ErrPatchInvalid            = NewEditError(ErrCodeEditPatchInvalid, "Patch无效")
	ErrAnchorInvalid           = NewEditError(ErrCodeEditAnchorInvalid, "锚点无效")
	ErrCandidateNotFound       = NewEditError(ErrCodeEditCandidateNotFound, "候选不存在")
	ErrCandidateNotReady       = NewEditError(ErrCodeEditCandidateNotReady, "候选尚未就绪")
	ErrCandidateAlreadyDecided = NewEditError(ErrCodeEditCandidateAlreadyDecided, "候选已被处理")
	ErrJobNotCancelable        = NewEditError(ErrCodeEditJobNotCancelable, "Job不可取消")
	ErrProviderStatusUnknown   = NewEditError(ErrCodeEditProviderStatusUnknown, "Provider状态未知")
	ErrCostConfirmationRequired = NewEditError(ErrCodeEditCostConfirmationRequired, "需要成本确认")
	ErrRevisionPublishFailed   = NewEditError(ErrCodeEditRevisionPublishFailed, "Revision发布失败")
	ErrQualityPending          = NewEditError(ErrCodeEditQualityPending, "质量评估进行中")
	ErrQualityGateBlocked      = NewEditError(ErrCodeEditQualityGateBlocked, "质量门禁阻止")
	ErrActiveBindingConflict   = NewEditError(ErrCodeEditActiveBindingConflict, "Active绑定冲突")
	ErrPermissionDenied        = NewEditError(ErrCodeEditPermissionDenied, "权限不足")
)
