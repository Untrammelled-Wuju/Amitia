package workflow

import (
	"encoding/json"
	"testing"
)

func TestCompilerSimpleLinear(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-linear",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Linear Workflow",
		InputSchema:     json.RawMessage(`{"type":"object"}`),
		OutputSchema:    json.RawMessage(`{"type":"object"}`),
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10, MaxConcurrency: 4},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"first"}`)}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"second"}`)}},
			{ID: "c", Type: "tool", DependsOn: []string{"b"}, Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"third"}`)}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if dag.WorkflowID != "test-linear" {
		t.Errorf("WorkflowID = %s, want test-linear", dag.WorkflowID)
	}
	if dag.SchemaVersion != CompiledSchemaVersion {
		t.Errorf("SchemaVersion = %s, want %s", dag.SchemaVersion, CompiledSchemaVersion)
	}
	if dag.CompilerVersion != CompilerVersion {
		t.Errorf("CompilerVersion = %s, want %s", dag.CompilerVersion, CompilerVersion)
	}

	if len(dag.TopologicalOrder) != 3 {
		t.Fatalf("TopologicalOrder = %v, want 3 nodes", dag.TopologicalOrder)
	}
	if dag.TopologicalOrder[0] != "a" || dag.TopologicalOrder[1] != "b" || dag.TopologicalOrder[2] != "c" {
		t.Errorf("TopologicalOrder = %v, want [a b c]", dag.TopologicalOrder)
	}

	if len(dag.EntryNodes) != 1 || dag.EntryNodes[0] != "a" {
		t.Errorf("EntryNodes = %v, want [a]", dag.EntryNodes)
	}
	if len(dag.ExitNodes) != 1 || dag.ExitNodes[0] != "c" {
		t.Errorf("ExitNodes = %v, want [c]", dag.ExitNodes)
	}

	if len(dag.Nodes) != 3 {
		t.Errorf("Nodes = %d, want 3", len(dag.Nodes))
	}

	if dag.CompiledChecksum == "" {
		t.Error("CompiledChecksum should not be empty")
	}
	if dag.WorkflowChecksum == "" {
		t.Error("WorkflowChecksum should not be empty")
	}
}

func TestCompilerDAG(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-dag",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "DAG Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10, MaxConcurrency: 4},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{}},
			{ID: "c", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{}},
			{ID: "d", Type: "tool", DependsOn: []string{"b", "c"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if len(dag.TopologicalOrder) != 4 {
		t.Fatalf("TopologicalOrder = %v, want 4 nodes", dag.TopologicalOrder)
	}

	if dag.TopologicalOrder[0] != "a" {
		t.Errorf("first node should be a, got %s", dag.TopologicalOrder[0])
	}
	if dag.TopologicalOrder[3] != "d" {
		t.Errorf("last node should be d, got %s", dag.TopologicalOrder[3])
	}

	bIdx := -1
	cIdx := -1
	for i, id := range dag.TopologicalOrder {
		if id == "b" {
			bIdx = i
		}
		if id == "c" {
			cIdx = i
		}
	}
	if bIdx == -1 || cIdx == -1 {
		t.Fatalf("b or c missing from topological order")
	}
	if bIdx == cIdx {
		t.Error("b and c should have different positions")
	}

	if len(dag.EntryNodes) != 1 || dag.EntryNodes[0] != "a" {
		t.Errorf("EntryNodes = %v, want [a]", dag.EntryNodes)
	}
	if len(dag.ExitNodes) != 1 || dag.ExitNodes[0] != "d" {
		t.Errorf("ExitNodes = %v, want [d]", dag.ExitNodes)
	}

	dNode := dag.Nodes["d"]
	if len(dNode.DependsOn) != 2 {
		t.Errorf("d.DependsOn = %v, want [b c]", dNode.DependsOn)
	}

	dependents := dag.Dependents["a"]
	if len(dependents) != 2 {
		t.Errorf("dependents of a = %v, want [b c] (order may vary)", dependents)
	}
}

func TestCompilerWithWhenExpression(t *testing.T) {
	whenRaw := json.RawMessage(`{"op":"eq","left":{"ref":{"source":"input","path":["enabled"]}},"right":{"value":true}}`)

	def := WorkflowDefinition{
		ID:              "test-when",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "When Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{When: &whenRaw}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	aNode := dag.Nodes["a"]
	if aNode.When == nil {
		t.Fatal("When expression should be compiled")
	}
	if aNode.When.Op != OpEq {
		t.Errorf("When op = %s, want eq", aNode.When.Op)
	}
}

func TestCompilerWithDataRefs(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-refs",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Data Refs Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{Input: json.RawMessage(`{"msg":"hello"}`)}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{Input: json.RawMessage(`{"prev":"steps.a.result"}`)}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	bNode := dag.Nodes["b"]
	if len(bNode.DataRefs) == 0 {
		t.Error("expected data refs to be extracted")
	}
}

func TestCompilerCycleDetection(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-cycle",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Cycle Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", DependsOn: []string{"c"}, Step: WorkflowStepInput{}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{}},
			{ID: "c", Type: "tool", DependsOn: []string{"b"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.Compile(def, DefaultCompileOptions())
	if err == nil {
		t.Fatal("expected error for cyclic workflow")
	}
}

func TestCompilerDuplicateNodeID(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-dup",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Duplicate ID Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{}},
			{ID: "a", Type: "tool", Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.Compile(def, DefaultCompileOptions())
	if err == nil {
		t.Fatal("expected error for duplicate node id")
	}
}

func TestCompilerMissingDep(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-missing",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Missing Dep Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", DependsOn: []string{"nonexistent"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	_, err := compiler.Compile(def, DefaultCompileOptions())
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestCompilerMaxNodesExceeded(t *testing.T) {
	nodes := make([]WorkflowNode, 0, 200)
	for i := 0; i < 200; i++ {
		nodes = append(nodes, WorkflowNode{
			ID:   "node-" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Type: "tool",
			Step: WorkflowStepInput{},
		})
	}

	def := WorkflowDefinition{
		ID:              "test-max",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Max Nodes Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes:           nodes,
	}

	compiler := NewCompiler()
	_, err := compiler.Compile(def, DefaultCompileOptions())
	if err == nil {
		t.Fatal("expected error for exceeding max nodes")
	}
}

func TestCompilerFromLegacy(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-legacy",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Legacy Linear",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10, MaxConcurrency: 4},
		Nodes: []WorkflowNode{
			{ID: "step-a", Type: "tool", Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"first"}`)}},
			{ID: "step-b", Type: "tool", Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"second"}`)}},
			{ID: "step-c", Type: "tool", Step: WorkflowStepInput{Input: json.RawMessage(`{"op":"third"}`)}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.CompileFromLegacy(def)
	if err != nil {
		t.Fatalf("compile from legacy: %v", err)
	}

	bNode := dag.Nodes["step-b"]
	if len(bNode.DependsOn) != 1 || bNode.DependsOn[0] != "step-a" {
		t.Errorf("step-b.DependsOn = %v, want [step-a]", bNode.DependsOn)
	}

	cNode := dag.Nodes["step-c"]
	if len(cNode.DependsOn) != 1 || cNode.DependsOn[0] != "step-b" {
		t.Errorf("step-c.DependsOn = %v, want [step-b]", cNode.DependsOn)
	}
}

func TestCompilerPermissionClosure(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-perms",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Permissions Workflow",
		CallableByAgent: true,
		Enabled:         true,
		Permissions:     []string{"base.permission"},
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Permissions: []string{"tool.execute"}, Step: WorkflowStepInput{}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Permissions: []string{"http.request"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	found := false
	for _, pc := range dag.PermissionClosure {
		if len(pc.Permissions) > 0 {
			for _, p := range pc.Permissions {
				if p == "base.permission" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("permission closure missing base.permission: %+v", dag.PermissionClosure)
	}
}

func TestCompilerPurityClassification(t *testing.T) {
	tests := []struct {
		nodeType string
		expected NodePurity
	}{
		{"transform", NodePurityPure},
		{"template", NodePurityPure},
		{"condition", NodePurityPure},
		{"tool", NodePuritySideEffecting},
		{"skill", NodePuritySideEffecting},
		{"schedule", NodePurityIdempotent},
		{"notification", NodePurityIdempotent},
		{"unknown_type", NodePurityUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.nodeType, func(t *testing.T) {
			got := classifyPurity(tt.nodeType)
			if got != tt.expected {
				t.Errorf("classifyPurity(%q) = %s, want %s", tt.nodeType, got, tt.expected)
			}
		})
	}
}

func TestReadyNodes(t *testing.T) {
	dag := &CompiledWorkflowDAG{
		TopologicalOrder: []string{"a", "b", "c", "d"},
		DependedOnBy: map[string][]string{
			"a": {},
			"b": {"a"},
			"c": {"a"},
			"d": {"b", "c"},
		},
	}

	states := make(map[string]NodeState)
	for _, id := range dag.TopologicalOrder {
		if len(dag.DependedOnBy[id]) == 0 {
			states[id] = NodeStatePending
		} else {
			states[id] = NodeStateBlocked
		}
	}

	ready := ReadyNodes(dag, states)
	if len(ready) != 1 || ready[0] != "a" {
		t.Errorf("first ready = %v, want [a]", ready)
	}

	states["a"] = NodeStateSucceeded
	ready = ReadyNodes(dag, states)
	if len(ready) != 2 {
		t.Fatalf("after a succeeds, ready should have 2 nodes, got %v", ready)
	}
	foundB := false
	foundC := false
	for _, id := range ready {
		if id == "b" {
			foundB = true
		}
		if id == "c" {
			foundC = true
		}
	}
	if !foundB || !foundC {
		t.Errorf("after a succeeds, ready should be [b, c], got %v", ready)
	}

	states["b"] = NodeStateSucceeded
	states["c"] = NodeStateSucceeded
	ready = ReadyNodes(dag, states)
	if len(ready) != 1 || ready[0] != "d" {
		t.Errorf("after b,c succeed, ready = %v, want [d]", ready)
	}
}

func TestReadyNodesWithFailure(t *testing.T) {
	dag := &CompiledWorkflowDAG{
		TopologicalOrder: []string{"a", "b", "c"},
		DependedOnBy: map[string][]string{
			"a": {},
			"b": {"a"},
			"c": {"b"},
		},
	}

	states := make(map[string]NodeState)
	for _, id := range dag.TopologicalOrder {
		if len(dag.DependedOnBy[id]) == 0 {
			states[id] = NodeStateReady
		} else {
			states[id] = NodeStateBlocked
		}
	}

	states["a"] = NodeStateFailed
	ready := ReadyNodes(dag, states)
	if len(ready) != 0 {
		t.Errorf("after a fails, ready should be empty, got %v", ready)
	}
}

func TestCompilerChecksumsDeterministic(t *testing.T) {
	def := WorkflowDefinition{
		ID:              "test-deterministic",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Deterministic",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{ID: "a", Type: "tool", Step: WorkflowStepInput{}},
			{ID: "b", Type: "tool", DependsOn: []string{"a"}, Step: WorkflowStepInput{}},
		},
	}

	compiler := NewCompiler()
	dag1, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile 1: %v", err)
	}
	dag2, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile 2: %v", err)
	}

	if dag1.CompiledChecksum != dag2.CompiledChecksum {
		t.Errorf("compiled checksums differ: %s vs %s", dag1.CompiledChecksum, dag2.CompiledChecksum)
	}
	if dag1.WorkflowChecksum != dag2.WorkflowChecksum {
		t.Errorf("workflow checksums differ: %s vs %s", dag1.WorkflowChecksum, dag2.WorkflowChecksum)
	}
}

func TestCompiledWorkflowNodeFields(t *testing.T) {
	whenRaw := json.RawMessage(`{"op":"exists","ref":{"source":"input","path":["x"]}}`)

	def := WorkflowDefinition{
		ID:              "test-fields",
		SchemaVersion:   CompiledSchemaVersion,
		Name:            "Fields Test",
		CallableByAgent: true,
		Enabled:         true,
		Limits:          WorkflowLimits{MaxSteps: 10},
		Nodes: []WorkflowNode{
			{
				ID:          "a",
				Type:        "tool",
				Permissions: []string{"exec"},
				Scope:       "readonly",
				Step: WorkflowStepInput{
					Input: json.RawMessage(`{"x":1}`),
					When:  &whenRaw,
					OnError: WorkflowOnError{
						Mode:    "continue",
						Default: json.RawMessage(`{"fallback":true}`),
					},
				},
			},
		},
	}

	compiler := NewCompiler()
	dag, err := compiler.Compile(def, DefaultCompileOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	node := dag.Nodes["a"]
	if node.ID != "a" {
		t.Errorf("ID = %s, want a", node.ID)
	}
	if node.Type != "tool" {
		t.Errorf("Type = %s, want tool", node.Type)
	}
	if len(node.Permissions) != 1 || node.Permissions[0] != "exec" {
		t.Errorf("Permissions = %v, want [exec]", node.Permissions)
	}
	if node.Scope != "readonly" {
		t.Errorf("Scope = %s, want readonly", node.Scope)
	}
	if node.When == nil {
		t.Error("When should be compiled")
	}
	if node.OnError.Mode != WorkflowErrorModeContinue {
		t.Errorf("OnError.Mode = %s, want continue", node.OnError.Mode)
	}
	if string(node.OnError.Default) != `{"fallback":true}` {
		t.Errorf("OnError.Default = %s, want {\"fallback\":true}", node.OnError.Default)
	}
}
