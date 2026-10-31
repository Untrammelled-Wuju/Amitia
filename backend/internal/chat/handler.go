// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	amitialog "log"
)

type Handler struct {
	service Service
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func redactModelConfigForResponse(cfg *ModelConfig) {
	if cfg == nil {
		return
	}
	cfg.HasAPIKey = cfg.APIKey != ""
	cfg.APIKey = ""
	var capabilities ModelCapabilities
	if err := json.Unmarshal([]byte(cfg.CapabilitiesJSON), &capabilities); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	cfg.SupportsVision = capabilities.SupportsImage
	applyReasoningCapabilities(cfg)
}

func applyReasoningCapabilities(cfg *ModelConfig) {
	if cfg == nil {
		return
	}
	cfg.SupportsReasoning = false
	cfg.DefaultReasoningEffort = ""
	cfg.ReasoningLevels = []string{}
	if strings.TrimSpace(cfg.CapabilitiesJSON) == "" {
		return
	}
	var raw map[string]interface{}
	if json.Unmarshal([]byte(cfg.CapabilitiesJSON), &raw) != nil {
		return
	}
	if value, ok := raw["supportsReasoning"].(bool); ok {
		cfg.SupportsReasoning = value
	}
	if value, ok := raw["defaultReasoningEffort"].(string); ok {
		cfg.DefaultReasoningEffort = normalizeReasoningEffort(value)
	}
	if values, ok := raw["reasoningLevels"].([]interface{}); ok {
		for _, item := range values {
			level := normalizeReasoningEffort(fmt.Sprint(item))
			if level != "" && !containsString(cfg.ReasoningLevels, level) {
				cfg.ReasoningLevels = append(cfg.ReasoningLevels, level)
			}
		}
	}
	if cfg.SupportsReasoning && len(cfg.ReasoningLevels) == 0 {
		cfg.ReasoningLevels = []string{"low", "medium", "high", "xhigh"}
	}
	if cfg.SupportsReasoning && cfg.DefaultReasoningEffort == "" {
		cfg.DefaultReasoningEffort = "high"
	}
}

func mergeReasoningCapabilities(raw map[string]interface{}, existingJSON string) string {
	current := map[string]interface{}{}
	if strings.TrimSpace(existingJSON) != "" {
		if err := json.Unmarshal([]byte(existingJSON), &current); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	if value, ok := raw["supportsReasoning"]; ok {
		current["supportsReasoning"] = value == true || fmt.Sprint(value) == "1"
	}
	if value, ok := raw["supportsVision"]; ok {
		current["supportsImage"] = value == true || fmt.Sprint(value) == "1"
	}
	if value, ok := raw["defaultReasoningEffort"]; ok {
		current["defaultReasoningEffort"] = normalizeReasoningEffort(fmt.Sprint(value))
	}
	if value, ok := raw["reasoningLevels"]; ok {
		current["reasoningLevels"] = value
	}
	if current["supportsReasoning"] == true {
		if _, ok := current["reasoningLevels"]; !ok {
			current["reasoningLevels"] = []string{"low", "medium", "high", "xhigh"}
		}
		if _, ok := current["defaultReasoningEffort"]; !ok {
			current["defaultReasoningEffort"] = "high"
		}
	}
	data, err := json.Marshal(current)
	if err != nil {
		return existingJSON
	}
	return string(data)
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "低", "轻":
		return "low"
	case "medium", "中":
		return "medium"
	case "high", "高":
		return "high"
	case "xhigh", "极高":
		return "xhigh"
	default:
		return ""
	}
}

func NormalizeReasoningEffort(value string) string {
	return normalizeReasoningEffort(value)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (h *Handler) AvailableModels(c *gin.Context) {
	actor := security.GetActor(c)
	if actor == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if actor.HasPermission(auth.PermSystemAdmin) {
		h.ListModels(c)
		return
	}
	models, err := h.service.ListModels()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	available := make([]gin.H, 0, 1)
	for _, model := range models {
		if model.IsActive != 1 || model.APIType == "doubao-vision" {
			continue
		}
		available = append(available, gin.H{"id": 0, "name": "云端 AI 服务", "apiType": model.APIType, "modelName": model.ModelName, "isActive": 1, "supportsReasoning": model.SupportsReasoning, "supportsVision": model.SupportsVision, "managedByCore": true})
	}
	util.SuccessResponse(c, available)
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
		redactModelConfigForResponse(&m)
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
	if err := validateModelBudgetRateConfig(raw); err != nil {
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
	if v, ok := raw["protocol"]; ok {
		cfg.Protocol, _ = v.(string)
	}
	if v, ok := raw["temperature"].(float64); ok {
		cfg.Temperature = v
	}
	if v, ok := raw["maxTokens"].(float64); ok {
		cfg.MaxTokens = int(v)
	}
	if v, ok := raw["contextWindow"].(float64); ok {
		cfg.ContextWindow = int(v)
	}
	if v, ok := raw["maxOutputTokens"].(float64); ok {
		cfg.MaxOutputTokens = int(v)
	}
	if v, ok := raw["budgetRateVersion"].(string); ok {
		cfg.BudgetRateVersion = v
	}
	if v, ok := raw["budgetInputRate"].(float64); ok && v >= 0 && v <= 100000000 && v == float64(int64(v)) {
		cfg.BudgetInputRate = int64(v)
	}
	if v, ok := raw["budgetOutputRate"].(float64); ok && v >= 0 && v <= 100000000 && v == float64(int64(v)) {
		cfg.BudgetOutputRate = int64(v)
	}
	if v, ok := raw["topP"].(float64); ok {
		cfg.TopP = v
	}
	if v, ok := raw["timeoutSeconds"].(float64); ok {
		cfg.TimeoutSeconds = int(v)
	}
	if v, ok := raw["retryCount"].(float64); ok {
		cfg.RetryCount = int(v)
	}
	if v, ok := raw["providerConfig"].(string); ok {
		cfg.ProviderConfigJSON = v
	} else if v, ok := raw["providerConfigJSON"].(string); ok {
		cfg.ProviderConfigJSON = v
	} else if v, ok := raw["providerConfig"].(map[string]interface{}); ok {
		encoded, _ := json.Marshal(v)
		cfg.ProviderConfigJSON = string(encoded)
	} else if v, ok := raw["providerConfigJSON"].(map[string]interface{}); ok {
		encoded, _ := json.Marshal(v)
		cfg.ProviderConfigJSON = string(encoded)
	}
	if v, ok := raw["capabilitiesJson"].(string); ok {
		cfg.CapabilitiesJSON = v
	}
	cfg.CapabilitiesJSON = mergeReasoningCapabilities(raw, cfg.CapabilitiesJSON)
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
	result, err := h.configurationService(c.Request.Context()).CreateModel(&cfg)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	redactModelConfigForResponse(result)
	util.SuccessMsgResponse(c, "模型配置已创建", result)
}

func (h *Handler) UpdateModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if err := validateModelBudgetRateConfig(updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	_, hasReasoning := updates["supportsReasoning"]
	_, hasVision := updates["supportsVision"]
	if hasReasoning || hasVision {
		existing, err := h.configurationService(c.Request.Context()).GetModel(id)
		if err != nil {
			util.ErrorResponse(c, response.NotFound, err.Error(), nil)
			return
		}
		updates["capabilitiesJson"] = mergeReasoningCapabilities(updates, existing.CapabilitiesJSON)
		delete(updates, "supportsReasoning")
		delete(updates, "defaultReasoningEffort")
		delete(updates, "reasoningLevels")
		delete(updates, "supportsVision")
	}
	result, err := h.configurationService(c.Request.Context()).UpdateModel(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	redactModelConfigForResponse(result)
	util.SuccessMsgResponse(c, "模型配置已更新", result)
}

func (h *Handler) DeleteModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.configurationService(c.Request.Context()).DeleteModel(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "模型配置已删除", nil)
}

func (h *Handler) ActivateModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	result, err := h.configurationService(c.Request.Context()).ActivateModel(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	redactModelConfigForResponse(result)
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
	if err := h.configurationService(c.Request.Context()).UpdateModelRoutes(body.Routes); err != nil {
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
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if localmodel.IsEmbedded(body.APIType) {
		manager, ok := h.service.(LocalModelManager)
		actor := security.GetActor(c)
		if !ok || actor == nil {
			util.ErrorResponse(c, response.OperationFailed, "本地模型库不可用", nil)
			return
		}
		models, err := manager.DetectLocalModels(c.Request.Context(), actor.SpaceID.String(), body.APIType)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
			return
		}
		util.SuccessResponse(c, gin.H{"models": models})
		return
	}
	if body.BaseURL == "" {
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

func (h *Handler) GetModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	models, _ := h.service.ListModels()
	for _, m := range models {
		if m.ID == id {
			redactModelConfigForResponse(&m)
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
	if localmodel.IsEmbedded(cfg.APIType) {
		h.testLocalModel(c, cfg)
		return
	}
	h.doTestConnection(c, cfg.BaseURL, cfg.APIKey, cfg.ModelName, cfg.APIType)
}
func (h *Handler) ProviderSchema(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("id"))
	for _, provider := range h.service.ListProviders() {
		if provider.ID == providerID {
			util.SuccessResponse(c, provider)
			return
		}
	}
	util.ErrorResponse(c, response.NotFound, "模型提供商不存在", nil)
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
	client := timeoutpolicy.Client(&http.Client{Timeout: 15 * time.Second})
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
		BaseURL        string `json:"baseUrl"`
		APIKey         string `json:"apiKey"`
		ModelName      string `json:"modelName"`
		APIType        string `json:"apiType"`
		ProviderConfig any    `json:"providerConfig"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	c.ShouldBindJSON(&body)
	if localmodel.IsEmbedded(body.APIType) {
		cfg := &ModelConfig{BaseURL: body.BaseURL, APIKey: body.APIKey, ModelName: body.ModelName, APIType: body.APIType, TimeoutSeconds: body.TimeoutSeconds}
		if cfg.TimeoutSeconds <= 0 {
			cfg.TimeoutSeconds = 120
		}
		switch value := body.ProviderConfig.(type) {
		case string:
			cfg.ProviderConfigJSON = value
		case map[string]interface{}:
			encoded, _ := json.Marshal(value)
			cfg.ProviderConfigJSON = string(encoded)
		}
		h.testLocalModel(c, cfg)
		return
	}
	if body.BaseURL == "" {
		util.SuccessResponse(c, gin.H{"success": false, "latencyMs": 0, "status": "error", "message": "Base URL 不能为空"})
		return
	}
	h.doTestConnection(c, body.BaseURL, body.APIKey, body.ModelName, body.APIType)
}

func (h *Handler) testLocalModel(c *gin.Context, cfg *ModelConfig) {
	manager, ok := h.service.(LocalModelManager)
	if !ok {
		util.ErrorResponse(c, response.OperationFailed, "本地模型运行时不可用", nil)
		return
	}
	result, err := manager.TestLocalModel(c.Request.Context(), cfg)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ListLocalModels(c *gin.Context) {
	manager, ok := h.service.(LocalModelManager)
	actor := security.GetActor(c)
	if !ok || actor == nil {
		util.ErrorResponse(c, response.OperationFailed, "本地模型库不可用", nil)
		return
	}
	rows, err := manager.ListLocalModels(c.Request.Context(), actor.SpaceID.String())
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, rows)
}

func (h *Handler) UploadLocalModel(c *gin.Context) {
	manager, ok := h.service.(LocalModelManager)
	actor := security.GetActor(c)
	if !ok || actor == nil {
		util.ErrorResponse(c, response.OperationFailed, "本地模型库不可用", nil)
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	defer file.Close()
	row, err := manager.ImportLocalModel(c.Request.Context(), actor.SpaceID.String(), c.PostForm("name"), header.Filename, file)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, row)
}

func (h *Handler) DeleteLocalModel(c *gin.Context) {
	manager, ok := h.service.(LocalModelManager)
	actor := security.GetActor(c)
	if !ok || actor == nil {
		util.ErrorResponse(c, response.OperationFailed, "本地模型库不可用", nil)
		return
	}
	if err := manager.DeleteLocalModel(c.Request.Context(), actor.SpaceID.String(), c.Param("id")); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "模型文件已删除", nil)
}
