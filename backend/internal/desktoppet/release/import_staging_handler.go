// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type PackageImportPort interface {
	ImportPackage(
		ctx context.Context,
		req map[string]string,
	) (
		petID string,
		releaseID string,
		operationID string,
		err error,
	)
}

type ImportStagingHandler struct {
	registry  *security.PathRootRegistry
	repo      security.ImportStagingRepository
	ownership security.OwnershipGuard
	inspector interface {
		InspectAndMarkReady(ctx context.Context, staging *security.ImportStaging) error
	}
	importer PackageImportPort
}

func NewImportStagingHandler(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
	ownership security.OwnershipGuard,
	inspector interface {
		InspectAndMarkReady(ctx context.Context, staging *security.ImportStaging) error
	},
	packageImporter PackageImportPort,
) *ImportStagingHandler {
	return &ImportStagingHandler{
		registry:  registry,
		repo:      repo,
		ownership: ownership,
		inspector: inspector,
		importer:  packageImporter,
	}
}

const (
	MaxImportUploadSize = 200 * 1024 * 1024
	ImportStagingTTL    = 30 * time.Minute
)

type consumeStagingPayload struct {
	StagingID string `json:"stagingId" binding:"required"`
}

func (h *ImportStagingHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少上传文件", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		util.ErrorResponse(c, response.InvalidParams, "文件为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	if header.Size > MaxImportUploadSize {
		util.ErrorResponse(c, response.InvalidParams, "文件超过大小限制", gin.H{"errorCode": "FILE_TOO_LARGE"})
		return
	}

	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}

	safeFilename, err := security.SanitizeUploadName(header.Filename)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "文件名非法", gin.H{"errorCode": "INVALID_FILENAME"})
		return
	}
	contentType := header.Header.Get("Content-Type")
	sourceType := classifySourceType(contentType, safeFilename)

	stagingID := uuid.New().String()
	uploadDir := stagingID + "/" + filepath.Base(safeFilename) + ".bin"
	quarantinePath, err := h.registry.Resolve(security.RootImportQuarantine, uploadDir)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "暂存路径解析失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	storageKey, err := h.registry.StorageKeyFromPath(security.RootImportQuarantine, quarantinePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "暂存存储键生成失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	r := io.LimitReader(file, MaxImportUploadSize+1)
	data, err := io.ReadAll(r)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}
	if int64(len(data)) > MaxImportUploadSize {
		util.ErrorResponse(c, response.InvalidParams, "文件超过大小限制", gin.H{"errorCode": "FILE_TOO_LARGE"})
		return
	}

	hashBuf := sha256.Sum256(data)
	contentHash := hex.EncodeToString(hashBuf[:])

	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入暂存失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}
	if err := os.WriteFile(quarantinePath, data, 0o600); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入暂存失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	staging := &security.ImportStaging{
		ID:                stagingID,
		OwnerUserID:       actorID,
		SourceFilename:    safeFilename,
		SourceType:        sourceType,
		SourceContentHash: contentHash,
		SourceBytes:       int64(len(data)),
		RootKind:          string(security.RootImportQuarantine),
		StorageKey:        storageKey,
		Status:            security.StagingStatusQuarantined,
		CorrelationID:     c.GetHeader("X-Correlation-Id"),
		ExpiresAt:         time.Now().UTC().Add(ImportStagingTTL).Format(time.RFC3339Nano),
		QuarantinePath:    uploadDir,
	}

	if err := h.repo.Create(c.Request.Context(), staging); err != nil {
		storageKey, cleanErr := h.registry.StorageKeyFromPath(security.RootImportQuarantine, filepath.Dir(quarantinePath))
		if cleanErr == nil {
			_ = security.NewSafeArtifactResponder(h.registry).SafeDelete(
				security.RootImportQuarantine,
				storageKey,
				security.DeleteExpectation{EntityType: "import_staging", EntityID: staging.ID},
			)
		}
		util.ErrorResponse(c, response.InternalError, "暂存记录失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	if err := h.inspector.InspectAndMarkReady(c.Request.Context(), staging); err != nil {
		_, _ = h.repo.SetRejected(c.Request.Context(), staging.ID, actorID, err.Error())
		util.ErrorResponse(c, response.BusinessError, "导入包检查失败", gin.H{"errorCode": "IMPORT_INSPECTION_FAILED"})
		return
	}

	util.SuccessMsgResponse(c, "上传成功", gin.H{
		"stagingId":  staging.ID,
		"status":     security.StagingStatusReady,
		"sourceType": staging.SourceType,
	})
}

func (h *ImportStagingHandler) List(c *gin.Context) {
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}

	stagings, err := h.listForUser(c.Request.Context(), actorID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}
	util.SuccessResponse(c, gin.H{"items": stagings})
}

func (h *ImportStagingHandler) Inspect(c *gin.Context) {
	stagingID := c.Param("stagingId")
	if stagingID == "" {
		util.ErrorResponse(c, response.InvalidParams, "stagingId 为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}

	s, err := h.repo.GetForUser(c.Request.Context(), stagingID, actorID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "暂存不存在", gin.H{"errorCode": "NOT_FOUND"})
		return
	}
	util.SuccessResponse(c, s)
}

func (h *ImportStagingHandler) Consume(c *gin.Context) {
	var payload consumeStagingPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "stagingId 必填", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}

	s, err := h.repo.GetForUser(c.Request.Context(), payload.StagingID, actorID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "暂存不存在", gin.H{"errorCode": "NOT_FOUND"})
		return
	}
	if s.Status != security.StagingStatusReady {
		util.ErrorResponse(c, response.BusinessError, "暂存状态不可消费: "+string(s.Status), gin.H{"errorCode": "STAGING_NOT_READY"})
		return
	}

	acquired, err := h.repo.BeginConsumptionCAS(c.Request.Context(), payload.StagingID, actorID, s.StateRevision)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "消费加锁失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}
	if !acquired {
		util.ErrorResponse(c, response.BusinessError, "暂存正在被消费或已过期", gin.H{"errorCode": "STAGING_CONTENTION"})
		return
	}

	locked, err := h.repo.GetForUser(c.Request.Context(), payload.StagingID, actorID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "重新读取暂存失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	sourcePath, err := h.registry.Resolve(security.RootImportQuarantine, locked.StorageKey)
	if err != nil {
		failed, failErr := h.repo.FailConsumptionCAS(c.Request.Context(), locked.ID, actorID, locked.StateRevision, "resolve path failed")
		if failErr != nil {
			log.Error("failed to record path resolution failure: ", failErr)
		}
		if !failed {
			log.Warn("path resolution failed but staging state had changed: ", locked.ID)
		}
		util.ErrorResponse(c, response.InternalError, "暂存路径解析失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}

	petID, releaseID, operationID, err := h.importer.ImportPackage(c.Request.Context(), map[string]string{
		"userId":                  actorID,
		"importStagingId":         locked.ID,
		"sourceFilePath":          sourcePath,
		"idempotencyKey":          "import:" + locked.ID,
		"expectedStagingRevision": fmt.Sprintf("%d", locked.StateRevision),
	})
	if err != nil {
		failed, failErr := h.repo.FailConsumptionCAS(c.Request.Context(), locked.ID, actorID, locked.StateRevision, err.Error())
		if failErr != nil {
			log.Error("failed to record import failure: ", failErr)
		}
		if !failed {
			log.Warn("import failed but staging state had changed: ", locked.ID)
		}
		util.ErrorResponse(c, response.BusinessError, "导入失败", gin.H{"errorCode": "IMPORT_FAILED"})
		return
	}

	util.SuccessMsgResponse(c, "导入完成", gin.H{
		"stagingId":   locked.ID,
		"petId":       petID,
		"releaseId":   releaseID,
		"operationId": operationID,
	})
}

func (h *ImportStagingHandler) Reject(c *gin.Context) {
	stagingID := c.Param("stagingId")
	if stagingID == "" {
		util.ErrorResponse(c, response.InvalidParams, "stagingId 为空", gin.H{"errorCode": "INVALID_PARAMS"})
		return
	}
	actorID, err := middleware.ResolveActorID(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}

	ok, err := h.repo.SetRejected(c.Request.Context(), stagingID, actorID, "manual_reject")
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "操作失败", gin.H{"errorCode": "INTERNAL_ERROR"})
		return
	}
	if !ok {
		util.ErrorResponse(c, response.NotFound, "暂存不存在或不可拒绝", gin.H{"errorCode": "NOT_FOUND"})
		return
	}
	util.SuccessMsgResponse(c, "已拒绝", nil)
}

func (h *ImportStagingHandler) listForUser(ctx context.Context, userID string) ([]*security.ImportStaging, error) {
	return h.repo.ListForUser(ctx, userID)
}

func classifySourceType(mimeType, filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".zip"), strings.HasSuffix(lower, ".amitia"):
		return "package"
	case strings.Contains(mimeType, "zip"), strings.Contains(mimeType, "x-zip"):
		return "package"
	default:
		return "unknown"
	}
}
