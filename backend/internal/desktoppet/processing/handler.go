// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
	"archive/zip"
	"io"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(svc Service) *Handler { return &Handler{service: svc} }

type createPackagePayload struct {
	DefaultAction     string   `json:"defaultAction"`
	IncludedActions   []string `json:"includedActions"`
	UserDefaultAction string   `json:"userDefaultAction"`
}

type switchAttemptPayload struct {
	AttemptNumber int `json:"attemptNumber"`
}

func (h *Handler) CreateProcessingTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "生成任务 ID 为空", gin.H{"errorCode": ErrCodeGenerationTaskNotReady})
		return
	}

	outputWidth, _ := strconv.Atoi(c.PostForm("outputWidth"))
	outputHeight, _ := strconv.Atoi(c.PostForm("outputHeight"))
	ratioStr := c.PostForm("targetCharacterHeightRatio")
	targetCharacterHeightRatio, _ := strconv.ParseFloat(ratioStr, 64)
	anchorMode := c.PostForm("anchorMode")
	backgroundMode := c.PostForm("backgroundMode")
	outputFormat := c.PostForm("outputFormat")
	defaultFPS, _ := strconv.Atoi(c.PostForm("defaultFps"))

	userID := desktoppet.ResolveUserID(c)

	req := &CreateProcessingTaskRequest{
		GenerationTaskID:           taskID,
		UserID:                     userID,
		OutputWidth:                outputWidth,
		OutputHeight:               outputHeight,
		TargetCharacterHeightRatio: targetCharacterHeightRatio,
		AnchorMode:                 anchorMode,
		BackgroundMode:             backgroundMode,
		OutputFormat:               outputFormat,
		DefaultFPS:                 defaultFPS,
	}

	task, err := h.service.CreateProcessingTask(req)
	if err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(task.ID, "processing.task.created", map[string]interface{}{
		"processingTaskId": task.ID,
		"generationTaskId": task.GenerationTaskID,
		"status":           task.Status,
		"version":          task.ProcessingVersion,
	})

	util.SuccessMsgResponse(c, "处理任务已创建", task)
}

func (h *Handler) GetProcessingTask(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}

	resp, err := h.service.GetProcessingTask(processingTaskID)
	if err != nil {
		writeProcessingError(c, err)
		return
	}

	util.SuccessResponse(c, resp)
}

func (h *Handler) CancelProcessingTask(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}

	if err := h.service.CancelProcessingTask(processingTaskID); err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(processingTaskID, "processing.task.cancel_requested", map[string]interface{}{
		"processingTaskId": processingTaskID,
	})

	util.SuccessMsgResponse(c, "处理任务已请求取消", nil)
}

func (h *Handler) RetryProcessingAction(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}
	if actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeProcessingActionNotFound})
		return
	}

	if err := h.service.RetryProcessingAction(processingTaskID, actionKey); err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(processingTaskID, "processing.action.retry", map[string]interface{}{
		"processingTaskId": processingTaskID,
		"actionKey":        actionKey,
	})

	util.SuccessMsgResponse(c, "动作已重新加入处理队列", nil)
}

func (h *Handler) CreatePackage(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}

	var payload createPackagePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeProcessingPackageFailed})
		return
	}

	userID := desktoppet.ResolveUserID(c)

	req := &CreatePackageRequest{
		ProcessingTaskID:  processingTaskID,
		UserID:            userID,
		DefaultAction:     payload.DefaultAction,
		IncludedActions:   payload.IncludedActions,
		UserDefaultAction: payload.UserDefaultAction,
	}

	resp, err := h.service.CreatePackage(req)
	if err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(processingTaskID, "processing.package.created", map[string]interface{}{
		"processingTaskId": processingTaskID,
		"packageId":        resp.PackageID,
		"packageHash":      resp.PackageHash,
		"status":           resp.Status,
	})

	util.SuccessMsgResponse(c, "资源包已生成", resp)
}

func (h *Handler) SwitchAttempt(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}
	if actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeProcessingActionNotFound})
		return
	}

	var payload switchAttemptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数解析失败: "+err.Error(), gin.H{"errorCode": ErrCodeProcessingInvalidAttempt})
		return
	}
	if payload.AttemptNumber < 1 {
		util.ErrorResponse(c, response.InvalidParams, "attemptNumber 无效", gin.H{"errorCode": ErrCodeProcessingInvalidAttempt})
		return
	}

	if err := h.service.SwitchAttempt(processingTaskID, actionKey, payload.AttemptNumber); err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(processingTaskID, "processing.action.switch_attempt", map[string]interface{}{
		"processingTaskId": processingTaskID,
		"actionKey":        actionKey,
		"attemptNumber":    payload.AttemptNumber,
	})

	util.SuccessMsgResponse(c, "已切换源尝试", nil)
}

func (h *Handler) ExcludeAction(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	if processingTaskID == "" {
		util.ErrorResponse(c, response.InvalidParams, "处理任务 ID 为空", gin.H{"errorCode": ErrCodeProcessingTaskNotFound})
		return
	}
	if actionKey == "" {
		util.ErrorResponse(c, response.InvalidParams, "动作 Key 为空", gin.H{"errorCode": ErrCodeProcessingActionNotFound})
		return
	}

	if err := h.service.ExcludeAction(processingTaskID, actionKey); err != nil {
		writeProcessingError(c, err)
		return
	}

	desktoppet.PublishTaskEvent(processingTaskID, "processing.action.excluded", map[string]interface{}{
		"processingTaskId": processingTaskID,
		"actionKey":        actionKey,
	})

	util.SuccessMsgResponse(c, "动作已排除", nil)
}

func (h *Handler) ProcessingEventsStream(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	bus := desktoppet.DefaultEventBus()
	subscriberID := c.Query("subscriberId")
	if subscriberID == "" {
		subscriberID = fmt.Sprintf("sse-%p-%d", c.Request, time.Now().UnixNano())
	}
	events := bus.Subscribe(processingTaskID, subscriberID)
	defer bus.Unsubscribe(processingTaskID, subscriberID)

	c.SSEvent("connected", gin.H{"processingTaskId": processingTaskID})
	c.Writer.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return
			}
			payload, _ := json.Marshal(evt)
			c.SSEvent(evt.EventType, string(payload))
			c.Writer.Flush()
		case <-ticker.C:
			c.SSEvent("ping", "{}")
			c.Writer.Flush()
		case <-c.Done():
			return
		}
	}
}

func (h *Handler) ProcessedFrameImage(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	frameIndex, err := strconv.Atoi(c.Param("frameIndex"))
	if err != nil || frameIndex < 0 {
		util.ErrorResponse(c, response.InvalidParams, "帧索引无效", gin.H{"errorCode": ErrCodeProcessingInvalidAttempt})
		return
	}

	fullPath, mimeType, err := h.service.GetProcessedFrameImage(processingTaskID, actionKey, frameIndex)
	if err != nil {
		writeProcessingError(c, err)
		return
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		util.ErrorResponse(c, response.NotFound, "处理帧图片不存在", nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(fullPath)
}

func (h *Handler) SourceFrameImage(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")
	frameIndex, err := strconv.Atoi(c.Param("frameIndex"))
	if err != nil || frameIndex < 0 {
		util.ErrorResponse(c, response.InvalidParams, "帧索引无效", gin.H{"errorCode": ErrCodeProcessingInvalidAttempt})
		return
	}

	fullPath, mimeType, err := h.service.GetSourceFrameImage(processingTaskID, actionKey, frameIndex)
	if err != nil {
		writeProcessingError(c, err)
		return
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		util.ErrorResponse(c, response.NotFound, "源帧图片不存在", nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(fullPath)
}

func (h *Handler) ActionPreview(c *gin.Context) {
	processingTaskID := c.Param("processingTaskId")
	actionKey := c.Param("actionKey")

	fullPath, mimeType, err := h.service.GetActionPreview(processingTaskID, actionKey)
	if err != nil {
		writeProcessingError(c, err)
		return
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		util.ErrorResponse(c, response.NotFound, "动作预览图不存在", nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	c.File(fullPath)
}

func writeProcessingError(c *gin.Context, err error) {
	var pe *ProcessingError
	if errors.As(err, &pe) {
		httpCode := mapProcessingErrorCode(pe.Code)
		util.ErrorResponse(c, httpCode, pe.Message, gin.H{"errorCode": pe.Code})
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func mapProcessingErrorCode(code string) int {
	switch code {
	case "PACKAGE_NOT_FOUND":
		return response.NotFound
	case ErrCodeProcessingTaskAlreadyRunning,
		ErrCodeProcessingTaskStateConflict,
		ErrCodeProcessingActionNotRetryable,
		ErrCodeProcessingExcludedDefault:
		return response.BusinessError
	case ErrCodeProcessingInvalidAttempt,
		ErrCodeGenerationTaskNotReady,
		ErrCodeNoSuccessfulActions,
		ErrCodeDefaultIdleActionUnavailable,
		ErrCodePackageDefaultActionInvalid,
		ErrCodeActionFrameCountInvalid:
		return response.InvalidParams
	case ErrCodeProcessingTaskNotFound,
		ErrCodeProcessingActionNotFound:
		return response.NotFound
	case ErrCodeProcessingStorageFailed:
		return response.InternalError
	case ErrCodeProcessingPackageFailed:
		return response.OperationFailed
	case ErrCodeProcessingCancelled:
		return response.BusinessError
	case ErrCodeCanvasNormalizationFailed,
		ErrCodeSubjectAlignmentFailed,
		ErrCodeSubjectOutOfBounds,
		ErrCodeAnchorCalculationFailed:
		return response.OperationFailed
	case ErrCodeFrameQualityCheckFailed:
		return response.OperationFailed
	case ErrCodeLoopDiscontinuity,
		ErrCodeExcessiveFrameDrift:
		return response.BusinessError
	default:
		return response.InternalError
	}
}

func (h *Handler) ListPackages(c *gin.Context) {
	userID := desktoppet.ResolveUserID(c)
	generationTaskID := c.Query("generationTaskId")
	if generationTaskID != "" {
		packages, err := h.service.ListPackagesByGenerationTask(userID, generationTaskID)
		if err != nil {
			util.ErrorResponse(c, response.InternalError, err.Error(), nil)
			return
		}
		util.SuccessResponse(c, gin.H{"items": packages, "total": len(packages)})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	if pageSize > 100 { pageSize = 100 }

	packages, total, err := h.service.ListPackages(userID, page, pageSize)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": packages, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) DownloadPackage(c *gin.Context) {
	packageID := c.Param("packageId")
	if packageID == "" {
		util.ErrorResponse(c, response.InvalidParams, "包 ID 为空", nil)
		return
	}

	packageDir, pkg, err := h.service.DownloadPackage(packageID)
	if err != nil {
		writeProcessingError(c, err)
		return
	}

	fileName := fmt.Sprintf("%s.zip", pkg.Name)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	err = filepath.Walk(packageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() { return nil }
		relPath, _ := filepath.Rel(packageDir, path)
		zipEntry, zerr := zipWriter.Create(filepath.ToSlash(relPath))
		if zerr != nil { return zerr }
		f, ferr := os.Open(path)
		if ferr != nil { return ferr }
		defer f.Close()
		_, cperr := io.Copy(zipEntry, f)
		return cperr
	})
	if err != nil {
		_ = err
	}
}
