package features

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
)

type Caller interface {
	Call(context.Context, string, string, any, client.CallOptions) (json.RawMessage, error)
}

type Service struct {
	repository *mcp.Repository
	caller     Caller
}

type ResourceReadResult struct {
	Contents          []ResourceContent `json:"contents"`
	ExternalUntrusted bool              `json:"externalUntrusted"`
	SourceServerID    string            `json:"sourceServerId"`
}
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}
type PromptResult struct {
	Description       string          `json:"description,omitempty"`
	Messages          []PromptMessage `json:"messages"`
	ExternalUntrusted bool            `json:"externalUntrusted"`
	SourceServerID    string          `json:"sourceServerId"`
}
type PromptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}
type CompletionResult struct {
	Values  []string `json:"values"`
	Total   int      `json:"total,omitempty"`
	HasMore bool     `json:"hasMore,omitempty"`
}

func New(repository *mcp.Repository, caller Caller) *Service {
	return &Service{repository: repository, caller: caller}
}

func (s *Service) ReadResource(ctx context.Context, serverID, characterID, uri string) (ResourceReadResult, error) {
	if strings.TrimSpace(uri) == "" {
		return ResourceReadResult{}, fmt.Errorf("MCP_RESOURCE_NOT_FOUND")
	}
	if err := s.authorize(ctx, serverID, characterID); err != nil {
		return ResourceReadResult{}, err
	}
	raw, err := s.caller.Call(ctx, serverID, "resources/read", map[string]any{"uri": uri}, client.CallOptions{})
	if err != nil {
		return ResourceReadResult{}, err
	}
	if len(raw) > 4<<20 {
		return ResourceReadResult{}, fmt.Errorf("MCP_RESOURCE_TOO_LARGE")
	}
	var result ResourceReadResult
	if json.Unmarshal(raw, &result) != nil || len(result.Contents) > 32 {
		return ResourceReadResult{}, fmt.Errorf("MCP_RESOURCE_CONTENT_INVALID")
	}
	for _, content := range result.Contents {
		if content.URI == "" || len(content.Text) > 512<<10 || len(content.Blob) > 2<<20 {
			return ResourceReadResult{}, fmt.Errorf("MCP_RESOURCE_TOO_LARGE")
		}
	}
	result.ExternalUntrusted = true
	result.SourceServerID = serverID
	return result, nil
}

func (s *Service) GetPrompt(ctx context.Context, serverID, characterID, name string, arguments map[string]string) (PromptResult, error) {
	if strings.TrimSpace(name) == "" {
		return PromptResult{}, fmt.Errorf("MCP_PROMPT_NOT_FOUND")
	}
	if err := s.authorize(ctx, serverID, characterID); err != nil {
		return PromptResult{}, err
	}
	definition, err := s.repository.GetPromptByName(ctx, serverID, name)
	if err != nil {
		return PromptResult{}, fmt.Errorf("MCP_PROMPT_NOT_FOUND")
	}
	var declared []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	}
	if json.Unmarshal([]byte(definition.ArgumentsJSON), &declared) != nil {
		return PromptResult{}, fmt.Errorf("MCP_PROMPT_RESULT_INVALID")
	}
	allowed := map[string]bool{}
	for _, item := range declared {
		allowed[item.Name] = true
		if item.Required && strings.TrimSpace(arguments[item.Name]) == "" {
			return PromptResult{}, fmt.Errorf("MCP_PROMPT_ARGUMENT_REQUIRED: %s", item.Name)
		}
	}
	for name := range arguments {
		if !allowed[name] {
			return PromptResult{}, fmt.Errorf("MCP_PROMPT_ARGUMENT_INVALID: %s", name)
		}
	}
	raw, err := s.caller.Call(ctx, serverID, "prompts/get", map[string]any{"name": name, "arguments": arguments}, client.CallOptions{})
	if err != nil {
		return PromptResult{}, err
	}
	if len(raw) > 2<<20 {
		return PromptResult{}, fmt.Errorf("MCP_PROMPT_RESULT_INVALID")
	}
	var result PromptResult
	if json.Unmarshal(raw, &result) != nil || len(result.Messages) > 64 {
		return PromptResult{}, fmt.Errorf("MCP_PROMPT_RESULT_INVALID")
	}
	for _, message := range result.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return PromptResult{}, fmt.Errorf("MCP_PROMPT_RESULT_INVALID")
		}
		if len(message.Content) > 512<<10 {
			return PromptResult{}, fmt.Errorf("MCP_PROMPT_RESULT_INVALID")
		}
	}
	result.ExternalUntrusted = true
	result.SourceServerID = serverID
	return result, nil
}

func (s *Service) Complete(ctx context.Context, serverID, characterID string, reference map[string]any, argument map[string]string, contextArguments map[string]string) (CompletionResult, error) {
	if err := s.authorize(ctx, serverID, characterID); err != nil {
		return CompletionResult{}, err
	}
	if len(argument["name"]) > 200 || len(argument["value"]) > 2000 || sensitiveCompletion(argument["value"]) {
		return CompletionResult{}, fmt.Errorf("MCP_COMPLETION_INVALID")
	}
	limited, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := s.caller.Call(limited, serverID, "completion/complete", map[string]any{"ref": reference, "argument": argument, "context": map[string]any{"arguments": contextArguments}}, client.CallOptions{Timeout: 5 * time.Second})
	if err != nil {
		return CompletionResult{}, err
	}
	if len(raw) > 256<<10 {
		return CompletionResult{}, fmt.Errorf("MCP_COMPLETION_INVALID")
	}
	var payload struct {
		Completion CompletionResult `json:"completion"`
	}
	if json.Unmarshal(raw, &payload) != nil || len(payload.Completion.Values) > 100 {
		return CompletionResult{}, fmt.Errorf("MCP_COMPLETION_INVALID")
	}
	values := make([]string, 0, len(payload.Completion.Values))
	for _, value := range payload.Completion.Values {
		if len(value) <= 2000 && !sensitiveCompletion(value) {
			values = append(values, value)
		}
	}
	payload.Completion.Values = values
	return payload.Completion, nil
}

func sensitiveCompletion(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "bearer ") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "access_token") || strings.Contains(lower, "refresh_token") || strings.Contains(lower, "private_key") || strings.Contains(lower, "password=")
}

func (s *Service) Subscribe(ctx context.Context, serverID, characterID, uri string) error {
	if err := s.authorize(ctx, serverID, characterID); err != nil {
		return err
	}
	_, err := s.caller.Call(ctx, serverID, "resources/subscribe", map[string]any{"uri": uri}, client.CallOptions{})
	return err
}
func (s *Service) Unsubscribe(ctx context.Context, serverID, characterID, uri string) error {
	if err := s.authorize(ctx, serverID, characterID); err != nil {
		return err
	}
	_, err := s.caller.Call(ctx, serverID, "resources/unsubscribe", map[string]any{"uri": uri}, client.CallOptions{})
	return err
}
func (s *Service) Ping(ctx context.Context, serverID string) error {
	_, err := s.caller.Call(ctx, serverID, "ping", map[string]any{}, client.CallOptions{})
	return err
}

func (s *Service) authorize(ctx context.Context, serverID, characterID string) error {
	enabled, _, err := s.repository.ResolveScopeEnabled(ctx, serverID, characterID)
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("MCP_SERVER_SCOPE_DENIED")
	}
	return nil
}
