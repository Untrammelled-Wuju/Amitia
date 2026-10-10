package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/chat"
)

func (m *chatMultiAgentRuntime) wait(ctx context.Context, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	var args struct {
		CoordinationID string `json:"coordination_id"`
		WaitSeconds    int    `json:"wait_seconds"`
	}
	if err := json.Unmarshal(input, &args); err != nil || args.CoordinationID == "" ||
		args.WaitSeconds < 0 || args.WaitSeconds > 30 {
		return multiAgentToolFailure("INVALID_INPUT", fmt.Errorf("valid coordination_id and wait_seconds 1 to 30 required"))
	}
	waitSeconds := args.WaitSeconds
	if waitSeconds == 0 {
		waitSeconds = 20
	}
	timer := time.NewTimer(time.Duration(waitSeconds) * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		result := m.status(ctx, json.RawMessage(input), scope)
		if result.Error != nil {
			return result
		}
		var data struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(result.Output, &data); err != nil {
			return multiAgentToolFailure("INVALID_COORDINATION_SNAPSHOT", err)
		}
		switch data.Status {
		case "succeeded", "failed", "cancelled":
			return result
		}
		select {
		case <-ctx.Done():
			return multiAgentToolFailure("WAIT_INTERRUPTED", ctx.Err())
		case <-timer.C:
			var payload map[string]any
			if json.Unmarshal(result.Output, &payload) != nil {
				return multiAgentToolFailure("INVALID_COORDINATION_SNAPSHOT", fmt.Errorf("cannot parse pending coordination"))
			}
			payload["waitTimedOut"] = true
			payload["message"] = "Delegated workers are still running; do not claim completion"
			return multiAgentToolSuccess(payload)
		case <-tick.C:
		}
	}
}
