package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/chat/modelprotocol"
)

type ModelGatewayToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ModelGatewayTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ModelGatewayMessage struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []ModelGatewayToolCall `json:"toolCalls,omitempty"`
	ToolCallID string                 `json:"toolCallId,omitempty"`
}

type ModelGatewayRequest struct {
	Provider        string                `json:"provider"`
	Model           string                `json:"model"`
	System          string                `json:"system"`
	Messages        []ModelGatewayMessage `json:"messages"`
	Tools           []ModelGatewayTool    `json:"tools"`
	Temperature     *float64              `json:"temperature"`
	MaxOutputTokens int                   `json:"maxOutputTokens"`
	ReasoningEffort string                `json:"reasoningEffort"`
	SessionID       string                `json:"sessionId"`
	Purpose         string                `json:"purpose"`
}

type ModelGatewayUsage struct {
	Reported           bool `json:"reported"`
	InputTokens        int  `json:"inputTokens"`
	OutputTokens       int  `json:"outputTokens"`
	ReasoningTokens    int  `json:"reasoningTokens"`
	CachedInputTokens  int  `json:"cachedInputTokens"`
	CacheUsageReported bool `json:"cacheUsageReported"`
	TotalTokens        int  `json:"totalTokens"`
}

type ModelGatewayResult struct {
	Provider           string                 `json:"provider"`
	Model              string                 `json:"model"`
	Protocol           string                 `json:"protocol"`
	ContextWindow      int                    `json:"contextWindow"`
	Text               string                 `json:"text"`
	Reasoning          string                 `json:"reasoning"`
	Refusal            string                 `json:"refusal"`
	ToolCalls          []ModelGatewayToolCall `json:"toolCalls"`
	FinishReason       string                 `json:"finishReason"`
	Usage              ModelGatewayUsage      `json:"usage"`
	ProviderResponseID string                 `json:"providerResponseID"`
}

type modelGatewayCollector struct {
	text      strings.Builder
	reasoning strings.Builder
	refusal   strings.Builder
	calls     []ModelGatewayToolCall
	index     map[string]int
	usage     *ModelUsage
}

func (c *modelGatewayCollector) Emit(_ context.Context, event ModelEvent) error {
	switch event.Type {
	case ModelEventTextDelta:
		c.text.WriteString(event.TextDelta)
	case ModelEventReasoningSummaryDelta:
		c.reasoning.WriteString(event.TextDelta)
	case ModelEventRefusalDelta:
		c.refusal.WriteString(event.TextDelta)
	case ModelEventToolCallStarted:
		c.upsertCall(event.ToolCallID, event.ToolName, "")
	case ModelEventToolCallArgumentsDelta:
		c.upsertCall(event.ToolCallID, event.ToolName, event.ArgumentsDelta)
	case ModelEventUsage:
		if event.Usage != nil {
			usage := *event.Usage
			c.usage = &usage
		}
	}
	return nil
}

func (c *modelGatewayCollector) upsertCall(id string, name string, delta string) {
	if c.index == nil {
		c.index = map[string]int{}
	}
	position, ok := c.index[id]
	if !ok {
		c.calls = append(c.calls, ModelGatewayToolCall{ID: id, Name: name})
		position = len(c.calls) - 1
		c.index[id] = position
	}
	if name != "" {
		c.calls[position].Name = name
	}
	if delta != "" {
		c.calls[position].Arguments += delta
	}
}

func modelGatewayModelRequest(cfg *ModelConfig, request ModelGatewayRequest) (ModelRequest, error) {
	var instructions []string
	if system := request.System; strings.TrimSpace(system) != "" {
		instructions = append(instructions, system)
	}
	messages := make([]ModelMessage, 0, len(request.Messages))
	for position, message := range request.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		switch role {
		case "system", "developer":
			if strings.TrimSpace(message.Content) == "" {
				continue
			}
			instructions = append(instructions, message.Content)
		case "user":
			if message.Content == "" {
				continue
			}
			messages = append(messages, ModelMessage{
				Role:  "user",
				Parts: []ModelContentPart{{Type: ContentTypeText, Text: message.Content}},
			})
		case "assistant":
			var parts []ModelContentPart
			if message.Content != "" {
				parts = []ModelContentPart{{Type: ContentTypeText, Text: message.Content}}
			}
			calls := make([]ModelToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Name) == "" {
					return ModelRequest{}, fmt.Errorf("模型网关消息 %d 的 assistant 工具调用缺少 id 或 name", position)
				}
				calls = append(calls, ModelToolCall{
					ID:            strings.TrimSpace(call.ID),
					Name:          strings.TrimSpace(call.Name),
					ArgumentsJSON: call.Arguments,
				})
			}
			if len(parts) == 0 && len(calls) == 0 {
				continue
			}
			messages = append(messages, ModelMessage{Role: "assistant", Parts: parts, ToolCalls: calls})
		case "tool":
			callID := strings.TrimSpace(message.ToolCallID)
			if callID == "" {
				return ModelRequest{}, fmt.Errorf("模型网关消息 %d 的 tool 角色缺少 toolCallId", position)
			}
			messages = append(messages, ModelMessage{
				Role:       "tool",
				Parts:      []ModelContentPart{{Type: ContentTypeText, Text: message.Content}},
				ToolCallID: callID,
			})
		default:
			return ModelRequest{}, fmt.Errorf("模型网关消息 %d 的角色 %q 不受支持", position, message.Role)
		}
	}
	if len(messages) == 0 {
		return ModelRequest{}, fmt.Errorf("模型网关请求缺少可用消息")
	}
	req := ModelRequest{
		Model:           cfg.ModelName,
		Instructions:    instructions,
		Messages:        messages,
		Stream:          true,
		ReasoningEffort: cfg.ReasoningEffort,
	}
	if len(request.Tools) > 0 {
		definitions := make([]ModelToolDefinition, 0, len(request.Tools))
		for _, candidate := range request.Tools {
			name := strings.TrimSpace(candidate.Name)
			if name == "" {
				return ModelRequest{}, fmt.Errorf("模型网关工具定义缺少 name")
			}
			parameters := candidate.Parameters
			if parameters == nil {
				parameters = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			definitions = append(definitions, ModelToolDefinition{
				Name:        name,
				Description: candidate.Description,
				Parameters:  parameters,
			})
		}
		req.Tools = definitions
	}
	if cfg.MaxOutputTokens > 0 {
		req.MaxOutputTokens = cfg.MaxOutputTokens
	} else if cfg.MaxTokens > 0 {
		req.MaxOutputTokens = cfg.MaxTokens
	}
	if request.MaxOutputTokens > 0 {
		req.MaxOutputTokens = request.MaxOutputTokens
	}
	if request.Temperature != nil {
		temperature := *request.Temperature
		req.Temperature = &temperature
	} else if cfg.Temperature > 0 {
		temperature := cfg.Temperature
		req.Temperature = &temperature
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		topP := cfg.TopP
		req.TopP = &topP
	}
	return req, nil
}

func (s *service) InvokeModelGateway(ctx context.Context, request ModelGatewayRequest) (*ModelGatewayResult, error) {
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return nil, fmt.Errorf("获取模型配置失败: %w", err)
	}
	copyConfig := *cfg
	if model := strings.TrimSpace(request.Model); model != "" {
		copyConfig.ModelName = model
	}
	if effort := strings.TrimSpace(request.ReasoningEffort); effort != "" {
		copyConfig.ReasoningEffort = effort
	}
	if request.MaxOutputTokens > 0 {
		copyConfig.MaxOutputTokens = request.MaxOutputTokens
		copyConfig.MaxTokens = request.MaxOutputTokens
	}
	if request.Temperature != nil {
		copyConfig.Temperature = *request.Temperature
	}
	req, err := modelGatewayModelRequest(&copyConfig, request)
	if err != nil {
		return nil, err
	}
	ctx, finish, authorityErr := s.beginInference(ctx)
	if authorityErr != nil {
		return nil, authorityErr
	}
	defer finish()
	protocol := resolveProtocol(&copyConfig)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	finishUsage, usageErr := s.meterModel(ctx, &copyConfig)
	if usageErr != nil {
		return nil, usageErr
	}
	collector := &modelGatewayCollector{}
	result, err := adapter.Stream(ctx, cfgToProviderConfig(&copyConfig), req, collector)
	measured := result
	if collector.usage != nil && (measured == nil || measured.Usage.TotalTokens == 0) {
		measured = &ModelResult{Usage: *collector.usage}
	}
	err = errors.Join(err, finishUsage(measured, err))
	recordModelCacheUsage(result)
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(request.Provider)
	if provider == "" {
		provider = "amitia"
	}
	output := &ModelGatewayResult{
		Provider:      provider,
		Model:         copyConfig.ModelName,
		Protocol:      string(protocol),
		ContextWindow: copyConfig.ContextWindow,
		Text:          collector.text.String(),
		Reasoning:    collector.reasoning.String(),
		Refusal:      collector.refusal.String(),
		ToolCalls:    collector.calls,
		FinishReason: "",
		Usage:        ModelGatewayUsage{},
	}
	if output.ToolCalls == nil {
		output.ToolCalls = []ModelGatewayToolCall{}
	}
	if result != nil {
		output.ProviderResponseID = result.ProviderResponseID
		output.FinishReason = result.FinishReason
		if output.Text == "" {
			output.Text = result.Text
		}
		if output.Refusal == "" {
			output.Refusal = result.Refusal
		}
		if len(collector.calls) == 0 && len(result.ToolCalls) > 0 {
			calls := make([]ModelGatewayToolCall, 0, len(result.ToolCalls))
			for _, call := range result.ToolCalls {
				calls = append(calls, ModelGatewayToolCall{ID: call.ID, Name: call.Name, Arguments: call.ArgumentsJSON})
			}
			output.ToolCalls = calls
		}
		output.Usage = ModelGatewayUsage{
			Reported:           result.Usage.Reported,
			InputTokens:        result.Usage.InputTokens,
			OutputTokens:       result.Usage.OutputTokens,
			ReasoningTokens:    result.Usage.ReasoningTokens,
			CachedInputTokens:  result.Usage.CachedInputTokens,
			CacheUsageReported: result.Usage.CacheUsageReported,
			TotalTokens:        result.Usage.TotalTokens,
		}
	}
	if output.FinishReason == "" {
		if len(output.ToolCalls) > 0 {
			output.FinishReason = "tool_calls"
		} else {
			output.FinishReason = "stop"
		}
	}
	return output, nil
}
