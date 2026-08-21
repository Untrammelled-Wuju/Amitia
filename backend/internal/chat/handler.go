// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type Handler struct {
	service       Service
	deliveryStore DeliveryStore
	unifiedEntry  *interaction.UnifiedEntry
}

type conversationChangeScopedService interface {
	CreateConversationForUser(req *CreateConversationRequest, userID string) (*Conversation, error)
	DeleteConversationForUser(id string, userID string) (bool, error)
	DeleteAllConversationsForUser(userID string) error
	ChangeCharacterForUser(convID, charID, userID string) (*Conversation, error)
}

type messageChangeScopedService interface {
	DeleteMessagesForUser(convID string, userID string) error
	DeleteSingleMessageForUser(id string, userID string) error
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func NewHandlerWithUnifiedEntry(srv Service, entry *interaction.UnifiedEntry) *Handler {
	return &Handler{service: srv, unifiedEntry: entry, deliveryStore: nil}
}

func NewHandlerWithUnifiedEntryAndDelivery(srv Service, entry *interaction.UnifiedEntry, ds DeliveryStore) *Handler {
	return &Handler{service: srv, unifiedEntry: entry, deliveryStore: ds}
}

func (h *Handler) ListConversations(c *gin.Context) {
	var q ConversationQuery
	c.ShouldBindQuery(&q)
	resp, err := h.service.ListConversations(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	var conv *Conversation
	var err error
	if scoped, ok := h.service.(conversationChangeScopedService); ok {
		conv, err = scoped.CreateConversationForUser(&req, requestidentity.ResolveGin(c, ""))
	} else {
		conv, err = h.service.CreateConversation(&req)
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "对话已创建", conv)
}

func (h *Handler) GetMessages(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	msgs, total, err := h.service.GetMessages(id, page, pageSize)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	util.SuccessResponse(c, gin.H{"items": msgs, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages})
}

func (h *Handler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")
	var characterDeleted bool
	var err error
	if scoped, ok := h.service.(conversationChangeScopedService); ok {
		characterDeleted, err = scoped.DeleteConversationForUser(id, requestidentity.ResolveGin(c, ""))
	} else {
		characterDeleted, err = h.service.DeleteConversation(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true, "characterDeleted": characterDeleted})
}

func (h *Handler) DeleteAllConversations(c *gin.Context) {
	var err error
	if scoped, ok := h.service.(conversationChangeScopedService); ok {
		err = scoped.DeleteAllConversationsForUser(requestidentity.ResolveGin(c, ""))
	} else {
		err = h.service.DeleteAllConversations()
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "所有对话已删除", nil)
}

func (h *Handler) DeleteMessages(c *gin.Context) {
	id := c.Param("id")
	var err error
	if scoped, ok := h.service.(messageChangeScopedService); ok {
		err = scoped.DeleteMessagesForUser(id, requestidentity.ResolveGin(c, ""))
	} else {
		err = h.service.DeleteMessages(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "消息已清空", nil)
}

func (h *Handler) DeleteSingleMessage(c *gin.Context) {
	id := c.Param("id")
	var err error
	if scoped, ok := h.service.(messageChangeScopedService); ok {
		err = scoped.DeleteSingleMessageForUser(id, requestidentity.ResolveGin(c, ""))
	} else {
		err = h.service.DeleteSingleMessage(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "消息已删除", nil)
}

func (h *Handler) SearchMessages(c *gin.Context) {
	var q MessageSearchQuery
	c.ShouldBindQuery(&q)
	if q.Keyword == "" {
		util.ErrorResponse(c, response.InvalidParams, "关键词不能为空", nil)
		return
	}
	resp, err := h.service.SearchMessages(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) ChangeCharacter(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		CharacterID string `json:"characterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CharacterID == "" {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	var conv *Conversation
	var err error
	if scoped, ok := h.service.(conversationChangeScopedService); ok {
		conv, err = scoped.ChangeCharacterForUser(id, body.CharacterID, requestidentity.ResolveGin(c, ""))
	} else {
		conv, err = h.service.ChangeCharacter(id, body.CharacterID)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色已切换", conv)
}

func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, stats)
}

func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if h.unifiedEntry != nil {
		channel := strings.TrimSpace(req.Channel)
		if channel == "" {
			channel = "web"
		}
		source := strings.TrimSpace(req.Source)
		if source == "" {
			source = "web"
		}

		if req.CharacterID != "" {
			proactive.CancelLowPriorityOnUserInput(req.CharacterID)
			if h.deliveryStore != nil {
				_ = h.deliveryStore.PreemptActiveOutputLeases(req.CharacterID)
			}
		}

		deviceTimezone := strings.TrimSpace(req.DeviceTimezone)
		if deviceTimezone == "" {
			deviceTimezone = strings.TrimSpace(c.GetHeader("X-Device-Timezone"))
		}
		orchResult, err := h.unifiedEntry.Handle(c.Request.Context(), &interaction.UnifiedEntryRequest{
			CharacterID:    req.CharacterID,
			Message:        req.Message,
			ConversationID: req.ConversationID,
			Channel:        channel,
			Source:         source,
			PeerID:         req.PeerID,
			UserID:         requestidentity.ResolveGin(c, req.UserID),
			DeviceTimezone: deviceTimezone,
			SessionID:      req.SessionID,
			RequestID:      req.RequestID,
		})
		if err != nil {
			h.writeUnifiedEntryError(c, err)
			return
		}
		if orchResult == nil || orchResult.Response == nil {
			util.ErrorResponse(c, response.OperationFailed, "统一入口未返回回复", nil)
			return
		}
		resp := h.chatResponseFromProcessResponse(orchResult.Response)
		util.SuccessResponse(c, resp)
		return
	}
	resp, err := h.service.Chat(&req)
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) chatResponseFromProcessResponse(resp *interaction.ProcessResponse) *ChatResponse {
	msgID := ""
	if len(resp.MessageIDs) > 0 {
		msgID = resp.MessageIDs[len(resp.MessageIDs)-1]
	}
	return &ChatResponse{
		ConversationID: resp.ConversationID,
		Sequence:       resp.Sequence,
		Message: &MessageItem{
			ID:             msgID,
			ConversationID: resp.ConversationID,
			Sequence:       resp.Sequence,
			Role:           "assistant",
			Content:        resp.Reply,
			Source:         "assistant",
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		},
	}
}

func (h *Handler) writeUnifiedEntryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, interaction.ErrBackpressureShedding), errors.Is(err, interaction.ErrBackpressureCooldown), errors.Is(err, interaction.ErrOrchestratorBusy):
		util.ErrorResponse(c, response.TooManyRequests, "请求过于频繁", err.Error())
	case errors.Is(err, interaction.ErrOrchestratorNotReady):
		util.ErrorResponse(c, response.OperationFailed, "服务尚未就绪", err.Error())
	case errors.Is(err, interaction.ErrOrchestratorDuplicate):
		util.ErrorResponse(c, response.OperationFailed, "重复请求", err.Error())
	case errors.Is(err, interaction.ErrOrchestratorInvalidScope), errors.Is(err, interaction.ErrInvalidChannel), errors.Is(err, interaction.ErrScopeResolutionFailed):
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
	case errors.Is(err, interaction.ErrOrchestratorCancelled), errors.Is(err, interaction.ErrOrchestratorSuperseded):
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
	default:
		util.ErrorResponse(c, response.BusinessError, err.Error(), nil)
	}
}

func (h *Handler) ListModels(c *gin.Context) {
	models, err := h.service.ListModels()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	filtered := make([]ModelConfig, 0, len(models))
	for _, m := range models {
		if m.APIType == "doubao-vision" {
			continue
		}
		m.HasAPIKey = m.APIKey != ""
		m.APIKey = ""
		filtered = append(filtered, m)
	}
	util.SuccessResponse(c, filtered)
}

func (h *Handler) CreateModel(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	cfg := ModelConfig{}
	if v, ok := raw["apiType"]; ok {
		cfg.APIType, _ = v.(string)
	}
	if v, ok := raw["baseUrl"]; ok {
		cfg.BaseURL, _ = v.(string)
	}
	if v, ok := raw["apiKey"]; ok {
		cfg.APIKey, _ = v.(string)
	}
	if v, ok := raw["modelName"]; ok {
		cfg.ModelName, _ = v.(string)
	}
	if v, ok := raw["name"]; ok {
		cfg.Name, _ = v.(string)
	}
	if v, ok := raw["isActive"]; ok {
		switch val := v.(type) {
		case bool:
			if val {
				cfg.IsActive = 1
			} else {
				cfg.IsActive = 0
			}
		case float64:
			if val != 0 {
				cfg.IsActive = 1
			} else {
				cfg.IsActive = 0
			}
		case int:
			cfg.IsActive = val
		}
	}
	if cfg.APIType == "" {
		cfg.APIType = "openai"
	}
	result, err := h.service.CreateModel(&cfg)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	result.HasAPIKey = result.APIKey != ""
	result.APIKey = ""
	util.SuccessMsgResponse(c, "模型配置已创建", result)
}

func (h *Handler) UpdateModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	result, err := h.service.UpdateModel(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "模型配置已更新", result)
}

func (h *Handler) DeleteModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.DeleteModel(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "模型配置已删除", nil)
}

func (h *Handler) ActivateModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	result, err := h.service.ActivateModel(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "模型已激活", result)
}

func (h *Handler) GetModelRoutes(c *gin.Context) {
	routes, err := h.service.GetModelRoutes()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, routes)
}

func (h *Handler) UpdateModelRoutes(c *gin.Context) {
	var body struct {
		Routes []map[string]interface{} `json:"routes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if err := h.service.UpdateModelRoutes(body.Routes); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "路由已更新", nil)
}

func (h *Handler) DetectModels(c *gin.Context) {
	var body struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
		APIType string `json:"apiType"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.BaseURL == "" {
		util.ErrorResponse(c, response.InvalidParams, "baseUrl 不能为空", nil)
		return
	}
	models, err := h.service.DetectModels(body.BaseURL, body.APIKey, body.APIType)
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, err.Error(), nil)
		return
	}
	if models == nil {
		models = []ModelDetectItem{}
	}
	util.SuccessResponse(c, gin.H{"models": models})
}

func (h *Handler) GetSummary(c *gin.Context) {
	summary, err := h.service.GetConversationSummary(c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			util.SuccessResponse(c, gin.H{"conversationId": c.Param("id"), "summaryText": ""})
			return
		}
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, summary)
}

func (h *Handler) UpdateSummary(c *gin.Context) {
	var body struct {
		SummaryText string `json:"summaryText"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.SummaryText) == "" {
		util.ErrorResponse(c, response.InvalidParams, "summaryText 不能为空", nil)
		return
	}
	summary, err := h.service.UpdateConversationSummary(c.Param("id"), body.SummaryText)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, summary)
}

func (h *Handler) DeleteSummary(c *gin.Context) {
	if err := h.service.DeleteConversationSummary(c.Param("id")); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) GenerateSummary(c *gin.Context) {
	summary, err := h.service.GenerateConversationSummary(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, summary)
}
func (h *Handler) CleanupPreview(c *gin.Context) { util.SuccessResponse(c, gin.H{"deletable": 0}) }
func (h *Handler) CleanupConfirm(c *gin.Context) { util.SuccessResponse(c, gin.H{"cleaned": 0}) }
func (h *Handler) CleanupVacuum(c *gin.Context)  { util.SuccessResponse(c, gin.H{"vacuumed": true}) }
func (h *Handler) Export(c *gin.Context) {
	var body struct {
		Format          string   `json:"format"`
		ConversationIDs []string `json:"conversationIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.ConversationIDs) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "参数无效", nil)
		return
	}
	if body.Format != "json" {
		body.Format = "markdown"
	}

	convID := body.ConversationIDs[0]
	url, err := h.service.ExportConversation(convID, body.Format)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"exportUrl": url})
}
func (h *Handler) GetModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	models, _ := h.service.ListModels()
	for _, m := range models {
		if m.ID == id {
			m.HasAPIKey = m.APIKey != ""
			util.SuccessResponse(c, m)
			return
		}
	}
	util.ErrorResponse(c, response.NotFound, "模型配置不存在", nil)
}
func (h *Handler) TestModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	models, _ := h.service.ListModels()
	var cfg *ModelConfig
	for _, m := range models {
		if m.ID == id {
			cfg = &m
			break
		}
	}
	if cfg == nil {
		util.SuccessResponse(c, gin.H{"success": false, "latencyMs": 0, "status": "error", "message": "配置不存在"})
		return
	}
	h.doTestConnection(c, cfg.BaseURL, cfg.APIKey, cfg.ModelName, cfg.APIType)
}
func (h *Handler) ProviderSchema(c *gin.Context) {
	util.SuccessResponse(c, gin.H{"fields": []string{"baseUrl", "apiKey", "modelName"}})
}

func (h *Handler) ListProviders(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListProviders())
}

func (h *Handler) doTestConnection(c *gin.Context, baseURL, apiKey, modelName, apiType string) {
	start := time.Now()
	base := strings.TrimRight(baseURL, "/")

	var reqURL string
	if apiType == "ollama" {
		reqURL = base + "/api/tags"
	} else {
		reqURL = base + "/models"
	}
	req, _ := http.NewRequest("GET", reqURL, nil)
	if apiType != "ollama" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		util.SuccessResponse(c, gin.H{
			"success": false, "latencyMs": latency, "status": "error",
			"message": "连接失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		msg := "连接成功"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			msg = "服务可达，请检查 API Key"
		}
		util.SuccessResponse(c, gin.H{
			"success": true, "latencyMs": latency, "status": "ok", "message": msg,
		})
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500]
		}
		util.SuccessResponse(c, gin.H{
			"success": false, "latencyMs": latency, "status": "error",
			"message": fmt.Sprintf("服务器返回 %d", resp.StatusCode),
		})
	}
}
func (h *Handler) TestModelStandalone(c *gin.Context) {
	var body struct {
		BaseURL   string `json:"baseUrl"`
		APIKey    string `json:"apiKey"`
		ModelName string `json:"modelName"`
		APIType   string `json:"apiType"`
	}
	c.ShouldBindJSON(&body)
	if body.BaseURL == "" {
		util.SuccessResponse(c, gin.H{"success": false, "latencyMs": 0, "status": "error", "message": "Base URL 不能为空"})
		return
	}
	h.doTestConnection(c, body.BaseURL, body.APIKey, body.ModelName, body.APIType)
}

func (h *Handler) CompressionStatus(c *gin.Context) {
	id := c.Param("id")
	status := h.service.GetCompressionStatus(id)
	util.SuccessResponse(c, status)
}
