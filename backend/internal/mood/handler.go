// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct{ service Service }

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) List(c *gin.Context) { util.SuccessResponse(c, h.service.List()) }
func (h *Handler) GetByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetByConversation(c.Param("id")))
}
func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !h.service.Delete(id) {
		util.ErrorResponse(c, response.NotFound, "心情记录不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "已删除", nil)
}
func (h *Handler) DeleteByConversation(c *gin.Context) {
	h.service.DeleteByConversation(c.Param("id"))
	util.SuccessMsgResponse(c, "已清空", nil)
}
