// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
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
	spaceID := requestidentity.ResolveGin(c)
	result, err := h.svc.QueryNeighbors(id, depth, spaceID)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
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
	result, err := h.svc.FindPathsForSpace(from, to, maxDepth, requestidentity.ResolveGin(c))
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Stats(c *gin.Context) {
	spaceID := requestidentity.ResolveGin(c)
	result, err := h.svc.GetStats(spaceID)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) DeleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteNodeForSpace(id, requestidentity.ResolveGin(c)); err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, nil)
}

func (h *Handler) AllNodes(c *gin.Context) {
	spaceID := requestidentity.ResolveGin(c)
	result, err := h.svc.GetAllNodes(spaceID)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) AllEdges(c *gin.Context) {
	spaceID := requestidentity.ResolveGin(c)
	result, err := h.svc.GetAllEdges(spaceID)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
