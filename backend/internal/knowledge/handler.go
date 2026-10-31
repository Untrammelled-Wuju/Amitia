package knowledge

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func writeKnowledgeError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
}

func (h *Handler) ListSources(c *gin.Context) {
	items, err := h.service.ListSources(requestidentity.ResolveGin(c))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) GetSource(c *gin.Context) {
	item, err := h.service.GetSourceDetail(requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) GetSourceOutline(c *gin.Context) {
	revision, _ := strconv.ParseInt(strings.TrimSpace(c.Query("revision")), 10, 64)
	afterOrdinal, err := strconv.ParseInt(strings.TrimSpace(c.Query("afterOrdinal")), 10, 64)
	if err != nil {
		afterOrdinal = -1
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	result, err := h.service.GetSourceOutline(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), revision, afterOrdinal, limit)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	opened, err := file.Open()
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	defer opened.Close()
	logicalName := strings.TrimSpace(c.PostForm("logicalName"))
	mimeType := strings.TrimSpace(file.Header.Get("Content-Type"))
	source, job, err := h.service.Upload(c.Request.Context(), requestidentity.ResolveGin(c), logicalName, file.Filename, mimeType, opened)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
	util.SuccessMsgResponse(c, "知识源已进入解析队列", map[string]any{"source": source, "job": job})
}

func (h *Handler) UploadRevision(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	opened, err := file.Open()
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	defer opened.Close()
	revision, job, err := h.service.UploadRevision(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), file.Filename, strings.TrimSpace(file.Header.Get("Content-Type")), opened)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
	util.SuccessMsgResponse(c, "知识源新版本已进入解析队列，当前活动版本会保持可用直到切换完成", map[string]any{"revision": revision, "job": job})
}

func revisionParam(c *gin.Context) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param("revision")), 10, 64)
	if err != nil || value <= 0 {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return 0, false
	}
	return value, true
}

func (h *Handler) RollbackRevision(c *gin.Context) {
	revision, ok := revisionParam(c)
	if !ok {
		return
	}
	job, err := h.service.RollbackSourceRevision(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), revision)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
	util.SuccessMsgResponse(c, "知识源回滚已进入索引校验队列，当前活动版本会保持可用直到原子切换完成", job)
}

func (h *Handler) PurgeRevision(c *gin.Context) {
	revision, ok := revisionParam(c)
	if !ok {
		return
	}
	item, err := h.service.PurgeSourceRevision(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), revision)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "历史版本已进入最终清理流程", item)
}

func (h *Handler) GarbageCollect(c *gin.Context) {
	count, err := h.service.GarbageCollect(c.Request.Context(), requestidentity.ResolveGin(c))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "知识库历史版本清理已完成", map[string]any{"purged": count})
}

func (h *Handler) ReindexSource(c *gin.Context) {
	job, err := h.service.ReindexSource(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
	util.SuccessMsgResponse(c, "知识源已进入重建索引队列", job)
}

func (h *Handler) DeleteSource(c *gin.Context) {
	if err := h.service.DeleteSource(requestidentity.ResolveGin(c), c.Param("id")); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "知识源已删除", nil)
}

func (h *Handler) ListJobs(c *gin.Context) {
	items, err := h.service.ListJobs(requestidentity.ResolveGin(c), strings.TrimSpace(c.Query("sourceId")))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) GetJob(c *gin.Context) {
	item, err := h.service.GetJob(requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, item)
}

func (h *Handler) RetryJob(c *gin.Context) {
	item, err := h.service.RetryJob(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "知识库任务已重新入队", item)
}

func (h *Handler) CancelJob(c *gin.Context) {
	item, err := h.service.CancelJob(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "知识库任务已取消", item)
}

func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if _, err := resolveTemporalQuery(req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if _, err := buildQueryPlan(req, limit); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	result, err := h.service.Search(c.Request.Context(), requestidentity.ResolveGin(c), req)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Read(c *gin.Context) {
	var req ReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	result, err := h.service.Read(c.Request.Context(), requestidentity.ResolveGin(c), req)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) RunBenchmark(c *gin.Context) {
	var req BenchmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if err := validateBenchmarkRequest(req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	result, err := h.service.RunBenchmark(c.Request.Context(), requestidentity.ResolveGin(c), req)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListBenchmarkRuns(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, err := h.service.ListBenchmarkRuns(requestidentity.ResolveGin(c), limit)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) ListRetrievalTraces(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, err := h.service.ListRetrievalTraces(requestidentity.ResolveGin(c), limit)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) GetRuntimeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeStatus())
}

func (h *Handler) GetSLOSummary(c *gin.Context) {
	result, err := h.service.GetSLOSummary(requestidentity.ResolveGin(c))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) GetIndexOutboxHealth(c *gin.Context) {
	result, err := h.service.GetIndexOutboxHealth(c.Request.Context(), requestidentity.ResolveGin(c))
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) AuditVectors(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	result, err := h.service.AuditVectors(c.Request.Context(), requestidentity.ResolveGin(c), limit)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) AuditOrphanVectors(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	result, err := h.service.AuditOrphanVectors(c.Request.Context(), requestidentity.ResolveGin(c), limit)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) AuditOrphanVectorPage(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	result, err := h.service.AuditOrphanVectorPage(c.Request.Context(), requestidentity.ResolveGin(c), limit, c.Query("cursor"))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListOrphanAuditSessions(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}
	sessions, err := h.service.ListOrphanAuditSessions(c.Request.Context(), requestidentity.ResolveGin(c), limit)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": sessions})
}

func (h *Handler) ListOrphanAuditPageReceipts(c *gin.Context) {
	after, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("after", "0")))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "25")))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	result, err := h.service.ListOrphanAuditPageReceipts(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), after, limit)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) RecheckOrphanAuditLatest(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, err := h.service.RecheckOrphanAuditLatest(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrOrphanRecheckCooldown) {
			c.Header("Retry-After", "30")
			c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": err.Error()})
			return
		}
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListOrphanAuditRechecks(c *gin.Context) {
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "10")))
	if err != nil || limit < 1 || limit > 30 {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	items, err := h.service.ListOrphanAuditRechecks(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), limit)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, map[string]any{"items": items})
}

func (h *Handler) Reconcile(c *gin.Context) {
	if err := h.service.Reconcile(c.Request.Context(), requestidentity.ResolveGin(c)); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "知识库一致性检查已完成", nil)
}

func recoveryReviewerHash(c *gin.Context) (string, bool) {
	return recoveryDevicePrincipalHash(security.GetActor(c))
}

func (h *Handler) CreateRecoveryReview(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	actorHash, ok := recoveryReviewerHash(c)
	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var body struct {
		RecheckID  string `json:"recheckId" binding:"required"`
		RequestKey string `json:"requestKey" binding:"required"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	result, err := h.service.CreateRecoveryReview(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), body.RecheckID, body.RequestKey, actorHash)
	if err != nil {
		writeRecoveryReviewError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ApproveRecoveryReview(c *gin.Context) { h.transitionRecoveryReview(c, "approve") }
func (h *Handler) RevokeRecoveryReview(c *gin.Context)  { h.transitionRecoveryReview(c, "revoke") }

func (h *Handler) transitionRecoveryReview(c *gin.Context, transition string) {
	c.Header("Cache-Control", "no-store")
	actorHash, ok := recoveryReviewerHash(c)
	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	result, err := h.service.TransitionRecoveryReview(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), actorHash, transition)
	if err != nil {
		writeRecoveryReviewError(c, err)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListRecoveryReviews(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if _, ok := recoveryReviewerHash(c); !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "10")))
	if err != nil || limit < 1 || limit > 30 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	items, err := h.service.ListRecoveryReviews(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), limit)
	if err != nil {
		writeRecoveryReviewError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "executable": false})
}

func writeRecoveryReviewError(c *gin.Context, err error) {
	if errors.Is(err, ErrRecoveryReviewHistoryInvalid) {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	if errors.Is(err, ErrRecoveryReviewConflict) || errors.Is(err, ErrRecoveryReviewEvidenceInvalid) {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": err.Error()})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
}

func (h *Handler) ListRecoveryReviewEvents(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if _, ok := recoveryReviewerHash(c); !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	events, err := h.service.ListRecoveryReviewEvents(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		writeRecoveryReviewError(c, err)
		return
	}
	util.SuccessResponse(c, events)
}

func (h *Handler) GetRecoveryReviewPlan(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	actorHash, ok := recoveryReviewerHash(c)
	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	plan, err := h.service.GetRecoveryReviewPlan(c.Request.Context(), requestidentity.ResolveGin(c), c.Param("id"), actorHash)
	if err != nil {
		writeRecoveryReviewError(c, err)
		return
	}
	util.SuccessResponse(c, plan)
}
