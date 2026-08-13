package hook

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNopHostRevalidator(t *testing.T) {
	v := NopHostRevalidator{}
	err := v.Revalidate(context.Background(), HookPointDefinition{}, PhaseBefore, []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Errorf("nop revalidator should never error, got: %v", err)
	}
}

func TestSchemaRevalidator_EmptySchema(t *testing.T) {
	v := NewSchemaRevalidator()
	point := HookPointDefinition{
		HookPointID: "test/1",
	}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{}`), []byte(`{"a":1}`))
	if err != nil {
		t.Errorf("expected no error for empty schema, got: %v", err)
	}
}

func TestSchemaRevalidator_WithValidator(t *testing.T) {
	v := NewSchemaRevalidator()
	v.Register("test/1", &alwaysValid{})
	point := HookPointDefinition{HookPointID: "test/1"}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Errorf("expected no error with always-valid, got: %v", err)
	}
}

func TestSchemaRevalidator_FailingValidator(t *testing.T) {
	v := NewSchemaRevalidator()
	v.Register("test/2", &alwaysInvalid{errMsg: "schema mismatch"})
	point := HookPointDefinition{HookPointID: "test/2"}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{}`), []byte(`{}`))
	if err == nil {
		t.Error("expected error from failing validator")
	} else if err.Error() != "schema mismatch" {
		t.Errorf("expected 'schema mismatch', got: %v", err)
	}
}

func TestImmutableFieldRevalidator_NoPaths(t *testing.T) {
	v := NewImmutableFieldRevalidator()
	point := HookPointDefinition{HookPointID: "unknown.point/1"}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{"a":1}`), []byte(`{"a":2}`))
	if err != nil {
		t.Errorf("expected no error for unknown point, got: %v", err)
	}
}

func TestImmutableFieldRevalidator_ImmutablePreserved(t *testing.T) {
	v := NewImmutableFieldRevalidator()
	point := HookPointDefinition{HookPointID: "tool.before_execute/1"}
	original := []byte(`{"toolId":"t1","args":{"x":1}}`)
	mutated := []byte(`{"toolId":"t1","args":{"x":2}}`)
	err := v.Revalidate(context.Background(), point, PhaseBefore, original, mutated)
	if err != nil {
		t.Errorf("expected no error when immutable preserved, got: %v", err)
	}
}

func TestImmutableFieldRevalidator_ImmutableChanged(t *testing.T) {
	v := NewImmutableFieldRevalidator()
	point := HookPointDefinition{HookPointID: "tool.before_execute/1"}
	original := []byte(`{"toolId":"t1","args":{"x":1}}`)
	mutated := []byte(`{"toolId":"t2","args":{"x":2}}`)
	err := v.Revalidate(context.Background(), point, PhaseBefore, original, mutated)
	if err == nil {
		t.Error("expected error when immutable field changed")
	}
}

func TestImmutableFieldRevalidator_CustomPaths(t *testing.T) {
	v := NewImmutableFieldRevalidator()
	v.Register("custom/1", []string{"/secret"})
	point := HookPointDefinition{HookPointID: "custom/1"}
	original := []byte(`{"secret":"abc","data":1}`)
	mutated := []byte(`{"secret":"abc","data":2}`)
	err := v.Revalidate(context.Background(), point, PhaseBefore, original, mutated)
	if err != nil {
		t.Errorf("expected no error when custom immutable preserved, got: %v", err)
	}
}

func TestImmutableFieldRevalidator_CustomPathsViolated(t *testing.T) {
	v := NewImmutableFieldRevalidator()
	v.Register("custom/2", []string{"/secret"})
	point := HookPointDefinition{HookPointID: "custom/2"}
	original := []byte(`{"secret":"abc","data":1}`)
	mutated := []byte(`{"secret":"xyz","data":2}`)
	err := v.Revalidate(context.Background(), point, PhaseBefore, original, mutated)
	if err == nil {
		t.Error("expected error when custom immutable changed")
	}
}

func TestCompositeRevalidator(t *testing.T) {
	v := NewCompositeRevalidator(&nopRevalidator{}, &failingRevalidator{errMsg: "second"})
	point := HookPointDefinition{HookPointID: "test/1"}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{}`), []byte(`{}`))
	if err == nil || err.Error() != "second" {
		t.Errorf("expected 'second' error, got: %v", err)
	}
}

func TestCompositeRevalidator_AllPass(t *testing.T) {
	v := NewCompositeRevalidator(&nopRevalidator{}, &nopRevalidator{})
	point := HookPointDefinition{HookPointID: "test/1"}
	err := v.Revalidate(context.Background(), point, PhaseBefore, []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestTraverseJSONPath(t *testing.T) {
	obj := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": float64(42),
			},
		},
	}

	val, ok := traverseJSONPath(obj, "/a/b/c")
	if !ok {
		t.Fatal("expected to find /a/b/c")
	}
	if num, isNum := val.(float64); !isNum || num != 42 {
		t.Errorf("expected 42, got %v", val)
	}

	_, ok = traverseJSONPath(obj, "/a/b/x")
	if ok {
		t.Error("did not expect to find /a/b/x")
	}

	_, ok = traverseJSONPath(obj, "/x")
	if ok {
		t.Error("did not expect to find /x")
	}
}

func TestSplitJSONPath(t *testing.T) {
	cases := map[string][]string{
		"":       nil,
		"/":      nil,
		"/a":     {"a"},
		"/a/b":   {"a", "b"},
		"/a/b/c": {"a", "b", "c"},
	}
	for input, expected := range cases {
		got := splitJSONPath(input)
		if len(got) != len(expected) {
			t.Errorf("splitJSONPath(%q) = %v, want %v", input, got, expected)
			continue
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Errorf("splitJSONPath(%q)[%d] = %s, want %s", input, i, got[i], expected[i])
			}
		}
	}
}

func TestJsonEqual(t *testing.T) {
	if !jsonEqual(1, 1) {
		t.Error("1 should equal 1")
	}
	if jsonEqual(1, 2) {
		t.Error("1 should not equal 2")
	}
	if !jsonEqual("a", "a") {
		t.Error("\"a\" should equal \"a\"")
	}
	if !jsonEqual(map[string]any{"a": 1}, map[string]any{"a": 1}) {
		t.Error("maps should be equal")
	}
}

var _ HostRevalidator = (*NopHostRevalidator)(nil)
var _ HostRevalidator = (*SchemaRevalidator)(nil)
var _ HostRevalidator = (*ImmutableFieldRevalidator)(nil)
var _ HostRevalidator = (*CompositeRevalidator)(nil)

type alwaysValid struct{}

func (alwaysValid) Validate(payload json.RawMessage) error { return nil }

type alwaysInvalid struct {
	errMsg string
}

func (a *alwaysInvalid) Validate(payload json.RawMessage) error {
	return &validationError{msg: a.errMsg}
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

type nopRevalidator struct{}

func (nopRevalidator) Revalidate(_ context.Context, _ HookPointDefinition, _ HookPhase, _, _ json.RawMessage) error {
	return nil
}

type failingRevalidator struct {
	errMsg string
}

func (f *failingRevalidator) Revalidate(_ context.Context, _ HookPointDefinition, _ HookPhase, _, _ json.RawMessage) error {
	return &validationError{msg: f.errMsg}
}
