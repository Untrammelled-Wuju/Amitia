package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Mode                ExecutionMode                                                `json:"mode"`
	Mocks               []MockBehavior                                               `json:"mocks,omitempty"`
	ResumeFrom          string                                                       `json:"resumeFrom,omitempty"`
	SideEffectLimit     int                                                          `json:"sideEffectLimit,omitempty"`
	ApprovedSideEffects []string                                                     `json:"approvedSideEffects,omitempty"`
	ConfirmSideEffect   func(nodeID string, kind SideEffectKind, target string) bool `json:"-"`
}

func DefaultExecutionOptions() ExecutionOptions {
	return ExecutionOptions{
		Mode:            ExecutionModeLive,
		SideEffectLimit: -1,
	}
}

func (o ExecutionOptions) Normalize() (ExecutionOptions, error) {
	o.Mode = ExecutionMode(strings.ToLower(strings.TrimSpace(string(o.Mode))))
	if o.Mode == "" {
		o.Mode = ExecutionModeLive
	}
	switch o.Mode {
	case ExecutionModeLive, ExecutionModeDryRun, ExecutionModeMocked, ExecutionModeControlled:
	default:
		return ExecutionOptions{}, fmt.Errorf("workflow: unsupported execution mode %q", o.Mode)
	}
	if o.SideEffectLimit == 0 {
		// Zero-value requests historically meant unlimited. Keep that compatibility
		// while DefaultExecutionOptions continues to use -1 explicitly.
		o.SideEffectLimit = -1
	}
	seen := make(map[string]struct{}, len(o.ApprovedSideEffects))
	approved := make([]string, 0, len(o.ApprovedSideEffects))
	for _, id := range o.ApprovedSideEffects {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		approved = append(approved, id)
	}
	o.ApprovedSideEffects = approved
	return o, nil
}

func (o ExecutionOptions) IsSideEffectApproved(nodeID string, kind SideEffectKind, target string) bool {
	if o.Mode != ExecutionModeControlled {
		return true
	}
	if o.ConfirmSideEffect != nil {
		return o.ConfirmSideEffect(nodeID, kind, target)
	}
	nodeID = strings.TrimSpace(nodeID)
	for _, approved := range o.ApprovedSideEffects {
		if approved == "*" || approved == nodeID {
			return true
		}
	}
	return false
}

func (o ExecutionOptions) MissingControlledApprovals(nodes []WorkflowNode) []string {
	if o.Mode != ExecutionModeControlled {
		return nil
	}
	missing := make([]string, 0)
	for _, node := range nodes {
		kind, sideEffecting := sideEffectKindForNode(node)
		if !sideEffecting {
			continue
		}
		if !o.IsSideEffectApproved(node.ID, kind, node.TargetID) {
			missing = append(missing, node.ID)
		}
	}
	return missing
}

// MissingControlledApprovalsForRun reconstructs the immutable workflow definition
// captured by a controlled-live run and reports approvals that are still required.
// It deliberately avoids consulting the current registry so a later workflow revision
// cannot change the confirmation boundary of an already-created run.
func MissingControlledApprovalsForRun(run *WorkflowRun) []string {
	if run == nil || run.Status != RunStatusWaitingConfirmation {
		return nil
	}
	opts, err := run.Context.ExecutionOptions.Normalize()
	if err != nil || opts.Mode != ExecutionModeControlled || len(run.Context.DefinitionSnapshot) == 0 {
		return nil
	}
	var def WorkflowDefinition
	if err := json.Unmarshal(run.Context.DefinitionSnapshot, &def); err != nil {
		return nil
	}
	return opts.MissingControlledApprovals(def.Nodes)
}

type DryRunResult struct {
	WouldExecute []string              `json:"wouldExecute"`
	WouldSkip    []string              `json:"wouldSkip"`
	Transitions  []NodeStateTransition `json:"transitions"`
	WouldFail    []string              `json:"wouldFail,omitempty"`
	NodeOrder    []string              `json:"nodeOrder"`
	Duration     time.Duration         `json:"duration"`
	SideEffects  int                   `json:"sideEffects"`
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
	MocksApplied []string             `json:"mocksApplied"`
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
	if o == nil || o.Mode == ExecutionModeDryRun {
		return false
	}
	if o.Mode == ExecutionModeControlled {
		return o.IsSideEffectApproved(nodeID, kind, target)
	}
	return true
}

// ExecutionOptionsForRerun preserves the source run's execution semantics while
// deliberately clearing per-run side-effect approvals. A new controlled-live run
// must earn confirmation again; approvals are never transferable authorization.
func ExecutionOptionsForRerun(run *WorkflowRun) ExecutionOptions {
	if run == nil {
		return DefaultExecutionOptions()
	}
	opts, err := run.Context.ExecutionOptions.Normalize()
	if err != nil {
		return DefaultExecutionOptions()
	}
	opts.ApprovedSideEffects = nil
	opts.ConfirmSideEffect = nil
	opts.ResumeFrom = ""
	return opts
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
