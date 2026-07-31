package installation

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type ReleaseHandler struct {
	svc ReleaseService
}

func NewReleaseHandler(svc ReleaseService) *ReleaseHandler {
	return &ReleaseHandler{svc: svc}
}

type buildReleasePayload struct {
	ProcessingTaskID string   `json:"processingTaskId"`
	PetID            string   `json:"petId"`
	Version          string   `json:"version"`
	DefaultAction    string   `json:"defaultAction"`
	IncludedActions  []string `json:"includedActions"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

func (h *ReleaseHandler) BuildRelease(c *gin.Context) {
	var payload buildReleasePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.ProcessingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	req := &BuildReleaseRequest{
		ProcessingTaskID: payload.ProcessingTaskID,
		UserID:           userID,
		PetID:            payload.PetID,
		Version:          payload.Version,
		DefaultAction:    payload.DefaultAction,
		IncludedActions:  payload.IncludedActions,
		IdempotencyKey:   payload.IdempotencyKey,
	}
	result, err := h.svc.BuildRelease(req)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "Release 构建成功", gin.H{
		"release":    result.Release,
		"validation": result.Validation,
	})
}

type importPackagePayload struct {
	CharacterID      string   `json:"characterId"`
	PackageName      string   `json:"packageName"`
	LegacyPackageID  string   `json:"legacyPackageId"`
	LegacyVersion    int      `json:"legacyVersion"`
	ImportStagingID  string   `json:"importStagingId"`
	DefaultAction    string   `json:"defaultAction"`
	CanvasWidth      int      `json:"canvasWidth"`
	CanvasHeight     int      `json:"canvasHeight"`
	IncludedActions  []string `json:"includedActions"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

func (h *ReleaseHandler) ImportPackage(c *gin.Context) {
	var payload importPackagePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.ImportStagingID == "" {
		util.ErrorResponse(c, response.InvalidParams, "导入暂存 ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	req := &ImportPackageRequest{
		UserID:          userID,
		CharacterID:     payload.CharacterID,
		PackageName:     payload.PackageName,
		LegacyPackageID: payload.LegacyPackageID,
		LegacyVersion:   payload.LegacyVersion,
		ImportStagingID: payload.ImportStagingID,
		DefaultAction:   payload.DefaultAction,
		CanvasWidth:     payload.CanvasWidth,
		CanvasHeight:    payload.CanvasHeight,
		IncludedActions: payload.IncludedActions,
		IdempotencyKey:  payload.IdempotencyKey,
	}
	result, err := h.svc.ImportPackage(req)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "包导入成功", result)
}

func (h *ReleaseHandler) ListReleases(c *gin.Context) {
	userID := desktoppet.ResolveUserID(c)
	releases, err := h.svc.ListReleases(userID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	if releases == nil {
		releases = []*PackageRelease{}
	}
	util.SuccessResponse(c, gin.H{"items": releases, "total": len(releases)})
}

func (h *ReleaseHandler) GetRelease(c *gin.Context) {
	releaseID := c.Param("releaseId")
	if releaseID == "" {
		util.ErrorResponse(c, response.InvalidParams, "Release ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.CheckReleaseOwnership(releaseID, userID); err != nil {
		writeInstallationError(c, err)
		return
	}
	release, err := h.svc.GetRelease(releaseID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	files, _ := h.svc.GetReleaseFiles(releaseID)
	util.SuccessResponse(c, gin.H{"release": release, "files": files})
}

func (h *ReleaseHandler) ListReleasesByPet(c *gin.Context) {
	petID := c.Param("petId")
	if petID == "" {
		util.ErrorResponse(c, response.InvalidParams, "宠物 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.CheckPetIdentityOwnership(petID, userID); err != nil {
		writeInstallationError(c, err)
		return
	}
	releases, err := h.svc.ListReleasesByPet(petID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	if releases == nil {
		releases = []*PackageRelease{}
	}
	util.SuccessResponse(c, gin.H{"items": releases, "total": len(releases)})
}

func (h *ReleaseHandler) ListPets(c *gin.Context) {
	userID := desktoppet.ResolveUserID(c)
	pets, err := h.svc.ListPetIdentities(userID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	if pets == nil {
		pets = []*PetIdentity{}
	}
	util.SuccessResponse(c, gin.H{"items": pets, "total": len(pets)})
}

func (h *ReleaseHandler) GetPet(c *gin.Context) {
	petID := c.Param("petId")
	if petID == "" {
		util.ErrorResponse(c, response.InvalidParams, "宠物 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.CheckPetIdentityOwnership(petID, userID); err != nil {
		writeInstallationError(c, err)
		return
	}
	pet, err := h.svc.GetPetIdentity(petID)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessResponse(c, pet)
}

type installReleasePayload struct {
	CharacterID    string `json:"characterId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *ReleaseHandler) InstallRelease(c *gin.Context) {
	petID := c.Param("petId")
	releaseID := c.Param("releaseId")
	if petID == "" || releaseID == "" {
		util.ErrorResponse(c, response.InvalidParams, "宠物 ID 或 Release ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload installReleasePayload
	_ = c.ShouldBindJSON(&payload)
	userID := desktoppet.ResolveUserID(c)
	inst, err := h.svc.InstallRelease(userID, petID, releaseID, payload.CharacterID, payload.IdempotencyKey)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "安装成功", inst)
}

type upgradePayload struct {
	TargetReleaseID string `json:"targetReleaseId"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

func (h *ReleaseHandler) UpgradeInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload upgradePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	if payload.TargetReleaseID == "" {
		util.ErrorResponse(c, response.InvalidParams, "目标 Release ID 为空", gin.H{"errorCode": ErrCodeInstallationFailed})
		return
	}
	userID := desktoppet.ResolveUserID(c)
	inst, err := h.svc.UpgradeInstallation(userID, installationID, payload.TargetReleaseID, payload.IdempotencyKey)
	if err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "升级成功", inst)
}

type idempotencyPayload struct {
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *ReleaseHandler) SwitchInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload idempotencyPayload
	_ = c.ShouldBindJSON(&payload)
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.SwitchInstallation(userID, installationID, payload.IdempotencyKey); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "切换成功", nil)
}

func (h *ReleaseHandler) RepairInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload idempotencyPayload
	_ = c.ShouldBindJSON(&payload)
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.RepairInstallation(userID, installationID, payload.IdempotencyKey); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "修复成功", nil)
}

func (h *ReleaseHandler) UninstallInstallation(c *gin.Context) {
	installationID := c.Param("installationId")
	if installationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "安装 ID 为空", gin.H{"errorCode": ErrCodeInstallationNotFound})
		return
	}
	var payload idempotencyPayload
	_ = c.ShouldBindJSON(&payload)
	userID := desktoppet.ResolveUserID(c)
	if err := h.svc.UninstallInstallation(userID, installationID, payload.IdempotencyKey); err != nil {
		writeInstallationError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "卸载成功", nil)
}

var _ = errors.New
