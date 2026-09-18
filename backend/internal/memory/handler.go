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
	ListForSpace(q MemoryListQuery, spaceID string) (*MemoryListResponse, error)
	CreateForSpace(req *CreateMemoryRequest, spaceID string) (*Memory, error)
	UpdateForSpace(id, spaceID string, req *UpdateMemoryRequest) (*Memory, error)
	RestoreForSpace(id, spaceID string) (*Memory, error)
	DeleteForSpace(id, spaceID string) error
	SearchForSpace(req *SearchMemoryRequest, spaceID string) ([]Memory, error)
	VectorSearchForSpace(req *VectorSearchRequest, spaceID string) ([]VectorSearchResult, error)
	HybridSearchForSpace(req *VectorSearchRequest, spaceID string) ([]HybridSearchResult, error)
	RebuildEmbeddingsForSpace(spaceID string) (map[string]interface{}, error)
	RecordUseForSpace(id, spaceID string) (*Memory, error)
	DeleteAllForSpace(characterID, spaceID string) error
	GetTimelineForSpace(page, pageSize int, spaceID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error)
	CheckConflictForSpace(req *CheckConflictRequest, spaceID string) (*CheckConflictResponse, error)
	ResolveConflictForSpace(req *ResolveConflictRequest, spaceID string) (*ResolveConflictResponse, error)
	ListCandidatesForSpace(spaceID string) []MemoryCandidate
	RebuildIndexForSpace(spaceID string) (map[string]interface{}, error)
	UpdateCandidateForSpace(id, spaceID string, req *UpdateCandidateRequest) (*MemoryCandidate, error)
	DeleteCandidateForSpace(id, spaceID string) error
	GenerateCandidatesForSpace(conversationID, spaceID string) ([]MemoryCandidate, error)
	SubmitCandidateForSpace(req *SubmitCandidateRequest, spaceID string) (*MemoryCandidate, error)
	AcceptCandidateForSpace(id, spaceID string) (*Memory, error)
	RejectCandidateForSpace(id, spaceID string) error
	BatchAcceptCandidatesForSpace(ids []string, spaceID string) ([]Memory, error)
	GetVectorStatusForSpace(spaceID string) map[string]interface{}
	BatchVerifyForSpace(ids []string, status, spaceID string) error
	BatchSetImportanceForSpace(ids []string, importance int, spaceID string) error
	GetRankedMemoriesForSpace(characterID, spaceID, query string, limit int) ([]RankedMemory, error)
	RetrieveStatsForSpace(spaceID string) (map[string]interface{}, error)
}

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) scoped(c *gin.Context) (userScopedMemoryService, string, bool) {
	svc, ok := h.service.(userScopedMemoryService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "memory service does not provide user-scoped operations", nil)
		return nil, "", false
	}
	return svc, requestidentity.ResolveGin(c), true
}

func (h *Handler) List(c *gin.Context) {
	var q MemoryListQuery
	_ = c.ShouldBindQuery(&q)
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	resp, err := svc.ListForSpace(q, spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.CreateForSpace(&req, spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.UpdateForSpace(c.Param("id"), spaceID, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆更新成功", m)
}

func (h *Handler) Restore(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.RestoreForSpace(c.Param("id"), spaceID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已恢复", m)
}

func (h *Handler) Delete(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteForSpace(c.Param("id"), spaceID); err != nil {
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.SearchForSpace(&req, spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.VectorSearchForSpace(&req, spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.HybridSearchForSpace(&req, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) RebuildEmbeddings(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.RebuildEmbeddingsForSpace(spaceID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "嵌入重建完成", result)
}

func (h *Handler) RecordUse(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.RecordUseForSpace(c.Param("id"), spaceID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "使用已记录", m)
}

func (h *Handler) DeleteAll(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteAllForSpace(c.Query("characterId"), spaceID); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "所有记忆已删除", nil)
}

func (h *Handler) Timeline(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, total, err := svc.GetTimelineForSpace(page, pageSize, spaceID, c.Query("source"), c.Query("memoryType"), c.Query("type"))
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.CheckConflictForSpace(&req, spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.ResolveConflictForSpace(&req, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ExtractCandidates(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items := svc.ListCandidatesForSpace(spaceID)
	util.SuccessResponse(c, gin.H{"candidates": items, "total": len(items)})
}

func (h *Handler) RebuildIndex(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	result, err := svc.RebuildIndexForSpace(spaceID)
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	item, err := svc.UpdateCandidateForSpace(c.Param("id"), spaceID, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "候选记忆已更新", item)
}

func (h *Handler) DeleteCandidate(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.DeleteCandidateForSpace(c.Param("id"), spaceID); err != nil {
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.GenerateCandidatesForSpace(req.ConversationID, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"candidates": items, "generated": len(items)})
}

func (h *Handler) ListCandidates(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items := svc.ListCandidatesForSpace(spaceID)
	util.SuccessResponse(c, gin.H{"candidates": items, "total": len(items)})
}

func (h *Handler) AcceptCandidate(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	m, err := svc.AcceptCandidateForSpace(c.Param("id"), spaceID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已保存", m)
}

func (h *Handler) RejectCandidate(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.RejectCandidateForSpace(c.Param("id"), spaceID); err != nil {
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.BatchAcceptCandidatesForSpace(req.IDs, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "批量保存完成", gin.H{"accepted": len(items), "memories": items})
}

func (h *Handler) VectorStatus(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	util.SuccessResponse(c, svc.GetVectorStatusForSpace(spaceID))
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.BatchVerifyForSpace(req.IDs, req.Status, spaceID); err != nil {
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	if err := svc.BatchSetImportanceForSpace(req.IDs, req.Importance, spaceID); err != nil {
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
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	items, err := svc.GetRankedMemoriesForSpace(c.Query("characterId"), spaceID, c.Query("query"), limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) RetrieveStats(c *gin.Context) {
	svc, spaceID, ok := h.scoped(c)
	if !ok {
		return
	}
	stats, err := svc.RetrieveStatsForSpace(spaceID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, stats)
}
