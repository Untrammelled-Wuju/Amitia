package workflow

import (
	"context"
	"encoding/json"
	"time"
)

type ExecutionMode string

const (
	ExecutionModeLive       ExecutionMode = "live"
	ExecutionModeDryRun     ExecutionMode = "dry_run"
	ExecutionModeMocked     ExecutionMode = "mocked"
	ExecutionModeControlled ExecutionMode = "controlled_live"
)

type MockBehavior struct {
	NodeID string          `json:"nodeId"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type ExecutionOptions struct {
	Mode            ExecutionMode        `json:"mode"`
	Mocks           []MockBehavior       `json:"mocks,omitempty"`
	ResumeFrom      string               `json:"resumeFrom,omitempty"`
	SideEffectLimit int                  `json:"sideEffectLimit,omitempty"`
	ConfirmSideEffect func(nodeID string, kind SideEffectKind, target string) bool `json:"-"`
}

func DefaultExecutionOptions() ExecutionOptions {
	return ExecutionOptions{
		Mode:            ExecutionModeLive,
		SideEffectLimit: -1,
	}
}

type DryRunResult struct {
	WouldExecute []string        `json:"wouldExecute"`
	WouldSkip    []string        `json:"wouldSkip"`
	Transitions  []NodeStateTransition `json:"transitions"`
	WouldFail    []string        `json:"wouldFail,omitempty"`
	NodeOrder    []string        `json:"nodeOrder"`
	Duration     time.Duration   `json:"duration"`
	SideEffects  int             `json:"sideEffects"`
}

func ExecuteDryRun(ctx context.Context, dag *CompiledWorkflowDAG) *DryRunResult {
	result := &DryRunResult{
		Transitions: make([]NodeStateTransition, 0),
		NodeOrder:   dag.TopologicalOrder,
	}
	states := make(map[string]NodeState)
	for _, id := range dag.TopologicalOrder {
		states[id] = NodeStatePending
	}
	now := time.Now().UTC().UnixMilli()

	isReady := func(id string) bool {
		for _, dep := range dag.DependedOnBy[id] {
			if states[dep] != NodeStateSucceeded {
				return false
			}
		}
		return true
	}

	for _, id := range dag.TopologicalOrder {
		node := dag.Nodes[id]
		if isReady(id) {
			if node.HasSideEffects {
				result.SideEffects++
			}
			result.WouldExecute = append(result.WouldExecute, id)
			states[id] = NodeStateSucceeded
			result.Transitions = append(result.Transitions, NodeStateTransition{
				NodeID:    id,
				From:      NodeStatePending,
				To:        NodeStateSucceeded,
				Timestamp: now,
			})
		} else {
			result.WouldSkip = append(result.WouldSkip, id)
			states[id] = NodeStateSkipped
			result.Transitions = append(result.Transitions, NodeStateTransition{
				NodeID:    id,
				From:      NodeStatePending,
				To:        NodeStateSkipped,
				Timestamp: now,
			})
		}
	}
	return result
}

type MockedExecutionResult struct {
	ExecuteResult
	MocksApplied []string        `json:"mocksApplied"`
	NodeStates   map[string]NodeState `json:"nodeStates"`
}

func BuildMockLookup(mocks []MockBehavior) map[string]*MockBehavior {
	lookup := make(map[string]*MockBehavior)
	for i := range mocks {
		lookup[mocks[i].NodeID] = &mocks[i]
	}
	return lookup
}

func (o *ExecutionOptions) IsDryRun() bool {
	return o.Mode == ExecutionModeDryRun
}

func (o *ExecutionOptions) IsMocked() bool {
	return o.Mode == ExecutionModeMocked
}

func (o *ExecutionOptions) IsLive() bool {
	return o.Mode == ExecutionModeLive || o.Mode == ExecutionModeControlled
}

func (o *ExecutionOptions) AllowSideEffect(nodeID string, kind SideEffectKind, target string) bool {
	if o.Mode == ExecutionModeDryRun {
		return false
	}
	if o.Mode == ExecutionModeControlled && o.ConfirmSideEffect != nil {
		return o.ConfirmSideEffect(nodeID, kind, target)
	}
	if o.SideEffectLimit >= 0 {
		return true
	}
	return true
}

func (o *ExecutionOptions) EffectiveMockOutput(nodeID string) (json.RawMessage, string, bool) {
	if !o.IsMocked() {
		return nil, "", false
	}
	for _, m := range o.Mocks {
		if m.NodeID == nodeID {
			return m.Output, m.Error, true
		}
	}
	return nil, "", false
}
