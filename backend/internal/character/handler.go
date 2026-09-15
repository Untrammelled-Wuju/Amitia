// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/character/card"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type readScopedCharacterService interface {
	ListForSpace(includeDisabled bool, spaceID string) ([]Character, error)
	GetByIDForSpace(id, spaceID string) (*Character, error)
	ListPackHistoryForSpace(spaceID string) ([]map[string]interface{}, error)
	GetRoleProfileForSpace(characterID, spaceID string) (*RoleProfileResponse, error)
	ExportCardForSpace(characterID, format, spaceID string) (*CardExportResult, []byte, error)
	GetCardDataForSpace(characterID, spaceID string) (*card.CharacterCardData, error)
	UpdateCardDataForSpace(characterID string, cardData *card.CharacterCardData, spaceID string) error
}

type syncScopedCharacterService interface {
	CreateForSpace(req *CreateCharacterRequest, spaceID string) (*Character, error)
	UpdateForSpace(id string, req *UpdateCharacterRequest, spaceID string) (*Character, error)
	DeleteForSpace(id string, spaceID string) error
	SetActiveForSpace(id string, spaceID string) (*Character, error)
	UpdateRoleProfileForSpace(characterID string, updates map[string]interface{}, spaceID string) (*RoleProfileResponse, error)
	UpdateAvatarForSpace(id string, avatarURL string, spaceID string) error
	ImportCardForSpace(data []byte, filename string, confirm bool, spaceID string) (*CardImportResult, error)
}

type ChatTester interface {
	TestChat(ctx context.Context, characterID string, userMessage string) (string, error)
}

type Handler struct {
	service    Service
	chatTester ChatTester
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func (h *Handler) List(c *gin.Context) {
	includeDisabled := c.Query("includeDisabled") == "true"
	var chars []Character
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		chars, err = scoped.ListForSpace(includeDisabled, requestidentity.ResolveGin(c))
	} else {
		chars, err = h.service.List(includeDisabled)
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, chars)
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	var char *Character
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		char, err = scoped.GetByIDForSpace(id, requestidentity.ResolveGin(c))
	} else {
		char, err = h.service.GetByID(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "角色不存在", nil)
		return
	}
	util.SuccessResponse(c, char)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	var char *Character
	var err error
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		char, err = scoped.CreateForSpace(&req, requestidentity.ResolveGin(c))
	} else {
		char, err = h.service.Create(&req)
	}
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
	var char *Character
	var err error
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		char, err = scoped.UpdateForSpace(id, &req, requestidentity.ResolveGin(c))
	} else {
		char, err = h.service.Update(id, &req)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色更新成功", char)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	var err error
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		err = scoped.DeleteForSpace(id, requestidentity.ResolveGin(c))
	} else {
		err = h.service.Delete(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色已删除", nil)
}

func (h *Handler) SetActive(c *gin.Context) {
	id := c.Param("id")
	var char *Character
	var err error
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		char, err = scoped.SetActiveForSpace(id, requestidentity.ResolveGin(c))
	} else {
		char, err = h.service.SetActive(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已切换活跃角色", char)
}

func (h *Handler) ListTemplates(c *gin.Context) {
	templates, err := h.service.ListTemplates()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, templates)
}

func (h *Handler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	t, err := h.service.GetTemplateByID(id)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, t)
}

func (h *Handler) GetRoleProfile(c *gin.Context) {
	characterID := c.Query("characterId")
	var profile *RoleProfileResponse
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		profile, err = scoped.GetRoleProfileForSpace(characterID, requestidentity.ResolveGin(c))
	} else {
		profile, err = h.service.GetRoleProfile(characterID)
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profile)
}

func (h *Handler) Test(c *gin.Context) {
	id := c.Param("id")
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		if _, err := scoped.GetByIDForSpace(id, requestidentity.ResolveGin(c)); err != nil {
			util.ErrorResponse(c, response.NotFound, "角色不存在", nil)
			return
		}
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息不能为空", nil)
		return
	}
	if h.chatTester == nil {
		util.ErrorResponse(c, response.InternalError, "测试功能不可用", nil)
		return
	}
	reply, err := h.chatTester.TestChat(c.Request.Context(), id, req.Message)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"characterId": id, "reply": reply})
}
func (h *Handler) ExportPack(c *gin.Context) {
	characterID := c.Param("id")
	format := c.DefaultQuery("format", "v3_charx")

	var result *CardExportResult
	var data []byte
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		result, data, err = scoped.ExportCardForSpace(characterID, format, requestidentity.ResolveGin(c))
	} else {
		result, data, err = h.service.ExportCard(characterID, format)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}

	if c.Query("download") == "true" {
		c.Header("Content-Disposition", "attachment; filename="+result.Filename)
		c.Header("Content-Type", "application/octet-stream")
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) ImportPackPreview(c *gin.Context) {
	file, header, err := c.Request.FormFile("card")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少卡片文件（字段名: card）", nil)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	result, err := h.service.PreviewCard(data, header.Filename)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) ImportPackConfirm(c *gin.Context) {
	file, header, err := c.Request.FormFile("card")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少卡片文件（字段名: card）", nil)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	var result *CardImportResult
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		result, err = scoped.ImportCardForSpace(data, header.Filename, true, requestidentity.ResolveGin(c))
	} else {
		result, err = h.service.ImportCard(data, header.Filename, true)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}

	util.SuccessMsgResponse(c, "导入成功", result)
}

func (h *Handler) PacksHistory(c *gin.Context) {
	var history []map[string]interface{}
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		history, err = scoped.ListPackHistoryForSpace(requestidentity.ResolveGin(c))
	} else {
		history, err = h.service.ListPackHistory()
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, history)
}

func (h *Handler) CreateFromTemplate(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	created, err := h.service.CreateFromTemplate(
		c.Param("id"),
		body.Name,
		requestidentity.ResolveGin(c),
	)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "创建成功", created)
}

func (h *Handler) ExportCardV2(c *gin.Context) {
	characterID := c.Param("id")
	format := c.DefaultQuery("format", "v3_charx")

	var result *CardExportResult
	var data []byte
	var err error
	if scoped, ok := h.service.(readScopedCharacterService); ok {
		result, data, err = scoped.ExportCardForSpace(characterID, format, requestidentity.ResolveGin(c))
	} else {
		result, data, err = h.service.ExportCard(characterID, format)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}

	if c.Query("download") == "true" {
		c.Header("Content-Disposition", "attachment; filename="+result.Filename)
		c.Header("Content-Type", "application/octet-stream")
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) UpdateRoleProfile(c *gin.Context) {
	characterID := c.Query("characterId")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	var profile *RoleProfileResponse
	var err error
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		profile, err = scoped.UpdateRoleProfileForSpace(characterID, updates, requestidentity.ResolveGin(c))
	} else {
		profile, err = h.service.UpdateRoleProfile(characterID, updates)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profile)
}

func (h *Handler) UploadAvatar(c *gin.Context) {
	id := c.Param("id")
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少头像文件", nil)
		return
	}
	defer file.Close()

	avatarDir := filepath.Join("data", "avatars")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(avatarDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}

	avatarUrl := "/avatars/" + filename
	if scoped, ok := h.service.(syncScopedCharacterService); ok {
		err = scoped.UpdateAvatarForSpace(id, avatarUrl, requestidentity.ResolveGin(c))
	} else {
		err = h.service.UpdateAvatar(id, avatarUrl)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}

	util.SuccessResponse(c, gin.H{"avatarUrl": avatarUrl})
}
