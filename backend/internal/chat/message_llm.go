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
	"github.com/u-ai/backend/internal/decision"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

type agentToolCall struct {
	ID          string
	Name        string
	Arguments   string
	Scope       SkillScope
	Fingerprint string
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

	for round := 0; ; round++ {
		if steers := conversationstream.DefaultManager().ConsumeSteer(convID, turnRecorder.TurnID); len(steers) > 0 {
			for _, steer := range steers {
				messages = append(messages, map[string]interface{}{"role": "user", "content": steer})
			}
			_, _ = conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{ConversationID: convID, RequestID: requestID, ExecutionID: turnRecorder.ExecutionID, TurnID: turnRecorder.TurnID, TurnSequence: turnRecorder.TurnSequence, Type: "turn.steered", Status: assistantTurnStatusRunning, Payload: map[string]any{"inputs": steers}}, true)
		}
		messages = s.compactAgentMessages(ctx, cfg, messages, baseMessageCount)
		applog.TraceInfo(trace.WithStage("model_call_started"), applog.Fields{"round": round, "message_count": len(messages)}, "process message model call started")
		callStartedAt := time.Now()
		projector := newModelEventProjector(turnRecorder)
		providerCtx, providerCancel := context.WithCancel(ctx)
		conversationstream.DefaultManager().RegisterProviderCancel(convID, turnRecorder.TurnID, providerCancel)
		aiContent, reasoning, toolCalls, tok, llmErr := s.invokeProcessLLMWithToolsStream(providerCtx, cfg, messages, toolDefs, projector)
		providerCancel()
		conversationstream.DefaultManager().ClearProviderCancel(convID, turnRecorder.TurnID)
		totalTokens = tok
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
				if err := projector.EnsureText(ctx, "操作已完成"); err != nil {
					return "", "", false, 0, 0, err
				}
				aiContent = projector.Text()
			}
			reply = aiContent
			break
		}
		if s.hasActionDirective && s.actionDirective.Kind == decision.ActionDirectiveRespond {
			applog.TraceWarn(trace.WithStage("tool_call_blocked_by_directive"), applog.Fields{"plan_id": s.actionDirective.PlanID, "tool_call_count": len(toolCalls)}, "模型返回 tool_calls 但 ActionDirective=respond，拒绝执行并仅使用文本")
			reply = aiContent
			break
		}

		assistantToolCall := map[string]interface{}{"role": "assistant", "content": aiContent, "tool_calls": toolCalls}
		if reasoning != "" {
			assistantToolCall["reasoning_content"] = reasoning
		}
		messages = append(messages, assistantToolCall)
		calls, err := s.prepareAgentToolCalls(ctx, toolCalls, seenTools, convID, charID, channel, requestID, spaceID, sessionID, permissionMode, trace, execCtx, turnRecorder, fingerprints)
		if err != nil {
			return "", "", false, 0, 0, err
		}
		if len(calls) == 0 {
			reply = aiContent
			break
		}
		executions := s.executeAgentToolCalls(toolExecCtx, cfg, calls, trace, round)
		roundFingerprints := map[string]struct{}{}
		for index, call := range calls {
			execution := executions[index]
			outcome := execution.Outcome
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
			toolContent := toolResultContent(outcome)
			if err := turnRecorder.AddToolResult(ctx, call.ID, call.Name, toolContent, toolStatus, outcome.ErrorCode, execution.DurationMS); err != nil {
				return "", "", false, 0, 0, err
			}
			applog.TraceInfo(trace.WithStage("tool_call_completed"), applog.Fields{"round": round, "tool_name": call.Name, "tool_call_id": call.ID, "ok": outcome.Found, "status": outcome.Status, "error_code": outcome.ErrorCode, "result_size": len(toolContent), "force_voice": outcome.ForceVoice}, "process message tool call completed")
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": call.ID, "content": toolContent})
			activationPrompt, traceItem := agentSkillTraceFromOutcome(promptTrace, call.Name, call.Arguments, outcome)
			if traceItem != nil {
				appendAgentSkillPromptTrace(promptTrace, *traceItem)
			}
			if activationPrompt != "" {
				content := promptir.RenderAgentSkillContribution([]promptir.AgentSkillContribution{{Content: activationPrompt, InstructionPosition: "after_character_rules"}})
				if len(messages) > 0 && messages[0]["role"] == "system" {
					messages[0]["content"] = fmt.Sprint(messages[0]["content"]) + "\n\n" + content
				}
			}
		}
		allRepeated := len(roundFingerprints) > 0
		for fingerprint := range roundFingerprints {
			if fingerprints[fingerprint] < 4 {
				allRepeated = false
				break
			}
		}
		if allRepeated {
			reply = aiContent
			break
		}
	}
	return reply, strings.Join(reasoningParts, "\n\n"), forceVoice, totalTokens, reasoningDurationMS, nil
}

func (s *service) prepareAgentToolCalls(ctx context.Context, toolCalls []map[string]interface{}, seenTools map[string]bool, convID, charID, channel, requestID, spaceID, sessionID, permissionMode string, trace applog.TraceFields, execCtx *coreexec.ExecutionContext, turnRecorder *assistantTurnRecorder, fingerprints map[string]int) ([]agentToolCall, error) {
	result := make([]agentToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		function, _ := tc["function"].(map[string]interface{})
		name, _ := function["name"].(string)
		args, _ := function["arguments"].(string)
		toolCallID, _ := tc["id"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		if name == "create_schedule" {
			dedupKey := name + "|" + args
			if seenTools[dedupKey] {
				continue
			}
			seenTools[dedupKey] = true
			var toolArgs map[string]interface{}
			json.Unmarshal([]byte(args), &toolArgs)
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
		result = append(result, agentToolCall{ID: toolCallID, Name: name, Arguments: args, Scope: scope, Fingerprint: fingerprint})
	}
	return result, nil
}

func (s *service) executeAgentToolCalls(ctx context.Context, cfg *ModelConfig, calls []agentToolCall, trace applog.TraceFields, round int) []agentToolExecution {
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
			executions[index] = s.executeAgentToolCall(ctx, call, trace, round)
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
			executions[index] = s.executeAgentToolCall(ctx, call, trace, round)
			<-sem
		}()
	}
	waitGroup.Wait()
	return executions
}

func (s *service) executeAgentToolCall(ctx context.Context, call agentToolCall, trace applog.TraceFields, round int) agentToolExecution {
	startedAt := time.Now()
	if s.toolRuntime == nil {
		return agentToolExecution{Outcome: toolExecOutcome{VisibleText: "工具运行时不可用", Status: "FAILED", ErrorCode: extension.ErrSkillExecutionFailed, HasError: true, Found: false}, DurationMS: time.Since(startedAt).Milliseconds()}
	}
	toolResult, found := s.toolRuntime.ExecuteModelTool(ctx, call.Name, json.RawMessage(call.Arguments), call.Scope, "")
	return agentToolExecution{Outcome: toolResultToOutcome(toolResult, found), DurationMS: time.Since(startedAt).Milliseconds()}
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
	if len(messages) <= baseMessageCount+5 {
		return messages
	}
	contextWindow := 128000
	if cfg != nil && cfg.ContextWindow > 0 {
		contextWindow = cfg.ContextWindow
	}
	if estimateModelMessagesTokens(messages) <= int(float64(contextWindow)*0.82) {
		return messages
	}
	tailCount := 6
	if len(messages)-baseMessageCount <= tailCount {
		return messages
	}
	middleEnd := len(messages) - tailCount
	if middleEnd <= baseMessageCount {
		return messages
	}
	var text strings.Builder
	for _, message := range messages[baseMessageCount:middleEnd] {
		content, _ := message["content"].(string)
		if strings.TrimSpace(content) == "" {
			continue
		}
		text.WriteString(fmt.Sprint(message["role"]))
		text.WriteString(": ")
		text.WriteString(content)
		text.WriteByte('\n')
	}
	summary := ""
	if s.compressor != nil && text.Len() > 0 {
		summary = strings.TrimSpace(s.compressor.generateSummary(ctx, text.String(), ""))
	}
	if summary == "" {
		raw := text.String()
		if len(raw) > 12000 {
			raw = raw[len(raw)-12000:]
		}
		summary = raw
	}
	if strings.TrimSpace(summary) == "" {
		return messages
	}
	compacted := make([]map[string]interface{}, 0, baseMessageCount+1+tailCount)
	compacted = append(compacted, messages[:baseMessageCount]...)
	compacted = append(compacted, map[string]interface{}{"role": "system", "content": "【本轮工具执行摘要】\n" + summary})
	compacted = append(compacted, messages[middleEnd:]...)
	return compacted
}

func estimateModelMessagesTokens(messages []map[string]interface{}) int {
	total := 0
	for _, message := range messages {
		total += 8
		if content, ok := message["content"].(string); ok {
			total += estimateTextTokens(content)
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
