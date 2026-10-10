// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/conversationstream"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/interaction"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

type agentToolCall struct {
	ID            string
	Name          string
	Arguments     string
	Scope         SkillScope
	Fingerprint   string
	Duplicate     bool
	InvalidReason string
}

type agentToolExecution struct {
	Outcome    toolExecOutcome
	DurationMS int64
}

type agentToolParallelRuntime interface {
	IsModelToolParallelSafe(ctx context.Context, modelName string, scope SkillScope) bool
}

func (s *service) invokeLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, trace applog.TraceFields, promptTrace *promptir.PromptTrace, userMsgID, convID, charID, channel, requestID, spaceID, sessionID, permissionMode string, execCtx *coreexec.ExecutionContext, toolDefs []tool.Tool, seenTools map[string]bool, toolExecCtx context.Context, turnRecorder *assistantTurnRecorder) (string, string, bool, int, int64, error) {
	if turnRecorder == nil {
		turnRecorder = newAssistantTurnRecorder(nil, convID, charID, userMsgID, requestID)
	}
	var reply string
	var reasoningParts []string
	var totalTokens int
	var reasoningDurationMS int64
	forceVoice := false
	baseMessageCount := len(messages)
	fingerprints := map[string]int{}
	modelTools, toolAliases := prepareAgentModelTools(toolDefs)
	verificationGate := agentVerificationGate{}
	workspaceVerification := workspaceAgentBound(execCtx)

	if saved := turnRecorder.snapshotItems(); len(saved) > 0 {
		recovered, recoveryErr := restoreAgentToolMessages(saved, toolAliases)
		if recoveryErr != nil {
			return "", "", false, 0, 0, fmt.Errorf("restore agent tool checkpoint: %w", recoveryErr)
		}
		messages = append(messages, recovered...)
		if workspaceVerification {
			verificationGate.Restore(saved)
		}
		if len(recovered) > 0 {
			applog.TraceInfo(trace.WithStage("agent_checkpoint_restored"), applog.Fields{
				"turn_id": turnRecorder.TurnID, "restored_messages": len(recovered),
			}, "agent tool results restored from persisted turn journal")
		}
	}

	for round := 0; ; round++ {
		if steers := conversationstream.DefaultManager().ConsumeSteer(convID, turnRecorder.TurnID); len(steers) > 0 {
			for _, steer := range steers {
				messages = append(messages, map[string]interface{}{"role": "user", "content": steer})
			}
			_, _ = conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{ConversationID: convID, RequestID: requestID, ExecutionID: turnRecorder.ExecutionID, TurnID: turnRecorder.TurnID, TurnSequence: turnRecorder.TurnSequence, Type: "turn.steered", Status: assistantTurnStatusRunning, Payload: map[string]any{"inputs": steers}}, true)
		}
		messages = s.compactAgentMessagesWithToolBudget(ctx, cfg, messages, baseMessageCount, modelTools)
		if cfg != nil && cfg.ContextWindow >= 4096 {
			used := agentCompactionMessageTokens(messages, agentToolDefinitionTokens(modelTools))
			if used > agentCompactionHardInputLimit(cfg) {
				return "", strings.Join(reasoningParts, "\n\n"), false, totalTokens, reasoningDurationMS,
					fmt.Errorf("agent context requires approximately %d input tokens; usable context is %d after reserving output; reduce tool exposure or prompt size", used, agentCompactionHardInputLimit(cfg))
			}
		}
		applog.TraceInfo(trace.WithStage("model_call_started"), applog.Fields{"round": round, "message_count": len(messages)}, "process message model call started")
		callStartedAt := time.Now()
		projector := newModelEventProjector(turnRecorder)
		providerCtx, providerCancel := context.WithCancel(ctx)
		conversationstream.DefaultManager().RegisterProviderCancel(convID, turnRecorder.TurnID, providerCancel)
		aiContent, reasoning, toolCalls, tok, llmErr := s.invokeProcessLLMWithToolsStream(providerCtx, cfg, messages, modelTools, projector)
		providerCancel()
		conversationstream.DefaultManager().ClearProviderCancel(convID, turnRecorder.TurnID)
		totalTokens += tok
		if streamedReasoning := strings.TrimSpace(projector.Reasoning()); streamedReasoning != "" {
			reasoning = streamedReasoning
		}
		if streamedText := projector.Text(); streamedText != "" {
			aiContent = streamedText
		}
		if strings.TrimSpace(reasoning) != "" {
			elapsedMS := time.Since(callStartedAt).Milliseconds()
			if elapsedMS <= 0 {
				elapsedMS = 1
			}
			reasoningDurationMS += elapsedMS
			reasoningParts = append(reasoningParts, strings.TrimSpace(reasoning))
		}
		if llmErr != nil {
			if ctx.Err() == nil && providerCtx.Err() != nil {
				steers := conversationstream.DefaultManager().ConsumeSteer(convID, turnRecorder.TurnID)
				if len(steers) > 0 {
					_ = projector.Complete(context.Background(), assistantTurnStatusInterrupted)
					for _, steer := range steers {
						messages = append(messages, map[string]interface{}{"role": "user", "content": steer})
					}
					_, _ = conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{ConversationID: convID, RequestID: requestID, ExecutionID: turnRecorder.ExecutionID, TurnID: turnRecorder.TurnID, TurnSequence: turnRecorder.TurnSequence, Type: "turn.steered", Status: assistantTurnStatusRunning, Payload: map[string]any{"inputs": steers}}, true)
					continue
				}
			}
			if ctx.Err() != nil {
				_ = projector.Complete(context.Background(), assistantTurnStatusInterrupted)
				return "", "", false, 0, 0, ctx.Err()
			}
			_ = projector.Complete(context.Background(), assistantTurnStatusFailed)
			applog.TraceError(trace.WithStage("model_call_failed"), applog.Fields{"round": round, "user_message_id": userMsgID}, llmErr, "process message model call failed")
			return "", "", false, 0, 0, &TextModelCallError{RawError: llmErr.Error()}
		}
		applog.TraceInfo(trace.WithStage("model_call_completed"), applog.Fields{"round": round, "tool_call_count": len(toolCalls), "reply_size": len(aiContent), "reasoning_size": len(reasoning)}, "process message model call completed")
		if len(toolCalls) == 0 {
			if strings.TrimSpace(aiContent) == "" {
				_ = projector.Complete(context.Background(), assistantTurnStatusFailed)
				return "", strings.Join(reasoningParts, "\n\n"), false, totalTokens, reasoningDurationMS, fmt.Errorf("model returned neither a response nor tool calls; task completion was not verified")
			}
			if workspaceVerification && verificationGate.PendingDelegation() {
				if verificationGate.ShouldRequestDelegationStatus() {
					messages = append(messages, map[string]interface{}{"role": "system", "content": "【子任务完成门禁】有已分派的子 Agent 尚未返回终态。必须调用 harness_multi_agent_wait 或 harness_multi_agent_status 查询协调任务 " + verificationGate.activeDelegation + " 的实际状态；只有协调任务结束后才能进行父任务验收。不能把运行中或等待中的 Worker 称为完成。"})
					continue
				}
				return "", strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS, fmt.Errorf("multi-agent coordination %s has not reached a terminal state; parent turn cannot claim task completion", verificationGate.activeDelegation)
			}
			if workspaceVerification && verificationGate.Pending() {
				if agentVerificationToolAvailable(modelTools) && verificationGate.ShouldRequestVerification() {
					instruction := "【执行验收门禁】本次工作区文件修改已有实际工具结果，但尚无修改后的成功测试、构建或静态检查证据。请使用已暴露的命令工具执行最小相关验证，检查退出码并修复发现的问题。不得将未验证的修改称为已完成。若无法验证，明确报告阻碍。"
					if verificationGate.verificationAttempt > 0 {
						instruction = "【执行验收门禁·失败修复】刚才执行过验证，但最新的工作区修改仍没有成功的测试、构建或静态检查证据。请检查上一个验证工具结果的错误信息和退出码，定位根因，实际修复后重新运行相关验证。不得只重复声称已完成；若当前环境或权限阻止修复，必须说明阻碍。"
					}
					messages = append(messages, map[string]interface{}{"role": "system", "content": instruction})
					continue
				}
				return "", strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS, fmt.Errorf("workspace changes were applied but no successful build or test evidence was recorded after the latest edit")
			}
			reply = aiContent
			break
		}
		assistantToolCall := map[string]interface{}{"role": "assistant", "content": aiContent, "tool_calls": toolCalls}
		if reasoning != "" {
			assistantToolCall["reasoning_content"] = reasoning
		}
		messages = append(messages, assistantToolCall)
		calls, err := s.prepareAgentToolCalls(ctx, toolCalls, toolAliases, seenTools, convID, charID, channel, requestID, spaceID, sessionID, permissionMode, trace, execCtx, turnRecorder, fingerprints)
		if err != nil {
			return "", "", false, 0, 0, err
		}
		if len(calls) == 0 {
			reply = aiContent
			break
		}
		executions := s.executeAgentToolCalls(toolExecCtx, cfg, calls, trace, round, turnRecorder)
		roundHasMutation := false
		for _, call := range calls {
			if agentToolMutatesWorkspace(call.Name, call.Arguments) {
				roundHasMutation = true
				break
			}
		}
		roundFingerprints := map[string]struct{}{}
		for index, call := range calls {
			execution := executions[index]
			outcome := execution.Outcome
			if outcome.ErrorCode == "TOOL_REQUIRES_RECONCILIATION" || outcome.ErrorCode == "TOOL_RESULT_NOT_DURABLE" {
				return "", strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS,
					fmt.Errorf("%w: tool %s: %s", interaction.ErrToolReconciliationRequired, call.ID, outcome.ErrorMessage)
			}
			roundFingerprints[call.Fingerprint] = struct{}{}
			if outcome.ForceVoice {
				forceVoice = true
			}
			if outcome.HasError || !outcome.Found {
				s.emitDesktopPetTool(ctx, call.Scope, call.ID, call.Name, "failed", outcome.ErrorCode, round)
			} else {
				s.emitDesktopPetTool(ctx, call.Scope, call.ID, call.Name, "completed", "", round)
			}
			toolStatus := assistantTurnStatusCompleted
			if outcome.HasError || !outcome.Found {
				toolStatus = assistantTurnStatusFailed
			}
			toolContent := toolResultContent(call.Name, outcome)
			if workspaceVerification {
				if !roundHasMutation || agentToolMutatesWorkspace(call.Name, call.Arguments) {
					verificationGate.Observe(call.Name, call.Arguments, outcome)
				}
			}
			if err := turnRecorder.AddToolResult(ctx, call.ID, call.Name, toolContent, toolStatus, outcome.ErrorCode, execution.DurationMS); err != nil {
				return "", "", false, 0, 0, fmt.Errorf("%w: tool %s result checkpoint failed: %v", interaction.ErrToolReconciliationRequired, call.ID, err)
			}
			applog.TraceInfo(trace.WithStage("tool_call_completed"), applog.Fields{"round": round, "tool_name": call.Name, "tool_call_id": call.ID, "ok": outcome.Found, "status": outcome.Status, "error_code": outcome.ErrorCode, "result_size": len(toolContent), "force_voice": outcome.ForceVoice}, "process message tool call completed")
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": toolContent})
			activationPrompt, traceItem := agentSkillTraceFromOutcome(promptTrace, call.Name, call.Arguments, outcome)
			if traceItem != nil {
				appendAgentSkillPromptTrace(promptTrace, *traceItem)
			}
			if activationPrompt != "" {
				if s.toolRuntime != nil {
					if refreshed, refreshErr := s.toolRuntime.ModelTools(ctx, call.Scope); refreshErr == nil {
						modelTools, toolAliases = prepareAgentModelTools(refreshed)
					} else {
						applog.TraceWarn(trace.WithStage("skill_tools_refresh_failed"), applog.Fields{"error": refreshErr.Error()}, "agent skill tool refresh failed")
					}
				}
				content := promptir.RenderAgentSkillContribution([]promptir.AgentSkillContribution{{Content: activationPrompt, InstructionPosition: "after_character_rules"}})
				if len(messages) > 0 && messages[0]["role"] == "system" {
					messages[0]["content"] = fmt.Sprint(messages[0]["content"]) + "\n\n" + content
				}
			}
		}
		repeatLimit := 0
		if config.AppCfg != nil {
			repeatLimit = config.AppCfg.Chat.AgentToolRepeatLimit
		}
		if repeatLimit > 0 {
			allRepeated := len(roundFingerprints) > 0
			for fingerprint := range roundFingerprints {
				if fingerprints[fingerprint] < repeatLimit {
					allRepeated = false
					break
				}
			}
			if allRepeated {
				return "", strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS,
					fmt.Errorf("agent tool execution stalled: repeated identical calls without verifiable progress; task is not complete")
			}
		}
	}
	citationAudit := turnRecorder.AuditCitationMarkers(reply)
	if len(citationAudit.Available) > 0 || len(citationAudit.Unknown) > 0 || len(citationAudit.MissingCitation) > 0 {
		claimStatusCounts := map[string]int{}
		for _, claim := range citationAudit.Claims {
			claimStatusCounts[claim.Status]++
		}
		fields := applog.Fields{
			"available_citations":     len(citationAudit.Available),
			"used_citations":          len(citationAudit.Used),
			"unknown_citations":       len(citationAudit.Unknown),
			"claims_checked":          len(citationAudit.Claims),
			"claim_statuses":          claimStatusCounts,
			"missing_citation_claims": len(citationAudit.MissingCitation),
		}
		if len(citationAudit.Unknown) > 0 {
			fields["unknown_citation_ids"] = citationAudit.Unknown
			applog.TraceWarn(trace.WithStage("citation_audit_invalid"), fields, "assistant reply referenced citation ids that were not produced by web research")
		} else if len(citationAudit.MissingCitation) > 0 {
			fields["missing_citation_samples"] = citationAudit.MissingCitation
			applog.TraceWarn(trace.WithStage("citation_audit_missing"), fields, "assistant reply contains factual claims without a citation after web research")
		} else if claimStatusCounts["unsupported"] > 0 || claimStatusCounts["insufficient"] > 0 {
			applog.TraceWarn(trace.WithStage("citation_claim_verification_incomplete"), fields, "assistant reply contains claims whose cited evidence did not pass structural support verification")
		} else {
			applog.TraceInfo(trace.WithStage("citation_audit_completed"), fields, "assistant reply citation audit completed")
		}
	}
	return reply, strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS, nil
}

func (s *service) prepareAgentToolCalls(ctx context.Context, toolCalls []map[string]interface{}, toolAliases map[string]string, seenTools map[string]bool, convID, charID, channel, requestID, spaceID, sessionID, permissionMode string, trace applog.TraceFields, execCtx *coreexec.ExecutionContext, turnRecorder *assistantTurnRecorder, fingerprints map[string]int) ([]agentToolCall, error) {
	seenCallIDs := make(map[string]bool, len(toolCalls))
	for _, tc := range toolCalls {
		id, _ := tc["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("model tool call is missing a required id")
		}
		if seenCallIDs[id] {
			return nil, fmt.Errorf("model emitted duplicate tool call id %q in one response", id)
		}
		seenCallIDs[id] = true
	}
	result := make([]agentToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		function, _ := tc["function"].(map[string]interface{})
		modelName, _ := function["name"].(string)
		name := modelName
		original, advertised := toolAliases[modelName]
		if advertised {
			name = original
		}
		args, _ := function["arguments"].(string)
		toolCallID, _ := tc["id"].(string)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(toolCallID) == "" {
			return nil, fmt.Errorf("model tool call is missing a required name or id")
		}
		var invalidReason string
		args, invalidReason = validateAgentToolArguments(args)
		if !advertised {
			invalidReason = "tool was not advertised to this model turn"
		}
		duplicate := false
		if name == "create_schedule" && invalidReason == "" {
			dedupKey := name + "|" + args
			duplicate = seenTools[dedupKey]
			seenTools[dedupKey] = true
			var toolArgs map[string]interface{}
			if err := json.Unmarshal([]byte(args), &toolArgs); err != nil || toolArgs == nil {
				return nil, fmt.Errorf("create_schedule requires a JSON object of arguments")
			}
			toolArgs["conversation_id"] = convID
			toolArgs["character_id"] = charID
			if channel == "web" {
				toolArgs["channel"] = "all"
			} else if channel != "" {
				toolArgs["channel"] = channel
			}
			newArgs, _ := json.Marshal(toolArgs)
			args = string(newArgs)
		}
		scope := SkillScope{SpaceID: spaceID, CharacterID: charID, ConversationID: convID, Channel: channel, SessionID: sessionID, Trigger: string(extension.TriggerLLM), TraceID: requestID, RequestID: requestID, ToolCallID: toolCallID, CorrelationID: trace.CorrelationID, CausationID: trace.CausationID, PermissionMode: permissionMode, ExecContext: execCtx}
		fingerprint := name + "|" + args
		fingerprints[fingerprint]++
		applog.TraceInfo(trace.WithStage("tool_call_started"), applog.Fields{"tool_name": name, "tool_call_id": toolCallID, "args_size": len(args)}, "process message tool call started")
		if err := turnRecorder.AddToolCall(ctx, toolCallID, name, args, "running"); err != nil {
			return nil, err
		}
		s.emitDesktopPetTool(ctx, scope, toolCallID, name, "started", "", fingerprints[fingerprint])
		result = append(result, agentToolCall{ID: toolCallID, Name: name, Arguments: args, Scope: scope, Fingerprint: fingerprint, Duplicate: duplicate, InvalidReason: invalidReason})
	}
	return result, nil
}

func (s *service) executeAgentToolCalls(ctx context.Context, cfg *ModelConfig, calls []agentToolCall, trace applog.TraceFields, round int, turnRecorder *assistantTurnRecorder) []agentToolExecution {
	executions := make([]agentToolExecution, len(calls))
	if len(calls) == 0 {
		return executions
	}
	parallel := len(calls) > 1
	if parallelRuntime, ok := s.toolRuntime.(agentToolParallelRuntime); ok {
		for _, call := range calls {
			if !parallelRuntime.IsModelToolParallelSafe(ctx, call.Name, call.Scope) {
				parallel = false
				break
			}
		}
	} else {
		parallel = false
	}
	if !parallel {
		for index, call := range calls {
			executions[index] = s.executeAgentToolCall(ctx, call, trace, round, turnRecorder)
		}
		return executions
	}

	limit := 4
	if config.AppCfg != nil && config.AppCfg.Chat.AgentMaxParallelTools > 0 {
		limit = config.AppCfg.Chat.AgentMaxParallelTools
	}
	sem := make(chan struct{}, limit)
	var waitGroup sync.WaitGroup
	for index, call := range calls {
		index := index
		call := call
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			sem <- struct{}{}
			executions[index] = s.executeAgentToolCall(ctx, call, trace, round, turnRecorder)
			<-sem
		}()
	}
	waitGroup.Wait()
	return executions
}

func (s *service) executeAgentToolCall(ctx context.Context, call agentToolCall, trace applog.TraceFields, round int, turnRecorder *assistantTurnRecorder) agentToolExecution {
	startedAt := time.Now()
	if call.InvalidReason != "" {
		return agentToolExecution{Outcome: toolExecOutcome{VisibleText: call.InvalidReason, Status: "FAILED", ErrorCode: "INVALID_TOOL_CALL", ErrorMessage: call.InvalidReason, HasError: true, Found: true}, DurationMS: 0}
	}
	if call.Duplicate {
		return agentToolExecution{Outcome: toolExecOutcome{VisibleText: "重复的定时任务创建请求已拒绝", Status: "FAILED", ErrorCode: "DUPLICATE_TOOL_CALL", HasError: true, Found: true}, DurationMS: 0}
	}
	idempotencyKey := call.Scope.ConversationID + ":" + call.Scope.RequestID + ":" + call.ID
	if s.toolRuntime == nil {
		return agentToolExecution{Outcome: toolExecOutcome{VisibleText: "工具运行时不可用", Status: "FAILED", ErrorCode: extension.ErrSkillExecutionFailed, HasError: true, Found: false}, DurationMS: time.Since(startedAt).Milliseconds()}
	}
	run := func() agentToolExecution {
		if streamingRuntime, ok := s.toolRuntime.(ModelToolProgressRuntime); ok {
			toolResult, found, err := streamingRuntime.ExecuteModelToolWithProgress(ctx, call.Name, json.RawMessage(call.Arguments), call.Scope, idempotencyKey, func(progressCtx context.Context, event ToolProgressEvent) error {
				if turnRecorder == nil {
					return nil
				}
				return turnRecorder.AddToolProgress(progressCtx, call.ID, call.Name, event)
			})
			if err != nil {
				if toolResult.Error == nil {
					toolResult.Status = "FAILED"
					toolResult.VisibleText = "工具流式执行失败"
					toolResult.Error = &ToolError{Code: "TOOL_STREAM_FAILED", Message: err.Error(), Detail: err.Error(), Retryable: false}
				}
			}
			return agentToolExecution{Outcome: toolResultToOutcome(toolResult, found), DurationMS: time.Since(startedAt).Milliseconds()}
		}
		toolResult, found := s.toolRuntime.ExecuteModelTool(ctx, call.Name, json.RawMessage(call.Arguments), call.Scope, idempotencyKey)
		return agentToolExecution{Outcome: toolResultToOutcome(toolResult, found), DurationMS: time.Since(startedAt).Milliseconds()}
	}
	return newAgentToolLedger(turnRecorder, call).execution(ctx, run)
}

func agentSkillTraceFromOutcome(promptTrace *promptir.PromptTrace, name, arguments string, outcome toolExecOutcome) (string, *promptir.AgentSkillTrace) {
	switch name {
	case "agent_skill_activate":
		if outcome.HasError {
			var input struct {
				AgentSkill string `json:"agentSkill"`
			}
			_ = json.Unmarshal([]byte(arguments), &input)
			return "", &promptir.AgentSkillTrace{Name: input.AgentSkill, Trigger: "automatic", ScriptsUsed: false, Status: "failed", ErrorCode: outcome.ErrorCode}
		}
		var activation struct {
			Prompt              string      `json:"prompt"`
			ActivationID        string      `json:"activationId"`
			ExtensionID         string      `json:"extensionId"`
			Name                string      `json:"name"`
			Source              string      `json:"source"`
			Scope               string      `json:"scope"`
			CompatibilityStatus string      `json:"compatibilityStatus"`
			BodyTokens          int         `json:"bodyTokens"`
			ToolMappings        interface{} `json:"toolMappings"`
			InstructionPosition string      `json:"instructionPosition"`
			Status              string      `json:"status"`
		}
		if json.Unmarshal(outcome.Output, &activation) != nil {
			return "", nil
		}
		trace := &promptir.AgentSkillTrace{ActivationID: activation.ActivationID, ExtensionID: activation.ExtensionID, Name: activation.Name, Source: activation.Source, Scope: activation.Scope, Trigger: "automatic", CompatibilityStatus: activation.CompatibilityStatus, BodyTokens: activation.BodyTokens, ScriptsUsed: false, ToolMappings: activation.ToolMappings, InstructionPosition: activation.InstructionPosition, Status: activation.Status}
		return activation.Prompt, trace
	case "agent_skill_read_resource":
		if outcome.HasError {
			return "", nil
		}
		var input struct {
			AgentSkill string `json:"agentSkill"`
		}
		var content struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal([]byte(arguments), &input)
		if json.Unmarshal(outcome.Output, &content) == nil && content.Path != "" {
			appendAgentSkillResourceTrace(promptTrace, input.AgentSkill, content.Path)
		}
		return "", nil
	default:
		return "", nil
	}
}

func (s *service) compactAgentMessages(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, baseMessageCount int) []map[string]interface{} {
	return s.compactAgentMessagesWithToolBudget(ctx, cfg, messages, baseMessageCount, nil)
}

func estimateModelMessagesTokens(messages []map[string]interface{}) int {
	total := 0
	for _, message := range messages {
		total += 8
		if content, ok := message["content"].(string); ok {
			total += estimateTextTokens(content)
		}
		if parts, ok := message["parts"].([]ModelContentPart); ok {
			for _, part := range parts {
				total += estimateTextTokens(part.Text) + estimateTextTokens(part.Filename) + 12
				switch part.Type {
				case ContentTypeImage:
					switch strings.ToLower(part.Detail) {
					case "low":
						total += 100
					case "high":
						total += 2048
					default:
						total += 1536
					}
				case ContentTypeAudio:
					total += 4096
				case ContentTypeVideo:
					total += 8192
				case ContentTypeFile:
					total += 2048
				}
			}
		}
		if toolCalls, ok := message["tool_calls"]; ok {
			encoded, err := json.Marshal(toolCalls)
			if err == nil {
				total += len(encoded)/4 + 1
			}
		}
	}
	return total
}

func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	ascii := 0
	for _, value := range text {
		if value < 128 {
			ascii++
		}
	}
	nonASCII := runes - ascii
	return ascii/4 + nonASCII + 1
}

func appendAgentSkillPromptTrace(trace *promptir.PromptTrace, item promptir.AgentSkillTrace) {
	if trace == nil {
		return
	}
	for _, existing := range trace.AgentSkills {
		if item.ActivationID != "" && existing.ActivationID == item.ActivationID {
			return
		}
	}
	trace.AgentSkills = append(trace.AgentSkills, item)
}

func appendAgentSkillResourceTrace(trace *promptir.PromptTrace, name, resourcePath string) {
	if trace == nil || resourcePath == "" {
		return
	}
	for index := range trace.AgentSkills {
		if trace.AgentSkills[index].Name == name || trace.AgentSkills[index].ExtensionID == name {
			trace.AgentSkills[index].ResourceReads++
			trace.AgentSkills[index].ResourcePaths = append(trace.AgentSkills[index].ResourcePaths, resourcePath)
			return
		}
	}
}
