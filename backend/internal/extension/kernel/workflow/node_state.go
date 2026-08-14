package workflow

import "fmt"

type NodeState string

const (
	NodeStatePending   NodeState = "pending"
	NodeStateBlocked   NodeState = "blocked"
	NodeStateReady     NodeState = "ready"
	NodeStateRunning   NodeState = "running"
	NodeStateSucceeded NodeState = "succeeded"
	NodeStateFailed    NodeState = "failed"
	NodeStateDefaulted NodeState = "defaulted"
	NodeStateSkipped   NodeState = "skipped"
	NodeStateCancelled NodeState = "cancelled"
	NodeStateRetryWait NodeState = "retry_wait"
)

func (s NodeState) IsTerminal() bool {
	switch s {
	case NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted,
		NodeStateSkipped, NodeStateCancelled:
		return true
	default:
		return false
	}
}

func (s NodeState) IsActive() bool {
	switch s {
	case NodeStateRunning, NodeStateRetryWait:
		return true
	default:
		return false
	}
}

func (s NodeState) IsValid() bool {
	switch s {
	case NodeStatePending, NodeStateBlocked, NodeStateReady, NodeStateRunning,
		NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted, NodeStateSkipped,
		NodeStateCancelled, NodeStateRetryWait:
		return true
	default:
		return false
	}
}

type DAGState string

const (
	DAGStateCreated          DAGState = "created"
	DAGStateRunning          DAGState = "running"
	DAGStateWaiting          DAGState = "waiting"
	DAGStateSucceeded        DAGState = "succeeded"
	DAGStateFailed           DAGState = "failed"
	DAGStateCancelled        DAGState = "cancelled"
	DAGStateBlocked          DAGState = "blocked"
	DAGStateRecoveryRequired DAGState = "recovery_required"
)

func (s DAGState) IsTerminal() bool {
	switch s {
	case DAGStateSucceeded, DAGStateFailed, DAGStateCancelled:
		return true
	default:
		return false
	}
}

func (s DAGState) IsActive() bool {
	switch s {
	case DAGStateRunning, DAGStateWaiting:
		return true
	default:
		return false
	}
}

func (s DAGState) ToRunStatus() RunStatus {
	switch s {
	case DAGStateRunning, DAGStateWaiting:
		return RunStatusRunning
	case DAGStateSucceeded:
		return RunStatusSucceeded
	case DAGStateFailed:
		return RunStatusFailed
	case DAGStateCancelled:
		return RunStatusCancelled
	case DAGStateBlocked:
		return RunStatusPaused
	case DAGStateRecoveryRequired:
		return RunStatusRunning
	default:
		return RunStatusRunning
	}
}

func NodeStateToStepStatus(s NodeState) string {
	switch s {
	case NodeStateSucceeded:
		return "succeeded"
	case NodeStateFailed:
		return "failed"
	case NodeStateSkipped:
		return "skipped"
	case NodeStateDefaulted:
		return "defaulted"
	case NodeStateCancelled:
		return "cancelled"
	default:
		return string(s)
	}
}

type NodeStateTransition struct {
	NodeID    string
	From      NodeState
	To        NodeState
	Timestamp int64
	Reason    string
}

func ValidateNodeTransition(from, to NodeState) error {
	if from == to {
		return nil
	}
	validTransitions := map[NodeState][]NodeState{
		NodeStatePending:   {NodeStateReady, NodeStateBlocked, NodeStateCancelled},
		NodeStateBlocked:   {NodeStateReady, NodeStateCancelled},
		NodeStateReady:     {NodeStateRunning, NodeStateCancelled},
		NodeStateRunning:   {NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted, NodeStateSkipped, NodeStateRetryWait, NodeStateCancelled},
		NodeStateRetryWait: {NodeStateRunning, NodeStateFailed, NodeStateCancelled},
		NodeStateSucceeded: {},
		NodeStateFailed:    {},
		NodeStateDefaulted: {},
		NodeStateSkipped:   {},
		NodeStateCancelled: {},
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown from state: %s", from)
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return fmt.Errorf("invalid node state transition: %s -> %s", from, to)
}
