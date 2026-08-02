// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service        Service
	safeResponder  *security.SafeArtifactResponder
	ownershipGuard security.OwnershipGuard
}

func NewHandler(svc Service, responder *security.SafeArtifactResponder, guard security.OwnershipGuard) *Handler {
	return &Handler{service: svc, safeResponder: responder, ownershipGuard: guard}
}

func (h *Handler) GetActionDefinitions(c *gin.Context) {
	data, err := h.service.GetActionDefinitions()
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *Handler) CreateTask(c *gin.Context) {
	characterID := c.PostForm("characterId")
	modelConfigID, _ := strconv.Atoi(c.PostForm("modelConfigId"))
	name := c.PostForm("name")
	prompt := c.PostForm("prompt")
	negativePrompt := c.PostForm("negativePrompt")
	outputWidth, _ := strconv.Atoi(c.PostForm("outputWidth"))
	outputHeight, _ := strconv.Atoi(c.PostForm("outputHeight"))

	selectedActionKeysJSON := c.PostForm("selectedActionKeys")
	var selectedActionKeys []string
	if selectedActionKeysJSON != "" {
		if err := json.Unmarshal([]byte(selectedActionKeysJSON), &selectedActionKeys); err != nil {
			util.ErrorResponse(c, response.InvalidParams, "selectedActionKeys 解析失败", gin.H{"errorCode": ErrCodeActionSelectionRequired})
			return
		}
	}

	fileHeader, err := c.FormFile("referenceImage")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少参考图片", gin.H{"errorCode": ErrCodeReferenceImageRequired})
		return
	}

	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actor.UserID

	taskSummary, err := h.service.CreateTask(c.Request.Context(), userID, characterID, modelConfigID, name, prompt, negativePrompt, outputWidth, outputHeight, selectedActionKeys, fileHeader)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "任务已创建", taskSummary)
}

func (h *Handler) GetTask(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	data, err := h.service.GetTask(taskID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *Handler) ListTasks(c *gin.Context) {
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	userID := actor.UserID
	characterID := c.Query("characterId")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	data, err := h.service.ListTasks(userID, characterID, status, page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	if err := h.service.DeleteTask(taskID); err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "任务已删除", nil)
}

func (h *Handler) ReferenceImage(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	fullPath, mimeType, err := h.service.GetTaskSourceImage(taskID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		util.ErrorResponse(c, response.NotFound, "参考图片不存在", nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	h.safeResponder.SafeFileResponse(c, fullPath)
}

func (h *Handler) StartTask(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	summary, err := h.service.StartTask(taskID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	PublishTaskEvent(taskID, "task.started", map[string]interface{}{
		"taskId":   summary.ID,
		"status":   summary.Status,
		"stage":    summary.CurrentStage,
		"progress": summary.Progress,
	})
	util.SuccessMsgResponse(c, "任务已开始", summary)
}

func (h *Handler) CancelTask(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	if err := h.service.CancelTask(taskID); err != nil {
		writeServiceError(c, err)
		return
	}
	PublishTaskEvent(taskID, "task.cancel_requested", map[string]interface{}{
		"taskId": taskID,
	})
	util.SuccessMsgResponse(c, "任务已请求取消", nil)
}

func (h *Handler) RetryAction(c *gin.Context) {
	taskID := c.Param("taskId")
	actionKey := c.Param("actionKey")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	action, err := h.service.RetryAction(taskID, actionKey)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	PublishTaskEvent(taskID, "action.retry", map[string]interface{}{
		"taskId":    taskID,
		"actionKey": actionKey,
		"status":    action.Status,
	})
	util.SuccessMsgResponse(c, "动作已重新加入队列", action)
}

func (h *Handler) ActionFrameImage(c *gin.Context) {
	taskID := c.Param("taskId")
	actionKey := c.Param("actionKey")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	frameIndex, err := strconv.Atoi(c.Param("frameIndex"))
	if err != nil || frameIndex < 0 {
		util.ErrorResponse(c, response.InvalidParams, "帧索引无效", nil)
		return
	}
	fullPath, mimeType, err := h.service.GetFrameImage(taskID, actionKey, frameIndex)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if _, statErr := os.Stat(fullPath); statErr != nil {
		util.ErrorResponse(c, response.NotFound, "帧图片不存在", nil)
		return
	}
	if mimeType != "" {
		c.Header("Content-Type", mimeType)
	}
	h.safeResponder.SafeFileResponse(c, fullPath)
}

func (h *Handler) GetTaskTransitions(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	records, err := h.service.GetTaskTransitions(taskID, limit)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, records)
}

func (h *Handler) TaskEventsStream(c *gin.Context) {
	taskID := c.Param("taskId")
	actor, err := middleware.GetActorFromContext(c)
	if err != nil {
		util.ErrorResponse(c, response.Unauthorized, "认证失败", gin.H{"errorCode": "SSE_AUTH_REQUIRED"})
		return
	}
	if _, err := h.ownershipGuard.RequireGenerationTask(c.Request.Context(), actor, taskID); err != nil {
		writeOwnershipError(c, err)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	bus := DefaultEventBus()
	subscriberID := fmt.Sprintf("sse-%s-%p-%d", actor.UserID, c.Request, time.Now().UnixNano())
	events := bus.Subscribe(taskID, subscriberID)
	defer bus.Unsubscribe(taskID, subscriberID)

	c.SSEvent("connected", gin.H{"taskId": taskID})
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

func writeServiceError(c *gin.Context, err error) {
	var be *BusinessError
	if errors.As(err, &be) {
		util.ErrorResponse(c, be.Code, be.Msg, gin.H{"errorCode": be.ErrCode})
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func writeOwnershipError(c *gin.Context, err error) {
	if ownErr := security.MapOwnershipError(err); ownErr != nil {
		util.ErrorResponse(c, ownErr.Code, ownErr.Msg, gin.H{"errorCode": ownErr.ErrCode})
		return
	}
	writeServiceError(c, err)
}
