package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/agentrun"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

const agentRunListTool = "harness_agent_run_list"
const agentRunSendMessageTool = "harness_agent_message_send"
const agentRunReadMessagesTool = "harness_agent_message_read"

func isAgentRunMessageTool(name string) bool {
	return name == agentRunListTool || name == agentRunSendMessageTool || name == agentRunReadMessagesTool
}

func (m *chatMultiAgentRuntime) agentRunTools(ctx context.Context, scope chat.SkillScope) []tool.Tool {
	if m == nil || m.agentRuns == nil || m.tracker == nil || coordination.ValidateCurrent(ctx) != nil {
		return nil
	}
	if _, err := m.currentRun(ctx, scope); err != nil {
		return nil
	}
	return []tool.Tool{
		{Type: "function", Function: tool.Function{
			Name:        agentRunListTool,
			Description: "List the parent and child agent runs in the current conversation and coordination.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{},"additionalProperties":false}`),
		}},
		{Type: "function", Function: tool.Function{
			Name:        agentRunSendMessageTool,
			Description: "Send a durable asynchronous message to the parent or a child agent run. Child agents should use this for progress, questions and final results. Parent agents should use this to add instructions while a child is still running.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{"target_run_id":{"type":"string","maxLength":180},"kind":{"type":"string","enum":["instruction","progress","question","result","error"]},"content":{"type":"string","minLength":1,"maxLength":16000}},"required":["content"],"additionalProperties":false}`),
		}},
		{Type: "function", Function: tool.Function{
			Name:        agentRunReadMessagesTool,
			Description: "Read and acknowledge new durable messages sent to the current parent or child agent run.",
			Parameters:  multiAgentToolParameters(`{"type":"object","properties":{},"additionalProperties":false}`),
		}},
	}
}

func (m *chatMultiAgentRuntime) currentRun(ctx context.Context, scope chat.SkillScope) (agentrun.Run, error) {
	if m == nil || m.agentRuns == nil || m.tracker == nil {
		return agentrun.Run{}, fmt.Errorf("agent run messaging is unavailable")
	}
	scope.SpaceID = strings.TrimSpace(scope.SpaceID)
	scope.RequestID = strings.TrimSpace(scope.RequestID)
	if scope.SpaceID == "" || scope.RequestID == "" {
		return agentrun.Run{}, fmt.Errorf("agent run scope is incomplete")
	}
	if scope.Source == "multi_agent" {
		assignmentID := strings.TrimPrefix(scope.RequestID, "ma-")
		if assignmentID == scope.RequestID || assignmentID == "" {
			return agentrun.Run{}, fmt.Errorf("child agent run identity is unavailable")
		}
		return m.agentRuns.Get(ctx, scope.SpaceID, agentrun.ChildRunID(assignmentID))
	}
	parent, found, err := m.tracker.GetByRequestID(ctx, scope.SpaceID, scope.RequestID)
	if err != nil {
		return agentrun.Run{}, err
	}
	if !found || parent == nil || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		return agentrun.Run{}, fmt.Errorf("parent agent run is unavailable")
	}
	return m.agentRuns.Get(ctx, scope.SpaceID, agentrun.RootRunID(parent.RecoveryDescriptor.MultiAgent.CoordinationID))
}

func (m *chatMultiAgentRuntime) executeAgentRunMessageTool(ctx context.Context, name string, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return multiAgentToolFailure("AGENT_RUN_NOT_AUTHORIZED", err)
	}
	current, err := m.currentRun(ctx, scope)
	if err != nil {
		return multiAgentToolFailure("AGENT_RUN_NOT_FOUND", err)
	}
	switch name {
	case agentRunListTool:
		var items []agentrun.Run
		if current.ParentRunID == "" {
			items, err = m.agentRuns.ListChildren(ctx, current.SpaceID, current.ID)
		} else {
			parent, parentErr := m.agentRuns.Get(ctx, current.SpaceID, current.ParentRunID)
			if parentErr != nil {
				return multiAgentToolFailure("AGENT_RUN_NOT_FOUND", parentErr)
			}
			items, err = m.agentRuns.ListChildren(ctx, current.SpaceID, parent.ID)
		}
		if err != nil {
			return multiAgentToolFailure("AGENT_RUN_LIST_FAILED", err)
		}
		return multiAgentToolSuccess(map[string]any{"current": current, "runs": items})
	case agentRunSendMessageTool:
		var args struct {
			TargetRunID string `json:"target_run_id"`
			Kind        string `json:"kind"`
			Content     string `json:"content"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return multiAgentToolFailure("INVALID_INPUT", err)
		}
		args.TargetRunID = strings.TrimSpace(args.TargetRunID)
		if args.TargetRunID == "" && current.ParentRunID != "" {
			args.TargetRunID = current.ParentRunID
		}
		message, err := m.agentRuns.Send(ctx, agentrun.SendInput{
			SpaceID: current.SpaceID, SenderRunID: current.ID, RecipientID: args.TargetRunID,
			Kind: args.Kind, Content: args.Content, CorrelationID: current.CoordinationID,
		})
		if err != nil {
			return multiAgentToolFailure("AGENT_MESSAGE_SEND_FAILED", err)
		}
		return multiAgentToolSuccess(message)
	case agentRunReadMessagesTool:
		messages, err := m.agentRuns.Inbox(ctx, current.SpaceID, current.ID, true)
		if err != nil {
			return multiAgentToolFailure("AGENT_MESSAGE_READ_FAILED", err)
		}
		return multiAgentToolSuccess(map[string]any{"runId": current.ID, "messages": messages})
	default:
		return multiAgentToolFailure("UNKNOWN_TOOL", fmt.Errorf("unknown agent run message tool"))
	}
}
