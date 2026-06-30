// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func (h *Handler) ListConversations(c *gin.Context) {
	var q ConversationQuery
	c.ShouldBindQuery(&q)
	resp, err := h.service.ListConversations(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少必要参数", nil)
		return
	}
	conv, err := h.service.CreateConversation(&req)
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
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	util.SuccessResponse(c, gin.H{"items": msgs, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages})
}

func (h *Handler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteConversation(id); err != nil {
		util.ErrorResponse(c, response.InternalError, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "对话已删除", nil)
}

func (h *Handler) DeleteAllConversations(c *gin.Context) {
	if err := h.service.DeleteAllConversations(); err != nil {
		util.ErrorResponse(c, response.InternalError, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "所有对话已删除", nil)
}

func (h *Handler) DeleteMessages(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteMessages(id); err != nil {
		util.ErrorResponse(c, response.InternalError, "清空失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "消息已清空", nil)
}

func (h *Handler) DeleteSingleMessage(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteSingleMessage(id); err != nil {
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
		util.ErrorResponse(c, response.InternalError, "搜索失败", nil)
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
		util.ErrorResponse(c, response.InvalidParams, "characterId 不能为空", nil)
		return
	}
	conv, err := h.service.ChangeCharacter(id, body.CharacterID)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "角色已切换", conv)
}

func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, stats)
}

func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少必要参数", nil)
		return
	}
	resp, err := h.service.Chat(&req)
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) ListModels(c *gin.Context) {
	models, err := h.service.ListModels()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
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
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
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
		cfg.APIType = "openai-compatible"
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
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
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
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
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
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, routes)
}

func (h *Handler) UpdateModelRoutes(c *gin.Context) {
	var body struct {
		Routes []map[string]interface{} `json:"routes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
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
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.BaseURL == "" {
		util.ErrorResponse(c, response.InvalidParams, "baseUrl 不能为空", nil)
		return
	}
	models, err := h.service.DetectModels(body.BaseURL, body.APIKey)
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
	util.SuccessResponse(c, gin.H{"summary": "", "conversationId": c.Param("id")})
}
func (h *Handler) UpdateSummary(c *gin.Context)   { util.SuccessResponse(c, gin.H{"updated": true}) }
func (h *Handler) DeleteSummary(c *gin.Context)   { util.SuccessResponse(c, gin.H{"deleted": true}) }
func (h *Handler) GenerateSummary(c *gin.Context) { util.SuccessResponse(c, gin.H{"generated": true}) }
func (h *Handler) CleanupPreview(c *gin.Context)  { util.SuccessResponse(c, gin.H{"deletable": 0}) }
func (h *Handler) CleanupConfirm(c *gin.Context)  { util.SuccessResponse(c, gin.H{"cleaned": 0}) }
func (h *Handler) CleanupVacuum(c *gin.Context)   { util.SuccessResponse(c, gin.H{"vacuumed": true}) }
func (h *Handler) Export(c *gin.Context)          { util.SuccessResponse(c, gin.H{"exportUrl": ""}) }
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
	h.doTestConnection(c, cfg.BaseURL, cfg.APIKey, cfg.ModelName)
}
func (h *Handler) ProviderSchema(c *gin.Context) {
	util.SuccessResponse(c, gin.H{"fields": []string{"baseUrl", "apiKey", "modelName"}})
}

func (h *Handler) ListProviders(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListProviders())
}

func (h *Handler) doTestConnection(c *gin.Context, baseURL, apiKey, modelName string) {
	start := time.Now()
	base := strings.TrimRight(baseURL, "/")

	req, _ := http.NewRequest("GET", base+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
	}
	c.ShouldBindJSON(&body)
	if body.BaseURL == "" {
		util.SuccessResponse(c, gin.H{"success": false, "latencyMs": 0, "status": "error", "message": "Base URL 不能为空"})
		return
	}
	h.doTestConnection(c, body.BaseURL, body.APIKey, body.ModelName)
}

func (h *Handler) CompressionStatus(c *gin.Context) {
	id := c.Param("id")
	status := h.service.GetCompressionStatus(id)
	util.SuccessResponse(c, status)
}
