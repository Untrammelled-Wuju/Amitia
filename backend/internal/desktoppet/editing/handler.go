package editing

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(svc Service) *Handler { return &Handler{service: svc} }

func (h *Handler) ListRevisions(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" || actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少处理任务ID或动作Key", nil)
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	revs, err := h.service.ListRevisions(c.Request.Context(), processingTaskID, actionKey)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, revs)
}

func (h *Handler) GetRevision(c *gin.Context) {
	revisionID := c.Param("revisionId")
	if revisionID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少Revision ID", nil)
		return
	}
	detail, err := h.service.GetRevision(c.Request.Context(), revisionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, detail)
}

func (h *Handler) GetActiveRevision(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	detail, err := h.service.GetActiveRevision(c.Request.Context(), processingTaskID, actionKey)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, detail)
}

func (h *Handler) ActivateRevision(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	var req ActivateRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	err = h.service.ActivateRevision(c.Request.Context(), processingTaskID, actionKey, req.RevisionID, req.ExpectedBindingVersion, req.Reason, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) GetPreviewManifest(c *gin.Context) {
	revisionID := c.Param("revisionId")
	manifest, err := h.service.GetPreviewManifest(c.Request.Context(), revisionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, manifest)
}

func (h *Handler) GetFrameImage(c *gin.Context) {
	revisionID := c.Param("revisionId")
	frameID := c.Param("frameId")
	path, mimeType, err := h.service.GetFrameImage(c.Request.Context(), revisionID, frameID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	serveImageFile(c, path, mimeType)
}

func (h *Handler) GetFrameThumbnail(c *gin.Context) {
	revisionID := c.Param("revisionId")
	frameID := c.Param("frameId")
	path, mimeType, err := h.service.GetFrameThumbnail(c.Request.Context(), revisionID, frameID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	serveImageFile(c, path, mimeType)
}

func (h *Handler) GetActionEditSummary(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	summary, err := h.service.GetActionEditSummary(c.Request.Context(), processingTaskID, actionKey)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, summary)
}

func (h *Handler) CreateSession(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	resp, err := h.service.CreateSession(c.Request.Context(), processingTaskID, actionKey, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	session, err := h.service.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, session)
}

func (h *Handler) ApplyOperation(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req ApplyOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	resp, err := h.service.ApplyOperation(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Undo(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	baseVersion, _ := strconv.ParseInt(c.Query("baseSessionVersion"), 10, 64)
	resp, err := h.service.Undo(c.Request.Context(), sessionID, userID, baseVersion)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Redo(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	baseVersion, _ := strconv.ParseInt(c.Query("baseSessionVersion"), 10, 64)
	resp, err := h.service.Redo(c.Request.Context(), sessionID, userID, baseVersion)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) CreateCheckpoint(c *gin.Context) {
	sessionID := c.Param("sessionId")
	err := h.service.CreateCheckpoint(c.Request.Context(), sessionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) CommitSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req CommitSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	resp, err := h.service.CommitSession(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) AbandonSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	err = h.service.AbandonSession(c.Request.Context(), sessionID, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) SessionEvents(c *gin.Context) {
	sessionID := c.Param("sessionId")
	events, err := h.service.GetSessionEvents(c.Request.Context(), sessionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	if events == nil {
		events = []SessionEvent{}
	}
	util.SuccessResponse(c, events)
}

func (h *Handler) CreateRegenerationJob(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req CreateRegenerationJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	resp, err := h.service.CreateRegenerationJob(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) GetRegenerationJob(c *gin.Context) {
	sessionID := c.Param("sessionId")
	jobID := c.Param("jobId")
	_ = sessionID
	job, err := h.service.GetRegenerationJob(c.Request.Context(), jobID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, job)
}

func (h *Handler) CancelRegenerationJob(c *gin.Context) {
	jobID := c.Param("jobId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	err = h.service.CancelRegenerationJob(c.Request.Context(), jobID, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) GetRegenerationJobByID(c *gin.Context) {
	jobID := c.Param("jobId")
	if jobID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少Job ID", nil)
		return
	}
	job, err := h.service.GetRegenerationJob(c.Request.Context(), jobID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, job)
}

func (h *Handler) ListRegenerationJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	jobs, err := h.service.ListRegenerationJobs(c.Request.Context(), limit, offset)
	if err != nil {
		writeEditError(c, err)
		return
	}
	if jobs == nil {
		jobs = []RegenerationJob{}
	}
	util.SuccessResponse(c, jobs)
}

func (h *Handler) AcceptCandidate(c *gin.Context) {
	candidateID := c.Param("candidateId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req AcceptCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = AcceptCandidateRequest{}
	}
	err = h.service.AcceptCandidate(c.Request.Context(), candidateID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) RejectCandidate(c *gin.Context) {
	candidateID := c.Param("candidateId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req RejectCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = RejectCandidateRequest{}
	}
	err = h.service.RejectCandidate(c.Request.Context(), candidateID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) UploadCandidate(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	targetFrameID := c.PostForm("targetFrameId")
	file, err := c.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少上传文件", gin.H{"errorCode": ErrCodeEditUploadInvalid})
		return
	}
	if file.Size > 50*1024*1024 {
		util.ErrorResponse(c, response.InvalidParams, "文件过大", gin.H{"errorCode": ErrCodeEditUploadTooLarge})
		return
	}
	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/png"
	}
	src, err := file.Open()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "打开文件失败", nil)
		return
	}
	defer src.Close()
	const maxFileBytes = 100 * 1024 * 1024
	if file.Size > maxFileBytes {
		util.ErrorResponse(c, response.InvalidParams, "文件超过最大允许大小", gin.H{"errorCode": ErrCodeEditUploadTooLarge})
		return
	}
	data := make([]byte, file.Size) // audit:ok:size_validated
	_, err = src.Read(data)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取文件失败", nil)
		return
	}
	resp, err := h.service.UploadCandidate(c.Request.Context(), sessionID, userID, data, mimeType, targetFrameID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) ApplyBackgroundPatch(c *gin.Context) {
	sessionID := c.Param("sessionId")
	frameID := c.Param("frameId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req BackgroundApplyPatchPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	req.FrameID = frameID
	err = h.service.ApplyBackgroundPatch(c.Request.Context(), sessionID, frameID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) ResetBackgroundPatch(c *gin.Context) {
	sessionID := c.Param("sessionId")
	frameID := c.Param("frameId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	err = h.service.ResetBackgroundPatch(c.Request.Context(), sessionID, frameID, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) SetFrameAnchor(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req AnchorSetFramePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	err = h.service.SetFrameAnchor(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) BatchOffsetAnchors(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req AnchorBatchOffsetPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", gin.H{"error": err.Error()})
		return
	}
	err = h.service.BatchOffsetAnchors(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) ResetAnchors(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	var req AnchorResetPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		req = AnchorResetPayload{}
	}
	err = h.service.ResetAnchors(c.Request.Context(), sessionID, userID, req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) TriggerQualityEvaluation(c *gin.Context) {
	revisionID := c.Param("revisionId")
	jobID, err := h.service.TriggerQualityEvaluation(c.Request.Context(), revisionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"qualityJobId": jobID})
}

func (h *Handler) GetLatestQualityEvaluation(c *gin.Context) {
	revisionID := c.Param("revisionId")
	info, err := h.service.GetLatestQualityEvaluation(c.Request.Context(), revisionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, info)
}

func (h *Handler) ImportLegacyRevision(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	if err := h.service.CheckProcessingTaskOwnership(c.Request.Context(), processingTaskID, userID); err != nil {
		writeEditError(c, err)
		return
	}
	resp, err := h.service.ImportLegacyRevision(c.Request.Context(), processingTaskID, actionKey, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func serveImageFile(c *gin.Context, path, mimeType string) {
	if path == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", mimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=300")
	c.File(path) // audit:ok:path_origin=internal_service
}

func writeEditError(c *gin.Context, err error) {
	var ee *EditError
	if errors.As(err, &ee) {
		httpCode := mapEditErrorCode(ee.Code)
		util.ErrorResponse(c, httpCode, ee.Message, gin.H{"errorCode": ee.Code, "detail": ee.Detail})
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func mapEditErrorCode(code string) int {
	switch code {
	case ErrCodeEditTaskNotFound, ErrCodeEditActionNotFound, ErrCodeEditRevisionNotFound,
		ErrCodeEditSessionNotFound, ErrCodeEditFrameNotFound, ErrCodeEditAssetNotFound,
		ErrCodeEditCandidateNotFound:
		return response.NotFound
	case ErrCodeEditSessionExpired, ErrCodeEditSessionAlreadyCommitted,
		ErrCodeEditRevisionNotReady, ErrCodeEditCandidateNotReady,
		ErrCodeEditJobNotCancelable, ErrCodeEditQualityPending,
		ErrCodeEditQualityGateBlocked:
		return response.BusinessError
	case ErrCodeEditSessionVersionConflict, ErrCodeEditActiveBindingConflict,
		ErrCodeEditCandidateAlreadyDecided:
		status := response.BusinessError
		if code == ErrCodeEditSessionVersionConflict {
			status = 409
		}
		return status
	case ErrCodeEditOperationInvalid, ErrCodeEditFrameCountInvalid,
		ErrCodeEditFrameDurationInvalid, ErrCodeEditUploadInvalid,
		ErrCodeEditUploadTooLarge, ErrCodeEditPatchInvalid,
		ErrCodeEditAnchorInvalid, ErrCodeEditOperationNotReversible,
		ErrCodeEditCostConfirmationRequired:
		return response.InvalidParams
	case ErrCodeEditAssetHashMismatch, ErrCodeEditRevisionPublishFailed:
		return response.OperationFailed
	case ErrCodeEditProviderStatusUnknown:
		return response.OperationFailed
	case ErrCodeEditPermissionDenied:
		return http.StatusForbidden
	default:
		return response.InternalError
	}
}

func (h *Handler) ListActionStreams(c *gin.Context) {
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	streams, err := h.service.ListActionStreams(c.Request.Context(), userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, streams)
}

func (h *Handler) ListRevisionsByStream(c *gin.Context) {
	streamID := c.Param("streamId")
	if streamID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少Stream ID", nil)
		return
	}
	revs, err := h.service.ListRevisionsByStream(c.Request.Context(), streamID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, revs)
}

func (h *Handler) GetActiveRevisionByStream(c *gin.Context) {
	streamID := c.Param("streamId")
	if streamID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少Stream ID", nil)
		return
	}
	detail, err := h.service.GetActiveRevisionByStream(c.Request.Context(), streamID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, detail)
}
