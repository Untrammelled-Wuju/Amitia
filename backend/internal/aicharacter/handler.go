// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package aicharacter

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct{ service Service }

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) GetDefault(c *gin.Context) { util.SuccessResponse(c, h.service.GetDefault()) }
func (h *Handler) GetCharacter(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetCharacter(c.Param("id")))
}
func (h *Handler) GetPresets(c *gin.Context) { util.SuccessResponse(c, h.service.GetPresets()) }
func (h *Handler) GetPreset(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetPreset(c.Param("id")))
}
func (h *Handler) SetDefault(c *gin.Context) {
	util.SuccessResponse(c, h.service.SetDefault(c.Param("id")))
}
func (h *Handler) ResetDefault(c *gin.Context) { util.SuccessResponse(c, h.service.ResetDefault()) }
func (h *Handler) PreviewPrompt(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.PreviewPrompt(body))
}
func (h *Handler) SaveCharacter(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.SaveCharacter(body))
}
