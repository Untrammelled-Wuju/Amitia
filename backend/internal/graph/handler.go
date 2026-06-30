// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Neighbors(c *gin.Context) {
	id := c.Param("id")
	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "2"))
	userID := c.DefaultQuery("userId", "default")
	result, err := h.svc.QueryNeighbors(id, depth, userID)
	if err != nil {
		util.ErrorResponse(c, 500, "查询邻居失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) FindPath(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	maxDepth, _ := strconv.Atoi(c.DefaultQuery("maxDepth", "4"))
	if from == "" || to == "" {
		util.ErrorResponse(c, 400, "from和to不能为空", nil)
		return
	}
	result, err := h.svc.FindPaths(from, to, maxDepth)
	if err != nil {
		util.ErrorResponse(c, 500, "路径查询失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Stats(c *gin.Context) {
	userID := c.DefaultQuery("userId", "default")
	result, err := h.svc.GetStats(userID)
	if err != nil {
		util.ErrorResponse(c, 500, "统计查询失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DeleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteNode(id); err != nil {
		util.ErrorResponse(c, 500, "删除节点失败", nil)
		return
	}
	util.SuccessResponse(c, nil)
}

func (h *Handler) AllNodes(c *gin.Context) {
	userID := c.DefaultQuery("userId", "default")
	result, err := h.svc.GetAllNodes(userID)
	if err != nil {
		util.ErrorResponse(c, 500, "查询节点失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) AllEdges(c *gin.Context) {
	userID := c.DefaultQuery("userId", "default")
	result, err := h.svc.GetAllEdges(userID)
	if err != nil {
		util.ErrorResponse(c, 500, "查询边失败", nil)
		return
	}
	util.SuccessResponse(c, result)
}
