package workflow

import (
	"encoding/json"
	"testing"
)

func TestParseWorkflowValueRef(t *testing.T) {
	tests := []struct {
		input    string
		wantSrc  RefSource
		wantNode string
		wantPath []string
		wantErr  bool
	}{
		{"input.user.name", RefSourceInput, "", []string{"user", "name"}, false},
		{"config.api.url", RefSourceConfig, "", []string{"api", "url"}, false},
		{"runtime.version", RefSourceRuntime, "", []string{"version"}, false},
		{"steps.node1.output", RefSourceNodeOutput, "node1", []string{"output"}, false},
		{"node.n1.data.value", RefSourceNodeOutput, "n1", []string{"data", "value"}, false},
		{"nodes.charge.output.transactionId", RefSourceNodeOutput, "charge", []string{"transactionId"}, false},
		{"${nodes.charge.output.transactionId}", RefSourceNodeOutput, "charge", []string{"transactionId"}, false},
		{"literal:hello world", RefSourceLiteral, "", []string{"hello world"}, false},
		{"", "", "", nil, true},
		{"unknown.foo", "", "", nil, true},
		{"steps.n1", "", "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseWorkflowValueRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseWorkflowValueRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ref.Source != tt.wantSrc {
				t.Errorf("Source = %q, want %q", ref.Source, tt.wantSrc)
			}
			if ref.NodeID != tt.wantNode {
				t.Errorf("NodeID = %q, want %q", ref.NodeID, tt.wantNode)
			}
			if len(ref.Path) != len(tt.wantPath) {
				t.Errorf("Path = %v, want %v", ref.Path, tt.wantPath)
			}
		})
	}
}

func TestCompileExpression(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		expr, err := CompileExpression(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expr != nil {
			t.Errorf("expected nil expression")
		}
	})

	t.Run("simple eq expression", func(t *testing.T) {
		raw := json.RawMessage(`{"op":"eq","left":{"ref":{"source":"input","path":["x"]}},"right":{"value":1}}`)
		expr, err := CompileExpression(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expr.Op != OpEq {
			t.Errorf("expected op eq, got %s", expr.Op)
		}
	})

	t.Run("and expression with args", func(t *testing.T) {
		raw := json.RawMessage(`{"op":"and","args":[{"op":"eq","left":{"ref":{"source":"input","path":["a"]}},"right":{"value":1}},{"op":"gt","left":{"ref":{"source":"input","path":["b"]}},"right":{"value":0}}]}`)
		expr, err := CompileExpression(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expr.Op != OpAnd {
			t.Errorf("expected op and, got %s", expr.Op)
		}
		if len(expr.Args) != 2 {
			t.Errorf("expected 2 args, got %d", len(expr.Args))
		}
	})

	t.Run("unknown op rejected", func(t *testing.T) {
		raw := json.RawMessage(`{"op":"invalid_op"}`)
		_, err := CompileExpression(raw)
		if err == nil {
			t.Fatal("expected error for unknown op")
		}
	})

	t.Run("too deep expression rejected", func(t *testing.T) {
		depth := MaxExpressionDepth + 2
		s := `{"op":"not","right":`
		for i := 0; i < depth; i++ {
			s += `{"op":"not","right":`
		}
		s += `{"op":"exists","ref":{"source":"input","path":["x"]}}`
		for i := 0; i < depth; i++ {
			s += `}`
		}
		s += `}`
		_, err := CompileExpression(json.RawMessage(s))
		if err == nil {
			t.Fatal("expected error for too deep expression")
		}
	})
}

func TestEvaluateExpression(t *testing.T) {
	t.Run("nil expression is true", func(t *testing.T) {
		result, err := EvaluateExpression(nil, ExpressionEvalConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected nil expression to evaluate to true")
		}
	})

	t.Run("eq with matching values", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:    OpEq,
			Left:  &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"x"}}},
			Right: &WorkflowExpression{Value: float64(42)},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": float64(42)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected eq(42, 42) to be true")
		}
	})

	t.Run("eq with non-matching values", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:    OpEq,
			Left:  &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"x"}}},
			Right: &WorkflowExpression{Value: float64(42)},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": float64(100)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result {
			t.Error("expected eq(100, 42) to be false")
		}
	})

	t.Run("and with all true", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op: OpAnd,
			Args: []*WorkflowExpression{
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"a"}}}, Right: &WorkflowExpression{Value: float64(1)}},
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"b"}}}, Right: &WorkflowExpression{Value: float64(2)}},
			},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"a": float64(1), "b": float64(2)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected and(true, true) to be true")
		}
	})

	t.Run("and with one false", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op: OpAnd,
			Args: []*WorkflowExpression{
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"a"}}}, Right: &WorkflowExpression{Value: float64(1)}},
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"b"}}}, Right: &WorkflowExpression{Value: float64(2)}},
			},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"a": float64(1), "b": float64(999)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result {
			t.Error("expected and(true, false) to be false")
		}
	})

	t.Run("or with one true", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op: OpOr,
			Args: []*WorkflowExpression{
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"a"}}}, Right: &WorkflowExpression{Value: float64(1)}},
				{Op: OpEq, Left: &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"b"}}}, Right: &WorkflowExpression{Value: float64(2)}},
			},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"a": float64(999), "b": float64(2)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected or(false, true) to be true")
		}
	})

	t.Run("not inverts", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op: OpNot,
			Right: &WorkflowExpression{
				Op:    OpEq,
				Left:  &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"x"}}},
				Right: &WorkflowExpression{Value: float64(1)},
			},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": float64(1)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result {
			t.Error("expected not(true) to be false")
		}
	})

	t.Run("exists when path present", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:  OpExists,
			Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"x"}},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": "anything"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected exists to be true when path present")
		}
	})

	t.Run("exists when path absent", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:  OpExists,
			Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"missing"}},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": "anything"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result {
			t.Error("expected exists to be false when path absent")
		}
	})

	t.Run("is_null when nil", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:  OpIsNull,
			Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"missing"}},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": "anything"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected is_null to be true for nil value")
		}
	})

	t.Run("gt comparison", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:    OpGt,
			Left:  &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"x"}}},
			Right: &WorkflowExpression{Value: float64(10)},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"x": float64(20)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected gt(20, 10) to be true")
		}
	})

	t.Run("in check", func(t *testing.T) {
		expr := &WorkflowExpression{
			Op:    OpIn,
			Left:  &WorkflowExpression{Ref: &WorkflowValueRef{Source: RefSourceInput, Path: []string{"item"}}},
			Right: &WorkflowExpression{Value: []any{"a", "b", "c"}},
		}
		result, err := EvaluateExpression(expr, ExpressionEvalConfig{
			Input: map[string]any{"item": "b"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Error("expected in(b, [a,b,c]) to be true")
		}
	})
}
