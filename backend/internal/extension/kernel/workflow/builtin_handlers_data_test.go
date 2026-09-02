package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func decodeTestJSON(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return value
}

func TestExtractHandlerPathAndWildcard(t *testing.T) {
	h := ExtractHandler{}
	out, err := h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{
		"source":{"user":{"name":"Amitia"},"items":[{"id":1},{"id":2}]},
		"paths":["user.name","items[*].id"],
		"aliases":{"user.name":"name","items[*].id":"ids"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	value := decodeTestJSON(t, out).(map[string]any)
	if value["name"] != "Amitia" {
		t.Fatalf("unexpected name: %#v", value["name"])
	}
	ids, ok := value["ids"].([]any)
	if !ok || len(ids) != 2 || ids[0].(float64) != 1 || ids[1].(float64) != 2 {
		t.Fatalf("unexpected ids: %#v", value["ids"])
	}
}

func TestLogicHandlerComparisonAndBoolean(t *testing.T) {
	h := LogicHandler{}
	out, err := h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{"op":"gte","left":9,"right":8}`))
	if err != nil {
		t.Fatal(err)
	}
	value := decodeTestJSON(t, out).(map[string]any)
	if value["result"] != true {
		t.Fatalf("expected true, got %#v", value)
	}

	out, err = h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{"op":"and","args":[true,1,"yes"]}`))
	if err != nil {
		t.Fatal(err)
	}
	value = decodeTestJSON(t, out).(map[string]any)
	if value["result"] != true {
		t.Fatalf("expected true, got %#v", value)
	}
}

func TestLogicHandlerNestedExpressions(t *testing.T) {
	h := LogicHandler{}
	out, err := h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{
		"op":"and",
		"args":[
			{"op":"gte","left":9,"right":8},
			{"op":"contains","left":"hello amitia","right":"amitia"},
			{"op":"not","value":{"op":"eq","left":1,"right":2}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	value := decodeTestJSON(t, out).(map[string]any)
	if value["result"] != true {
		t.Fatalf("expected nested logic result=true, got %#v", value)
	}
}

func TestTransformHandlerLegacyAndDataOperations(t *testing.T) {
	h := TransformHandler{}

	out, err := h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{"op":"array_filter","source":[{"score":3},{"score":9}],"field":"score","operator":"gte","expected":5}`))
	if err != nil {
		t.Fatal(err)
	}
	items := decodeTestJSON(t, out).([]any)
	if len(items) != 1 || items[0].(map[string]any)["score"].(float64) != 9 {
		t.Fatalf("unexpected filter output: %#v", items)
	}

	out, err = h.Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{"op":"merge","source":{"a":1},"with":{"b":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	merged := decodeTestJSON(t, out).(map[string]any)
	if merged["a"].(float64) != 1 || merged["b"].(float64) != 2 {
		t.Fatalf("unexpected merge output: %#v", merged)
	}

	legacyNode := WorkflowNode{Runtime: capability.RuntimeBinding{Metadata: map[string]any{"field": "value"}}}
	out, err = h.Execute(context.Background(), legacyNode, json.RawMessage(`{"value":{"ok":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	legacy := decodeTestJSON(t, out).(map[string]any)
	if legacy["ok"] != true {
		t.Fatalf("legacy field compatibility broken: %#v", legacy)
	}
}

func TestExtractHandlerRequiredMissing(t *testing.T) {
	_, err := (ExtractHandler{}).Execute(context.Background(), WorkflowNode{}, json.RawMessage(`{"source":{"a":1},"path":"missing","required":true}`))
	if err == nil {
		t.Fatal("expected missing required path error")
	}
}

func TestConditionHandlerPreservesLegacyPayloadOp(t *testing.T) {
	h := ConditionHandler{}
	in := json.RawMessage(`{"op":"delete","value":42}`)
	out, err := h.Execute(context.Background(), WorkflowNode{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(in) {
		t.Fatalf("legacy condition passthrough changed: %s", out)
	}

	node := WorkflowNode{Runtime: capability.RuntimeBinding{Metadata: map[string]any{"op": "gte"}}}
	out, err = h.Execute(context.Background(), node, json.RawMessage(`{"left":9,"right":8}`))
	if err != nil {
		t.Fatal(err)
	}
	value := decodeTestJSON(t, out).(map[string]any)
	if value["result"] != true {
		t.Fatalf("expected explicit condition result=true, got %#v", value)
	}
}
