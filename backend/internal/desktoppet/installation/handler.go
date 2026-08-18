// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service        Service
	ownershipGuard security.OwnershipGuard
}

func NewHandler(svc Service, guard security.OwnershipGuard) *Handler {
	return &Handler{service: svc, ownershipGuard: guard}
}

func requireDeviceID(c *gin.Context) (string, bool) {
	deviceID := strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
	if deviceID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少设备ID", gin.H{"errorCode": "DEVICE_ID_REQUIRED"})
		return "", false
	}
	return deviceID, true
}

type installPackagePayload struct {
	CharacterID string `json:"character_id"`
}

type updateDefaultActionPayload struct {
	ActionKey string `json:"action_key"`
}

type switchReleasePayload struct {
	TargetReleaseID string `json:"targetReleaseId"`
}

type downgradePayload struct {
	TargetReleaseID string `json:"targetReleaseId"`
	SafetyConfirm   bool   `json:"safetyConfirm"`
}

type operationStatusResponse struct {
	Operation *operation.InstallationOperation `json:"operation"`
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
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "角色 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
	inst, err := h.service.InstallPackage(packageID, userID, payload.CharacterID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "安装成功", inst)
}

func (h *Handler) ListInstallations(c *gin.Context) {
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.service.EnableInstallation(string(actor.UserID), installationID); err != nil {
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.service.DisableInstallation(string(actor.UserID), installationID); err != nil {
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	var payload updateDefaultActionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeInstallationFailed})
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
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	settings, err := h.service.UpdateRuntimeSettings(string(actor.UserID), installationID, &payload)
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.service.Recenter(string(actor.UserID), installationID); err != nil {
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.service.PlayAction(string(actor.UserID), installationID, actionKey); err != nil {
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.ownershipGuard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.service.Uninstall(string(actor.UserID), installationID); err != nil {
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
	util.ErrorResponse(c, response.InternalError, "服务器内部错误", nil)
}

func writeInstallationOwnershipError(c *gin.Context, err error) {
	if ownErr := security.MapOwnershipError(err); ownErr != nil {
		util.ErrorResponse(c, ownErr.Code, ownErr.Msg, gin.H{"errorCode": ownErr.ErrCode})
		return
	}
	writeInstallationError(c, err)
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

type CoordinatorHandler struct {
	coordinator coordinator.InstallationCoordinator
	repo        RepositoryV2
	guard       security.OwnershipGuard
}

func NewCoordinatorHandler(coord coordinator.InstallationCoordinator, repo RepositoryV2, guard security.OwnershipGuard) *CoordinatorHandler {
	return &CoordinatorHandler{coordinator: coord, repo: repo, guard: guard}
}

func (h *CoordinatorHandler) buildDeviceCtx(c *gin.Context, actorID string) device.DeviceContext {
	deviceID := strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
	return device.DeviceContext{
		UserID:   actorID,
		DeviceID: deviceID,
	}
}

func (h *CoordinatorHandler) InstallPackage(c *gin.Context) {
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
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "角色 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceCtx := h.buildDeviceCtx(c, actorID)
	result, err := h.coordinator.Install(c.Request.Context(), coordinator.InstallRequest{
		DeviceCtx:       deviceCtx,
		TargetReleaseID: packageID,
		CharacterID:     payload.CharacterID,
		IdempotencyKey:  strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "安装任务已创建", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) ListInstallations(c *gin.Context) {
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	items, err := h.repo.ListInstallationsForUserDevice(actorID, deviceID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	if items == nil {
		items = []*Installation{}
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func (h *CoordinatorHandler) GetInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	inst, err := h.repo.GetInstallationForUserDevice(string(actor.UserID), deviceID, installationID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessResponse(c, inst)
}

func (h *CoordinatorHandler) EnableInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Enable(c.Request.Context(), coordinator.EnableDisableRequest{
		DeviceCtx: device.DeviceContext{
			UserID:   string(actor.UserID),
			DeviceID: deviceID,
		},
		InstallationID: installationID,
		IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已启用", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) DisableInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Disable(c.Request.Context(), coordinator.EnableDisableRequest{
		DeviceCtx: device.DeviceContext{
			UserID:   string(actor.UserID),
			DeviceID: deviceID,
		},
		InstallationID: installationID,
		IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已停用", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) UpdateDefaultAction(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	var payload updateDefaultActionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeActionNotFound})
		return
	}
	if payload.ActionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeActionNotFound})
		return
	}
	result, err := h.coordinator.ChangeDefaultAction(c.Request.Context(), coordinator.DefaultActionRequest{
		DeviceCtx: device.DeviceContext{
			UserID:   string(actor.UserID),
			DeviceID: deviceID,
		},
		InstallationID:   installationID,
		DesiredActionKey: payload.ActionKey,
		IdempotencyKey:   strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "默认动作已更新", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) UpdateRuntimeSettings(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload UpdateRuntimeSettingsRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := payload.Validate(); err != nil {
		writeInstallationError(c, err)
		return
	}
	expectedRevision := 0
	if payload.ExpectedRevision != nil {
		expectedRevision = *payload.ExpectedRevision
	} else {
		currentSettings, err := h.repo.GetRuntimeSettingsForUserDevice(string(actor.UserID), deviceID, installationID)
		if err != nil {
			writeInstallationError(c, err)
			return
		}
		expectedRevision = currentSettings.SettingsRevision
	}
	result, err := h.coordinator.UpdateSettings(c.Request.Context(), coordinator.SettingsRequest{
		DeviceCtx: device.DeviceContext{
			UserID:   string(actor.UserID),
			DeviceID: deviceID,
		},
		InstallationID:   installationID,
		ExpectedRevision: expectedRevision,
		Updates:          payload.ToUpdates(),
		IdempotencyKey:   strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "运行配置已更新", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) Recenter(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Recenter(c.Request.Context(), coordinator.RecenterRequest{
		DeviceCtx: device.DeviceContext{
			UserID:   string(actor.UserID),
			DeviceID: deviceID,
		},
		InstallationID: installationID,
		IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "桌宠已重置位置", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) PlayAction(c *gin.Context) {
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
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	if err := h.coordinator.PlayAction(c.Request.Context(), device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID}, installationID, actionKey); err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "动作已触发", gin.H{"installationId": installationID, "actionKey": actionKey})
}

func (h *CoordinatorHandler) Uninstall(c *gin.Context) {
	installationID := c.Param("installationId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Uninstall(c.Request.Context(), coordinator.UninstallRequest{
		DeviceCtx:      device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID},
		InstallationID: installationID,
		IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "卸载任务已创建", gin.H{"operationId": result.OperationID, "status": result.Status})
}

func (h *CoordinatorHandler) SwitchRelease(c *gin.Context) {
	installationID := c.Param("installationId")
	var payload switchReleasePayload
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.TargetReleaseID) == "" {
		util.ErrorResponse(c, response.InvalidParams, "targetReleaseId 不能为空", gin.H{"errorCode": ErrCodeInstallationInvalid})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Switch(c.Request.Context(), coordinator.SwitchRequest{
		DeviceCtx:            device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID},
		SourceInstallationID: installationID,
		TargetReleaseID:      strings.TrimSpace(payload.TargetReleaseID),
		IdempotencyKey:       strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "切换任务已创建", result)
}

func (h *CoordinatorHandler) Upgrade(c *gin.Context) {
	installationID := c.Param("installationId")
	var payload switchReleasePayload
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.TargetReleaseID) == "" {
		util.ErrorResponse(c, response.InvalidParams, "targetReleaseId 不能为空", gin.H{"errorCode": ErrCodeInstallationInvalid})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Upgrade(c.Request.Context(), coordinator.UpgradeRequest{
		DeviceCtx:       device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID},
		InstallationID:  installationID,
		TargetReleaseID: strings.TrimSpace(payload.TargetReleaseID),
		IdempotencyKey:  strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "升级任务已创建", result)
}

func (h *CoordinatorHandler) Downgrade(c *gin.Context) {
	installationID := c.Param("installationId")
	var payload downgradePayload
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.TargetReleaseID) == "" {
		util.ErrorResponse(c, response.InvalidParams, "targetReleaseId 不能为空", gin.H{"errorCode": ErrCodeInstallationInvalid})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Downgrade(c.Request.Context(), coordinator.DowngradeRequest{
		DeviceCtx:       device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID},
		InstallationID:  installationID,
		TargetReleaseID: strings.TrimSpace(payload.TargetReleaseID),
		IdempotencyKey:  strings.TrimSpace(c.GetHeader("Idempotency-Key")),
		SafetyConfirm:   payload.SafetyConfirm,
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "降级任务已创建", result)
}

func (h *CoordinatorHandler) Repair(c *gin.Context) {
	installationID := c.Param("installationId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if _, err := h.guard.RequireInstallationStrict(c.Request.Context(), actor, deviceID, installationID); err != nil {
		writeInstallationOwnershipError(c, err)
		return
	}
	result, err := h.coordinator.Repair(c.Request.Context(), coordinator.RepairRequest{
		DeviceCtx:      device.DeviceContext{UserID: string(actor.UserID), DeviceID: deviceID},
		InstallationID: installationID,
		IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
	})
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "修复任务已创建", result)
}

func (h *CoordinatorHandler) GetOperationStatus(c *gin.Context) {
	operationID := c.Param("operationId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	op, err := h.coordinator.GetOperationStatus(c.Request.Context(), actorID, deviceID, operationID)
	if err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessResponse(c, operationStatusResponse{Operation: op})
}

func (h *CoordinatorHandler) CancelOperation(c *gin.Context) {
	operationID := c.Param("operationId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	deviceID, ok := requireDeviceID(c)
	if !ok {
		return
	}
	if err := h.coordinator.CancelOperation(c.Request.Context(), actorID, deviceID, operationID); err != nil {
		writeCoordinatorError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "取消请求已提交", gin.H{"operationId": operationID, "status": operation.OpStatusCancelRequested})
}

func writeCoordinatorError(c *gin.Context, err error) {
	var ie *InstallationError
	if errors.As(err, &ie) {
		writeInstallationError(c, err)
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
}
