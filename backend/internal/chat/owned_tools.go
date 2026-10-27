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
	"github.com/u-ai/backend/internal/mindruntime"
	applog "github.com/u-ai/backend/log"
)

const (
	ownedToolFailureLimit     = 3
	ownedToolProgressMaxRunes = 1200
)

type OwnedToolRuntime interface {
	Tools(context.Context, business.Inference) ([]tool.Tool, error)
	Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error)
}

type OwnedTurnRuntime interface {
	BeginOwnedTurn(context.Context, business.Inference) (context.Context, func(context.Context, business.Generation, error) error, error)
}

type OwnedToolLifecycleRuntime interface {
	PrepareOwnedPrompt(context.Context, business.Inference) (string, func(), error)
	PlanOwnedOutputs(context.Context, business.Inference, business.Generation) ([]MessageOutput, error)
	OwnedReplyCommitted(context.Context, business.Inference, business.Generation, string) error
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
	citations := ownedCitationAuditFromContext(ctx)
	if citations == nil {
		citations = &ownedCitationAudit{}
	}
	workspaceVerification := len(inference.Workspaces) > 0
	if workspaceVerification {
		bufferNativeOutput = true
		staged := inference
		staged.Emit = nil
		modelSink = &ownedReplySink{inference: staged}
	}
	var nativeStatus, nativeCoordination string
	modelDefinitions, toolAliases := prepareAgentModelTools(definitions)
	allowed := map[string]bool{}
	for _, definition := range definitions {
		allowed[definition.Function.Name] = true
	}
	seen := map[string]bool{}
	if inference.RecoveredRuntime != nil {
		items, err := ownedRecoveredToolItems(inference.RecoveredRuntime)
		if err != nil {
			return business.Generation{}, err
		}
		recovered, err := restoreAgentToolMessages(items, toolAliases)
		if err != nil {
			return business.Generation{}, err
		}
		messages = append(messages, recovered...)
		verification.Restore(items)
		for _, item := range items {
			if item.ItemType == assistantTurnItemToolCall {
				seen[item.CallID] = true
			}
		}
	}
	baseMessageCount := len(messages)
	totalTokens := 0
	forceVoice := false
	var outputs []MessageOutput
	lastFailureKey := ""
	lastFailureCount := 0
	partial := func(err error) (business.Generation, error) {
		return business.Generation{Text: sink.text.String(), Reasoning: sink.reasoning.String(), ReasoningDurationMS: sink.reasoningDurationMS(), Tokens: totalTokens, Partial: true}, err
	}
	for round := 0; ; round++ {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return partial(err)
		}
		runtime := business.RuntimeManager()
		runtimeKey := business.RuntimeKey(inference.Scope, inference.ConversationID)
		for _, steer := range runtime.ConsumeSteer(runtimeKey, inference.Scope.TurnID) {
			messages = append(messages, map[string]interface{}{"role": "user", "content": steer})
		}
		messages = s.compactAgentMessagesWithToolBudget(ctx, cfg, messages, baseMessageCount, modelDefinitions)
		if cfg != nil && cfg.ContextWindow >= 4096 && agentCompactionMessageTokens(messages, agentToolDefinitionTokens(modelDefinitions)) > agentCompactionHardInputLimit(cfg) {
			return partial(fmt.Errorf("agent context exceeds usable context %d after reserving output", agentCompactionHardInputLimit(cfg)))
		}
		if workspaceVerification {
			staged := inference
			staged.Emit = nil
			modelSink = &ownedReplySink{inference: staged}
		}
		providerCtx, providerCancel := context.WithCancel(ctx)
		runtime.RegisterProviderCancel(runtimeKey, inference.Scope.TurnID, providerCancel)
		text, _, calls, tokens, err := s.invokeProcessLLMWithToolsStream(providerCtx, cfg, messages, modelDefinitions, modelSink)
		providerCancel()
		runtime.ClearProviderCancel(runtimeKey, inference.Scope.TurnID)
		totalTokens += tokens
		if err != nil {
			if ctx.Err() == nil {
				steers := runtime.ConsumeSteer(runtimeKey, inference.Scope.TurnID)
				if len(steers) > 0 {
					if text != "" {
						messages = append(messages, map[string]interface{}{"role": "assistant", "content": text})
					}
					for _, steer := range steers {
						messages = append(messages, map[string]interface{}{"role": "user", "content": steer})
					}
					continue
				}
			}
			return partial(err)
		}
		if len(calls) == 0 {
			if workspaceVerification && verification.PendingDelegation() {
				if verification.ShouldRequestDelegationStatus() {
					messages = append(messages, map[string]interface{}{"role": "system", "content": "【子任务完成门禁】必须查询协调任务 " + verification.activeDelegation + " 的实际终态，再进行父任务验证；不得将运行中的任务称为完成。"})
					continue
				}
				return partial(fmt.Errorf("multi-agent coordination %s has not reached a terminal state; parent turn cannot claim task completion", verification.activeDelegation))
			}
			if workspaceVerification && verification.Pending() {
				if agentVerificationToolAvailable(modelDefinitions) && verification.ShouldRequestVerification() {
					instruction := "【执行验收门禁】最新工作区修改尚无成功测试、构建或静态检查证据。检查上一个工具的退出码和错误，实际修复后执行相关验证，不能只声称完成。"
					if verification.verificationAttempt > 0 {
						instruction = "【执行验收门禁·失败修复】刚才验证未通过。检查上一个工具的错误信息和退出码，实际修复后重新运行相关验证；不得只重复声称完成。"
					}
					messages = append(messages, map[string]interface{}{"role": "system", "content": instruction})
					continue
				}
				return partial(errors.New("workspace changes were applied but no successful build or test evidence was recorded after the latest edit"))
			}
			if strings.TrimSpace(modelSink.text.String()) == "" && nativeCoordination == "" {
				return partial(errors.New("模型未返回内容或能力调用，不能判定任务已完成"))
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return partial(err)
			}
			if bufferNativeOutput {
				finalText := modelSink.text.String()
				if nativeCoordination != "" && !workspaceVerification {
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
			audit := citations.AuditCitationMarkers(sink.text.String())
			if len(audit.Available)+len(audit.Unknown)+len(audit.MissingCitation) > 0 {
				applog.TraceInfo(newProcessTrace(inference.Scope.RequestID, inference.ConversationID, inference.Scope.RoleID, "owned").WithStage("citation_audit_completed"), applog.Fields{"available_citations": len(audit.Available), "unknown_citations": len(audit.Unknown), "missing_citation_claims": len(audit.MissingCitation)}, "owned reply citation audit completed")
			}
			generation := business.Generation{Text: sink.text.String(), Reasoning: sink.reasoning.String(), ReasoningDurationMS: sink.reasoningDurationMS(), Tokens: totalTokens, ForceVoice: forceVoice}
			snapshot, err := json.Marshal(citations.BuildCitationSnapshot(generation.Text))
			if err != nil {
				return partial(err)
			}
			generation.CitationSnapshot = snapshot
			return s.finishOwnedOutputs(ctx, inference, generation, outputs)
		}
		if len(calls) > 16 || s.ownedToolRuntime == nil {
			return partial(errors.New("设备能力调用超出授权范围或数量上限"))
		}
		if bufferNativeOutput && strings.TrimSpace(text) != "" {
			if err := emitOwnedProgressText(ctx, inference, text); err != nil {
				return partial(err)
			}
		}
		messages = append(messages, map[string]interface{}{"role": "assistant", "content": text, "tool_calls": calls})
		batch := map[string]bool{}
		for _, raw := range calls {
			id, _ := raw["id"].(string)
			if id == "" || len(id) > 128 || seen[id] || batch[id] {
				return partial(errors.New("duplicate tool call id or invalid identity; no batch effects dispatched"))
			}
			batch[id] = true
		}
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
			if json.Unmarshal(encoded, &call) != nil {
				return partial(errors.New("模型返回了重复或无效的能力调用，已拦截"))
			}
			modelName := call.Function.Name
			if original, exists := toolAliases[modelName]; exists {
				modelName = original
			}
			if !allowed[modelName] || inference.InternalWorker && ownedNativeTaskTool(modelName) {
				seen[call.ID] = true
				messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": "工具执行失败：TOOL_NOT_ADVERTISED: 该能力未授权或未暴露，不能执行。请使用当前可用能力。"})
				continue
			}
			seen[call.ID] = true
			arguments, invalid := validateAgentToolArguments(call.Function.Arguments)
			if invalid != "" {
				messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": "工具执行失败：INVALID_ARGUMENTS: " + invalid})
				continue
			}
			call.Function.Arguments = arguments
			if budget, limited := mindruntime.OwnedToolBudgetFromContext(ctx); limited {
				if s.usageStore == nil {
					return partial(errors.New("multi-agent durable tool budget store unavailable"))
				}
				if err := s.usageStore.ReserveOwnedToolCall(ctx, budget, call.ID, modelName); err != nil {
					return partial(err)
				}
			}
			result, err := s.ownedToolRuntime.Execute(ctx, inference, call.ID, modelName, json.RawMessage(call.Function.Arguments))
			if err != nil {
				if ctx.Err() != nil {
					return partial(err)
				}
				messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": "工具执行失败：TOOL_EXECUTION_FAILED: " + err.Error()})
				continue
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
			forceVoice = forceVoice || result.ForceVoice
			if !outcome.HasError {
				outputs = append(outputs, messageOutputsFromTurnItems([]AssistantTurnItem{{ItemType: assistantTurnItemToolResult, Status: assistantTurnStatusCompleted, ToolName: modelName, ResultJSON: string(result.Output)}})...)
			}
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
				output = toolResultContent(modelName, outcome)
				failureKey := modelName + "\x00" + call.Function.Arguments + "\x00" + outcome.ErrorCode
				if failureKey == lastFailureKey {
					lastFailureCount++
				} else {
					lastFailureKey = failureKey
					lastFailureCount = 1
				}
				if lastFailureCount >= ownedToolFailureLimit {
					return partial(fmt.Errorf("tool loop repeated tool failure: %s returned %s %d times", modelName, outcome.ErrorCode, lastFailureCount))
				}
			} else {
				lastFailureKey = ""
				lastFailureCount = 0
			}
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": output})
		}
	}
}

func emitOwnedProgressText(ctx context.Context, inference business.Inference, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) > ownedToolProgressMaxRunes {
		text = strings.TrimSpace(string(runes[:ownedToolProgressMaxRunes])) + "…"
	}
	if !strings.HasSuffix(text, "\n\n") {
		text += "\n\n"
	}
	progressSink := &ownedReplySink{inference: inference}
	return progressSink.Emit(ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: text})
}

func ownedNativeTaskTool(name string) bool {
	return name == "harness_multi_agent_delegate" || name == "harness_multi_agent_status" || name == "harness_multi_agent_wait"
}
