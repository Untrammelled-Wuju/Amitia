package notificationruntime

import (
	"testing"

	"github.com/u-ai/backend/internal/conversationstream"
)

func TestExecutionTracksPublishedToolEvents(t *testing.T) {
	for _, kind := range []string{"tool.started", "tool.progress", "tool.running", "tool.failed", "tool.completed", "tool.interrupted"} {
		if !interestingEvent(kind) {
			t.Fatalf("published event %q is not observed", kind)
		}
	}
	for _, kind := range []string{"tool.started", "tool.progress"} {
		if !executionEventNeedsSurface(kind) {
			t.Fatalf("published event %q does not promote a task notification", kind)
		}
	}

	state := ExecutionState{RunID: "real-coding-task"}
	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "tool.started", Status: "running",
		Payload: map[string]any{"toolName": "edit_file"},
	})
	if state.Summary != "edit_file" || state.CurrentStep != 0 || state.Progress != 0 {
		t.Fatalf("tool start must report its real stage without inventing progress: %#v", state)
	}

	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "tool.progress", Status: "running",
		Payload: map[string]any{"content": "正在写入文件", "fraction": 0.85},
	})
	if state.Summary != "正在写入文件" || state.CurrentStep != 0 || state.Progress != 0 {
		t.Fatalf("tool-local fraction is not overall task completion: %#v", state)
	}

	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "tool.running", Status: "completed",
		Payload: map[string]any{"toolName": "edit_file"},
	})
	if state.Summary != "工具执行完成" {
		t.Fatalf("tool completion was not reflected in notification: %#v", state)
	}
}

func TestExecutionTerminalOverridesIncompleteStepEstimate(t *testing.T) {
	state := ExecutionState{RunID: "task", TotalSteps: 6, CurrentStep: 3}
	updateExecutionState(&state, conversationstream.AgentUIEvent{Type: "turn.completed"})
	if state.Phase != "completed" || state.CurrentStep != 6 || state.Progress != 1 {
		t.Fatalf("completed turn must publish actual terminal state: %#v", state)
	}

	failed := ExecutionState{RunID: "failed", Progress: 0.4}
	updateExecutionState(&failed, conversationstream.AgentUIEvent{Type: "turn.failed"})
	if failed.Phase != "failed" || failed.Progress != 0.4 {
		t.Fatalf("failed turn must not be presented as successful: %#v", failed)
	}
}

func TestNotificationHoldsUnknownSideEffectWithoutInventingProgress(t *testing.T) {
	if !interestingEvent("turn.waiting") {
		t.Fatal("unconfirmed tool outcome not observed by notification runtime")
	}
	state := ExecutionState{RunID: "exec-1", Progress: 0.3, TotalSteps: 10, CurrentStep: 3}
	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "turn.waiting", Status: "needs_reconciliation",
		Payload: map[string]any{"errorCode": "tool_requires_reconciliation"},
	})
	if state.Phase != "needs_reconciliation" || state.Progress != 0.3 ||
		state.Summary != "工具执行状态待对账，已停止自动重试" {
		t.Fatalf("unknown side effect presented as successful or running: %+v", state)
	}
	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "tool.completed", Status: "completed",
		Payload: map[string]any{"currentStep": 10, "totalSteps": 10},
	})
	if state.Phase != "needs_reconciliation" || state.Progress != 0.3 {
		t.Fatalf("late stale tool event invented task completion: %+v", state)
	}
	updateExecutionState(&state, conversationstream.AgentUIEvent{Type: "turn.completed"})
	if state.Phase != "completed" || state.Progress != 1 {
		t.Fatalf("verified final event did not finish the notification: %+v", state)
	}
}
