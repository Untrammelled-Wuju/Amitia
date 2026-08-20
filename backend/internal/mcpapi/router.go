package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/auth"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/dependency"
	"github.com/u-ai/backend/internal/mcp/discovery"
	"github.com/u-ai/backend/internal/mcp/features"
	"github.com/u-ai/backend/internal/mcp/host"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type ConnectionManager interface {
	Connect(context.Context, string) error
	Disconnect(context.Context, string) error
	Reconnect(context.Context, string) error
	Call(context.Context, string, string, any, client.CallOptions) (json.RawMessage, error)
	Connection(string) (*client.Connection, bool)
}

type ToolSyncer interface {
	RegisterServer(context.Context, string) error
	UnregisterServer(context.Context, string) error
}

type Services struct {
	Repository   *mcp.Repository
	Connections  ConnectionManager
	Auth         *auth.Manager
	Discovery    *discovery.Service
	Skills       ToolSyncer
	Secrets      auth.SecretStore
	Extensions   *extension.Runtime
	Features     *features.Service
	Dependencies *dependency.Service
	Interactions *host.Broker
}

type Handler struct{ services Services }

type serverRequest struct {
	mcp.ServerInput
	Credential              json.RawMessage `json:"credential"`
	PrivateNetworkConfirmed bool            `json:"privateNetworkConfirmed"`
}

func RegisterOAuthCallback(routes gin.IRoutes, services Services) {
	handler := &Handler{services: services}
	routes.GET("/api/mcp/oauth/callback", handler.oauthCallback)
}

func RegisterRouter(group *gin.RouterGroup, _ *app.AppContext, services Services) {
	handler := &Handler{services: services}
	routes := group.Group("/mcp")
	routes.GET("/servers", handler.listServers)
	routes.POST("/servers", handler.createServer)
	routes.GET("/servers/:id", handler.getServer)
	routes.PUT("/servers/:id", handler.updateServer)
	routes.DELETE("/servers/:id", handler.deleteServer)
	routes.POST("/servers/:id/test", handler.testServer)
	routes.POST("/servers/:id/connect", handler.connectServer)
	routes.POST("/servers/:id/disconnect", handler.disconnectServer)
	routes.POST("/servers/:id/reconnect", handler.reconnectServer)
	routes.POST("/servers/:id/refresh", handler.refreshServer)
	routes.GET("/servers/:id/tools", handler.tools)
	routes.GET("/servers/:id/resources", handler.resources)
	routes.GET("/servers/:id/prompts", handler.prompts)
	routes.POST("/servers/:id/resources/read", handler.readResource)
	routes.POST("/servers/:id/resources/subscribe", handler.subscribeResource)
	routes.POST("/servers/:id/resources/unsubscribe", handler.unsubscribeResource)
	routes.POST("/servers/:id/prompts/get", handler.getPrompt)
	routes.POST("/servers/:id/completion", handler.complete)
	routes.GET("/servers/:id/logs", handler.logs)
	routes.PUT("/servers/:id/scope", handler.serverScope)
	routes.PUT("/servers/:id/tools/:toolId/scope", handler.toolScope)
	routes.PUT("/servers/:id/tools/:toolId/permissions", handler.toolPermissions)
	routes.GET("/servers/:id/capabilities", handler.capabilities)
	routes.PUT("/servers/:id/capabilities/:capability", handler.capability)
	routes.GET("/servers/:id/tasks", handler.tasks)
	routes.POST("/servers/:id/tasks/:taskId/cancel", handler.cancelRemoteTask)
	routes.POST("/servers/:id/oauth/start", handler.oauthStart)
	routes.POST("/servers/:id/oauth/revoke", handler.oauthRevoke)
	routes.POST("/agent-skills/dependencies/preview", handler.dependencyPreview)
	routes.POST("/agent-skills/dependencies/install", handler.dependencyInstall)
	routes.GET("/agent-skills/:skillId/dependencies", handler.dependencies)
	routes.DELETE("/agent-skills/:skillId/dependencies", handler.removeDependencies)
	routes.GET("/operations", handler.operations)
	routes.GET("/operations/:id", handler.operation)
	routes.GET("/interactions", handler.interactions)
	routes.POST("/interactions/:id/resolve", handler.resolveInteraction)
}

func (h *Handler) listServers(c *gin.Context) {
	records, err := h.services.Repository.ListServers(c)
	if err == nil {
		for index := range records {
			records[index].PrivateNetworkConfirmed, _, _ = h.services.Repository.ServerCapabilityEnabled(c, records[index].ID, "private_network")
		}
	}
	respond(c, records, err)
}
func (h *Handler) getServer(c *gin.Context) {
	record, err := h.services.Repository.GetServer(c, c.Param("id"))
	if err == nil {
		record.PrivateNetworkConfirmed, _, _ = h.services.Repository.ServerCapabilityEnabled(c, record.ID, "private_network")
	}
	respond(c, record, err)
}

func (h *Handler) createServer(c *gin.Context) {
	var request serverRequest
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "配置格式无效")
		return
	}
	record, err := h.services.Repository.CreateServer(c, request.ServerInput)
	if err == nil {
		err = h.storeCredential(c, record, request.Credential)
	}
	if err == nil {
		err = h.services.Repository.SetScopeEnabled(c, record.ID, "global", "", request.Enabled)
	}
	if err == nil {
		_, err = h.services.Repository.SetServerCapability(c, record.ID, "private_network", request.PrivateNetworkConfirmed, json.RawMessage(`{}`))
	}
	if err != nil {
		if record.ID != "" {
			_ = h.services.Repository.DeleteServer(context.Background(), record.ID)
		}
		respond(c, nil, err)
		return
	}
	if request.Enabled {
		go h.connectAndSync(context.Background(), record.ID)
	}
	record.PrivateNetworkConfirmed = request.PrivateNetworkConfirmed
	c.JSON(http.StatusCreated, gin.H{"data": record})
}

func (h *Handler) updateServer(c *gin.Context) {
	var request serverRequest
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "配置格式无效")
		return
	}
	record, err := h.services.Repository.UpdateServer(c, c.Param("id"), request.ServerInput)
	if err == nil && len(request.Credential) > 0 && string(request.Credential) != "null" {
		err = h.storeCredential(c, record, request.Credential)
	}
	if err == nil {
		err = h.services.Repository.SetScopeEnabled(c, record.ID, "global", "", request.Enabled)
	}
	if err == nil {
		_, err = h.services.Repository.SetServerCapability(c, record.ID, "private_network", request.PrivateNetworkConfirmed, json.RawMessage(`{}`))
	}
	if err == nil {
		if record.Enabled == 1 {
			go h.reconnectAndSync(context.Background(), record.ID)
		} else {
			_ = h.services.Skills.UnregisterServer(c, record.ID)
			go h.services.Connections.Disconnect(context.Background(), record.ID)
		}
	}
	record.PrivateNetworkConfirmed = request.PrivateNetworkConfirmed
	respond(c, record, err)
}

func (h *Handler) deleteServer(c *gin.Context) {
	id := c.Param("id")
	_ = h.services.Skills.UnregisterServer(c, id)
	references, err := h.services.Repository.CredentialReferences(c, id)
	if err == nil {
		h.cancelServerTasks(c, id)
	}
	if err == nil {
		err = h.services.Connections.Disconnect(c, id)
	}
	if err == nil {
		err = h.services.Repository.DeleteServer(c, id)
	}
	if err == nil {
		for _, reference := range references {
			_ = h.services.Secrets.Delete(context.Background(), reference)
		}
	}
	respond(c, gin.H{"deleted": err == nil}, err)
}

func (h *Handler) cancelServerTasks(ctx context.Context, serverID string) {
	tasks, err := h.services.Repository.ListTasks(ctx, serverID, 20)
	if err != nil {
		return
	}
	limited, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for _, task := range tasks {
		if task.Status != "working" && task.Status != "input_required" {
			continue
		}
		_, _ = h.services.Connections.Call(limited, serverID, "tasks/cancel", map[string]any{"taskId": task.RemoteTaskID}, client.CallOptions{Timeout: time.Second})
		if limited.Err() != nil {
			return
		}
	}
}

func (h *Handler) testServer(c *gin.Context) {
	err := h.services.Connections.Connect(c, c.Param("id"))
	if err == nil {
		err = h.services.Discovery.Discover(c, c.Param("id"))
	}
	respond(c, gin.H{"ok": err == nil}, err)
}
func (h *Handler) connectServer(c *gin.Context) {
	err := h.connectAndSync(c, c.Param("id"))
	respond(c, gin.H{"connected": err == nil}, err)
}
func (h *Handler) disconnectServer(c *gin.Context) {
	respond(c, gin.H{"disconnected": true}, h.services.Connections.Disconnect(c, c.Param("id")))
}
func (h *Handler) reconnectServer(c *gin.Context) {
	err := h.reconnectAndSync(c, c.Param("id"))
	respond(c, gin.H{"connected": err == nil}, err)
}
func (h *Handler) refreshServer(c *gin.Context) {
	err := h.services.Discovery.Discover(c, c.Param("id"))
	if err == nil {
		err = h.services.Skills.RegisterServer(c, c.Param("id"))
	}
	respond(c, gin.H{"refreshed": err == nil}, err)
}

func (h *Handler) connectAndSync(ctx context.Context, serverID string) error {
	if h.services.Connections == nil || h.services.Discovery == nil || h.services.Skills == nil {
		return fmt.Errorf("MCP runtime unavailable")
	}
	if err := h.services.Connections.Connect(ctx, serverID); err != nil {
		return err
	}
	if err := h.services.Discovery.Discover(ctx, serverID); err != nil {
		_ = h.services.Connections.Disconnect(context.Background(), serverID)
		return err
	}
	if err := h.services.Skills.RegisterServer(ctx, serverID); err != nil {
		_ = h.services.Connections.Disconnect(context.Background(), serverID)
		return err
	}
	return nil
}

func (h *Handler) reconnectAndSync(ctx context.Context, serverID string) error {
	if h.services.Connections == nil {
		return fmt.Errorf("MCP runtime unavailable")
	}
	if err := h.services.Connections.Reconnect(ctx, serverID); err != nil {
		return err
	}
	if h.services.Discovery == nil || h.services.Skills == nil {
		return fmt.Errorf("MCP runtime unavailable")
	}
	if err := h.services.Discovery.Discover(ctx, serverID); err != nil {
		return err
	}
	return h.services.Skills.RegisterServer(ctx, serverID)
}

func (h *Handler) tools(c *gin.Context) {
	records, err := h.services.Repository.ListTools(c, c.Param("id"), false)
	respond(c, records, err)
}
func (h *Handler) resources(c *gin.Context) {
	records, templates, err := h.services.Repository.ListResources(c, c.Param("id"), false)
	respond(c, gin.H{"resources": records, "resourceTemplates": templates}, err)
}
func (h *Handler) prompts(c *gin.Context) {
	records, err := h.services.Repository.ListPrompts(c, c.Param("id"), false)
	respond(c, records, err)
}
func (h *Handler) readResource(c *gin.Context) {
	var request struct {
		CharacterID string `json:"characterId"`
		URI         string `json:"uri"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_RESOURCE_NOT_FOUND", "资源参数无效")
		return
	}
	result, err := h.services.Features.ReadResource(c, c.Param("id"), request.CharacterID, request.URI)
	respond(c, result, err)
}
func (h *Handler) subscribeResource(c *gin.Context) {
	var request struct {
		CharacterID string `json:"characterId"`
		URI         string `json:"uri"`
	}
	if c.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.URI) == "" {
		problem(c, http.StatusBadRequest, "MCP_RESOURCE_NOT_FOUND", "资源参数无效")
		return
	}
	err := h.services.Features.Subscribe(c, c.Param("id"), request.CharacterID, request.URI)
	respond(c, gin.H{"subscribed": err == nil}, err)
}
func (h *Handler) unsubscribeResource(c *gin.Context) {
	var request struct {
		CharacterID string `json:"characterId"`
		URI         string `json:"uri"`
	}
	if c.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.URI) == "" {
		problem(c, http.StatusBadRequest, "MCP_RESOURCE_NOT_FOUND", "资源参数无效")
		return
	}
	err := h.services.Features.Unsubscribe(c, c.Param("id"), request.CharacterID, request.URI)
	respond(c, gin.H{"subscribed": false}, err)
}
func (h *Handler) getPrompt(c *gin.Context) {
	var request struct {
		CharacterID string            `json:"characterId"`
		Name        string            `json:"name"`
		Arguments   map[string]string `json:"arguments"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_PROMPT_NOT_FOUND", "Prompt 参数无效")
		return
	}
	result, err := h.services.Features.GetPrompt(c, c.Param("id"), request.CharacterID, request.Name, request.Arguments)
	respond(c, result, err)
}
func (h *Handler) complete(c *gin.Context) {
	var request struct {
		CharacterID      string            `json:"characterId"`
		Reference        map[string]any    `json:"ref"`
		Argument         map[string]string `json:"argument"`
		ContextArguments map[string]string `json:"contextArguments"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_COMPLETION_INVALID", "补全参数无效")
		return
	}
	result, err := h.services.Features.Complete(c, c.Param("id"), request.CharacterID, request.Reference, request.Argument, request.ContextArguments)
	respond(c, result, err)
}
func (h *Handler) logs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	records, err := h.services.Repository.ListAuditLogs(c, c.Param("id"), limit)
	respond(c, records, err)
}

func (h *Handler) serverScope(c *gin.Context) {
	var request struct {
		ScopeType string `json:"scopeType"`
		ScopeID   string `json:"scopeId"`
		Enabled   bool   `json:"enabled"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "作用域无效")
		return
	}
	err := h.services.Repository.SetScopeEnabled(c, c.Param("id"), request.ScopeType, request.ScopeID, request.Enabled)
	respond(c, gin.H{"updated": err == nil}, err)
}

func (h *Handler) toolScope(c *gin.Context) {
	tool, err := h.services.Repository.GetTool(c, c.Param("toolId"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	if tool.ServerID != c.Param("id") {
		problem(c, http.StatusNotFound, "MCP_TOOL_NOT_FOUND", "工具不存在")
		return
	}
	var request struct {
		CharacterID string `json:"characterId"`
		Enabled     bool   `json:"enabled"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "作用域无效")
		return
	}
	if request.CharacterID == "" {
		err = h.services.Repository.SetToolEnabled(c, tool.ID, request.Enabled)
		if err == nil {
			err = h.services.Skills.RegisterServer(c, tool.ServerID)
		}
	} else {
		err = h.services.Extensions.Registry.SetScopeEnabled(c, tool.SkillID, extension.ExecutionScope{CharacterID: request.CharacterID}, request.Enabled)
	}
	respond(c, gin.H{"updated": err == nil}, err)
}

func (h *Handler) toolPermissions(c *gin.Context) {
	tool, err := h.services.Repository.GetTool(c, c.Param("toolId"))
	if err != nil || tool.ServerID != c.Param("id") {
		respond(c, nil, err)
		return
	}
	var grants []extension.PermissionGrantInput
	if c.ShouldBindJSON(&grants) != nil {
		problem(c, http.StatusBadRequest, "MCP_TOOL_PERMISSION_DENIED", "权限格式无效")
		return
	}
	err = h.services.Extensions.Repository.ReplaceGrants(c, tool.SkillID, grants)
	respond(c, gin.H{"updated": err == nil}, err)
}

func (h *Handler) capabilities(c *gin.Context) {
	records, err := h.services.Repository.ListServerCapabilities(c, c.Param("id"))
	respond(c, records, err)
}

func (h *Handler) capability(c *gin.Context) {
	var request struct {
		Enabled       bool            `json:"enabled"`
		Configuration json.RawMessage `json:"configuration"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "能力配置无效")
		return
	}
	capability := strings.ToLower(strings.TrimSpace(c.Param("capability")))
	if capability == "sampling" && request.Enabled {
		var config struct {
			MaxTokens      int `json:"maxTokens"`
			TimeoutSeconds int `json:"timeoutSeconds"`
			MaxConcurrent  int `json:"maxConcurrent"`
		}
		if json.Unmarshal(request.Configuration, &config) != nil || config.MaxTokens < 1 || config.MaxTokens > 8192 || config.TimeoutSeconds < 1 || config.TimeoutSeconds > 300 || config.MaxConcurrent < 1 || config.MaxConcurrent > 4 {
			problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "Sampling 限额无效")
			return
		}
	}
	if capability == "tasks" && request.Enabled {
		var config struct {
			MaxConcurrent int `json:"maxConcurrent"`
			MaxTTLSeconds int `json:"maxTTLSeconds"`
		}
		if json.Unmarshal(request.Configuration, &config) != nil || config.MaxConcurrent < 1 || config.MaxConcurrent > 20 || config.MaxTTLSeconds < 60 || config.MaxTTLSeconds > 604800 {
			problem(c, http.StatusBadRequest, "MCP_SERVER_CONFIGURATION_INVALID", "Tasks 限额无效")
			return
		}
	}
	record, err := h.services.Repository.SetServerCapability(c, c.Param("id"), capability, request.Enabled, request.Configuration)
	if err == nil {
		server, serverErr := h.services.Repository.GetServer(c, c.Param("id"))
		if serverErr != nil {
			err = serverErr
		} else if server.Enabled == 1 {
			err = h.services.Connections.Reconnect(c, c.Param("id"))
		}
	}
	respond(c, record, err)
}

func (h *Handler) tasks(c *gin.Context) {
	enabled, _, err := h.services.Repository.ServerCapabilityEnabled(c, c.Param("id"), "tasks")
	if err == nil && !enabled {
		problem(c, http.StatusForbidden, "MCP_TASKS_DISABLED", "该服务未启用实验性 Tasks 能力")
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if err == nil {
		err = h.services.Repository.DeleteExpiredTasks(c, c.Param("id"), time.Now())
	}
	records, listErr := h.services.Repository.ListTasks(c, c.Param("id"), limit)
	if err == nil {
		err = listErr
	}
	respond(c, records, err)
}

func (h *Handler) cancelRemoteTask(c *gin.Context) {
	enabled, _, err := h.services.Repository.ServerCapabilityEnabled(c, c.Param("id"), "tasks")
	if err == nil && !enabled {
		problem(c, http.StatusForbidden, "MCP_TASKS_DISABLED", "该服务未启用实验性 Tasks 能力")
		return
	}
	if err == nil {
		_, err = h.services.Connections.Call(c, c.Param("id"), "tasks/cancel", map[string]any{"taskId": c.Param("taskId")}, client.CallOptions{Timeout: 10 * time.Second})
	}
	if err == nil {
		task, taskErr := h.services.Repository.GetTask(c, c.Param("id"), c.Param("taskId"))
		if taskErr == nil {
			task.Status = "cancelled"
			task.StatusMessage = "Cancelled by user"
			err = h.services.Repository.UpsertTask(c, task)
		}
	}
	respond(c, gin.H{"cancelled": err == nil}, err)
}

func (h *Handler) oauthStart(c *gin.Context) {
	var request struct {
		ResourceURL  string   `json:"resourceUrl"`
		MetadataURL  string   `json:"metadataUrl"`
		RedirectURI  string   `json:"redirectUri"`
		ClientID     string   `json:"clientId"`
		ClientSecret string   `json:"clientSecret"`
		Scopes       []string `json:"scopes"`
	}
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_OAUTH_DISCOVERY_FAILED", "授权配置无效")
		return
	}
	if strings.TrimSpace(request.RedirectURI) == "" {
		request.RedirectURI = oauthCallbackURL(c)
	}
	result, err := h.services.Auth.Begin(c, auth.BeginRequest{ServerID: c.Param("id"), ResourceURL: request.ResourceURL, MetadataURL: request.MetadataURL, RedirectURI: request.RedirectURI, ClientID: request.ClientID, ClientSecret: request.ClientSecret, Scopes: request.Scopes})
	respond(c, result, err)
}

func oauthCallbackURL(c *gin.Context) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if comma := strings.Index(scheme, ","); comma >= 0 {
		scheme = strings.TrimSpace(scheme[:comma])
	}
	if scheme != "http" && scheme != "https" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if comma := strings.Index(host, ","); comma >= 0 {
		host = strings.TrimSpace(host[:comma])
	}
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + "/api/mcp/oauth/callback"
}

func (h *Handler) oauthCallback(c *gin.Context) {
	if oauthError := c.Query("error"); oauthError != "" {
		problem(c, http.StatusBadRequest, "MCP_OAUTH_CALLBACK_FAILED", "授权已取消")
		return
	}
	sessionID := c.Query("session")
	serverID := ""
	var lookupErr error
	if sessionID == "" {
		session, findErr := h.services.Repository.FindOAuthSessionByStateHash(c, auth.HashState(c.Query("state")))
		lookupErr = findErr
		if findErr == nil {
			sessionID = session.ID
			serverID = session.ServerID
		}
	} else {
		serverID, lookupErr = h.services.Repository.OAuthSessionServerID(c, sessionID)
	}
	if lookupErr != nil {
		respond(c, nil, lookupErr)
		return
	}
	_, err := h.services.Auth.Callback(c, sessionID, c.Query("state"), c.Query("code"), "", "", "")
	if err == nil {
		err = h.services.Connections.Connect(c, serverID)
	}
	if err == nil {
		err = h.services.Dependencies.AuthorizationCompleted(c, serverID)
	}
	respond(c, gin.H{"authorized": err == nil, "serverId": serverID}, err)
}

func (h *Handler) oauthRevoke(c *gin.Context) {
	respond(c, gin.H{"revoked": true}, h.services.Auth.Revoke(c, c.Param("id")))
}
func (h *Handler) dependencyPreview(c *gin.Context) {
	var request dependency.PreviewRequest
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_DEPENDENCY_PLAN_INVALID", "依赖声明无效")
		return
	}
	result, err := h.services.Dependencies.Preview(c, request)
	respond(c, result, err)
}
func (h *Handler) dependencyInstall(c *gin.Context) {
	var request dependency.InstallRequest
	if c.ShouldBindJSON(&request) != nil {
		problem(c, http.StatusBadRequest, "MCP_DEPENDENCY_PLAN_INVALID", "安装计划无效")
		return
	}
	result, err := h.services.Dependencies.Install(c, request)
	respond(c, result, err)
}
func (h *Handler) dependencies(c *gin.Context) {
	records, err := h.services.Repository.ListDependencyLinks(c, c.Param("skillId"))
	respond(c, records, err)
}

func (h *Handler) removeDependencies(c *gin.Context) {
	serverIDs, err := h.services.Dependencies.Uninstall(c, c.Param("skillId"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	unreferenced := []string{}
	for _, serverID := range serverIDs {
		count, countErr := h.services.Repository.ServerDependencyReferenceCount(c, serverID)
		if countErr == nil && count == 0 {
			unreferenced = append(unreferenced, serverID)
		}
	}
	respond(c, gin.H{"serverIds": serverIDs, "unreferencedServerIds": unreferenced}, nil)
}
func (h *Handler) operations(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	records, err := h.services.Repository.ListOperations(c, limit)
	respond(c, records, err)
}
func (h *Handler) operation(c *gin.Context) {
	record, err := h.services.Repository.GetOperation(c, c.Param("id"))
	respond(c, record, err)
}

func (h *Handler) interactions(c *gin.Context) {
	if h.services.Interactions == nil {
		respond(c, []host.PendingInteraction{}, nil)
		return
	}
	items := h.services.Interactions.List()
	servers, err := h.services.Repository.ListServers(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	names := map[string]string{}
	for _, server := range servers {
		name := server.DisplayName
		if name == "" {
			name = server.Name
		}
		names[server.ID] = name
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"id": item.ID, "serverId": item.ServerID, "serverName": names[item.ServerID], "kind": item.Kind, "request": item.Request, "createdAt": item.CreatedAt, "expiresAt": item.ExpiresAt})
	}
	respond(c, result, nil)
}

func (h *Handler) resolveInteraction(c *gin.Context) {
	if h.services.Interactions == nil {
		problem(c, http.StatusNotFound, "MCP_INTERACTION_NOT_FOUND", "待处理请求不存在")
		return
	}
	var decision host.InteractionDecision
	if c.ShouldBindJSON(&decision) != nil {
		problem(c, http.StatusBadRequest, "MCP_INTERACTION_INVALID", "处理结果无效")
		return
	}
	err := h.services.Interactions.Resolve(c.Param("id"), decision)
	respond(c, gin.H{"resolved": err == nil}, err)
}

func (h *Handler) storeCredential(ctx context.Context, server mcp.Server, value json.RawMessage) error {
	if server.AuthType == "none" || server.AuthType == "oauth" || len(value) == 0 || string(value) == "null" {
		return nil
	}
	raw := value
	if server.AuthType == "bearer_token" {
		var token string
		if json.Unmarshal(value, &token) != nil || strings.TrimSpace(token) == "" {
			return &apiError{code: "MCP_SERVER_CONFIGURATION_INVALID", message: "Token 无效"}
		}
		raw = []byte(token)
	}
	reference, err := h.services.Secrets.Put(ctx, server.ID+"-"+server.AuthType, raw)
	if err != nil {
		return err
	}
	_, err = h.services.Repository.PutCredentialReference(ctx, server.ID, server.AuthType, reference, "", nil)
	if err != nil {
		_ = h.services.Secrets.Delete(ctx, reference)
	}
	return err
}

func callbackURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/api/mcp/oauth/callback"
}

type apiError struct{ code, message string }

func (e *apiError) Error() string { return e.code + ": " + e.message }
func respond(c *gin.Context, data any, err error) {
	if err != nil {
		code := "MCP_INTERNAL_ERROR"
		message := err.Error()
		if index := strings.Index(message, ":"); index > 0 && strings.HasPrefix(message[:index], "MCP_") {
			code = message[:index]
		} else if strings.HasPrefix(message, "MCP_") {
			code = strings.Fields(message)[0]
		}
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound), strings.Contains(code, "NOT_FOUND"):
			status = http.StatusNotFound
		case strings.Contains(code, "AUTH_REQUIRED"), strings.Contains(code, "AUTH_EXPIRED"):
			status = http.StatusUnauthorized
		case strings.Contains(code, "DENIED"), strings.Contains(code, "DISABLED"), strings.Contains(code, "PERMISSION"):
			status = http.StatusForbidden
		case strings.Contains(code, "ALREADY_EXISTS"), strings.Contains(code, "CONFLICT"), strings.Contains(code, "IN_USE"):
			status = http.StatusConflict
		case strings.Contains(code, "TIMEOUT"):
			status = http.StatusGatewayTimeout
		case code == "MCP_INTERNAL_ERROR":
			status = http.StatusInternalServerError
		}
		problem(c, status, code, message)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
func problem(c *gin.Context, status int, code, detail string) {
	c.AbortWithStatusJSON(status, gin.H{"type": "https://errors.amitia.dev/mcp/" + strings.ToLower(code), "title": code, "status": status, "detail": detail, "code": code, "traceId": c.GetString("trace_request_id"), "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
