// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

type userScopedMemoryService interface {
	ListForUser(q MemoryListQuery, userID string) (*MemoryListResponse, error)
	CreateForUser(req *CreateMemoryRequest, userID string) (*Memory, error)
	UpdateForUser(id, userID string, req *UpdateMemoryRequest) (*Memory, error)
	RestoreForUser(id, userID string) (*Memory, error)
	DeleteForUser(id, userID string) error
	SearchForUser(req *SearchMemoryRequest, userID string) ([]Memory, error)
	VectorSearchForUser(req *VectorSearchRequest, userID string) ([]VectorSearchResult, error)
	HybridSearchForUser(req *VectorSearchRequest, userID string) ([]HybridSearchResult, error)
	RebuildEmbeddingsForUser(userID string) (map[string]interface{}, error)
	RecordUseForUser(id, userID string) (*Memory, error)
	DeleteAllForUser(characterID, userID string) error
	GetTimelineForUser(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error)
	CheckConflictForUser(req *CheckConflictRequest, userID string) (*CheckConflictResponse, error)
	ResolveConflictForUser(req *ResolveConflictRequest, userID string) (*ResolveConflictResponse, error)
	ListCandidatesForUser(userID string) []MemoryCandidate
	RebuildIndexForUser(userID string) (map[string]interface{}, error)
	UpdateCandidateForUser(id, userID string, req *UpdateCandidateRequest) (*MemoryCandidate, error)
	DeleteCandidateForUser(id, userID string) error
	GenerateCandidatesForUser(conversationID, userID string) ([]MemoryCandidate, error)
	SubmitCandidateForUser(req *SubmitCandidateRequest, userID string) (*MemoryCandidate, error)
	AcceptCandidateForUser(id, userID string) (*Memory, error)
	RejectCandidateForUser(id, userID string) error
	BatchAcceptCandidatesForUser(ids []string, userID string) ([]Memory, error)
	GetVectorStatusForUser(userID string) map[string]interface{}
	BatchVerifyForUser(ids []string, status, userID string) error
	BatchSetImportanceForUser(ids []string, importance int, userID string) error
	GetRankedMemoriesForUser(characterID, userID, query string, limit int) ([]RankedMemory, error)
	RetrieveStatsForUser(userID string) (map[string]interface{}, error)
}

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) scoped(c *gin.Context) (userScopedMemoryService, string, bool) {
	svc, ok := h.service.(userScopedMemoryService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "memory service does not provide user-scoped operations", nil)
		return nil, "", false
	}
	return svc, requestidentity.ResolveGin(c, ""), true
}

func (h *Handler) List(c *gin.Context) {
	var q MemoryListQuery
	_ = c.ShouldBindQuery(&q)
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	resp, err := svc.ListForUser(q, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.CreateForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆创建成功", m)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.UpdateForUser(c.Param("id"), userID, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆更新成功", m)
}

func (h *Handler) Restore(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.RestoreForUser(c.Param("id"), userID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已恢复", m)
}

func (h *Handler) Delete(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteForUser(c.Param("id"), userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已删除", nil)
}

func (h *Handler) Search(c *gin.Context) {
	var req SearchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if req.Keyword == "" && req.Time == nil && len(req.Types) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "keyword、time或types至少需要一个", nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.SearchForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) VectorSearch(c *gin.Context) {
	var req VectorSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.VectorSearchForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) HybridSearch(c *gin.Context) {
	var req VectorSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.HybridSearchForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) RebuildEmbeddings(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.RebuildEmbeddingsForUser(userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "嵌入重建完成", result)
}

func (h *Handler) RecordUse(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.RecordUseForUser(c.Param("id"), userID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "使用已记录", m)
}

func (h *Handler) DeleteAll(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteAllForUser(c.Query("characterId"), userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "所有记忆已删除", nil)
}

func (h *Handler) Timeline(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, total, err := svc.GetTimelineForUser(page, pageSize, userID, c.Query("source"), c.Query("memoryType"), c.Query("type"))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) CheckConflict(c *gin.Context) {
	var req CheckConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.CheckConflictForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ResolveConflict(c *gin.Context) {
	var req ResolveConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "参数错误", nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.ResolveConflictForUser(&req, userID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ExtractCandidates(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items := svc.ListCandidatesForUser(userID)
	util.SuccessResponse(c, gin.H{"candidates": items, "total": len(items)})
}

func (h *Handler) RebuildIndex(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.RebuildIndexForUser(userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "索引重建完成", result)
}

func (h *Handler) UpdateCandidate(c *gin.Context) {
	var req UpdateCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "参数错误", nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	item, err := svc.UpdateCandidateForUser(c.Param("id"), userID, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "候选记忆已更新", item)
}

func (h *Handler) DeleteCandidate(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteCandidateForUser(c.Param("id"), userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "候选记忆已删除", nil)
}

func (h *Handler) GenerateCandidates(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ConversationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "conversationId不能为空", nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.GenerateCandidatesForUser(req.ConversationID, userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"candidates": items, "generated": len(items)})
}

func (h *Handler) ListCandidates(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items := svc.ListCandidatesForUser(userID)
	util.SuccessResponse(c, gin.H{"candidates": items, "total": len(items)})
}

func (h *Handler) AcceptCandidate(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.AcceptCandidateForUser(c.Param("id"), userID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已保存", m)
}

func (h *Handler) RejectCandidate(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.RejectCandidateForUser(c.Param("id"), userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已拒绝", nil)
}

func (h *Handler) BatchAcceptCandidates(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "ids不能为空", nil)
		return
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.BatchAcceptCandidatesForUser(req.IDs, userID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "批量保存完成", gin.H{"accepted": len(items), "memories": items})
}

func (h *Handler) VectorStatus(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	util.SuccessResponse(c, svc.GetVectorStatusForUser(userID))
}

func (h *Handler) BatchVerify(c *gin.Context) {
	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Status string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "ids不能为空", nil)
		return
	}
	if req.Status == "" {
		req.Status = "user_verified"
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.BatchVerifyForUser(req.IDs, req.Status, userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "批量确认完成", nil)
}

func (h *Handler) BatchSetImportance(c *gin.Context) {
	var req struct {
		IDs        []string `json:"ids" binding:"required"`
		Importance int      `json:"importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "ids不能为空", nil)
		return
	}
	if req.Importance <= 0 {
		req.Importance = 10
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.BatchSetImportanceForUser(req.IDs, req.Importance, userID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "批量设置完成", nil)
}

func (h *Handler) GetRankedMemories(c *gin.Context) {
	limit := 10
	if raw := c.Query("limit"); raw != "" {
		_, _ = fmt.Sscanf(raw, "%d", &limit)
	}
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.GetRankedMemoriesForUser(c.Query("characterId"), userID, c.Query("query"), limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) RetrieveStats(c *gin.Context) {
	svc, userID, ok := h.scoped(c)
	if !ok {
		return
	}
	stats, err := svc.RetrieveStatsForUser(userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, stats)
}
