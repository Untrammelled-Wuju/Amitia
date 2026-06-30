// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package vision

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct{ service Service }

func NewHandler(svc Service) *Handler { return &Handler{service: svc} }

func (h *Handler) List(c *gin.Context) {
	configs, err := h.service.List()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, configs)
}

func (h *Handler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cfg, err := h.service.GetByID(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, cfg)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateVisionConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	cfg, err := h.service.Create(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "视觉模型配置已创建", cfg)
}

func (h *Handler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	cfg, err := h.service.Update(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "视觉模型配置已更新", cfg)
}

func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "视觉模型配置已删除", nil)
}

func (h *Handler) Activate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cfg, err := h.service.Activate(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已设为默认视觉模型", cfg)
}

func (h *Handler) TestConnection(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	result, err := h.service.TestConnection(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	if result["success"] == true {
		util.SuccessMsgResponse(c, "连接测试通过", result)
	} else {
		util.ErrorResponse(c, response.OperationFailed, fmt.Sprint(result["message"]), result)
	}
}
