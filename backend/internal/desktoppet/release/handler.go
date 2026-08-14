package release

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct {
	svc            ReleaseService
	ownershipGuard security.OwnershipGuard
}

func NewHandler(svc ReleaseService, guard security.OwnershipGuard) *Handler {
	return &Handler{svc: svc, ownershipGuard: guard}
}

type buildReleasePayload struct {
	ProcessingTaskID string   `json:"processingTaskId"`
	PetID            string   `json:"petId"`
	CharacterID      string   `json:"characterId"`
	DefaultAction    string   `json:"defaultAction"`
	IncludedActions  []string `json:"includedActions"`
	ProfileID        string   `json:"profileId"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

func (h *Handler) BuildRelease(c *gin.Context) {
	var payload buildReleasePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数格式错误", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	if payload.ProcessingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID

	req := &BuildReleaseRequest{
		UserID:             userID,
		ProcessingTaskID:   payload.ProcessingTaskID,
		PetID:              payload.PetID,
		CharacterID:        payload.CharacterID,
		DefaultAction:      payload.DefaultAction,
		IncludedActionKeys: payload.IncludedActions,
		BuildProfileID:     payload.ProfileID,
		IdempotencyKey:     payload.IdempotencyKey,
	}
	result, err := h.svc.BuildRelease(c.Request.Context(), req)
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "构建成功", gin.H{
		"operation": result.Operation,
		"release":   result.Release,
		"snapshot":  result.Snapshot,
	})
}

func (h *Handler) GetBuildOperation(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "操作 ID 为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID

	op, err := h.svc.GetBuildOperation(c.Request.Context(), operationID, userID)
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, op)
}

func (h *Handler) CancelBuildOperation(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "操作 ID 为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID

	if err := h.svc.CancelBuildOperation(c.Request.Context(), operationID, userID); err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "已取消", nil)
}

func (h *Handler) ListReleases(c *gin.Context) {
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID

	releases, err := h.svc.ListReleases(c.Request.Context(), userID)
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"items": releases, "total": len(releases)})
}

func (h *Handler) ListReleasesForPet(c *gin.Context) {
	petID := c.Param("petId")
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actorID

	releases, err := h.svc.ListReleasesForPet(c.Request.Context(), userID, petID)
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"items": releases, "total": len(releases)})
}

func (h *Handler) GetRelease(c *gin.Context) {
	releaseID := c.Param("releaseId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRelease(c.Request.Context(), actor, releaseID); err != nil {
		writeReleaseOwnershipError(c, err)
		return
	}
	release, err := h.svc.GetRelease(c.Request.Context(), releaseID, string(actor.UserID))
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, release)
}

func (h *Handler) GetReleaseFiles(c *gin.Context) {
	releaseID := c.Param("releaseId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRelease(c.Request.Context(), actor, releaseID); err != nil {
		writeReleaseOwnershipError(c, err)
		return
	}
	files, err := h.svc.GetReleaseFiles(c.Request.Context(), releaseID, string(actor.UserID))
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{"items": files})
}

func (h *Handler) ArchiveRelease(c *gin.Context) {
	releaseID := c.Param("releaseId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRelease(c.Request.Context(), actor, releaseID); err != nil {
		writeReleaseOwnershipError(c, err)
		return
	}
	if err := h.svc.ArchiveRelease(c.Request.Context(), releaseID, string(actor.UserID)); err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "已归档", nil)
}

func (h *Handler) RevokeRelease(c *gin.Context) {
	releaseID := c.Param("releaseId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRelease(c.Request.Context(), actor, releaseID); err != nil {
		writeReleaseOwnershipError(c, err)
		return
	}
	reason := c.Query("reason")
	if err := h.svc.RevokeRelease(c.Request.Context(), releaseID, string(actor.UserID), reason); err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "已撤销", nil)
}

func (h *Handler) GetPetIdentity(c *gin.Context) {
	petID := c.Query("petId")
	if petID == "" {
		util.ErrorResponse(c, response.InvalidParams, "petId 不能为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	identity, err := h.svc.GetPetIdentity(c.Request.Context(), string(actor.UserID), petID)
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, identity)
}

func (h *Handler) DownloadRelease(c *gin.Context) {
	releaseID := c.Param("releaseId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireRelease(c.Request.Context(), actor, releaseID); err != nil {
		writeReleaseOwnershipError(c, err)
		return
	}
	release, err := h.svc.GetRelease(c.Request.Context(), releaseID, string(actor.UserID))
	if err != nil {
		writeReleaseError(c, err)
		return
	}
	util.SuccessResponse(c, gin.H{
		"downloadUrl": "/api/v2/releases/" + releaseID + "/archive",
		"release":     release,
	})
}

func writeReleaseOwnershipError(c *gin.Context, err error) {
	if ownErr := security.MapOwnershipError(err); ownErr != nil {
		util.ErrorResponse(c, ownErr.Code, ownErr.Msg, gin.H{"errorCode": ownErr.ErrCode})
		return
	}
	writeReleaseError(c, err)
}

func writeReleaseError(c *gin.Context, err error) {
	var releaseErr *ReleaseError
	if errors.As(err, &releaseErr) {
		switch releaseErr.Code {
		case "INVALID_REQUEST", "IDEMPOTENCY_CONFLICT", "QUALITY_GATE_READ_FAILED":
			util.ErrorResponse(c, response.InvalidParams, releaseErr.Msg, gin.H{"errorCode": releaseErr.Code})
		case "OPERATION_NOT_FOUND", "RELEASE_NOT_FOUND", "PET_IDENTITY_NOT_FOUND":
			util.ErrorResponse(c, response.NotFound, releaseErr.Msg, gin.H{"errorCode": releaseErr.Code})
		case "OPERATION_CONFLICT", "RELEASE_INTEGRITY_MISMATCH":
			util.ErrorResponse(c, response.BusinessError, releaseErr.Msg, gin.H{"errorCode": releaseErr.Code})
		case "LEGACY_PACKAGE_WRITE_DISABLED":
			util.ErrorResponse(c, response.BusinessError, releaseErr.Msg, gin.H{"errorCode": releaseErr.Code, "successor": "/api/v2/releases/build"})
		default:
			util.ErrorResponse(c, response.InternalError, releaseErr.Msg, gin.H{"errorCode": releaseErr.Code})
		}
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, gorm.ErrRecordNotFound) {
		util.ErrorResponse(c, response.NotFound, "资源不存在", gin.H{"errorCode": "NOT_FOUND"})
		return
	}
	util.ErrorResponse(c, response.InternalError, "服务器内部错误", gin.H{"errorCode": "INTERNAL_ERROR"})
}
