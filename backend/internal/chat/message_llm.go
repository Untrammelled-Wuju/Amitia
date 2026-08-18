// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/decision"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

func (s *service) invokeLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, trace applog.TraceFields, promptTrace *promptir.PromptTrace, userMsgID, convID, charID, channel, requestID, userID, sessionID string, execCtx *coreexec.ExecutionContext, toolDefs []tool.Tool, seenTools map[string]bool, toolExecCtx context.Context) (string, bool, int, error) {
	var reply string
	var totalTokens int
	forceVoice := false
	for round := 0; round < 3; round++ {
		applog.TraceInfo(trace.WithStage("model_call_started"), applog.Fields{"round": round, "message_count": len(messages)}, "process message model call started")
		aiContent, reasoning, toolCalls, tok, llmErr := s.invokeProcessLLMWithTools(ctx, cfg, messages, toolDefs)
		totalTokens = tok
		if llmErr != nil {
			s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
			applog.TraceError(trace.WithStage("model_call_failed"), applog.Fields{"round": round, "user_message_id": userMsgID}, llmErr, "process message model call failed")
			if ctx.Err() != nil {
				return "", false, 0, ctx.Err()
			}
			return "", false, 0, &TextModelCallError{RawError: llmErr.Error()}
		}
		applog.TraceInfo(trace.WithStage("model_call_completed"), applog.Fields{"round": round, "tool_call_count": len(toolCalls), "reply_size": len(aiContent), "reasoning_size": len(reasoning)}, "process message model call completed")
		if len(toolCalls) == 0 {
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
		for _, tc := range toolCalls {
			name, _ := tc["function"].(map[string]interface{})["name"].(string)
			args, _ := tc["function"].(map[string]interface{})["arguments"].(string)
			toolCallID, _ := tc["id"].(string)
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
			applog.TraceInfo(trace.WithStage("tool_call_started"), applog.Fields{"round": round, "tool_name": name, "tool_call_id": toolCallID, "args_size": len(args)}, "process message tool call started")
			result := ""
			ok := false
			status := "FAILED"
			errorCode := ""
			toolForceVoice := false
			activationPrompt := ""
			var outcome toolExecOutcome
			if s.toolRuntime != nil {
				toolScope := SkillScope{UserID: userID, CharacterID: charID, ConversationID: convID, Channel: channel, SessionID: sessionID, Trigger: string(extension.TriggerLLM), TraceID: requestID, RequestID: requestID, ToolCallID: toolCallID, CorrelationID: trace.CorrelationID, CausationID: trace.CausationID, ExecContext: execCtx}
				toolResult, found := s.toolRuntime.ExecuteModelTool(toolExecCtx, name, json.RawMessage(args), toolScope, "")
				outcome = toolResultToOutcome(toolResult, found)
			} else {
				outcome = toolExecOutcome{VisibleText: "工具运行时不可用", Status: "FAILED", ErrorCode: extension.ErrSkillExecutionFailed, HasError: true, Found: false}
			}
			ok = outcome.Found
			result = outcome.VisibleText
			status = outcome.Status
			toolForceVoice = outcome.ForceVoice
			if outcome.HasError {
				errorCode = outcome.ErrorCode
			}
			if name == "agent_skill_activate" && !outcome.HasError {
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
				if json.Unmarshal(outcome.Output, &activation) == nil && activation.Prompt != "" {
					activationPrompt = activation.Prompt
					appendAgentSkillPromptTrace(promptTrace, promptir.AgentSkillTrace{ActivationID: activation.ActivationID, ExtensionID: activation.ExtensionID, Name: activation.Name, Source: activation.Source, Scope: activation.Scope, Trigger: "automatic", CompatibilityStatus: activation.CompatibilityStatus, BodyTokens: activation.BodyTokens, ScriptsUsed: false, ToolMappings: activation.ToolMappings, InstructionPosition: activation.InstructionPosition, Status: activation.Status})
				}
			} else if name == "agent_skill_activate" && outcome.HasError {
				var input struct {
					AgentSkill string `json:"agentSkill"`
				}
				_ = json.Unmarshal([]byte(args), &input)
				appendAgentSkillPromptTrace(promptTrace, promptir.AgentSkillTrace{Name: input.AgentSkill, Trigger: "automatic", ScriptsUsed: false, Status: "failed", ErrorCode: outcome.ErrorCode})
			} else if name == "agent_skill_read_resource" && !outcome.HasError {
				var input struct {
					AgentSkill string `json:"agentSkill"`
				}
				var content struct {
					Path string `json:"path"`
				}
				_ = json.Unmarshal([]byte(args), &input)
				if json.Unmarshal(outcome.Output, &content) == nil {
					appendAgentSkillResourceTrace(promptTrace, input.AgentSkill, content.Path)
				}
			}
			if toolForceVoice {
				forceVoice = true
			}
			applog.TraceInfo(trace.WithStage("tool_call_completed"), applog.Fields{"round": round, "tool_name": name, "tool_call_id": toolCallID, "ok": ok, "status": status, "error_code": errorCode, "result_size": len(result), "force_voice": toolForceVoice}, "process message tool call completed")
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": tc["id"], "content": result})
			if activationPrompt != "" {
				content := promptir.RenderAgentSkillContribution([]promptir.AgentSkillContribution{{Content: activationPrompt, InstructionPosition: "after_character_rules"}})
				if len(messages) > 0 && messages[0]["role"] == "system" {
					messages[0]["content"] = fmt.Sprint(messages[0]["content"]) + "\n\n" + content
				}
			}
		}
	}
	return reply, forceVoice, totalTokens, nil
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
