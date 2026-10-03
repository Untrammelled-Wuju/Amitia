package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/tts"
	"gorm.io/gorm"
)

type mcpReconnectController interface {
	Reconnect(context.Context, string) error
}

type serverAgentAdminController struct {
	chat       chat.Service
	characters character.Repository
	db         *gorm.DB
	graph      graph.Service
	tts        tts.Repository
	asr        asr.Repository
	extensions *extensionkernel.Runtime
	mcpRepo    *mcp.Repository
	mcpConn    mcpReconnectController

	selectionMu sync.RWMutex
	selection   map[string]string
}

func newServerAgentAdminController(chatSvc chat.Service, characters character.Repository, db *gorm.DB, graphSvc graph.Service, extensions *extensionkernel.Runtime, mcpRepo *mcp.Repository) *serverAgentAdminController {
	return &serverAgentAdminController{
		chat: chatSvc, characters: characters, db: db, graph: graphSvc, extensions: extensions, mcpRepo: mcpRepo,
		tts: tts.NewRepository(db), asr: asr.NewRepository(db), selection: map[string]string{},
	}
}

func (c *serverAgentAdminController) SetMCPConnections(connections mcpReconnectController) {
	if c != nil {
		c.mcpConn = connections
	}
}

var serverAgentAdminToolNames = map[string]struct{}{
	"start_chat_service": {}, "create_new_chat": {}, "list_chats": {}, "find_chat": {}, "agent_status": {}, "switch_chat": {}, "update_chat_title": {}, "delete_chat": {}, "send_message_to_ai": {}, "list_character_cards": {}, "get_chat_messages": {}, "get_chat_messages_range": {},
	"list_model_configs": {}, "create_model_config": {}, "update_model_config": {}, "delete_model_config": {}, "activate_model_config": {}, "test_model_config_connection": {}, "get_model_routes": {}, "update_model_routes": {},
	"list_function_model_configs": {}, "get_function_model_config": {}, "set_function_model_config": {},
	"list_tts_configs": {}, "create_tts_config": {}, "update_tts_config": {}, "delete_tts_config": {}, "activate_tts_config": {}, "list_asr_configs": {}, "create_asr_config": {}, "update_asr_config": {}, "delete_asr_config": {}, "activate_asr_config": {},
	"get_speech_services_config": {}, "set_speech_services_config": {},
	"list_sandbox_packages": {}, "set_sandbox_package_enabled": {}, "restart_mcp_with_logs": {},
	"link_memories": {}, "query_memory_links": {},
}

func (c *serverAgentAdminController) CanExecuteAgentAdminTool(toolName string) bool {
	_, ok := serverAgentAdminToolNames[toolName]
	return ok && c != nil
}

func (c *serverAgentAdminController) ExecuteAgentAdminTool(ctx context.Context, toolName string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	if c == nil || c.chat == nil || c.db == nil {
		return nil, fmt.Errorf("agent admin services unavailable")
	}
	scopedChat, err := c.scopedChat()
	if err != nil {
		return nil, err
	}
	spaceID := agentAdminSpaceID(invocation)
	switch toolName {
	case "start_chat_service":
		stats, err := scopedChat.GetStatsForSpace(spaceID)
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"ready": true, "status": "running", "stats": stats})
	case "agent_status":
		stats, err := scopedChat.GetStatsForSpace(spaceID)
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"ready": true, "stats": stats, "selectedConversationId": c.selectedConversation(invocation)})
	case "create_new_chat":
		var req struct {
			ProjectID string `json:"projectId"`
			Title     string `json:"title"`
			Channel   string `json:"channel"`
			PeerID    string `json:"peerId"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if req.Channel == "" {
			req.Channel = "web"
		}
		conv, err := scopedChat.CreateConversationForSpace(&chat.CreateConversationRequest{ProjectID: req.ProjectID, Title: req.Title, Channel: req.Channel, Source: "agent_tool", PeerID: req.PeerID}, spaceID)
		if err != nil {
			return nil, err
		}
		c.setSelectedConversation(invocation, conv.ID)
		return jsonResult(conv)
	case "list_chats":
		var req struct {
			Page      int    `json:"page"`
			PageSize  int    `json:"pageSize"`
			Channel   string `json:"channel"`
			ProjectID string `json:"projectId"`
			Keyword   string `json:"keyword"`
		}
		_ = parseAdminInput(input, &req)
		if req.Page < 1 {
			req.Page = 1
		}
		if req.PageSize < 1 {
			req.PageSize = 50
		}
		if req.PageSize > 100 {
			req.PageSize = 100
		}
		return resultOrError(scopedChat.ListConversationsForSpace(chat.ConversationQuery{Page: req.Page, PageSize: req.PageSize, Channel: req.Channel, ProjectID: req.ProjectID, Keyword: req.Keyword}, spaceID))
	case "find_chat":
		var req struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if req.Limit < 1 {
			req.Limit = 20
		}
		if req.Limit > 100 {
			req.Limit = 100
		}
		return resultOrError(scopedChat.ListConversationsForSpace(chat.ConversationQuery{Page: 1, PageSize: req.Limit, Keyword: req.Query}, spaceID))
	case "switch_chat":
		var req struct {
			ConversationID string `json:"conversationId"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		conv, err := scopedChat.GetConversationForSpace(req.ConversationID, spaceID)
		if err != nil {
			return nil, err
		}
		c.setSelectedConversation(invocation, conv.ID)
		return jsonResult(map[string]any{"switched": true, "conversation": conv})
	case "update_chat_title":
		var req struct {
			ConversationID string `json:"conversationId"`
			Title          string `json:"title"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		id := c.resolveConversationID(invocation, req.ConversationID)
		if id == "" {
			return nil, fmt.Errorf("conversationId is required")
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			return nil, fmt.Errorf("title is required")
		}
		if len([]rune(title)) > 200 {
			return nil, fmt.Errorf("title is too long")
		}
		if _, err := scopedChat.GetConversationForSpace(id, spaceID); err != nil {
			return nil, err
		}
		if err := c.db.Model(&chat.Conversation{}).Where("id = ?", id).Updates(map[string]any{"title": title, "updated_at": time.Now().Format("2006-01-02 15:04:05")}).Error; err != nil {
			return nil, err
		}
		return resultOrError(scopedChat.GetConversationForSpace(id, spaceID))
	case "delete_chat":
		var req struct {
			ConversationID string `json:"conversationId"`
		}
		_ = parseAdminInput(input, &req)
		id := c.resolveConversationID(invocation, req.ConversationID)
		if id == "" {
			return nil, fmt.Errorf("conversationId is required")
		}
		ok, err := scopedChat.DeleteConversationForSpace(id, spaceID)
		if err != nil {
			return nil, err
		}
		c.clearSelectedConversation(invocation, id)
		return jsonResult(map[string]any{"deleted": ok, "conversationId": id})
	case "send_message_to_ai":
		var req struct {
			ConversationID string `json:"conversationId"`
			CharacterID    string `json:"characterId"`
			Message        string `json:"message"`
			RequestID      string `json:"requestId"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.Message) == "" {
			return nil, fmt.Errorf("message is required")
		}
		req.ConversationID = c.resolveConversationID(invocation, req.ConversationID)
		if req.CharacterID == "" {
			req.CharacterID = invocation.CharacterID
		}
		if req.CharacterID == "" {
			return nil, fmt.Errorf("characterId is required")
		}
		if req.RequestID == "" {
			req.RequestID = uuid.NewString()
		}
		resp, err := c.chat.ProcessMessage(ctx, &chat.ProcessMessageRequest{CharacterID: req.CharacterID, Message: req.Message, ConversationID: req.ConversationID, Channel: "web", Source: "agent_tool", RequestID: req.RequestID, SpaceID: spaceID, SessionID: invocation.InvocationID, IsInternal: true})
		if err != nil {
			return nil, err
		}
		c.setSelectedConversation(invocation, resp.ConversationID)
		return jsonResult(map[string]any{"response": resp})
	case "list_character_cards":
		chars, err := c.ownedCharacterList(spaceID, false)
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"characters": chars, "count": len(chars)})
	case "get_chat_messages":
		var req struct {
			ConversationID string `json:"conversationId"`
			Page           int    `json:"page"`
			PageSize       int    `json:"pageSize"`
		}
		_ = parseAdminInput(input, &req)
		id := c.resolveConversationID(invocation, req.ConversationID)
		if id == "" {
			return nil, fmt.Errorf("conversationId is required")
		}
		if req.Page < 1 {
			req.Page = 1
		}
		if req.PageSize < 1 {
			req.PageSize = 50
		}
		if req.PageSize > 200 {
			req.PageSize = 200
		}
		items, total, err := scopedChat.GetMessagesForSpace(id, spaceID, req.Page, req.PageSize)
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"items": items, "total": total, "page": req.Page, "pageSize": req.PageSize})
	case "get_chat_messages_range":
		var req struct {
			ConversationID string `json:"conversationId"`
			StartSequence  int64  `json:"startSequence"`
			EndSequence    int64  `json:"endSequence"`
			Limit          int    `json:"limit"`
		}
		_ = parseAdminInput(input, &req)
		id := c.resolveConversationID(invocation, req.ConversationID)
		if id == "" {
			return nil, fmt.Errorf("conversationId is required")
		}
		if req.Limit < 1 {
			req.Limit = 100
		}
		if req.Limit > 500 {
			req.Limit = 500
		}
		if _, err := scopedChat.GetConversationForSpace(id, spaceID); err != nil {
			return nil, err
		}
		q := c.db.Where("conversation_id = ?", id)
		if req.StartSequence > 0 {
			q = q.Where("sequence >= ?", req.StartSequence)
		}
		if req.EndSequence > 0 {
			q = q.Where("sequence <= ?", req.EndSequence)
		}
		var msgs []chat.Message
		if err := q.Order("sequence ASC").Limit(req.Limit).Find(&msgs).Error; err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"items": msgs, "count": len(msgs)})
	case "list_model_configs":
		cfgs, err := c.chat.ListModels()
		if err != nil {
			return nil, err
		}
		for i := range cfgs {
			sanitizeModelConfig(&cfgs[i])
		}
		routes, _ := c.chat.GetModelRoutes()
		return jsonResult(map[string]any{"configs": cfgs, "count": len(cfgs), "functionBindings": routes})
	case "create_model_config":
		var req struct {
			Config chat.ModelConfig `json:"config"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		cfg, err := c.chat.CreateModel(&req.Config)
		if err != nil {
			return nil, err
		}
		sanitizeModelConfig(cfg)
		return jsonResult(cfg)
	case "update_model_config":
		var req struct {
			ID      int            `json:"id"`
			Updates map[string]any `json:"updates"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if req.ID <= 0 {
			return nil, fmt.Errorf("id is required")
		}
		filterModelUpdates(req.Updates)
		cfg, err := c.chat.UpdateModel(req.ID, req.Updates)
		if err != nil {
			return nil, err
		}
		sanitizeModelConfig(cfg)
		return jsonResult(cfg)
	case "delete_model_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.chat.DeleteModel(req.ID); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"deleted": true, "id": req.ID})
	case "activate_model_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		cfg, err := c.chat.ActivateModel(req.ID)
		if err != nil {
			return nil, err
		}
		sanitizeModelConfig(cfg)
		return jsonResult(cfg)
	case "test_model_config_connection":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		var cfg chat.ModelConfig
		if err := c.db.First(&cfg, req.ID).Error; err != nil {
			return nil, err
		}
		models, err := c.chat.DetectModels(cfg.BaseURL, cfg.APIKey, cfg.APIType)
		if err != nil {
			return nil, err
		}
		if len(models) > 100 {
			models = models[:100]
		}
		return jsonResult(map[string]any{"ok": true, "modelCount": len(models), "models": models})
	case "get_model_routes":
		return resultOrError(c.chat.GetModelRoutes())
	case "update_model_routes":
		var req struct {
			Routes []map[string]any `json:"routes"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if len(req.Routes) > 100 {
			return nil, fmt.Errorf("too many routes")
		}
		if err := c.chat.UpdateModelRoutes(req.Routes); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"updated": true, "count": len(req.Routes)})
	case "list_function_model_configs":
		routes, err := c.chat.GetModelRoutes()
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"bindings": routes, "count": len(routes)})
	case "get_function_model_config":
		var req struct {
			FunctionType string `json:"functionType"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		functionType := strings.TrimSpace(req.FunctionType)
		if functionType == "" {
			return nil, fmt.Errorf("functionType is required")
		}
		routes, err := c.chat.GetModelRoutes()
		if err != nil {
			return nil, err
		}
		for _, route := range routes {
			if routeScenario(route) != functionType {
				continue
			}
			configID := routeModelConfigID(route)
			var cfg chat.ModelConfig
			if configID > 0 && c.db.First(&cfg, configID).Error == nil {
				sanitizeModelConfig(&cfg)
				return jsonResult(map[string]any{"functionType": functionType, "binding": route, "config": cfg})
			}
			return jsonResult(map[string]any{"functionType": functionType, "binding": route})
		}
		return jsonResult(map[string]any{"functionType": functionType, "binding": nil})
	case "set_function_model_config":
		var req struct {
			FunctionType string `json:"functionType"`
			ConfigID     int    `json:"configId"`
			ModelIndex   int    `json:"modelIndex"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		functionType := strings.TrimSpace(req.FunctionType)
		if functionType == "" || req.ConfigID <= 0 {
			return nil, fmt.Errorf("functionType and configId are required")
		}
		var cfg chat.ModelConfig
		if err := c.db.First(&cfg, req.ConfigID).Error; err != nil {
			return nil, fmt.Errorf("model config %d not found: %w", req.ConfigID, err)
		}
		routes, err := c.chat.GetModelRoutes()
		if err != nil {
			return nil, err
		}
		updated := false
		for i := range routes {
			if routeScenario(routes[i]) == functionType {
				routes[i] = map[string]any{"scenario": functionType, "modelConfigId": req.ConfigID}
				updated = true
				break
			}
		}
		if !updated {
			routes = append(routes, map[string]any{"scenario": functionType, "modelConfigId": req.ConfigID})
		}
		if err := c.chat.UpdateModelRoutes(routes); err != nil {
			return nil, err
		}
		sanitizeModelConfig(&cfg)
		return jsonResult(map[string]any{"functionType": functionType, "configId": req.ConfigID, "modelIndex": req.ModelIndex, "config": cfg})
	case "get_speech_services_config":
		return c.speechServicesConfig()
	case "set_speech_services_config":
		var req map[string]any
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.updateSpeechServices(req); err != nil {
			return nil, err
		}
		return c.speechServicesConfig()
	case "list_sandbox_packages":
		if c.extensions == nil {
			return nil, fmt.Errorf("extension runtime unavailable")
		}
		items, err := listKernelPackages(ctx, c.extensions.Container())
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"packages": items, "count": len(items), "runtime": "amitia_extension_kernel"})
	case "set_sandbox_package_enabled":
		if c.extensions == nil {
			return nil, fmt.Errorf("extension runtime unavailable")
		}
		var req struct {
			PackageName string `json:"packageName"`
			Enabled     bool   `json:"enabled"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		req.PackageName = strings.TrimSpace(req.PackageName)
		if req.PackageName == "" {
			return nil, fmt.Errorf("packageName is required")
		}
		var err error
		if req.Enabled {
			err = c.extensions.Enable(ctx, req.PackageName)
		} else {
			err = c.extensions.Disable(ctx, req.PackageName)
		}
		if err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"packageName": req.PackageName, "enabled": req.Enabled, "runtime": "amitia_extension_kernel"})
	case "restart_mcp_with_logs":
		if c.mcpRepo == nil || c.mcpConn == nil {
			return nil, fmt.Errorf("MCP compatibility runtime unavailable")
		}
		var req struct {
			TimeoutMS int64 `json:"timeout_ms"`
		}
		_ = parseAdminInput(input, &req)
		if req.TimeoutMS <= 0 {
			req.TimeoutMS = 60000
		}
		if req.TimeoutMS > 120000 {
			req.TimeoutMS = 120000
		}
		restartCtx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
		defer cancel()
		servers, err := c.mcpRepo.ListEnabledServers(restartCtx)
		if err != nil {
			return nil, err
		}
		logs := make([]map[string]any, 0, len(servers))
		succeeded := 0
		for _, server := range servers {
			started := time.Now()
			reconnectErr := c.mcpConn.Reconnect(restartCtx, server.ID)
			fresh, freshErr := c.mcpRepo.GetServer(context.Background(), server.ID)
			entry := map[string]any{"serverId": server.ID, "name": server.Name, "durationMs": time.Since(started).Milliseconds()}
			if freshErr == nil {
				entry["status"] = fresh.Status
				entry["lastErrorCode"] = fresh.LastErrorCode
				entry["lastErrorMessage"] = fresh.LastErrorMessage
			}
			if reconnectErr != nil {
				entry["ok"] = false
				entry["log"] = reconnectErr.Error()
			} else {
				entry["ok"] = true
				entry["log"] = "MCP server reconnected and canonical discovery/tool synchronization was triggered"
				succeeded++
			}
			logs = append(logs, entry)
			if restartCtx.Err() != nil {
				break
			}
		}
		return jsonResult(map[string]any{"ok": succeeded == len(servers), "total": len(servers), "succeeded": succeeded, "plugins": logs})
	case "list_tts_configs":
		cfgs, err := c.tts.List()
		if err != nil {
			return nil, err
		}
		for i := range cfgs {
			sanitizeTTSConfig(&cfgs[i])
		}
		return jsonResult(map[string]any{"configs": cfgs, "count": len(cfgs)})
	case "create_tts_config":
		var req struct {
			Config tts.TtsConfig `json:"config"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.tts.Create(&req.Config); err != nil {
			return nil, err
		}
		sanitizeTTSConfig(&req.Config)
		return jsonResult(req.Config)
	case "update_tts_config":
		var req struct {
			ID      int            `json:"id"`
			Updates map[string]any `json:"updates"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		req.Updates = normalizeConfigUpdates(req.Updates, ttsUpdateAliases, allowedTTSUpdateFields)
		if err := c.tts.Update(req.ID, req.Updates); err != nil {
			return nil, err
		}
		cfg, err := c.tts.GetByID(req.ID)
		if err != nil {
			return nil, err
		}
		sanitizeTTSConfig(cfg)
		return jsonResult(cfg)
	case "delete_tts_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.tts.Delete(req.ID); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"deleted": true, "id": req.ID})
	case "activate_tts_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.tts.Activate(req.ID); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"activated": true, "id": req.ID})
	case "list_asr_configs":
		cfgs, err := c.asr.List()
		if err != nil {
			return nil, err
		}
		for i := range cfgs {
			sanitizeASRConfig(&cfgs[i])
		}
		return jsonResult(map[string]any{"configs": cfgs, "count": len(cfgs)})
	case "create_asr_config":
		var req struct {
			Config asr.AsrConfig `json:"config"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.asr.Create(&req.Config); err != nil {
			return nil, err
		}
		sanitizeASRConfig(&req.Config)
		return jsonResult(req.Config)
	case "update_asr_config":
		var req struct {
			ID      int            `json:"id"`
			Updates map[string]any `json:"updates"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		req.Updates = normalizeConfigUpdates(req.Updates, asrUpdateAliases, allowedASRUpdateFields)
		if err := c.asr.Update(req.ID, req.Updates); err != nil {
			return nil, err
		}
		cfg, err := c.asr.GetByID(req.ID)
		if err != nil {
			return nil, err
		}
		sanitizeASRConfig(cfg)
		return jsonResult(cfg)
	case "delete_asr_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.asr.Delete(req.ID); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"deleted": true, "id": req.ID})
	case "activate_asr_config":
		var req struct {
			ID int `json:"id"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if err := c.asr.Activate(req.ID); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"activated": true, "id": req.ID})
	case "link_memories":
		var req struct {
			SourceID     string  `json:"sourceId"`
			TargetID     string  `json:"targetId"`
			RelationType string  `json:"relationType"`
			Weight       float64 `json:"weight"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if c.graph == nil {
			return nil, fmt.Errorf("graph service unavailable")
		}
		if req.SourceID == "" || req.TargetID == "" {
			return nil, fmt.Errorf("sourceId and targetId are required")
		}
		if req.SourceID == req.TargetID {
			return nil, fmt.Errorf("cannot link a memory to itself")
		}
		if req.RelationType == "" {
			req.RelationType = "related_to"
		}
		if req.Weight == 0 {
			req.Weight = 1
		}
		if req.Weight < 0 || req.Weight > 1 {
			return nil, fmt.Errorf("weight must be between 0 and 1")
		}
		if err := c.ensureMemoryScope(req.SourceID, invocation); err != nil {
			return nil, err
		}
		if err := c.ensureMemoryScope(req.TargetID, invocation); err != nil {
			return nil, err
		}
		if err := c.graph.SyncEdge("memory:"+req.SourceID, "memory:"+req.TargetID, req.RelationType, req.Weight); err != nil {
			return nil, err
		}
		return jsonResult(map[string]any{"linked": true, "sourceId": req.SourceID, "targetId": req.TargetID, "relationType": req.RelationType, "weight": req.Weight})
	case "query_memory_links":
		var req struct {
			MemoryID string `json:"memoryId"`
			Depth    int    `json:"depth"`
		}
		if err := parseAdminInput(input, &req); err != nil {
			return nil, err
		}
		if c.graph == nil {
			return nil, fmt.Errorf("graph service unavailable")
		}
		if err := c.ensureMemoryScope(req.MemoryID, invocation); err != nil {
			return nil, err
		}
		if req.Depth < 1 {
			req.Depth = 1
		}
		if req.Depth > 3 {
			req.Depth = 3
		}
		return resultOrError(c.graph.QueryNeighbors("memory:"+req.MemoryID, req.Depth, spaceID))
	default:
		return nil, fmt.Errorf("agent admin tool %s is not supported", toolName)
	}
}

func routeScenario(route map[string]any) string {
	for _, key := range []string{"scenario", "functionType", "function_type"} {
		if value, ok := route[key]; ok {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func routeModelConfigID(route map[string]any) int {
	for _, key := range []string{"modelConfigId", "model_config_id", "configId", "config_id"} {
		if value, ok := route[key]; ok {
			switch v := value.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			case json.Number:
				i, _ := v.Int64()
				return int(i)
			default:
				var i int
				_, _ = fmt.Sscan(fmt.Sprint(v), &i)
				return i
			}
		}
	}
	return 0
}

func listKernelPackages(ctx context.Context, container *extensionkernel.Container) ([]map[string]any, error) {
	if container == nil || container.InstallationRepository == nil {
		return nil, fmt.Errorf("kernel installation repository unavailable")
	}
	installed, err := container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(installed, func(i, j int) bool {
		return installed[i].ExtensionID < installed[j].ExtensionID
	})
	items := make([]map[string]any, 0, len(installed))
	for _, item := range installed {
		name := string(item.ExtensionID)
		publisher := ""
		if container.DefinitionRepository != nil {
			if definition, definitionErr := container.DefinitionRepository.GetExtension(ctx, item.ExtensionID, item.InstalledVersion); definitionErr == nil {
				if displayName := definition.Name.Get("zh-CN"); displayName != "" {
					name = displayName
				} else if definition.Name.Default != "" {
					name = definition.Name.Default
				}
				publisher = definition.Publisher.PublisherID
			}
		}
		state := string(item.EnablementState)
		items = append(items, map[string]any{
			"packageName":       string(item.ExtensionID),
			"name":              name,
			"version":           item.InstalledVersion.String(),
			"publisher":         publisher,
			"installationState": string(item.InstallationState),
			"enablementState":   state,
			"enabled":           item.EnablementState == domain.EnablementEnabled,
		})
	}
	return items, nil
}

func (c *serverAgentAdminController) speechServicesConfig() (json.RawMessage, error) {
	ttsConfigs, err := c.tts.List()
	if err != nil {
		return nil, err
	}
	asrConfigs, err := c.asr.List()
	if err != nil {
		return nil, err
	}
	var activeTTS *tts.TtsConfig
	var activeASR *asr.AsrConfig
	for i := range ttsConfigs {
		if ttsConfigs[i].IsActive == 1 {
			copy := ttsConfigs[i]
			sanitizeTTSConfig(&copy)
			activeTTS = &copy
		}
		sanitizeTTSConfig(&ttsConfigs[i])
	}
	for i := range asrConfigs {
		if asrConfigs[i].IsActive == 1 {
			copy := asrConfigs[i]
			sanitizeASRConfig(&copy)
			activeASR = &copy
		}
		sanitizeASRConfig(&asrConfigs[i])
	}
	return jsonResult(map[string]any{"tts": activeTTS, "stt": activeASR, "ttsConfigs": ttsConfigs, "sttConfigs": asrConfigs})
}

func (c *serverAgentAdminController) updateSpeechServices(req map[string]any) error {
	ttsUpdates := map[string]any{}
	asrUpdates := map[string]any{}
	mapIfPresent := func(src, dst string, target map[string]any) {
		if value, ok := req[src]; ok {
			target[dst] = value
		}
	}
	mapIfPresent("tts_service_type", "api_type", ttsUpdates)
	mapIfPresent("tts_url_template", "base_url", ttsUpdates)
	mapIfPresent("tts_api_key", "api_key", ttsUpdates)
	mapIfPresent("tts_voice_id", "voice_type", ttsUpdates)
	mapIfPresent("tts_speech_rate", "speed", ttsUpdates)
	mapIfPresent("tts_pitch", "pitch", ttsUpdates)
	mapIfPresent("stt_service_type", "api_type", asrUpdates)
	mapIfPresent("stt_endpoint_url", "base_url", asrUpdates)
	mapIfPresent("stt_api_key", "api_key", asrUpdates)
	if nested, ok := req["ttsUpdates"].(map[string]any); ok {
		for key, value := range normalizeConfigUpdates(nested, ttsUpdateAliases, allowedTTSUpdateFields) {
			ttsUpdates[key] = value
		}
	}
	if nested, ok := req["sttUpdates"].(map[string]any); ok {
		for key, value := range normalizeConfigUpdates(nested, asrUpdateAliases, allowedASRUpdateFields) {
			asrUpdates[key] = value
		}
	}
	if len(ttsUpdates) > 0 {
		id := positiveInt(req["ttsConfigId"])
		if id == 0 {
			configs, err := c.tts.List()
			if err != nil {
				return err
			}
			for _, cfg := range configs {
				if cfg.IsActive == 1 {
					id = cfg.ID
					break
				}
			}
		}
		if id == 0 {
			return fmt.Errorf("no active TTS config; provide ttsConfigId")
		}
		if err := c.tts.Update(id, ttsUpdates); err != nil {
			return err
		}
	}
	if len(asrUpdates) > 0 {
		id := positiveInt(req["sttConfigId"])
		if id == 0 {
			configs, err := c.asr.List()
			if err != nil {
				return err
			}
			for _, cfg := range configs {
				if cfg.IsActive == 1 {
					id = cfg.ID
					break
				}
			}
		}
		if id == 0 {
			return fmt.Errorf("no active STT config; provide sttConfigId")
		}
		if err := c.asr.Update(id, asrUpdates); err != nil {
			return err
		}
	}
	return nil
}

func positiveInt(value any) int {
	switch v := value.(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	}
	return 0
}

func (c *serverAgentAdminController) selectionKey(inv capability.ToolInvocationContext) string {
	if strings.TrimSpace(inv.SpaceID) != "" {
		return "space:" + strings.TrimSpace(inv.SpaceID)
	}
	return "local"
}
func (c *serverAgentAdminController) selectedConversation(inv capability.ToolInvocationContext) string {
	c.selectionMu.RLock()
	defer c.selectionMu.RUnlock()
	return c.selection[c.selectionKey(inv)]
}
func (c *serverAgentAdminController) setSelectedConversation(inv capability.ToolInvocationContext, id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	c.selectionMu.Lock()
	c.selection[c.selectionKey(inv)] = id
	c.selectionMu.Unlock()
}
func (c *serverAgentAdminController) clearSelectedConversation(inv capability.ToolInvocationContext, id string) {
	c.selectionMu.Lock()
	if c.selection[c.selectionKey(inv)] == id {
		delete(c.selection, c.selectionKey(inv))
	}
	c.selectionMu.Unlock()
}
func (c *serverAgentAdminController) resolveConversationID(inv capability.ToolInvocationContext, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if selected := c.selectedConversation(inv); selected != "" {
		return selected
	}
	return strings.TrimSpace(inv.ConversationID)
}

func (c *serverAgentAdminController) ensureMemoryScope(id string, inv capability.ToolInvocationContext) error {
	var m memory.Memory
	if err := c.db.Where("id = ?", strings.TrimSpace(id)).First(&m).Error; err != nil {
		return fmt.Errorf("memory %s not found", id)
	}
	if !agentAdminOwnerMatches(m.SpaceID, agentAdminSpaceID(inv)) {
		return fmt.Errorf("memory %s not found", id)
	}
	charID := strings.TrimSpace(inv.CharacterID)
	if charID != "" && strings.TrimSpace(m.CharacterID) != "" && m.CharacterID != charID {
		return fmt.Errorf("memory %s is outside the current character scope", id)
	}
	return nil
}

func parseAdminInput(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}
func jsonResult(v any) (json.RawMessage, error) { b, err := json.Marshal(v); return b, err }
func resultOrError[T any](v T, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	return jsonResult(v)
}

func sanitizeModelConfig(cfg *chat.ModelConfig) {
	if cfg == nil {
		return
	}
	cfg.HasAPIKey = strings.TrimSpace(cfg.APIKey) != ""
	cfg.APIKey = ""
	cfg.ProviderConfigJSON = redactJSONSecrets(cfg.ProviderConfigJSON)
}
func sanitizeTTSConfig(cfg *tts.TtsConfig) {
	if cfg == nil {
		return
	}
	cfg.HasApiKey = strings.TrimSpace(cfg.ApiKey) != ""
	cfg.ApiKey = ""
	cfg.RealtimeAccessToken = ""
	cfg.RealtimeSecretKey = ""
}
func sanitizeASRConfig(cfg *asr.AsrConfig) {
	if cfg == nil {
		return
	}
	cfg.HasApiKey = strings.TrimSpace(cfg.ApiKey) != ""
	cfg.ApiKey = ""
}
func redactJSONSecrets(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return "{}"
	}
	redactSecretValue(v)
	b, _ := json.Marshal(v)
	return string(b)
}
func redactSecretValue(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
				x[k] = "[REDACTED]"
			} else {
				redactSecretValue(val)
			}
		}
	case []any:
		for _, item := range x {
			redactSecretValue(item)
		}
	}
}

var allowedModelUpdateFields = map[string]bool{"name": true, "apiType": true, "protocol": true, "baseUrl": true, "apiKey": true, "modelName": true, "temperature": true, "maxTokens": true, "contextWindow": true, "maxOutputTokens": true, "capabilitiesJson": true, "providerConfig": true, "topP": true, "timeoutSeconds": true, "retryCount": true}

func filterModelUpdates(updates map[string]any) {
	filterConfigUpdates(updates, allowedModelUpdateFields)
}

var allowedTTSUpdateFields = map[string]bool{"name": true, "api_type": true, "api_key": true, "base_url": true, "resource_id": true, "voice_type": true, "emotion": true, "speed": true, "pitch": true, "volume": true, "realtime_app_id": true, "realtime_access_token": true, "realtime_secret_key": true}
var allowedASRUpdateFields = map[string]bool{"name": true, "api_type": true, "api_key": true, "base_url": true, "resource_id": true}

var ttsUpdateAliases = map[string]string{
	"apiType": "api_type", "apiKey": "api_key", "baseUrl": "base_url", "resourceId": "resource_id", "voiceType": "voice_type",
	"realtimeAppId": "realtime_app_id", "realtimeAccessToken": "realtime_access_token", "realtimeSecretKey": "realtime_secret_key",
}
var asrUpdateAliases = map[string]string{"apiType": "api_type", "apiKey": "api_key", "baseUrl": "base_url", "resourceId": "resource_id"}

func normalizeConfigUpdates(updates map[string]any, aliases map[string]string, allowed map[string]bool) map[string]any {
	normalized := make(map[string]any, len(updates))
	for key, value := range updates {
		if alias, ok := aliases[key]; ok {
			key = alias
		}
		if allowed[key] {
			normalized[key] = value
		}
	}
	return normalized
}

func filterConfigUpdates(updates map[string]any, allowed map[string]bool) {
	for k := range updates {
		if !allowed[k] {
			delete(updates, k)
		}
	}
}
