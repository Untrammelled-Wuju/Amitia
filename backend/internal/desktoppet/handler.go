// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(svc Service) *Handler { return &Handler{service: svc} }

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

	userID := resolveUserID(c)

	taskSummary, err := h.service.CreateTask(c.Request.Context(), userID, characterID, modelConfigID, name, prompt, negativePrompt, outputWidth, outputHeight, selectedActionKeys, fileHeader)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "任务已创建", taskSummary)
}

func (h *Handler) GetTask(c *gin.Context) {
	taskID := c.Param("taskId")
	data, err := h.service.GetTask(taskID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *Handler) ListTasks(c *gin.Context) {
	characterID := c.Query("characterId")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	data, err := h.service.ListTasks(characterID, status, page, pageSize)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.service.DeleteTask(taskID); err != nil {
		writeServiceError(c, err)
		return
	}
	util.SuccessMsgResponse(c, "任务已删除", nil)
}

func (h *Handler) ReferenceImage(c *gin.Context) {
	taskID := c.Param("taskId")
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
	c.File(fullPath)
}

func writeServiceError(c *gin.Context, err error) {
	var be *BusinessError
	if errors.As(err, &be) {
		util.ErrorResponse(c, be.Code, be.Msg, gin.H{"errorCode": be.ErrCode})
		return
	}
	util.ErrorResponse(c, response.InternalError, err.Error(), nil)
}

func resolveUserID(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) <= 7 || auth[:7] != "Bearer " {
		return "default"
	}
	tokenStr := auth[7:]
	if tokenStr == "" {
		return "default"
	}
	secret := config.AppCfg.JWT.Secret
	if secret == "" {
		return "default"
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || token == nil {
		return "default"
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "default"
	}
	uidRaw, exists := claims["userId"]
	if !exists {
		return "default"
	}
	switch v := uidRaw.(type) {
	case float64:
		if v == 0 {
			return "default"
		}
		return strconv.Itoa(int(v))
	case int:
		if v == 0 {
			return "default"
		}
		return strconv.Itoa(v)
	case int64:
		if v == 0 {
			return "default"
		}
		return strconv.FormatInt(v, 10)
	case string:
		if v == "" {
			return "default"
		}
		return v
	default:
		return "default"
	}
}
