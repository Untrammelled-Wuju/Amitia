package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

const UserWorkflowSchemaVersion = "workflow-v2"

// NormalizeDefinition keeps the editor-friendly explicit edge model and the
// executor-friendly DependsOn model in sync. Existing v1 definitions without
// edges remain valid; v2 definitions use edges as the source of truth.
func NormalizeDefinition(def WorkflowDefinition) (WorkflowDefinition, error) {
	// Registry values are returned by value, but slices still share backing arrays.
	// Clone editor-owned collections before normalizing so enable/update/compile
	// operations never mutate a definition that is already visible to readers.
	def.Nodes = append([]WorkflowNode(nil), def.Nodes...)
	for i := range def.Nodes {
		def.Nodes[i].DependsOn = append([]string(nil), def.Nodes[i].DependsOn...)
		def.Nodes[i].Permissions = append([]string(nil), def.Nodes[i].Permissions...)
	}
	def.Edges = append([]WorkflowEdge(nil), def.Edges...)
	def.Triggers = append([]WorkflowTriggerDefinition(nil), def.Triggers...)

	if strings.TrimSpace(def.ID) == "" {
		return def, fmt.Errorf("workflow id is required")
	}
	if strings.TrimSpace(def.Name) == "" {
		return def, fmt.Errorf("workflow name is required")
	}
	if def.SchemaVersion == "" {
		def.SchemaVersion = UserWorkflowSchemaVersion
	}
	if def.Version == "" {
		def.Version = "1.0.0"
	}
	if def.Source == "" {
		def.Source = "user"
	}
	if len(def.InputSchema) == 0 {
		def.InputSchema = json.RawMessage(`{"type":"object"}`)
	}
	if len(def.OutputSchema) == 0 {
		def.OutputSchema = json.RawMessage(`{}`)
	}
	if def.Metadata == nil {
		def.Metadata = map[string]any{}
	}

	// workflow-v2 uses explicit edges as the source of truth even when the
	// edge list is empty. Legacy definitions without explicit edges continue
	// to derive them from DependsOn for backwards compatibility.
	if def.SchemaVersion != UserWorkflowSchemaVersion && len(def.Edges) == 0 {
		def.Edges = DeriveEdges(def.Nodes)
	} else {
		ids := make(map[string]struct{}, len(def.Nodes))
		for i := range def.Nodes {
			ids[def.Nodes[i].ID] = struct{}{}
			def.Nodes[i].DependsOn = nil
		}
		seenEdges := map[string]struct{}{}
		for i := range def.Edges {
			e := &def.Edges[i]
			if strings.TrimSpace(e.ID) == "" {
				e.ID = fmt.Sprintf("edge-%s-%s-%d", e.Source, e.Target, i+1)
			}
			if _, ok := seenEdges[e.ID]; ok {
				return def, fmt.Errorf("duplicate edge id %s", e.ID)
			}
			seenEdges[e.ID] = struct{}{}
			if _, ok := ids[e.Source]; !ok {
				return def, fmt.Errorf("edge %s source node %s not found", e.ID, e.Source)
			}
			if _, ok := ids[e.Target]; !ok {
				return def, fmt.Errorf("edge %s target node %s not found", e.ID, e.Target)
			}
			if e.Source == e.Target {
				return def, fmt.Errorf("edge %s cannot connect node to itself", e.ID)
			}
			if len(e.Condition) > 0 {
				if _, err := CompileExpression(e.Condition); err != nil {
					return def, fmt.Errorf("edge %s condition: %w", e.ID, err)
				}
			}
			for n := range def.Nodes {
				if def.Nodes[n].ID == e.Target && !containsString(def.Nodes[n].DependsOn, e.Source) {
					def.Nodes[n].DependsOn = append(def.Nodes[n].DependsOn, e.Source)
					break
				}
			}
		}
	}

	if err := ValidateDAG(def.Nodes); err != nil {
		return def, err
	}
	seenTriggers := make(map[string]struct{}, len(def.Triggers))
	for i := range def.Triggers {
		t := &def.Triggers[i]
		t.ID = strings.TrimSpace(t.ID)
		t.Type = strings.TrimSpace(t.Type)
		if t.ID == "" {
			return def, fmt.Errorf("trigger id is required")
		}
		if t.Type == "" {
			return def, fmt.Errorf("trigger %s type is required", t.ID)
		}
		if _, exists := seenTriggers[t.ID]; exists {
			return def, fmt.Errorf("duplicate trigger id %s", t.ID)
		}
		seenTriggers[t.ID] = struct{}{}
	}
	def.DefinitionHash = ""
	def.DefinitionHash = ComputeDefinitionHash(def)
	return def, nil
}

func DeriveEdges(nodes []WorkflowNode) []WorkflowEdge {
	edges := make([]WorkflowEdge, 0)
	for _, node := range nodes {
		for i, dep := range node.DependsOn {
			edges = append(edges, WorkflowEdge{
				ID:     fmt.Sprintf("edge-%s-%s-%d", dep, node.ID, i+1),
				Source: dep,
				Target: node.ID,
			})
		}
	}
	return edges
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// MaterializeEdgeConditions converts editor edge predicates into the existing
// per-node When expression understood by both legacy and compiled executors.
// Multiple incoming edge predicates and an existing node predicate are ANDed.
func MaterializeEdgeConditions(def WorkflowDefinition) (WorkflowDefinition, error) {
	if len(def.Edges) == 0 {
		return def, nil
	}
	// Materialization is an execution/compile view only. Do not mutate the
	// registry's slice backing array or repeated compiles would keep AND-ing
	// the same edge predicates into Step.When.
	def.Nodes = append([]WorkflowNode(nil), def.Nodes...)
	byTarget := make(map[string][]json.RawMessage)
	for _, edge := range def.Edges {
		if len(edge.Condition) > 0 {
			byTarget[edge.Target] = append(byTarget[edge.Target], edge.Condition)
		}
	}
	for i := range def.Nodes {
		raws := byTarget[def.Nodes[i].ID]
		if len(raws) == 0 {
			continue
		}
		exprs := make([]*WorkflowExpression, 0, len(raws)+1)
		if def.Nodes[i].Step.When != nil && len(*def.Nodes[i].Step.When) > 0 {
			expr, err := CompileExpression(*def.Nodes[i].Step.When)
			if err != nil {
				return def, fmt.Errorf("node %s when: %w", def.Nodes[i].ID, err)
			}
			exprs = append(exprs, expr)
		}
		for _, raw := range raws {
			expr, err := CompileExpression(raw)
			if err != nil {
				return def, fmt.Errorf("node %s edge condition: %w", def.Nodes[i].ID, err)
			}
			exprs = append(exprs, expr)
		}
		var combined *WorkflowExpression
		if len(exprs) == 1 {
			combined = exprs[0]
		} else {
			combined = &WorkflowExpression{Op: OpAnd, Args: exprs}
		}
		raw, err := json.Marshal(combined)
		if err != nil {
			return def, err
		}
		value := json.RawMessage(raw)
		def.Nodes[i].Step.When = &value
	}
	return def, nil
}
