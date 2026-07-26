// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

func (s *service) invokeLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, trace applog.TraceFields, promptTrace *promptir.PromptTrace, userMsgID, convID, charID, channel, requestID, userID, sessionID string, toolDefs []tool.Tool, seenTools map[string]bool, toolExecCtx context.Context) (string, bool, int, error) {
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
			if s.skillRuntime != nil {
				skillScope := extension.ExecutionScope{UserID: userID, CharacterID: charID, ConversationID: convID, Channel: channel, SessionID: sessionID, Trigger: extension.TriggerLLM, TraceID: requestID, RequestID: requestID, ToolCallID: toolCallID, CorrelationID: trace.CorrelationID, CausationID: trace.CausationID}
				skillResult, found := s.skillRuntime.ExecuteModelTool(toolExecCtx, name, json.RawMessage(args), skillScope, "")
				ok = found
				result = skillResult.VisibleText
				status = string(skillResult.Status)
				toolForceVoice = skillResult.ForceVoice
				if skillResult.Error != nil {
					errorCode = skillResult.Error.Code
				}
				if name == "agent_skill_activate" && skillResult.Error == nil {
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
					if json.Unmarshal(skillResult.Output, &activation) == nil && activation.Prompt != "" {
						activationPrompt = activation.Prompt
						appendAgentSkillPromptTrace(promptTrace, promptir.AgentSkillTrace{ActivationID: activation.ActivationID, ExtensionID: activation.ExtensionID, Name: activation.Name, Source: activation.Source, Scope: activation.Scope, Trigger: "automatic", CompatibilityStatus: activation.CompatibilityStatus, BodyTokens: activation.BodyTokens, ScriptsUsed: false, ToolMappings: activation.ToolMappings, InstructionPosition: activation.InstructionPosition, Status: activation.Status})
					}
				} else if name == "agent_skill_activate" && skillResult.Error != nil {
					var input struct {
						AgentSkill string `json:"agentSkill"`
					}
					_ = json.Unmarshal([]byte(args), &input)
					appendAgentSkillPromptTrace(promptTrace, promptir.AgentSkillTrace{Name: input.AgentSkill, Trigger: "automatic", ScriptsUsed: false, Status: "failed", ErrorCode: skillResult.Error.Code})
				} else if name == "agent_skill_read_resource" && skillResult.Error == nil {
					var input struct {
						AgentSkill string `json:"agentSkill"`
					}
					var content struct {
						Path string `json:"path"`
					}
					_ = json.Unmarshal([]byte(args), &input)
					if json.Unmarshal(skillResult.Output, &content) == nil {
						appendAgentSkillResourceTrace(promptTrace, input.AgentSkill, content.Path)
					}
				}
			} else {
				result = "工具运行时不可用"
				errorCode = extension.ErrSkillExecutionFailed
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
