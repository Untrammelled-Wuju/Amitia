package interaction

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuildInteractionTools(t *testing.T) {
	tools := BuildInteractionTools()

	if len(tools) != 9 {
		t.Fatalf("expected 9 tools, got %d", len(tools))
	}

	expectedIDs := map[string]bool{
		"android.interaction.status":      false,
		"android.interaction.click":       false,
		"android.interaction.long_click":  false,
		"android.interaction.input_text":  false,
		"android.interaction.clear_text":  false,
		"android.interaction.scroll":      false,
		"android.interaction.swipe":       false,
		"android.interaction.visual_locate": false,
		"android.interaction.visual_click":  false,
	}

	for _, tool := range tools {
		if _, exists := expectedIDs[tool.ID]; !exists {
			t.Fatalf("unexpected tool ID: %s", tool.ID)
		}
		expectedIDs[tool.ID] = true
	}

	for id, found := range expectedIDs {
		if !found {
			t.Fatalf("missing tool: %s", id)
		}
	}
}

func TestInteractionTools_RuntimeBinding(t *testing.T) {
	tools := BuildInteractionTools()

	for _, tool := range tools {
		if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
			t.Fatalf("tool %s: expected RuntimeTypeAndroid_Native, got %s", tool.ID, tool.Runtime.RuntimeType)
		}
		if tool.Runtime.RuntimeID == "" {
			t.Fatalf("tool %s: expected non-empty RuntimeID", tool.ID)
		}
	}
}

func TestInteractionTools_MetadataOperation(t *testing.T) {
	tools := BuildInteractionTools()

	for _, tool := range tools {
		op, ok := tool.Metadata["androidNativeOperation"].(string)
		if !ok || op == "" {
			t.Fatalf("tool %s: expected androidNativeOperation metadata", tool.ID)
		}
		protocol, ok := tool.Metadata["bridgeProtocol"].(string)
		if !ok || protocol != "android_native" {
			t.Fatalf("tool %s: expected bridgeProtocol 'android_native', got %v", tool.ID, tool.Metadata["bridgeProtocol"])
		}
	}
}

func TestStatusTool_Definition(t *testing.T) {
	tools := BuildInteractionTools()
	var statusTool *capability.ToolDefinition
	for i := range tools {
		if tools[i].ID == "android.interaction.status" {
			statusTool = &tools[i]
			break
		}
	}

	if statusTool == nil {
		t.Fatal("status tool not found")
	}
	if statusTool.ModelName != "android.interaction.status" {
		t.Fatalf("expected model name 'android.interaction.status', got %s", statusTool.ModelName)
	}
	if !statusTool.Idempotent {
		t.Fatal("status tool should be idempotent")
	}
	if statusTool.HasSideEffects {
		t.Fatal("status tool should not have side effects")
	}
	if statusTool.SideEffect != capability.SideEffectReadOnly {
		t.Fatalf("status tool should be ReadOnly, got %s", statusTool.SideEffect)
	}
	if statusTool.RiskLevel != capability.RiskLow {
		t.Fatalf("status tool should be low risk, got %s", statusTool.RiskLevel)
	}
}

func TestClickTool_Definition(t *testing.T) {
	tools := BuildInteractionTools()
	var clickTool *capability.ToolDefinition
	for i := range tools {
		if tools[i].ID == "android.interaction.click" {
			clickTool = &tools[i]
			break
		}
	}

	if clickTool == nil {
		t.Fatal("click tool not found")
	}
	if clickTool.SideEffect != capability.SideEffectWrite {
		t.Fatalf("click tool should be Write, got %s", clickTool.SideEffect)
	}
	if clickTool.RiskLevel != capability.RiskMedium {
		t.Fatalf("click tool should be medium risk, got %s", clickTool.RiskLevel)
	}
	if clickTool.TimeoutMS <= 0 {
		t.Fatalf("click tool should have positive timeout, got %d", clickTool.TimeoutMS)
	}
}

func TestVisualLocateTool_Definition(t *testing.T) {
	tools := BuildInteractionTools()
	var tool *capability.ToolDefinition
	for i := range tools {
		if tools[i].ID == "android.interaction.visual_locate" {
			tool = &tools[i]
			break
		}
	}

	if tool == nil {
		t.Fatal("visual_locate tool not found")
	}
	if tool.SideEffect != capability.SideEffectReadOnly {
		t.Fatalf("visual_locate should be ReadOnly, got %s", tool.SideEffect)
	}
	if tool.TimeoutMS != int64(DefaultVisualLocateTimeoutMS) {
		t.Fatalf("expected timeout %d, got %d", int64(DefaultVisualLocateTimeoutMS), tool.TimeoutMS)
	}
}

func TestVisualClickTool_Definition(t *testing.T) {
	tools := BuildInteractionTools()
	var tool *capability.ToolDefinition
	for i := range tools {
		if tools[i].ID == "android.interaction.visual_click" {
			tool = &tools[i]
			break
		}
	}

	if tool == nil {
		t.Fatal("visual_click tool not found")
	}
	if tool.SideEffect != capability.SideEffectWrite {
		t.Fatalf("visual_click should be Write, got %s", tool.SideEffect)
	}
	if tool.TimeoutMS != int64(DefaultVisualClickTimeoutMS) {
		t.Fatalf("expected timeout %d, got %d", int64(DefaultVisualClickTimeoutMS), tool.TimeoutMS)
	}
}

func TestInputTextTool_Definition(t *testing.T) {
	tools := BuildInteractionTools()
	var tool *capability.ToolDefinition
	for i := range tools {
		if tools[i].ID == "android.interaction.input_text" {
			tool = &tools[i]
			break
		}
	}

	if tool == nil {
		t.Fatal("input_text tool not found")
	}
	if tool.SideEffect != capability.SideEffectWrite {
		t.Fatalf("input_text should be Write, got %s", tool.SideEffect)
	}
}

func TestTools_AllEnabled(t *testing.T) {
	tools := BuildInteractionTools()

	for _, tool := range tools {
		if !tool.Enabled {
			t.Fatalf("tool %s should be enabled", tool.ID)
		}
	}
}

func TestTools_HandlingRequestPayloads(t *testing.T) {
	tests := []struct {
		name       string
		toolID     string
		payload    map[string]any
		expectsErr bool
	}{
		{
			name:   "click with node target",
			toolID: "android.interaction.click",
			payload: map[string]any{
				"target": map[string]any{
					"snapshotId": "snap_1",
					"nodeId":     "node_1",
				},
			},
			expectsErr: false,
		},
		{
			name:   "click with coordinate target",
			toolID: "android.interaction.click",
			payload: map[string]any{
				"target": map[string]any{
					"x": 100,
					"y": 200,
				},
			},
			expectsErr: false,
		},
		{
			name:   "input_text",
			toolID: "android.interaction.input_text",
			payload: map[string]any{
				"target": map[string]any{
					"snapshotId": "snap_1",
					"nodeId":     "input_1",
				},
				"text": "hello",
			},
			expectsErr: false,
		},
		{
			name:   "scroll",
			toolID: "android.interaction.scroll",
			payload: map[string]any{
				"target": map[string]any{
					"snapshotId": "snap_1",
					"nodeId":     "scrollable_1",
				},
				"direction": "down",
			},
			expectsErr: false,
		},
		{
			name:   "swipe",
			toolID: "android.interaction.swipe",
			payload: map[string]any{
				"startX": 100,
				"startY": 200,
				"endX":   100,
				"endY":   400,
			},
			expectsErr: false,
		},
		{
			name:   "visual_locate",
			toolID: "android.interaction.visual_locate",
			payload: map[string]any{
				"description": "login button",
			},
			expectsErr: false,
		},
		{
			name:   "visual_click",
			toolID: "android.interaction.visual_click",
			payload: map[string]any{
				"text": "Submit",
			},
			expectsErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := BuildInteractionTools()
			var found bool
			for _, tool := range tools {
				if tool.ID == tt.toolID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("tool %s not found", tt.toolID)
			}
		})
	}
}
