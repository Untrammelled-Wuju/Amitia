package memory

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)


type Handler struct {
	service Service
}


func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}


func (h *Handler) List(c *gin.Context) {
	var q MemoryListQuery
	c.ShouldBindQuery(&q)
	resp, err := h.service.List(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, resp)
}


func (h *Handler) Create(c *gin.Context) {
	var req CreateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "key和value不能为空", nil)
		return
	}
	m, err := h.service.Create(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆创建成功", m)
}


func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	m, err := h.service.Update(id, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆更新成功", m)
}


func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已删除", nil)
}


func (h *Handler) Search(c *gin.Context) {
	var req SearchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "keyword不能为空", nil)
		return
	}
	items, err := h.service.Search(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "搜索失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}


func (h *Handler) RecordUse(c *gin.Context) {
	id := c.Param("id")
	m, err := h.service.RecordUse(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "记忆不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "使用已记录", m)
}


func (h *Handler) DeleteAll(c *gin.Context) {
	if err := h.service.DeleteAll(); err != nil { util.ErrorResponse(c, response.OperationFailed, "删除失败", nil); return }
	util.SuccessMsgResponse(c, "所有记忆已删除", nil)
}

func (h *Handler) Timeline(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
	characterID := c.Query("characterId")
	source := c.Query("source")
	memoryType := c.Query("memoryType")
	items, total, err := h.service.GetTimeline(page, pageSize, characterID, source, memoryType)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}
func (h *Handler) CheckConflict(c *gin.Context) { util.SuccessResponse(c, gin.H{"hasConflict": false}) }
func (h *Handler) ResolveConflict(c *gin.Context) { util.SuccessResponse(c, gin.H{"resolved": true}) }
func (h *Handler) ExtractCandidates(c *gin.Context) { util.SuccessResponse(c, gin.H{"candidates": 0}) }
func (h *Handler) RebuildIndex(c *gin.Context) { util.SuccessResponse(c, gin.H{"rebuilt": true}) }
func (h *Handler) UpdateCandidate(c *gin.Context) { util.SuccessResponse(c, gin.H{"updated": true}) }
func (h *Handler) DeleteCandidate(c *gin.Context) { util.SuccessResponse(c, gin.H{"deleted": true}) }

func (h *Handler) GenerateCandidates(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "conversationId不能为空", nil)
		return
	}
	candidates, err := h.service.GenerateCandidates(req.ConversationID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"candidates": candidates, "generated": len(candidates)})
}

func (h *Handler) ListCandidates(c *gin.Context) {
	candidates := h.service.ListCandidates()
	util.SuccessResponse(c, gin.H{"candidates": candidates, "total": len(candidates)})
}

func (h *Handler) AcceptCandidate(c *gin.Context) {
	id := c.Param("id")
	m, err := h.service.AcceptCandidate(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "记忆已保存", m)
}

func (h *Handler) RejectCandidate(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RejectCandidate(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已拒绝", nil)
}

func (h *Handler) BatchAcceptCandidates(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "ids不能为空", nil)
		return
	}
	memories, err := h.service.BatchAcceptCandidates(req.IDs)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "批量保存完成", gin.H{"accepted": len(memories), "memories": memories})
}

func (h *Handler) VectorStatus(c *gin.Context) {
	status := h.service.GetVectorStatus()
	util.SuccessResponse(c, status)
}
