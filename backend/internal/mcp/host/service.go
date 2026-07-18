package host

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
	"github.com/u-ai/backend/internal/mcp/protocol"
)

type ConnectionProvider interface {
	Connection(string) (*client.Connection, bool)
}
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}
type RootsProvider interface {
	Roots(context.Context, string) ([]Root, error)
}
type SamplingProvider interface {
	CreateMessage(context.Context, string, json.RawMessage) (any, error)
}
type ElicitationProvider interface {
	Elicit(context.Context, string, json.RawMessage) (any, error)
}

type Service struct {
	repository  *mcp.Repository
	connections ConnectionProvider
	roots       RootsProvider
	sampling    SamplingProvider
	elicitation ElicitationProvider
	mu          sync.Mutex
	active      map[string]int
	logs        map[string]*logWindow
}

func New(repository *mcp.Repository, connections ConnectionProvider, roots RootsProvider, sampling SamplingProvider, elicitation ElicitationProvider) *Service {
	return &Service{repository: repository, connections: connections, roots: roots, sampling: sampling, elicitation: elicitation, active: map[string]int{}, logs: map[string]*logWindow{}}
}

func (s *Service) Attach(serverID string) {
	connection, ok := s.connections.Connection(serverID)
	if !ok {
		return
	}
	connection.RegisterRequestHandler("roots/list", s.rootsList(serverID))
	connection.RegisterRequestHandler("sampling/createMessage", s.createMessage(serverID))
	connection.RegisterRequestHandler("elicitation/create", s.elicit(serverID))
	connection.RegisterRequestHandler("tasks/get", s.getTask(serverID))
	connection.RegisterRequestHandler("tasks/result", s.taskResult(serverID))
	connection.RegisterRequestHandler("tasks/list", s.listTasks(serverID))
	connection.RegisterRequestHandler("tasks/cancel", s.cancelTask(serverID))
	connection.RegisterNotificationHandler("notifications/tasks/status", s.taskStatus(serverID))
	connection.RegisterNotificationHandler("notifications/message", s.serverLog(serverID))
	connection.RegisterNotificationHandler("notifications/resources/updated", s.resourceUpdated(serverID))
}

type logWindow struct {
	started time.Time
	count   int
}

var logSecretPattern = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._~+/-]{8,}|(?:api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret)["'\s:=]+[a-z0-9._~+/-]{6,})`)

func (s *Service) serverLog(serverID string) client.NotificationHandler {
	return func(ctx context.Context, params json.RawMessage) {
		now := time.Now()
		s.mu.Lock()
		window := s.logs[serverID]
		if window == nil || now.Sub(window.started) >= time.Minute {
			window = &logWindow{started: now}
			s.logs[serverID] = window
		}
		if window.count >= 60 {
			s.mu.Unlock()
			return
		}
		window.count++
		s.mu.Unlock()
		if len(params) > 8<<10 {
			params = params[:8<<10]
		}
		redacted := logSecretPattern.ReplaceAllString(string(params), "[REDACTED]")
		if !json.Valid([]byte(redacted)) {
			encoded, _ := json.Marshal(map[string]any{"message": redacted})
			redacted = string(encoded)
		}
		_ = s.repository.AddAuditLog(ctx, mcp.AuditLog{ServerID: serverID, Operation: "server_log", Status: "received", SummaryJSON: redacted})
	}
}

func (s *Service) resourceUpdated(serverID string) client.NotificationHandler {
	return func(ctx context.Context, params json.RawMessage) {
		var update struct {
			URI string `json:"uri"`
		}
		if json.Unmarshal(params, &update) != nil || len(update.URI) > 2048 {
			return
		}
		summary, _ := json.Marshal(map[string]any{"uri": update.URI})
		_ = s.repository.AddAuditLog(ctx, mcp.AuditLog{ServerID: serverID, Operation: "resources/updated", Status: "received", SummaryJSON: string(summary)})
	}
}

func (s *Service) rootsList(serverID string) client.RequestHandler {
	return func(ctx context.Context, _ json.RawMessage) (any, *protocol.RPCError) {
		enabled, _, err := s.repository.ServerCapabilityEnabled(ctx, serverID, "roots")
		if err != nil || !enabled {
			return map[string]any{"roots": []Root{}}, nil
		}
		if s.roots == nil {
			return map[string]any{"roots": []Root{}}, nil
		}
		roots, err := s.roots.Roots(ctx, serverID)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrorInternal, "Roots unavailable", nil)
		}
		for _, root := range roots {
			parsed, parseErr := url.Parse(root.URI)
			if parseErr != nil || parsed.Scheme != "file" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (!strings.HasPrefix(parsed.Path, "/") && parsed.Host == "") {
				return nil, protocol.NewError(protocol.ErrorInvalidParams, "Invalid root", nil)
			}
		}
		return map[string]any{"roots": roots}, nil
	}
}
func (s *Service) createMessage(serverID string) client.RequestHandler {
	return func(ctx context.Context, params json.RawMessage) (any, *protocol.RPCError) {
		enabled, rawConfig, err := s.repository.ServerCapabilityEnabled(ctx, serverID, "sampling")
		if err != nil || !enabled {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Sampling is not enabled for this server", nil)
		}
		if s.sampling == nil {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Sampling requires user authorization", nil)
		}
		config := samplingConfiguration(rawConfig)
		if !s.acquire(serverID, config.MaxConcurrent) {
			return nil, protocol.NewError(protocol.ErrorInternal, "Sampling concurrency limit reached", nil)
		}
		defer s.release(serverID)
		params, rpcErr := limitSamplingRequest(params, config.MaxTokens)
		if rpcErr != nil {
			return nil, rpcErr
		}
		limited, cancel := context.WithTimeout(ctx, time.Duration(config.TimeoutSeconds)*time.Second)
		defer cancel()
		result, err := s.sampling.CreateMessage(limited, serverID, params)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrorInternal, "Sampling failed", nil)
		}
		return result, nil
	}
}
func (s *Service) elicit(serverID string) client.RequestHandler {
	return func(ctx context.Context, params json.RawMessage) (any, *protocol.RPCError) {
		enabled, _, err := s.repository.ServerCapabilityEnabled(ctx, serverID, "elicitation")
		if err != nil || !enabled {
			return map[string]any{"action": "decline"}, nil
		}
		if rpcErr := validateElicitation(params); rpcErr != nil {
			return map[string]any{"action": "decline"}, rpcErr
		}
		if s.elicitation == nil {
			return map[string]any{"action": "decline"}, nil
		}
		limited, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		result, err := s.elicitation.Elicit(limited, serverID, params)
		if err != nil {
			return map[string]any{"action": "cancel"}, nil
		}
		return result, nil
	}
}
func (s *Service) getTask(serverID string) client.RequestHandler {
	return func(ctx context.Context, params json.RawMessage) (any, *protocol.RPCError) {
		if !s.tasksEnabled(ctx, serverID) {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Tasks are not enabled for this server", nil)
		}
		var request struct {
			TaskID string `json:"taskId"`
		}
		if json.Unmarshal(params, &request) != nil || request.TaskID == "" {
			return nil, protocol.NewError(protocol.ErrorInvalidParams, "taskId is required", nil)
		}
		task, err := s.repository.GetTask(ctx, serverID, request.TaskID)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrorInvalidParams, "Task not found", nil)
		}
		return taskPayload(task), nil
	}
}
func (s *Service) taskResult(serverID string) client.RequestHandler {
	return func(ctx context.Context, params json.RawMessage) (any, *protocol.RPCError) {
		result, rpcErr := s.getTask(serverID)(ctx, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		payload := result.(map[string]any)
		if !terminal(payload["status"].(string)) {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Task result is not ready", nil)
		}
		return payload, nil
	}
}
func (s *Service) listTasks(serverID string) client.RequestHandler {
	return func(ctx context.Context, _ json.RawMessage) (any, *protocol.RPCError) {
		if !s.tasksEnabled(ctx, serverID) {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Tasks are not enabled for this server", nil)
		}
		tasks, err := s.repository.ListTasks(ctx, serverID, 100)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrorInternal, "Tasks unavailable", nil)
		}
		items := make([]map[string]any, 0, len(tasks))
		for _, task := range tasks {
			items = append(items, taskPayload(task))
		}
		return map[string]any{"tasks": items}, nil
	}
}
func (s *Service) cancelTask(serverID string) client.RequestHandler {
	return func(ctx context.Context, params json.RawMessage) (any, *protocol.RPCError) {
		if !s.tasksEnabled(ctx, serverID) {
			return nil, protocol.NewError(protocol.ErrorInvalidRequest, "Tasks are not enabled for this server", nil)
		}
		var request struct {
			TaskID string `json:"taskId"`
		}
		if json.Unmarshal(params, &request) != nil || request.TaskID == "" {
			return nil, protocol.NewError(protocol.ErrorInvalidParams, "taskId is required", nil)
		}
		task, err := s.repository.GetTask(ctx, serverID, request.TaskID)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrorInvalidParams, "Task not found", nil)
		}
		if terminal(task.Status) {
			return taskPayload(task), nil
		}
		task.Status = "cancelled"
		task.StatusMessage = "Cancelled by MCP client"
		if err := s.repository.UpsertTask(ctx, task); err != nil {
			return nil, protocol.NewError(protocol.ErrorInternal, "Task cancellation failed", nil)
		}
		return taskPayload(task), nil
	}
}
func (s *Service) taskStatus(serverID string) client.NotificationHandler {
	return func(ctx context.Context, params json.RawMessage) {
		enabled, rawConfig, capabilityErr := s.repository.ServerCapabilityEnabled(ctx, serverID, "tasks")
		if capabilityErr != nil || !enabled {
			return
		}
		var update struct {
			TaskID        string          `json:"taskId"`
			Status        string          `json:"status"`
			StatusMessage string          `json:"statusMessage"`
			Result        json.RawMessage `json:"result"`
			ExpiresAt     string          `json:"expiresAt"`
		}
		if json.Unmarshal(params, &update) != nil || update.TaskID == "" || !validStatus(update.Status) {
			return
		}
		result := update.Result
		if len(result) == 0 {
			result = json.RawMessage(`{}`)
		}
		if len(result) > 2<<20 {
			result = json.RawMessage(`{"truncated":true}`)
		}
		config := taskConfiguration(rawConfig)
		_ = s.repository.DeleteExpiredTasks(ctx, serverID, time.Now())
		if update.Status == "working" {
			tasks, listErr := s.repository.ListTasks(ctx, serverID, 500)
			if listErr != nil {
				return
			}
			active := 0
			known := false
			for _, task := range tasks {
				if task.RemoteTaskID == update.TaskID {
					known = true
				}
				if task.Status == "working" || task.Status == "input_required" {
					active++
				}
			}
			if !known && active >= config.MaxConcurrent {
				return
			}
		}
		expires := time.Now().Add(time.Duration(config.MaxTTLSeconds) * time.Second)
		if requested, parseErr := time.Parse(time.RFC3339Nano, update.ExpiresAt); parseErr == nil && requested.Before(expires) && requested.After(time.Now()) {
			expires = requested
		}
		update.ExpiresAt = expires.UTC().Format(time.RFC3339Nano)
		_ = s.repository.UpsertTask(ctx, mcp.Task{ServerID: serverID, RemoteTaskID: update.TaskID, Status: update.Status, StatusMessage: update.StatusMessage, ResultJSON: string(result), ExpiresAt: update.ExpiresAt})
	}
}

type taskConfig struct {
	MaxConcurrent int
	MaxTTLSeconds int
}

func taskConfiguration(raw json.RawMessage) taskConfig {
	config := taskConfig{MaxConcurrent: 5, MaxTTLSeconds: 86400}
	var value struct {
		MaxConcurrent int `json:"maxConcurrent"`
		MaxTTLSeconds int `json:"maxTTLSeconds"`
	}
	if json.Unmarshal(raw, &value) == nil {
		if value.MaxConcurrent > 0 && value.MaxConcurrent <= 20 {
			config.MaxConcurrent = value.MaxConcurrent
		}
		if value.MaxTTLSeconds >= 60 && value.MaxTTLSeconds <= 604800 {
			config.MaxTTLSeconds = value.MaxTTLSeconds
		}
	}
	return config
}

type samplingConfig struct {
	MaxTokens      int
	TimeoutSeconds int
	MaxConcurrent  int
}

func samplingConfiguration(raw json.RawMessage) samplingConfig {
	config := samplingConfig{MaxTokens: 2048, TimeoutSeconds: 60, MaxConcurrent: 1}
	var value struct {
		MaxTokens      int `json:"maxTokens"`
		TimeoutSeconds int `json:"timeoutSeconds"`
		MaxConcurrent  int `json:"maxConcurrent"`
	}
	if json.Unmarshal(raw, &value) == nil {
		if value.MaxTokens > 0 && value.MaxTokens <= 8192 {
			config.MaxTokens = value.MaxTokens
		}
		if value.TimeoutSeconds > 0 && value.TimeoutSeconds <= 300 {
			config.TimeoutSeconds = value.TimeoutSeconds
		}
		if value.MaxConcurrent > 0 && value.MaxConcurrent <= 4 {
			config.MaxConcurrent = value.MaxConcurrent
		}
	}
	return config
}

func limitSamplingRequest(raw json.RawMessage, maxTokens int) (json.RawMessage, *protocol.RPCError) {
	var request map[string]any
	if json.Unmarshal(raw, &request) != nil {
		return nil, protocol.NewError(protocol.ErrorInvalidParams, "Invalid sampling request", nil)
	}
	if requested, ok := request["maxTokens"].(float64); ok && requested > float64(maxTokens) {
		request["maxTokens"] = maxTokens
	}
	limited, err := json.Marshal(request)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrorInvalidParams, "Invalid sampling request", nil)
	}
	return limited, nil
}

func validateElicitation(raw json.RawMessage) *protocol.RPCError {
	var request map[string]any
	if json.Unmarshal(raw, &request) != nil {
		return protocol.NewError(protocol.ErrorInvalidParams, "Invalid elicitation request", nil)
	}
	mode, _ := request["mode"].(string)
	if mode == "url" {
		target, _ := request["url"].(string)
		parsed, err := url.Parse(target)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return protocol.NewError(protocol.ErrorInvalidParams, "Unsafe elicitation URL", nil)
		}
		return nil
	}
	if mode != "" && mode != "form" {
		return protocol.NewError(protocol.ErrorInvalidParams, "Unsupported elicitation mode", nil)
	}
	encoded := strings.ToLower(string(raw))
	for _, word := range []string{"password", "api_key", "apikey", "access_token", "refreshtoken", "refresh_token", "private_key", "payment", "credit_card", "银行卡", "密码", "私钥"} {
		if strings.Contains(encoded, word) {
			return protocol.NewError(protocol.ErrorInvalidParams, "Sensitive elicitation fields are not allowed", nil)
		}
	}
	schema, ok := request["requestedSchema"].(map[string]any)
	if !ok {
		schema, ok = request["schema"].(map[string]any)
	}
	if !ok || schema["type"] != "object" {
		return protocol.NewError(protocol.ErrorInvalidParams, "Invalid elicitation schema", nil)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || len(properties) > 50 {
		return protocol.NewError(protocol.ErrorInvalidParams, "Invalid elicitation schema", nil)
	}
	for name, rawField := range properties {
		if len(name) == 0 || len(name) > 100 {
			return protocol.NewError(protocol.ErrorInvalidParams, "Invalid elicitation field", nil)
		}
		field, valid := rawField.(map[string]any)
		if !valid {
			return protocol.NewError(protocol.ErrorInvalidParams, "Invalid elicitation field", nil)
		}
		fieldType, _ := field["type"].(string)
		if fieldType != "string" && fieldType != "number" && fieldType != "integer" && fieldType != "boolean" {
			return protocol.NewError(protocol.ErrorInvalidParams, "Unsupported elicitation field type", nil)
		}
		if values, exists := field["enum"].([]any); exists && len(values) > 100 {
			return protocol.NewError(protocol.ErrorInvalidParams, "Elicitation enum is too large", nil)
		}
		if maximum, exists := field["maxLength"].(float64); exists && maximum > 10000 {
			return protocol.NewError(protocol.ErrorInvalidParams, "Elicitation field is too large", nil)
		}
	}
	return nil
}

func (s *Service) tasksEnabled(ctx context.Context, serverID string) bool {
	enabled, _, err := s.repository.ServerCapabilityEnabled(ctx, serverID, "tasks")
	return err == nil && enabled
}

func (s *Service) acquire(serverID string, maximum int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[serverID] >= maximum {
		return false
	}
	s.active[serverID]++
	return true
}

func (s *Service) release(serverID string) {
	s.mu.Lock()
	if s.active[serverID] <= 1 {
		delete(s.active, serverID)
	} else {
		s.active[serverID]--
	}
	s.mu.Unlock()
}

func taskPayload(task mcp.Task) map[string]any {
	var result any
	_ = json.Unmarshal([]byte(task.ResultJSON), &result)
	return map[string]any{"taskId": task.RemoteTaskID, "status": task.Status, "statusMessage": task.StatusMessage, "result": result, "expiresAt": task.ExpiresAt}
}
func validStatus(status string) bool {
	switch status {
	case "working", "input_required", "completed", "failed", "cancelled":
		return true
	}
	return false
}
func terminal(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}
