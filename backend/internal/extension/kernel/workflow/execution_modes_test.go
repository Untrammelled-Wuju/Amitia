package workflow

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDryRunExecution(t *testing.T) {
	dag := &CompiledWorkflowDAG{
		WorkflowID:       "test-dryrun",
		SchemaVersion:    CompiledSchemaVersion,
		CompilerVersion:  CompilerVersion,
		TopologicalOrder: []string{"a", "b", "c", "d"},
		DependedOnBy: map[string][]string{
			"a": {},
			"b": {"a"},
			"c": {"a"},
			"d": {"b", "c"},
		},
		Nodes: map[string]CompiledWorkflowNode{
			"a": {ID: "a", Type: "tool", Purity: NodePurityPure},
			"b": {ID: "b", Type: "tool", HasSideEffects: true, Purity: NodePuritySideEffecting},
			"c": {ID: "c", Type: "tool", Purity: NodePurityPure},
			"d": {ID: "d", Type: "tool", HasSideEffects: true, Purity: NodePuritySideEffecting},
		},
	}

	result := ExecuteDryRun(context.Background(), dag)
	if len(result.WouldExecute) != 4 {
		t.Errorf("WouldExecute = %v, want 4 nodes", result.WouldExecute)
	}
	if len(result.WouldSkip) != 0 {
		t.Errorf("WouldSkip = %v, want empty", result.WouldSkip)
	}
	if result.SideEffects != 2 {
		t.Errorf("SideEffects = %d, want 2", result.SideEffects)
	}
	if len(result.Transitions) != 4 {
		t.Errorf("Transitions = %d, want 4", len(result.Transitions))
	}
	if len(result.NodeOrder) != 4 {
		t.Errorf("NodeOrder = %v, want 4 nodes", result.NodeOrder)
	}
}

func TestExecutionOptions(t *testing.T) {
	t.Run("is dry run", func(t *testing.T) {
		opts := ExecutionOptions{Mode: ExecutionModeDryRun}
		if !opts.IsDryRun() {
			t.Error("expected dry run")
		}
		if opts.IsLive() {
			t.Error("dry run should not be live")
		}
		if opts.AllowSideEffect("a", SideEffectToolCall, "t") {
			t.Error("dry run should not allow side effects")
		}
	})

	t.Run("is mocked", func(t *testing.T) {
		opts := ExecutionOptions{Mode: ExecutionModeMocked}
		if !opts.IsMocked() {
			t.Error("expected mocked")
		}
	})

	t.Run("is live", func(t *testing.T) {
		opts := ExecutionOptions{Mode: ExecutionModeLive}
		if !opts.IsLive() {
			t.Error("expected live")
		}
		if !opts.AllowSideEffect("a", SideEffectToolCall, "t") {
			t.Error("live mode should allow side effects")
		}
	})

	t.Run("controlled live with confirm", func(t *testing.T) {
		opts := ExecutionOptions{
			Mode: ExecutionModeControlled,
			ConfirmSideEffect: func(nodeID string, kind SideEffectKind, target string) bool {
				return nodeID == "allowed"
			},
		}
		if !opts.IsLive() {
			t.Error("controlled should be live")
		}
		if !opts.AllowSideEffect("allowed", SideEffectToolCall, "t") {
			t.Error("expected allowed node to be permitted")
		}
		if opts.AllowSideEffect("denied", SideEffectToolCall, "t") {
			t.Error("expected denied node to be rejected")
		}
	})
}

func TestMockLookup(t *testing.T) {
	mocks := []MockBehavior{
		{NodeID: "a", Output: json.RawMessage(`{"ok":true}`)},
		{NodeID: "b", Error: "mock error"},
	}

	lookup := BuildMockLookup(mocks)
	if len(lookup) != 2 {
		t.Fatalf("lookup size = %d, want 2", len(lookup))
	}

	opts := &ExecutionOptions{Mode: ExecutionModeMocked, Mocks: mocks}

	out, err, ok := opts.EffectiveMockOutput("a")
	if !ok {
		t.Fatal("expected mock for a")
	}
	if string(out) != `{"ok":true}` {
		t.Errorf("mock output = %s, want {ok:true}", out)
	}
	if err != "" {
		t.Errorf("mock error = %s, want empty", err)
	}

	out, err, ok = opts.EffectiveMockOutput("b")
	if !ok {
		t.Fatal("expected mock for b")
	}
	if err != "mock error" {
		t.Errorf("mock error = %s, want 'mock error'", err)
	}
	if out != nil {
		t.Errorf("mock output = %s, want nil", out)
	}

	_, _, ok = opts.EffectiveMockOutput("nonexistent")
	if ok {
		t.Error("should not find mock for nonexistent")
	}
}

func TestDefaultExecutionOptions(t *testing.T) {
	opts := DefaultExecutionOptions()
	if opts.Mode != ExecutionModeLive {
		t.Errorf("default mode = %s, want live", opts.Mode)
	}
	if opts.SideEffectLimit != -1 {
		t.Errorf("default side effect limit = %d, want -1", opts.SideEffectLimit)
	}
}
