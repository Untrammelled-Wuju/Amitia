package protocol

type ReconcileRequest struct {
	Identity             SessionIdentity `json:"identity"`
	DesiredStateRevision int64           `json:"desiredStateRevision"`
	AppliedStateRevision int64           `json:"appliedStateRevision"`
	ActualStateHash      string          `json:"actualStateHash,omitempty"`
}

type ReconcileAction string

const (
	ReconcileActionNoop       ReconcileAction = "noop"
	ReconcileActionReplay     ReconcileAction = "replay"
	ReconcileActionFullSync   ReconcileAction = "full_sync"
	ReconcileActionDisconnect ReconcileAction = "disconnect"
)

func (a ReconcileAction) IsValid() bool {
	switch a {
	case ReconcileActionNoop, ReconcileActionReplay, ReconcileActionFullSync, ReconcileActionDisconnect:
		return true
	}
	return false
}

type ReconcileDecision struct {
	Action  ReconcileAction `json:"action"`
	FromSeq int64           `json:"fromSeq,omitempty"`
	Reason  string          `json:"reason,omitempty"`
}
