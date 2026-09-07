// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct{ service Service }

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) List(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListForUser(requestidentity.ResolveGin(c, "")))
}

func (h *Handler) GetByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetByConversationForUser(c.Param("id"), requestidentity.ResolveGin(c, "")))
}

func (h *Handler) Delete(c *gin.Context) {
	deleted := h.service.DeleteForUser(c.Param("id"), requestidentity.ResolveGin(c, ""))
	util.SuccessResponse(c, map[string]interface{}{"deleted": deleted})
}

func (h *Handler) DeleteByConversation(c *gin.Context) {
	deleted := h.service.DeleteByConversationForUser(c.Param("id"), requestidentity.ResolveGin(c, ""))
	util.SuccessResponse(c, map[string]interface{}{"deleted": deleted})
}
