package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/androiduiagent"
)

func TestAndroidUIAgentScopeInheritsApprovedExecution(t *testing.T) {
	scope := androidUIAgentScope(tool.ToolExecutionContext{
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		Channel:        "web",
		RequestID:      "request-1",
		CorrelationID:  "correlation-1",
		CausationID:    "causation-1",
		ToolCallID:     "tool-call-1",
		PermissionMode: "full_access",
	})

	if scope.PermissionMode != "full_access" {
		t.Fatalf("expected child scope permission mode full_access, got %s", scope.PermissionMode)
	}
	if scope.ConversationID != "conversation-1" || scope.ToolCallID != "tool-call-1" {
		t.Fatalf("unexpected child scope: %#v", scope)
	}
}

func TestBrowserAgentScopeInheritsApprovedExecution(t *testing.T) {
	scope := browserAgentScope(tool.ToolExecutionContext{
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		Channel:        "web",
		RequestID:      "request-1",
		CorrelationID:  "correlation-1",
		CausationID:    "causation-1",
		ToolCallID:     "tool-call-1",
		PermissionMode: "full_access",
	})

	if scope.PermissionMode != "full_access" {
		t.Fatalf("expected child scope permission mode full_access, got %s", scope.PermissionMode)
	}
	if scope.ConversationID != "conversation-1" || scope.ToolCallID != "tool-call-1" {
		t.Fatalf("unexpected child scope: %#v", scope)
	}
}

func TestAnalyzeAndroidUIObservationLowInformationEscalatesToVisual(t *testing.T) {
	raw, err := json.Marshal(androidUITreeEnvelope{
		SnapshotID: "snapshot-1",
		Generation: 1,
		CapturedAt: 0,
		Windows:    []androidUIWindow{{WindowID: "w1", PackageName: "com.example", Active: true}},
		Nodes: []androidUINode{{
			NodeID:        "n1",
			WindowID:      "w1",
			PackageName:   "com.example",
			ClassName:     "android.view.View",
			VisibleToUser: true,
			Enabled:       true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	quality, _ := analyzeAndroidUIObservation(raw)
	if quality.Level != "LOW_INFORMATION" {
		t.Fatalf("expected LOW_INFORMATION, got %s", quality.Level)
	}
	if !quality.VisualRecommended {
		t.Fatal("low-information observation must recommend visual escalation")
	}
	if quality.PackageName != "com.example" {
		t.Fatalf("unexpected active package %q", quality.PackageName)
	}
}

func TestAnalyzeAndroidUIObservationWebViewEscalatesToVisual(t *testing.T) {
	raw, err := json.Marshal(androidUITreeEnvelope{
		SnapshotID: "snapshot-2",
		Windows:    []androidUIWindow{{WindowID: "w1", PackageName: "com.example", Focused: true}},
		Nodes: []androidUINode{{
			NodeID:        "web",
			WindowID:      "w1",
			PackageName:   "com.example",
			ClassName:     "android.webkit.WebView",
			VisibleToUser: true,
			Enabled:       true,
			Clickable:     true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	quality, _ := analyzeAndroidUIObservation(raw)
	if !quality.VisualRecommended {
		t.Fatal("WebView observation must recommend visual grounding")
	}
	if quality.VisualReason == "" {
		t.Fatal("visual escalation must carry a reason")
	}
}

func TestSemanticRematchUsesStableAttributesAfterNodeStale(t *testing.T) {
	tree := androidUITreeEnvelope{
		SnapshotID: "new-snapshot",
		Nodes: []androidUINode{{
			NodeID:             "new-node",
			ResourceID:         "com.example:id/confirm",
			Text:               "确认",
			ContentDescription: "确认订单",
			Role:               "button",
			VisibleToUser:      true,
			Enabled:            true,
		}},
	}
	original := map[string]any{
		"snapshotId":  "old-snapshot",
		"nodeId":      "old-node",
		"resourceId":  "com.example:id/confirm",
		"text":        "确认",
		"description": "确认订单",
		"role":        "button",
	}

	target, confidence, ok := semanticRematchTarget(tree, original)
	if !ok {
		t.Fatalf("expected semantic rematch, confidence=%f", confidence)
	}
	if target["nodeId"] != "new-node" || target["snapshotId"] != "new-snapshot" {
		t.Fatalf("unexpected rematched target: %#v", target)
	}
	if confidence < 0.9 {
		t.Fatalf("expected high-confidence rematch, got %f", confidence)
	}
}

func TestVisualFallbackRequiresGroundableDescription(t *testing.T) {
	action := plannedAndroidUIAction{Action: "click"}
	fallback, ok := visualFallbackAction(action, map[string]any{"text": "下一步", "role": "button"})
	if !ok {
		t.Fatal("expected visual fallback")
	}
	if fallback.Action != "visual_click" || fallback.Description != "下一步" {
		t.Fatalf("unexpected visual fallback: %#v", fallback)
	}

	if _, ok := visualFallbackAction(plannedAndroidUIAction{Action: "input_text"}, map[string]any{"text": "输入框"}); ok {
		t.Fatal("input_text must not be silently converted to visual_click")
	}
}

func TestSemanticHashIgnoresSnapshotIdentity(t *testing.T) {
	base := androidUITreeEnvelope{
		SnapshotID: "a",
		Windows:    []androidUIWindow{{PackageName: "com.example"}},
		Nodes: []androidUINode{{
			NodeID:        "node-a",
			ClassName:     "android.widget.Button",
			Text:          "完成",
			VisibleToUser: true,
			Enabled:       true,
			Clickable:     true,
		}},
	}
	other := base
	other.SnapshotID = "b"
	other.Generation = 99
	other.Nodes = append([]androidUINode(nil), base.Nodes...)
	other.Nodes[0].NodeID = "node-b"

	if androidUISemanticHash(base) != androidUISemanticHash(other) {
		t.Fatal("semantic hash should describe observable UI semantics, not ephemeral snapshot/node ids")
	}
}

func TestSemanticHashTracksVisualSnapshotIdentity(t *testing.T) {
	base := androidUITreeEnvelope{
		SnapshotID: "visual-a",
		Capability: androidUICapability{Source: "visual"},
	}
	other := base
	other.SnapshotID = "visual-b"

	if androidUISemanticHash(base) == androidUISemanticHash(other) {
		t.Fatal("visual semantic hash should change when screenshot content changes")
	}
}

func TestAndroidUIObservationImageExtractsScreenshot(t *testing.T) {
	raw := json.RawMessage(`{"snapshotId":"visual-a","screenshotMimeType":"image/jpeg","screenshotBase64":"abc123","width":480}`)
	image, mimeType, textObservation := androidUIObservationImage(raw)
	if image != "abc123" || mimeType != "image/jpeg" {
		t.Fatalf("unexpected screenshot payload: image=%q mime=%q", image, mimeType)
	}
	if string(raw) == textObservation || string(raw) == "" {
		t.Fatal("text observation must be derived without the raw screenshot payload")
	}
	if strings.Contains(textObservation, "abc123") {
		t.Fatal("text observation must not contain screenshot image data")
	}
}

func TestMapAndroidUIAction_InputTextWithoutNodeUsesVirtualDisplayText(t *testing.T) {
	toolID, raw, err := mapAndroidUIAction(androiduiagent.Request{
		DisplayID: 34,
		Ref:       "vd_34_1",
	}, plannedAndroidUIAction{
		Action: "input_text",
		Text:   "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if toolID != "android.virtual_display.text" {
		t.Fatalf("unexpected tool id %q", toolID)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ref"] != "vd_34_1" || payload["text"] != "hello" || payload["replace"] != true {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSanitizeUITargetKeepsDisplayID(t *testing.T) {
	target := sanitizeUITarget(map[string]any{
		"x":         float64(120),
		"y":         float64(240),
		"displayId": float64(34),
	})
	if target["x"] != 120 || target["y"] != 240 || target["displayId"] != 34 {
		t.Fatalf("unexpected sanitized target: %#v", target)
	}
}
