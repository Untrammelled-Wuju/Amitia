package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CompiledSchemaVersion = "workflow-v1"
	CompilerVersion       = "1.0.0"
)

type CompiledSchema struct {
	Raw    json.RawMessage `json:"raw"`
	Parsed map[string]any  `json:"-"`
}

type PermissionRequirement struct {
	NodeID      string   `json:"nodeId"`
	Permissions []string `json:"permissions"`
	Scope       string   `json:"scope,omitempty"`
}

type DependencyRequirement struct {
	NodeID    string `json:"nodeId"`
	RuntimeID string `json:"runtimeId,omitempty"`
}

type CompiledWorkflowNode struct {
	ID             string                  `json:"id"`
	Type           string                  `json:"type"`
	DependsOn      []string                `json:"dependsOn"`
	Index          int                     `json:"index"`
	TargetID       string                  `json:"targetId,omitempty"`
	Runtime        json.RawMessage         `json:"runtime,omitempty"`
	Permissions    []string                `json:"permissions,omitempty"`
	Scope          string                  `json:"scope,omitempty"`
	Input          map[string]any          `json:"input,omitempty"`
	When           *WorkflowExpression     `json:"when,omitempty"`
	Timeout        *time.Duration          `json:"timeout,omitempty"`
	Retry          *WorkflowRetryPolicy    `json:"retry,omitempty"`
	OnError        WorkflowNodeErrorPolicy `json:"onError,omitempty"`
	DataRefs       []*WorkflowValueRef     `json:"dataRefs,omitempty"`
	Purity         NodePurity              `json:"purity"`
	HasSideEffects bool                    `json:"hasSideEffects"`
}

type NodePurity string

const (
	NodePurityPure          NodePurity = "pure"
	NodePurityIdempotent    NodePurity = "idempotent"
	NodePuritySideEffecting NodePurity = "side_effecting"
	NodePurityUnknown       NodePurity = "unknown"
)

type CompiledWorkflowDAG struct {
	WorkflowID        string                          `json:"workflowId"`
	SchemaVersion     string                          `json:"schemaVersion"`
	CompilerVersion   string                          `json:"compilerVersion"`
	WorkflowChecksum  string                          `json:"workflowChecksum"`
	CompiledChecksum  string                          `json:"compiledChecksum"`
	InputSchema       CompiledSchema                  `json:"inputSchema"`
	OutputSchema      CompiledSchema                  `json:"outputSchema"`
	Nodes             map[string]CompiledWorkflowNode `json:"nodes"`
	TopologicalOrder  []string                        `json:"topologicalOrder"`
	Dependents        map[string][]string             `json:"dependents"`
	DependedOnBy      map[string][]string             `json:"dependedOnBy"`
	EntryNodes        []string                        `json:"entryNodes"`
	ExitNodes         []string                        `json:"exitNodes"`
	PermissionClosure []PermissionRequirement         `json:"permissionClosure"`
	DependencyClosure []DependencyRequirement         `json:"dependencyClosure"`
	Limits            WorkflowLimits                  `json:"limits"`
	DefinitionHash    string                          `json:"definitionHash"`
	Output            any                             `json:"output,omitempty"`
}

type Compiler struct {
	limits *CompilerLimits
}

type CompilerLimits struct {
	MaxNodes           int
	MaxDepth           int
	MaxExpressionDepth int
}

func DefaultCompilerLimits() *CompilerLimits {
	return &CompilerLimits{
		MaxNodes:           128,
		MaxDepth:           8,
		MaxExpressionDepth: MaxExpressionDepth,
	}
}

func NewCompiler() *Compiler {
	return &Compiler{limits: DefaultCompilerLimits()}
}

func NewCompilerWithLimits(l *CompilerLimits) *Compiler {
	return &Compiler{limits: l}
}

type CompileOptions struct {
	EnableWhen    bool
	EnableRetry   bool
	EnableTimeout bool
	StrictMode    bool
}

func DefaultCompileOptions() CompileOptions {
	return CompileOptions{
		EnableWhen:    true,
		EnableRetry:   true,
		EnableTimeout: true,
		StrictMode:    true,
	}
}

func (c *Compiler) Compile(def WorkflowDefinition, opts CompileOptions) (*CompiledWorkflowDAG, error) {
	var materializeErr error
	def, materializeErr = MaterializeEdgeConditions(def)
	if materializeErr != nil {
		return nil, materializeErr
	}
	if def.ID == "" {
		return nil, fmt.Errorf("workflow: missing id")
	}
	if def.SchemaVersion == "" {
		def.SchemaVersion = CompiledSchemaVersion
	}

	canonicalNodes := make([]WorkflowNode, 0, len(def.Nodes))
	for _, n := range def.Nodes {
		canonicalNodes = append(canonicalNodes, normalizeNode(n))
	}

	if c.limits.MaxNodes > 0 && len(canonicalNodes) > c.limits.MaxNodes {
		return nil, fmt.Errorf("workflow %s: exceeds max nodes %d > %d", def.ID, len(canonicalNodes), c.limits.MaxNodes)
	}

	if err := ValidateDAG(canonicalNodes); err != nil {
		return nil, err
	}

	topo, err := TopologicalSort(canonicalNodes)
	if err != nil {
		return nil, err
	}

	if err := c.checkDepth(canonicalNodes, topo); err != nil {
		return nil, err
	}

	dependents := make(map[string][]string)
	dependedOnBy := make(map[string][]string)
	for _, n := range canonicalNodes {
		dependents[n.ID] = []string{}
		dependedOnBy[n.ID] = []string{}
	}
	for _, n := range canonicalNodes {
		for _, dep := range n.DependsOn {
			dependents[dep] = append(dependents[dep], n.ID)
			dependedOnBy[n.ID] = append(dependedOnBy[n.ID], dep)
		}
	}

	var entryNodes []string
	for _, n := range canonicalNodes {
		if len(n.DependsOn) == 0 {
			entryNodes = append(entryNodes, n.ID)
		}
	}
	var exitNodes []string
	for _, n := range canonicalNodes {
		if len(dependents[n.ID]) == 0 {
			exitNodes = append(exitNodes, n.ID)
		}
	}
	sort.Strings(entryNodes)
	sort.Strings(exitNodes)

	compiled := &CompiledWorkflowDAG{
		WorkflowID:       def.ID,
		SchemaVersion:    CompiledSchemaVersion,
		CompilerVersion:  CompilerVersion,
		WorkflowChecksum: ComputeDefinitionHash(def),
		InputSchema:      CompiledSchema{Raw: def.InputSchema},
		OutputSchema:     CompiledSchema{Raw: def.OutputSchema},
		Nodes:            make(map[string]CompiledWorkflowNode),
		TopologicalOrder: topo,
		Dependents:       dependents,
		DependedOnBy:     dependedOnBy,
		EntryNodes:       entryNodes,
		ExitNodes:        exitNodes,
		Limits:           def.Limits,
		DefinitionHash:   ComputeDefinitionHash(def),
	}

	permMap := make(map[string]*PermissionRequirement)
	depMap := make(map[string]*DependencyRequirement)

	for i, n := range canonicalNodes {
		cn := CompiledWorkflowNode{
			ID:          n.ID,
			Type:        n.Type,
			DependsOn:   n.DependsOn,
			Index:       i,
			TargetID:    n.TargetID,
			Permissions: n.Permissions,
			Scope:       n.Scope,
			Input:       make(map[string]any),
			OnError:     WorkflowNodeErrorPolicy{Mode: WorkflowErrorModeFail},
			Purity:      classifyPurity(n.Type),
		}

		if n.Runtime.RuntimeID != "" {
			cn.Runtime, _ = json.Marshal(n.Runtime)
		}

		if opts.EnableWhen && n.Step.When != nil {
			expr, err := CompileExpression(*n.Step.When)
			if err != nil {
				return nil, fmt.Errorf("workflow %s node %s when: %w", def.ID, n.ID, err)
			}
			cn.When = expr
		}

		if len(n.Step.Input) > 0 {
			var inputMap map[string]any
			if err := json.Unmarshal(n.Step.Input, &inputMap); err == nil {
				cn.Input = inputMap
				cn.DataRefs = extractRefs(inputMap)
			} else {
				cn.Input = map[string]any{"input": json.RawMessage(n.Step.Input)}
			}
		}

		if opts.EnableRetry {
			cn.Retry = c.compileRetry(n)
		}

		if n.Step.OnError.Mode != "" {
			cn.OnError = WorkflowNodeErrorPolicy{
				Mode:    WorkflowErrorMode(n.Step.OnError.Mode),
				Default: n.Step.OnError.Default,
			}
			if cn.OnError.Mode == "" {
				cn.OnError.Mode = WorkflowErrorModeFail
			}
		}

		cn.HasSideEffects = classifyPurity(n.Type) == NodePuritySideEffecting

		compiled.Nodes[n.ID] = cn

		if len(n.Permissions) > 0 {
			permMap[n.ID] = &PermissionRequirement{
				NodeID:      n.ID,
				Permissions: n.Permissions,
				Scope:       n.Scope,
			}
		}
		if n.TargetID != "" {
			depMap[n.ID] = &DependencyRequirement{
				NodeID:    n.ID,
				RuntimeID: n.TargetID,
			}
		}
	}

	compiled.PermissionClosure = buildPermissionClosure(permMap, def)
	compiled.DependencyClosure = buildDependencyClosure(depMap, def)

	compiled.CompiledChecksum = computeCompiledChecksum(compiled)

	return compiled, nil
}

func (c *Compiler) CompileFromLegacy(def WorkflowDefinition) (*CompiledWorkflowDAG, error) {
	migrated := make([]WorkflowNode, 0, len(def.Nodes))
	var prevID string
	for i, n := range def.Nodes {
		node := n
		if node.ID == "" {
			node.ID = fmt.Sprintf("step-%d", i+1)
		}
		if i > 0 && len(node.DependsOn) == 0 && prevID != "" {
			node.DependsOn = []string{prevID}
		}
		migrated = append(migrated, node)
		prevID = node.ID
	}
	def.Nodes = migrated
	return c.Compile(def, DefaultCompileOptions())
}

func (c *Compiler) compileRetry(n WorkflowNode) *WorkflowRetryPolicy {
	rp := DefaultRetryPolicy()
	if n.Step.OnError.Mode == "retry" {
		rp.MaxAttempts = 3
	}
	return rp
}

func (c *Compiler) checkDepth(nodes []WorkflowNode, topo []string) error {
	if c.limits.MaxDepth <= 0 {
		return nil
	}
	depth := make(map[string]int)
	maxDepth := 0
	for _, id := range topo {
		node := findNode(nodes, id)
		if node == nil {
			continue
		}
		d := 0
		for _, dep := range node.DependsOn {
			if depth[dep]+1 > d {
				d = depth[dep] + 1
			}
		}
		depth[id] = d
		if d > maxDepth {
			maxDepth = d
		}
	}
	if maxDepth > c.limits.MaxDepth {
		return fmt.Errorf("workflow: DAG depth %d exceeds max %d", maxDepth, c.limits.MaxDepth)
	}
	return nil
}

func findNode(nodes []WorkflowNode, id string) *WorkflowNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func normalizeNode(n WorkflowNode) WorkflowNode {
	if n.Type == "" {
		n.Type = "tool"
	}
	if n.DependsOn == nil {
		n.DependsOn = []string{}
	}
	if n.Permissions == nil {
		n.Permissions = []string{}
	}
	return n
}

func classifyPurity(nodeType string) NodePurity {
	switch nodeType {
	case "transform", "template", "condition":
		return NodePurityPure
	case "tool", "skill", "runtime_handler", "http", "call_skill":
		return NodePuritySideEffecting
	case "schedule", "notification", "memory_candidate", "context_contribution":
		return NodePurityIdempotent
	default:
		return NodePurityUnknown
	}
}

func extractRefs(input map[string]any) []*WorkflowValueRef {
	var refs []*WorkflowValueRef
	collectRefs(input, &refs)
	return refs
}

func collectRefs(v any, refs *[]*WorkflowValueRef) {
	switch val := v.(type) {
	case map[string]any:
		for _, child := range val {
			collectRefs(child, refs)
		}
	case []any:
		for _, child := range val {
			collectRefs(child, refs)
		}
	case string:
		if strings.HasPrefix(val, "input.") ||
			strings.HasPrefix(val, "config.") ||
			strings.HasPrefix(val, "runtime.") ||
			strings.HasPrefix(val, "steps.") ||
			strings.HasPrefix(val, "node.") ||
			strings.HasPrefix(val, "literal:") {
			if ref, err := ParseWorkflowValueRef(val); err == nil {
				*refs = append(*refs, ref)
			}
		}
	}
}

func buildPermissionClosure(m map[string]*PermissionRequirement, def WorkflowDefinition) []PermissionRequirement {
	result := make([]PermissionRequirement, 0, len(m)+1)
	seen := make(map[string]bool)
	for _, p := range def.Permissions {
		key := "::" + p
		if !seen[key] {
			seen[key] = true
			result = append(result, PermissionRequirement{Permissions: []string{p}})
		}
	}
	for _, perm := range m {
		result = append(result, *perm)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeID < result[j].NodeID
	})
	deduped := make([]PermissionRequirement, 0, len(result))
	seenPerm := make(map[string]bool)
	for _, r := range result {
		key := r.NodeID + ":"
		for _, p := range r.Permissions {
			key += p + ","
		}
		if !seenPerm[key] {
			seenPerm[key] = true
			deduped = append(deduped, r)
		}
	}
	return deduped
}

func buildDependencyClosure(m map[string]*DependencyRequirement, def WorkflowDefinition) []DependencyRequirement {
	result := make([]DependencyRequirement, 0, len(m))
	for _, dep := range m {
		result = append(result, *dep)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeID < result[j].NodeID
	})
	return result
}

func computeCompiledChecksum(dag *CompiledWorkflowDAG) string {
	payload := map[string]any{
		"workflowId":       dag.WorkflowID,
		"schemaVersion":    dag.SchemaVersion,
		"compilerVersion":  dag.CompilerVersion,
		"workflowChecksum": dag.WorkflowChecksum,
		"topologicalOrder": dag.TopologicalOrder,
		"nodes":            dag.Nodes,
		"entryNodes":       dag.EntryNodes,
		"exitNodes":        dag.ExitNodes,
		"limits":           dag.Limits,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ComputeCompiledChecksum(dag *CompiledWorkflowDAG) string {
	return computeCompiledChecksum(dag)
}

func ReadyNodes(dag *CompiledWorkflowDAG, states map[string]NodeState) []string {
	var ready []string
	for _, id := range dag.TopologicalOrder {
		if states[id] != NodeStatePending && states[id] != NodeStateBlocked {
			continue
		}
		deps := dag.DependedOnBy[id]
		allTerminal := true
		allSatisfy := true
		for _, dep := range deps {
			depState := states[dep]
			if !depState.IsTerminal() {
				allTerminal = false
				allSatisfy = false
				break
			}
			if depState == NodeStateFailed || depState == NodeStateCancelled {
				allSatisfy = false
			}
		}
		if !allTerminal {
			if states[id] != NodeStateBlocked {
				states[id] = NodeStateBlocked
			}
			continue
		}
		if allSatisfy {
			ready = append(ready, id)
		} else {
			if states[id] != NodeStateBlocked {
				states[id] = NodeStateBlocked
			}
		}
	}
	return ready
}
