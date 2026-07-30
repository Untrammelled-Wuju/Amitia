// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	svc QualityService
}

func NewHandler(svc QualityService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetEvaluation(c *gin.Context) {
	evaluationID := c.Param("evaluationId")
	if evaluationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "评估 ID 为空", nil)
		return
	}

	result, err := h.svc.GetEvaluation(c.Request.Context(), evaluationID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if result == nil {
		util.ErrorResponse(c, response.NotFound, "评估结果不存在", nil)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) GetActiveActionQuality(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", nil)
		return
	}
	if actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", nil)
		return
	}

	result, err := h.svc.GetActiveActionQuality(c.Request.Context(), processingTaskID, actionKey)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if result == nil {
		util.ErrorResponse(c, response.NotFound, "未找到活跃的质量评估", nil)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) Reevaluate(c *gin.Context) {
	evaluationID := c.Param("evaluationId")
	if evaluationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "评估 ID 为空", nil)
		return
	}

	var payload struct {
		QualityMode string `json:"qualityMode"`
	}
	_ = c.ShouldBindJSON(&payload)

	req := ReevaluateRequest{
		EvaluationID: evaluationID,
		QualityMode:  payload.QualityMode,
	}

	eval, err := h.svc.Reevaluate(c.Request.Context(), req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}

	util.SuccessMsgResponse(c, "已创建重新评估", eval)
}

func (h *Handler) GetTaskGate(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", nil)
		return
	}

	result, err := h.svc.GetTaskGate(c.Request.Context(), processingTaskID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) ListProblemFrames(c *gin.Context) {
	evaluationID := c.Param("evaluationId")
	if evaluationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "评估 ID 为空", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	frames, total, err := h.svc.ListProblemFrames(c.Request.Context(), evaluationID, page, pageSize)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, gin.H{
		"items":    frames,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) ListFindings(c *gin.Context) {
	evaluationID := c.Param("evaluationId")
	if evaluationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "评估 ID 为空", nil)
		return
	}

	severity := c.Query("severity")
	dimension := c.Query("dimension")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	findings, total, err := h.svc.ListFindings(c.Request.Context(), evaluationID, severity, dimension, page, pageSize)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, gin.H{
		"items":    findings,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}
