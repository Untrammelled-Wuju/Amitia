package agent

import (
	"encoding/json"
	"testing"
)

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
