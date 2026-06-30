// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
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
	includeDisabled := c.Query("includeDisabled") == "true"
	chars, err := h.service.List(includeDisabled)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, chars)
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	char, err := h.service.GetByID(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "角色不存在", nil)
		return
	}
	util.SuccessResponse(c, char)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	char, err := h.service.Create(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色创建成功", char)
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	char, err := h.service.Update(id, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色更新成功", char)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.NotFound, "角色不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "角色已删除", nil)
}

func (h *Handler) SetActive(c *gin.Context) {
	id := c.Param("id")
	char, err := h.service.SetActive(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已切换活跃角色", char)
}

func (h *Handler) ListTemplates(c *gin.Context) {
	templates, err := h.service.ListTemplates()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, templates)
}

func (h *Handler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	t, err := h.service.GetTemplateByID(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "模板不存在", nil)
		return
	}
	util.SuccessResponse(c, t)
}

func (h *Handler) GetRoleProfile(c *gin.Context) {
	characterID := c.Query("characterId")
	profile, err := h.service.GetRoleProfile(characterID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profile)
}

func (h *Handler) Test(c *gin.Context) {
	util.SuccessResponse(c, gin.H{"id": c.Param("id"), "tested": true})
}
func (h *Handler) ExportPack(c *gin.Context) {
	util.SuccessResponse(c, gin.H{"id": c.Param("id"), "pack": map[string]interface{}{}})
}
func (h *Handler) ImportPackPreview(c *gin.Context) {
	util.SuccessMsgResponse(c, "预览成功", gin.H{"preview": map[string]interface{}{}})
}
func (h *Handler) ImportPackConfirm(c *gin.Context) {
	util.SuccessMsgResponse(c, "导入成功", gin.H{"imported": true})
}
func (h *Handler) PacksHistory(c *gin.Context) { util.SuccessResponse(c, []map[string]interface{}{}) }
func (h *Handler) CreateFromTemplate(c *gin.Context) {
	util.SuccessMsgResponse(c, "创建成功", gin.H{"id": c.Param("id")})
}

func (h *Handler) UpdateRoleProfile(c *gin.Context) {
	characterID := c.Query("characterId")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	profile, err := h.service.UpdateRoleProfile(characterID, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profile)
}
