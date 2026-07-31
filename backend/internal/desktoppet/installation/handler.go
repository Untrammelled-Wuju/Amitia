// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(svc Service) *Handler { return &Handler{service: svc} }

type installPackagePayload struct {
	CharacterID string `json:"character_id"`
}

type updateDefaultActionPayload struct {
	ActionKey string `json:"action_key"`
}

func (h *Handler) InstallPackage(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Sunset", "2026-12-31")
	c.Header("Link", `</api/desktop-pets/releases>; rel="successor-version"`)
	packageID := c.Param("packageId")
	if packageID == "" {
		util.ErrorResponse(c, response.InvalidParams, "资源包 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	var payload installPackagePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "角色 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	inst, err := h.service.InstallPackage(packageID, userID, payload.CharacterID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "安装成功", inst)
}

func (h *Handler) ListInstallations(c *gin.Context) {
	userID := desktoppet.ResolveUserID(c)
	items, err := h.service.ListInstallations(userID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	if items == nil {
		items = []*Installation{}
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) GetInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.CheckInstallationOwnership(installationID, userID); err != nil {
		writeInstallationError(c, err)
		return
	}
	inst, err := h.service.GetInstallation(installationID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessResponse(c, inst)
}

func (h *Handler) EnableInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.EnableInstallation(userID, installationID); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已启用", nil)
}

func (h *Handler) DisableInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.DisableInstallation(userID, installationID); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已停用", nil)
}

func (h *Handler) UpdateDefaultAction(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.CheckInstallationOwnership(installationID, userID); err != nil {
		writeInstallationError(c, err)
		return
	}
	var payload updateDefaultActionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeActionNotFound})
		return
	}
	if payload.ActionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeActionNotFound})
		return
	}
	if err := h.service.UpdateDefaultAction(installationID, payload.ActionKey); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "默认动作已更新", nil)
}

func (h *Handler) UpdateRuntimeSettings(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload UpdateRuntimeSettingsRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	settings, err := h.service.UpdateRuntimeSettings(userID, installationID, &payload)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "运行配置已更新", settings)
}

func (h *Handler) Recenter(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.Recenter(userID, installationID); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已重置位置", nil)
}

func (h *Handler) PlayAction(c *gin.Context) {
	installationID := c.Param("installationId")
	actionKey := c.Param("actionKey")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	if actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeActionNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.PlayAction(userID, installationID, actionKey); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "动作已触发", nil)
}

func (h *Handler) Uninstall(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.service.Uninstall(userID, installationID); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已卸载", nil)
}

func writeInstallationError(c *gin.Context, err error) {
	var ie *InstallationError
	if errors.As(err, &ie) {
		httpCode := mapInstallationErrorCode(ie.Code)
		util.ErrorResponse(c, httpCode, ie.Message, gin.H{"errorCode": ie.Code})
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func mapInstallationErrorCode(code string) int {
	switch code {
	case ErrCodeInstallationNotFound,
		ErrCodeRuntimeSettingsNotFound,
		ErrCodeCharacterNotFound,
		ErrCodeActionNotFound:
		return response.NotFound
	case ErrCodePackageNotReady,
		ErrCodePackagePathTraversal,
		ErrCodePackageSymlinkEscape,
		ErrCodePackageExecutableFound,
		ErrCodePackageHashMismatch,
		ErrCodePackageDefaultActionInvalid:
		return response.InvalidParams
	case ErrCodeInstallationDuplicate,
		ErrCodePetNotEnabled,
		ErrCodeDefaultActionNotIdle,
		ErrCodeInstallationInvalid,
		ErrCodePurgeNotConfirmed,
		ErrCodeRevisionConflict,
		ErrCodePackageQualityGateBlocked:
		return response.BusinessError
	case ErrCodeInstallationFailed:
		return response.InternalError
	default:
		return response.InternalError
	}
}
