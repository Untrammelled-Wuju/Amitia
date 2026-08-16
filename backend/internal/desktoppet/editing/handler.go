package editing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service        Service
	safeResponder  *security.SafeArtifactResponder
	ownershipGuard security.OwnershipGuard
}

func NewHandler(svc Service, responder *security.SafeArtifactResponder, guard security.OwnershipGuard) *Handler {
	return &Handler{service: svc, safeResponder: responder, ownershipGuard: guard}
}

func (h *Handler) ListRevisions(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" || actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少处理任务ID或动作Key", nil)
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	var req ActivateRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	err = h.service.ActivateRevision(c.Request.Context(), processingTaskID, actionKey, req.RevisionID, req.ExpectedBindingVersion, req.Reason, string(actor.UserID))
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) GetPreviewManifest(c *gin.Context) {
	revisionID := c.Param("revisionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	path, mimeType, err := h.service.GetFrameImage(c.Request.Context(), revisionID, frameID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	serveImageFile(h.safeResponder, c, actor, security.RootEditingAssets, path, mimeType, revisionID, frameID)
}

func (h *Handler) GetFrameThumbnail(c *gin.Context) {
	revisionID := c.Param("revisionId")
	frameID := c.Param("frameId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	path, mimeType, err := h.service.GetFrameThumbnail(c.Request.Context(), revisionID, frameID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	serveImageFile(h.safeResponder, c, actor, security.RootEditingAssets, path, mimeType, revisionID, frameID+"-thumb")
}

func (h *Handler) GetActionEditSummary(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
		return
	}
	resp, err := h.service.CreateSession(c.Request.Context(), processingTaskID, actionKey, string(actor.UserID), req)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	session, err := h.service.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, session)
}

func (h *Handler) ApplyOperation(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	var req ApplyOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	err = h.service.CreateCheckpoint(c.Request.Context(), sessionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) CommitSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	var req CommitSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	err = h.service.AbandonSession(c.Request.Context(), sessionID, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) SessionEvents(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
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
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	scope, err := h.ownershipGuard.RequireRegenerationJob(c.Request.Context(), actor, jobID)
	if err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	if scope.SessionID != sessionID {
		writeEditOwnershipError(c, security.ErrNotFound)
		return
	}
	job, err := h.service.GetRegenerationJob(c.Request.Context(), jobID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, job)
}

func (h *Handler) CancelRegenerationJob(c *gin.Context) {
	jobID := c.Param("jobId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	scope, err := h.ownershipGuard.RequireRegenerationJob(c.Request.Context(), actor, jobID)
	if err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	if scope.SessionID != c.Param("sessionId") {
		writeEditOwnershipError(c, security.ErrNotFound)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRegenerationJob(c.Request.Context(), actor, jobID); err != nil {
		writeEditOwnershipError(c, err)
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
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	jobs, err := h.service.ListRegenerationJobs(c.Request.Context(), string(actor.UserID), limit, offset)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	scope, err := h.ownershipGuard.RequireCandidate(c.Request.Context(), actor, candidateID)
	if err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	if scope.SessionID != c.Param("sessionId") {
		writeEditOwnershipError(c, security.ErrNotFound)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	scope, err := h.ownershipGuard.RequireCandidate(c.Request.Context(), actor, candidateID)
	if err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	if scope.SessionID != c.Param("sessionId") {
		writeEditOwnershipError(c, security.ErrNotFound)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
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
	data, err := io.ReadAll(io.LimitReader(src, maxFileBytes))
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	var req BackgroundApplyPatchPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	err = h.service.ResetBackgroundPatch(c.Request.Context(), sessionID, frameID, userID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"ok": true})
}

func (h *Handler) SetFrameAnchor(c *gin.Context) {
	sessionID := c.Param("sessionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	var req AnchorSetFramePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
	var req AnchorBatchOffsetPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数无效", nil)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireEditSession(c.Request.Context(), actor, sessionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	userID := string(actor.UserID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	jobID, err := h.service.TriggerQualityEvaluation(c.Request.Context(), revisionID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"qualityJobId": jobID})
}

func (h *Handler) GetLatestQualityEvaluation(c *gin.Context) {
	revisionID := c.Param("revisionId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionRevision(c.Request.Context(), actor, revisionID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireProcessingTask(c.Request.Context(), actor, processingTaskID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	resp, err := h.service.ImportLegacyRevision(c.Request.Context(), processingTaskID, actionKey, string(actor.UserID))
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, resp)
}

func serveImageFile(responder *security.SafeArtifactResponder, c *gin.Context, actor *desktoppetAuth.ActorContext, rootKind security.StorageRootKind, path, mimeType, artifactID, entityID string) {
	if path == "" || actor == nil {
		c.Status(http.StatusNotFound)
		return
	}
	storageKey, err := extractStorageKey(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	hash, size, err := computeFileHashAndSize(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=300")
	responder.ServeArtifact(c, actor, security.ArtifactReference{
		ArtifactID:  artifactID,
		OwnerUserID: entityID,
		RootKind:    rootKind,
		StorageKey:  storageKey,
		ContentHash: hash,
		ByteSize:    size,
		MIME:        mimeType,
	})
}

func extractStorageKey(path string) (string, error) {
	idx := strings.Index(path, "desktop-pets/")
	if idx == -1 {
		return "", errors.New("invalid path")
	}
	return path[idx+len("desktop-pets/"):], nil
}

func computeFileHashAndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), info.Size(), nil
}

func writeEditError(c *gin.Context, err error) {
	var ee *EditError
	if errors.As(err, &ee) {
		httpCode := mapEditErrorCode(ee.Code)
		util.ErrorResponse(c, httpCode, ee.Message, gin.H{"errorCode": ee.Code, "detail": ee.Detail})
		return
	}
	util.ErrorResponse(c, response.InternalError, "服务器内部错误", nil)
}

func writeEditOwnershipError(c *gin.Context, err error) {
	if ownErr := security.MapOwnershipError(err); ownErr != nil {
		util.ErrorResponse(c, ownErr.Code, ownErr.Msg, gin.H{"errorCode": ownErr.ErrCode})
		return
	}
	writeEditError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionStream(c.Request.Context(), actor, streamID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	revs, err := h.service.ListRevisionsByStream(c.Request.Context(), string(actor.UserID), streamID)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireActionStream(c.Request.Context(), actor, streamID); err != nil {
		writeEditOwnershipError(c, err)
		return
	}
	detail, err := h.service.GetActiveRevisionByStream(c.Request.Context(), string(actor.UserID), streamID)
	if err != nil {
		writeEditError(c, err)
		return
	}
	util.SuccessResponse(c, detail)
}
