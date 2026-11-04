package system

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) privacyDeletionOperation(c *gin.Context, operation string, input coordination.PrivacyDeletionRequest, id string) (map[string]any, error) {
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		return nil, err
	}
	port, ok := h.memoryConsumer.(business.PrivacyDeletionPort)
	if !ok {
		return nil, errors.New("统一数据删除入口不可用")
	}
	return port.PrivacyDeletion(c.Request.Context(), operation, input, id)
}

func (h *Handler) PrivacyDeletionPreview(c *gin.Context) {
	var input coordination.PrivacyDeletionRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.ShouldBindJSON(&input) != nil {
		privacyError(c, 400, "删除预览参数无效", nil)
		return
	}
	input.Target = nil
	input.ID = ""
	if input.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除预览入口不可用", nil)
			return
		}
		targets, err := port.PreviewMemoryDeletion(c.Request.Context(), input.TargetID)
		if err != nil {
			privacyError(c, 409, "无法确认记忆归属", err.Error())
			return
		}
		util.SuccessResponse(c, gin.H{"targets": targets})
		return
	}
	result, err := h.privacyDeletionOperation(c, "preview", input, "")
	if err != nil {
		privacyError(c, 409, "无法确认删除范围", err.Error())
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionRequest(c *gin.Context) {
	var payload struct {
		coordination.PrivacyDeletionRequest
		RequestID string `json:"requestId"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if c.ShouldBindJSON(&payload) != nil {
		privacyError(c, 400, "删除参数无效", nil)
		return
	}
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		privacyError(c, 409, "删除授权已变化", err.Error())
		return
	}
	if _, err := uuid.Parse(payload.RequestID); err != nil || payload.Target == nil {
		privacyError(c, 400, "删除缺少预览归属或有效请求编号", nil)
		return
	}
	if payload.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		result, err := port.DeleteMemoryIntent(c.Request.Context(), payload.TargetID, payload.Selection, *payload.Target, payload.RequestID)
		if err != nil {
			privacyError(c, 409, "记忆删除失败", err.Error())
			return
		}
		util.SuccessResponse(c, result)
		return
	}
	result, err := h.privacyDeletionOperation(c, "request", payload.PrivacyDeletionRequest, payload.RequestID)
	if err != nil {
		privacyError(c, 409, "删除未获数据持有方确认", err.Error())
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionStatus(c *gin.Context) {
	id := c.Param("id")
	if strings.HasPrefix(id, "owned-memory-delete.") {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		result, err := port.DeletionStatus(c.Request.Context(), id)
		if err != nil {
			privacyError(c, 409, "删除状态读取失败", err.Error())
			return
		}
		util.SuccessResponse(c, result)
		return
	}
	result, err := h.privacyDeletionOperation(c, "status", coordination.PrivacyDeletionRequest{ID: id}, "")
	if err != nil {
		privacyError(c, 409, "删除回执不属于当前数据范围或不可用", err.Error())
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionStats(c *gin.Context) {
	result, err := h.privacyDeletionOperation(c, "stats", coordination.PrivacyDeletionRequest{}, "")
	if err != nil {
		privacyError(c, 503, "删除统计未完整读取", err.Error())
		return
	}
	port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
	if !ok {
		privacyError(c, 503, "记忆删除统计不可用", nil)
		return
	}
	memory, err := port.DeletionStats(c.Request.Context())
	if err != nil {
		privacyError(c, 503, "记忆删除统计读取失败", err.Error())
		return
	}
	for key, value := range memory {
		result[key] = value
	}
	count := func(value any) int64 {
		switch number := value.(type) {
		case int:
			return int64(number)
		case int64:
			return number
		case float64:
			return int64(number)
		}
		return 0
	}
	result["total"] = count(result["tombstones"]) + count(memory["memoryDeletions"])
	result["totalTombstones"] = result["total"]
	result["pending"] = count(result["pending"]) + count(memory["memoryPending"])
	result["completed"] = count(result["completed"]) + count(memory["memoryCompleted"])
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionCleanup(c *gin.Context) {
	result, err := h.privacyDeletionOperation(c, "cleanup", coordination.PrivacyDeletionRequest{}, "")
	if err != nil {
		privacyError(c, 503, "删除清理未全部完成", err.Error())
		return
	}
	port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
	if !ok {
		privacyError(c, 503, "记忆清理入口不可用", nil)
		return
	}
	count, err := port.CleanupDeletions(c.Request.Context())
	if err != nil {
		privacyError(c, 503, "记忆索引清理未完成", err.Error())
		return
	}
	result["memoryCleaned"] = count
	util.SuccessResponse(c, result)
}

func (h *Handler) PrivacyDeletionSecurityTests(c *gin.Context) {
	var input coordination.PrivacyDeletionRequest
	if c.ShouldBindJSON(&input) != nil {
		privacyError(c, 400, "删除核验参数无效", nil)
		return
	}
	if input.TargetType == "memory" {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			privacyError(c, 503, "记忆核验入口不可用", nil)
			return
		}
		checks, err := port.DeletionChecks(c.Request.Context(), input.TargetID)
		if err != nil {
			privacyError(c, 409, "无法核验真实记忆删除状态", err.Error())
			return
		}
		util.SuccessResponse(c, checks)
		return
	}
	result, err := h.privacyDeletionOperation(c, "checks", input, "")
	if err != nil {
		privacyError(c, 409, "无法核验真实删除结果", err.Error())
		return
	}
	util.SuccessResponse(c, result["checks"])
}
