package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedToolRuntime interface {
	Tools(context.Context, business.Inference) ([]tool.Tool, error)
	Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error)
}

func (s *service) SetOwnedToolRuntime(runtime OwnedToolRuntime) { s.ownedToolRuntime = runtime }

func (s *service) generateOwnedWithTools(ctx context.Context, inference business.Inference, cfg *ModelConfig, messages []map[string]interface{}, sink *ownedReplySink) (business.Generation, error) {
	var definitions []tool.Tool
	var err error
	if s.ownedToolRuntime != nil {
		definitions, err = s.ownedToolRuntime.Tools(ctx, inference)
		if err != nil {
			return business.Generation{}, err
		}
	}
	modelDefinitions, toolAliases := prepareAgentModelTools(definitions)
	allowed := map[string]bool{}
	for _, definition := range definitions {
		allowed[definition.Function.Name] = true
	}
	seen := map[string]bool{}
	totalTokens := 0
	partial := func(err error) (business.Generation, error) {
		return business.Generation{Text: sink.text.String(), Reasoning: sink.reasoning.String(), Tokens: totalTokens, Partial: true}, err
	}
	maxRounds := 128
	if config.AppCfg != nil && config.AppCfg.Chat.AgentMaxRounds > 0 {
		maxRounds = config.AppCfg.Chat.AgentMaxRounds
	}
	for round := 0; round < maxRounds; round++ {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return partial(err)
		}
		text, _, calls, tokens, err := s.invokeProcessLLMWithToolsStream(ctx, cfg, messages, modelDefinitions, sink)
		totalTokens += tokens
		if err != nil {
			return partial(err)
		}
		if len(calls) == 0 {
			if strings.TrimSpace(sink.text.String()) == "" {
				return partial(errors.New("模型未返回内容或能力调用，不能判定任务已完成"))
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return partial(err)
			}
			return business.Generation{Text: sink.text.String(), Reasoning: sink.reasoning.String(), Tokens: totalTokens}, nil
		}
		if len(calls) > 16 || s.ownedToolRuntime == nil {
			return partial(errors.New("设备能力调用超出授权范围或数量上限"))
		}
		messages = append(messages, map[string]interface{}{"role": "assistant", "content": text, "tool_calls": calls})
		for _, raw := range calls {
			encoded, err := json.Marshal(raw)
			if err != nil {
				return partial(err)
			}
			var call struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			if json.Unmarshal(encoded, &call) != nil || call.ID == "" || len(call.ID) > 128 || seen[call.ID] || len(call.Function.Arguments) > 256<<10 || !json.Valid([]byte(call.Function.Arguments)) {
				return partial(errors.New("模型返回了重复或无效的能力调用，已拦截"))
			}
			modelName := call.Function.Name
			if original, exists := toolAliases[modelName]; exists {
				modelName = original
			}
			if !allowed[modelName] {
				return partial(errors.New("模型调用了未授权的设备能力，已拦截"))
			}
			seen[call.ID] = true
			result, err := s.ownedToolRuntime.Execute(ctx, inference, call.ID, modelName, json.RawMessage(call.Function.Arguments))
			if err != nil {
				return partial(err)
			}
			if strings.EqualFold(result.Status, "UNKNOWN") || result.Error != nil && strings.Contains(strings.ToLower(result.Error.Code), "unknown") {
				return partial(business.ErrUncertainExecution)
			}
			if len(result.Output) > 1<<20 || len(result.VisibleText) > 1<<20 {
				return partial(coordination.ErrPendingLimit)
			}
			output := result.VisibleText
			if len(result.Output) > 0 {
				output = string(result.Output)
			}
			if outcome := toolResultToOutcome(result, true); outcome.HasError {
				output = fmt.Sprintf("能力调用未成功：%s: %s", outcome.ErrorCode, outcome.ErrorMessage)
			}
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": output})
		}
	}
	return partial(fmt.Errorf("能力调用已达到 %d 轮上限，请核查已完成的动作后再继续", maxRounds))
}
