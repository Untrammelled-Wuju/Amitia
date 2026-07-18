package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
)

var identifierPattern = regexp.MustCompile(`[^a-z0-9_]+`)

type Caller interface {
	Call(context.Context, string, string, any, client.CallOptions) (json.RawMessage, error)
	Connection(string) (*client.Connection, bool)
}

type Service struct {
	repository *mcp.Repository
	caller     Caller
	mu         sync.Mutex
	refreshing map[string]bool
}

type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Annotations  json.RawMessage `json:"annotations"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MIMEType    string `json:"mimeType"`
}

type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	MIMEType    string `json:"mimeType"`
}

type Prompt struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Arguments   json.RawMessage `json:"arguments"`
}

func New(repository *mcp.Repository, caller Caller) *Service {
	return &Service{repository: repository, caller: caller, refreshing: map[string]bool{}}
}

func (s *Service) Discover(ctx context.Context, serverID string) error {
	server, err := s.repository.GetServer(ctx, serverID)
	if err != nil {
		return err
	}
	var capabilities map[string]json.RawMessage
	_ = json.Unmarshal([]byte(server.CapabilitiesJSON), &capabilities)
	if _, ok := capabilities["tools"]; ok {
		if err := s.discoverTools(ctx, server); err != nil {
			return err
		}
	}
	if _, ok := capabilities["resources"]; ok {
		if err := s.discoverResources(ctx, server); err != nil {
			return err
		}
	}
	if _, ok := capabilities["prompts"]; ok {
		if err := s.discoverPrompts(ctx, server); err != nil {
			return err
		}
	}
	s.Watch(serverID)
	return nil
}

func (s *Service) Watch(serverID string) {
	connection, ok := s.caller.Connection(serverID)
	if !ok {
		return
	}
	for _, method := range []string{"notifications/tools/list_changed", "notifications/resources/list_changed", "notifications/prompts/list_changed"} {
		connection.RegisterNotificationHandler(method, func(context.Context, json.RawMessage) { s.refresh(serverID) })
	}
}

func (s *Service) refresh(serverID string) {
	s.mu.Lock()
	if s.refreshing[serverID] {
		s.mu.Unlock()
		return
	}
	s.refreshing[serverID] = true
	s.mu.Unlock()
	go func() {
		defer func() { s.mu.Lock(); s.refreshing[serverID] = false; s.mu.Unlock() }()
		_ = s.Discover(context.Background(), serverID)
	}()
}

func (s *Service) discoverTools(ctx context.Context, server mcp.Server) error {
	items := []Tool{}
	err := s.pages(ctx, server.ID, "tools/list", func(raw json.RawMessage) (string, error) {
		var page struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		items = append(items, page.Tools...)
		return page.NextCursor, nil
	})
	if err != nil {
		return err
	}
	definitions := make([]mcp.ToolDefinition, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || !validSchema(item.InputSchema) {
			return fmt.Errorf("MCP_TOOL_INPUT_INVALID: %s", item.Name)
		}
		if len(item.OutputSchema) > 0 && !validSchema(item.OutputSchema) {
			return fmt.Errorf("MCP_TOOL_OUTPUT_INVALID: %s", item.Name)
		}
		annotations := objectJSON(item.Annotations)
		input := objectJSON(item.InputSchema)
		output := objectJSON(item.OutputSchema)
		risk, hints := classifyRisk(annotations)
		execution, _ := json.Marshal(map[string]any{"provider": "mcp", "serverId": server.ID, "toolName": item.Name})
		hintsJSON, _ := json.Marshal(hints)
		definitions = append(definitions, mcp.ToolDefinition{RemoteName: item.Name, SkillID: StableSkillID(server.ID, item.Name), Title: item.Title, Description: item.Description, InputSchemaJSON: string(input), OutputSchemaJSON: string(output), AnnotationsJSON: string(annotations), ExecutionJSON: string(execution), CapabilityHintsJSON: string(hintsJSON), RiskLevel: risk, Enabled: 0, Hash: hashJSON(map[string]any{"name": item.Name, "title": item.Title, "description": item.Description, "input": json.RawMessage(input), "output": json.RawMessage(output), "annotations": json.RawMessage(annotations)})})
	}
	return s.repository.SyncTools(ctx, server.ID, definitions)
}

func (s *Service) discoverResources(ctx context.Context, server mcp.Server) error {
	resources := []Resource{}
	templates := []ResourceTemplate{}
	if err := s.pages(ctx, server.ID, "resources/list", func(raw json.RawMessage) (string, error) {
		var page struct {
			Resources  []Resource `json:"resources"`
			NextCursor string     `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		resources = append(resources, page.Resources...)
		return page.NextCursor, nil
	}); err != nil {
		return err
	}
	if err := s.pages(ctx, server.ID, "resources/templates/list", func(raw json.RawMessage) (string, error) {
		var page struct {
			ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
			NextCursor        string             `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		templates = append(templates, page.ResourceTemplates...)
		return page.NextCursor, nil
	}); err != nil {
		return err
	}
	resourceRecords := make([]mcp.ResourceDefinition, 0, len(resources))
	for _, item := range resources {
		if strings.TrimSpace(item.URI) == "" {
			return fmt.Errorf("MCP_RESOURCE_NOT_FOUND")
		}
		resourceRecords = append(resourceRecords, mcp.ResourceDefinition{URI: item.URI, Name: item.Name, Title: item.Title, Description: item.Description, MIMEType: item.MIMEType, Enabled: 1, Hash: hashJSON(item)})
	}
	templateRecords := make([]mcp.ResourceTemplate, 0, len(templates))
	for _, item := range templates {
		if strings.TrimSpace(item.URITemplate) == "" {
			return fmt.Errorf("MCP_RESOURCE_NOT_FOUND")
		}
		templateRecords = append(templateRecords, mcp.ResourceTemplate{URITemplate: item.URITemplate, Name: item.Name, Title: item.Title, Description: item.Description, MIMEType: item.MIMEType, Enabled: 1, Hash: hashJSON(item)})
	}
	return s.repository.SyncResources(ctx, server.ID, resourceRecords, templateRecords)
}

func (s *Service) discoverPrompts(ctx context.Context, server mcp.Server) error {
	items := []Prompt{}
	if err := s.pages(ctx, server.ID, "prompts/list", func(raw json.RawMessage) (string, error) {
		var page struct {
			Prompts    []Prompt `json:"prompts"`
			NextCursor string   `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		items = append(items, page.Prompts...)
		return page.NextCursor, nil
	}); err != nil {
		return err
	}
	records := make([]mcp.PromptDefinition, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("MCP_PROMPT_NOT_FOUND")
		}
		arguments := item.Arguments
		if len(arguments) == 0 {
			arguments = json.RawMessage("[]")
		}
		records = append(records, mcp.PromptDefinition{RemoteName: item.Name, Title: item.Title, Description: item.Description, ArgumentsJSON: string(arguments), Enabled: 1, Hash: hashJSON(item)})
	}
	return s.repository.SyncPrompts(ctx, server.ID, records)
}

func (s *Service) pages(ctx context.Context, serverID, method string, consume func(json.RawMessage) (string, error)) error {
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 100; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := s.caller.Call(ctx, serverID, method, params, client.CallOptions{})
		if err != nil {
			return err
		}
		next, err := consume(raw)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		if seen[next] {
			return fmt.Errorf("MCP pagination cursor cycle")
		}
		seen[next] = true
		cursor = next
	}
	return fmt.Errorf("MCP pagination limit exceeded")
}

func StableSkillID(serverID, toolName string) string {
	return "mcp." + skillSegment(serverID) + "." + skillSegment(toolName)
}
func normalizeIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = identifierPattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "unnamed"
	}
	return value
}
func skillSegment(value string) string {
	normalized := strings.ReplaceAll(normalizeIdentifier(value), "_", "-")
	if len(normalized) <= 40 {
		return normalized
	}
	sum := sha256.Sum256([]byte(value))
	return strings.Trim(normalized[:31], "-") + "-" + hex.EncodeToString(sum[:4])
}
func validSchema(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value map[string]any
	return json.Unmarshal(raw, &value) == nil && (value["type"] == "object" || value["type"] == nil)
}
func objectJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return json.RawMessage("{}")
	}
	normalized, _ := json.Marshal(value)
	return normalized
}
func hashJSON(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func classifyRisk(raw json.RawMessage) (string, []string) {
	var values map[string]any
	_ = json.Unmarshal(raw, &values)
	hints := []string{}
	readOnly, _ := values["readOnlyHint"].(bool)
	destructive, _ := values["destructiveHint"].(bool)
	idempotent, _ := values["idempotentHint"].(bool)
	openWorld, _ := values["openWorldHint"].(bool)
	if readOnly {
		hints = append(hints, "read")
	}
	if destructive {
		hints = append(hints, "destructive")
	}
	if idempotent {
		hints = append(hints, "idempotent")
	}
	if openWorld {
		hints = append(hints, "network")
	}
	if destructive || openWorld {
		return "high", hints
	}
	if readOnly {
		return "low", hints
	}
	return "medium", hints
}
