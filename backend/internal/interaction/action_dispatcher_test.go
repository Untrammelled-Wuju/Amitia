package interaction

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeToolFacade struct {
	executeCalls int
	lastToolID   string
	lastCallID   string
	lastInput    json.RawMessage
	result       kernel.LegacyToolResult
	returnOK     bool
	toolDefs     map[string]capability.ToolDefinition
}

func (f *fakeToolFacade) ResolveModelTool(modelName string) (kernel.ResolvedToolReference, error) {
	if def, ok := f.toolDefs[modelName]; ok {
		return kernel.ResolvedToolReference{
			ID:        capability.CapabilityID(def.ID),
			ModelName: def.ModelName,
		}, nil
	}
	return kernel.ResolvedToolReference{}, nil
}

func (f *fakeToolFacade) ExecuteTool(ctx context.Context, toolID capability.CapabilityID, input json.RawMessage, scope kernel.LegacyScope, externalCallID string, idempotencyKey string) (kernel.LegacyToolResult, bool) {
	f.executeCalls++
	f.lastToolID = string(toolID)
	f.lastCallID = externalCallID
	f.lastInput = input
	return f.result, f.returnOK
}

func TestDispatchRespondReturnsSkipped(t *testing.T) {
	action := MaterializedAction{Kind: MaterializedActionRespond, ID: "a1"}
	d := NewActionDispatcher(&fakeToolFacade{})
	result := d.Dispatch(context.Background(), action, ActionMaterializationScope{}, time.Now())
	if result.State != ActionExecutionSkipped {
		t.Fatalf("expected skipped, got %s", result.State)
	}
	if result.ToolResult != nil {
		t.Fatal("skipped action should not have tool result")
	}
}

func TestDispatchWaitReturnsSkipped(t *testing.T) {
	action := MaterializedAction{Kind: MaterializedActionWait, ID: "a2"}
	d := NewActionDispatcher(&fakeToolFacade{})
	result := d.Dispatch(context.Background(), action, ActionMaterializationScope{}, time.Now())
	if result.State != ActionExecutionSkipped {
		t.Fatalf("expected skipped, got %s", result.State)
	}
}

func TestDispatchToolCallsToolFacade(t *testing.T) {
	facade := &fakeToolFacade{
		returnOK: true,
		result:   kernel.LegacyToolResult{Status: "success", VisibleText: "ok"},
	}
	action := MaterializedAction{
		Kind:          MaterializedActionTool,
		ID:            "a3",
		InteractionID: "i-1",
		Tool: &MaterializedToolAction{
			ToolID:         "builtin/browser/search",
			ExternalCallID: "call_123",
			Input:          json.RawMessage(`{"q":"hi"}`),
		},
	}
	scope := ActionMaterializationScope{InteractionID: "i-1", UserID: "u-1"}
	d := NewActionDispatcher(facade)
	result := d.Dispatch(context.Background(), action, scope, time.Now())
	if result.State != ActionExecutionCompleted {
		t.Fatalf("expected completed, got %s", result.State)
	}
	if facade.executeCalls != 1 {
		t.Fatalf("expected 1 execute call, got %d", facade.executeCalls)
	}
	if facade.lastToolID != "builtin/browser/search" {
		t.Fatalf("wrong tool id: %s", facade.lastToolID)
	}
	if facade.lastCallID != "call_123" {
		t.Fatalf("external call id not preserved: %s", facade.lastCallID)
	}
	if result.ToolResult == nil || result.ToolResult.Status != "success" {
		t.Fatal("tool result not propagated")
	}
}

func TestDispatchScopeMismatchFails(t *testing.T) {
	action := MaterializedAction{
		Kind:          MaterializedActionTool,
		ID:            "a4",
		InteractionID: "stale-int",
		Tool:          &MaterializedToolAction{ToolID: "t1"},
	}
	scope := ActionMaterializationScope{InteractionID: "fresh-int"}
	facade := &fakeToolFacade{}
	d := NewActionDispatcher(facade)
	result := d.Dispatch(context.Background(), action, scope, time.Now())
	if result.State != ActionExecutionFailedToDispatch {
		t.Fatalf("expected failed_to_dispatch, got %s", result.State)
	}
	if facade.executeCalls != 0 {
		t.Fatal("should not execute tool when scope mismatched")
	}
}

func TestDispatchReturnsFailedWhenExecuteReturnsFalse(t *testing.T) {
	facade := &fakeToolFacade{returnOK: false}
	action := MaterializedAction{
		Kind: MaterializedActionTool,
		ID:   "a5",
		Tool: &MaterializedToolAction{ToolID: "x"},
	}
	d := NewActionDispatcher(facade)
	result := d.Dispatch(context.Background(), action, ActionMaterializationScope{}, time.Now())
	if result.State != ActionExecutionFailedToDispatch {
		t.Fatalf("expected failed_to_dispatch, got %s", result.State)
	}
}

func TestDispatchToolMissingFails(t *testing.T) {
	action := MaterializedAction{Kind: MaterializedActionTool, ID: "a6"}
	d := NewActionDispatcher(&fakeToolFacade{returnOK: true})
	result := d.Dispatch(context.Background(), action, ActionMaterializationScope{}, time.Now())
	if result.State != ActionExecutionFailedToDispatch {
		t.Fatalf("expected failed_to_dispatch for nil tool, got %s", result.State)
	}
}
