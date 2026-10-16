package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedToolRuntime interface {
	Tools(context.Context, business.Inference) ([]tool.Tool, error)
	Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error)
}

type OwnedTurnRuntime interface {
	BeginOwnedTurn(context.Context, business.Inference) (context.Context, func(context.Context, business.Generation, error) error, error)
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
	filtered := make([]tool.Tool, 0, len(definitions))
	bufferNativeOutput := false
	for _, definition := range definitions {
		if ownedNativeTaskTool(definition.Function.Name) {
			if inference.InternalWorker {
				continue
			}
		}
		filtered = append(filtered, definition)
	}
	definitions = filtered
	modelSink := sink
	verification := agentVerificationGate{}
	var nativeStatus, nativeCoordination string
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
	for {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return partial(err)
		}
		text, _, calls, tokens, err := s.invokeProcessLLMWithToolsStream(ctx, cfg, messages, modelDefinitions, modelSink)
		totalTokens += tokens
		if err != nil {
			return partial(err)
		}
		if len(calls) == 0 {
			if strings.TrimSpace(modelSink.text.String()) == "" && nativeCoordination == "" {
				return partial(errors.New("模型未返回内容或能力调用，不能判定任务已完成"))
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return partial(err)
			}
			if bufferNativeOutput {
				finalText := modelSink.text.String()
				if nativeCoordination != "" {
					switch {
					case verification.PendingDelegation():
						finalText = fmt.Sprintf("多 Agent 任务 %s 已进入后台执行，当前状态为 %s，尚未完成。可在工作台查看任务进度和检查点。", nativeCoordination, nativeStatus)
					case nativeStatus == "failed" || nativeStatus == "cancelled":
						finalText = fmt.Sprintf("多 Agent 任务 %s 当前状态为 %s，不能视为工作已完成。请在工作台查看结果与检查点后处理。", nativeCoordination, nativeStatus)
					default:
						finalText = fmt.Sprintf("多 Agent 任务 %s 的 Worker 已返回结果，仍需父任务审核、验证与合并确认，不能视为代码工作已完成。可在工作台查看结果与检查点。", nativeCoordination)
					}
				}
				if err := sink.Emit(ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: finalText}); err != nil {
					return partial(err)
				}
				if modelSink.reasoning.Len() > 0 && nativeCoordination == "" {
					if err := sink.Emit(ctx, ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: modelSink.reasoning.String()}); err != nil {
						return partial(err)
					}
				}
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
			if !allowed[modelName] || inference.InternalWorker && ownedNativeTaskTool(modelName) {
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
			outcome := toolResultToOutcome(result, true)
			verification.Observe(modelName, call.Function.Arguments, outcome)
			if ownedNativeTaskTool(modelName) && !outcome.HasError {
				var task struct {
					CoordinationID string `json:"coordinationId"`
					Status         string `json:"status"`
				}
				if json.Unmarshal(outcome.Output, &task) == nil && task.CoordinationID != "" {
					nativeCoordination, nativeStatus = task.CoordinationID, task.Status
					if !bufferNativeOutput {
						bufferNativeOutput = true
						stagedInference := inference
						stagedInference.Emit = nil
						modelSink = &ownedReplySink{inference: stagedInference}
					}
				}
			}
			if outcome.HasError {
				output = fmt.Sprintf("能力调用未成功：%s: %s", outcome.ErrorCode, outcome.ErrorMessage)
			}
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": output})
		}
	}
}

func ownedNativeTaskTool(name string) bool {
	return name == "harness_multi_agent_delegate" || name == "harness_multi_agent_status" || name == "harness_multi_agent_wait"
}
