package capability

type ToolCancellationReasonCode string

const (
	CancellationReasonUserRequested       ToolCancellationReasonCode = "user_requested"
	CancellationReasonParentCancelled     ToolCancellationReasonCode = "parent_cancelled"
	CancellationReasonCallerContext       ToolCancellationReasonCode = "caller_context_cancelled"
	CancellationReasonStreamConsumerGone  ToolCancellationReasonCode = "stream_consumer_disconnected"
	CancellationReasonWorkflowCancelled   ToolCancellationReasonCode = "workflow_cancelled"
	CancellationReasonSystemShutdown      ToolCancellationReasonCode = "system_shutdown"
	CancellationReasonRuntimeRequested    ToolCancellationReasonCode = "runtime_requested"
)

func (code ToolCancellationReasonCode) Valid() bool {
	switch code {
	case CancellationReasonUserRequested,
		CancellationReasonParentCancelled,
		CancellationReasonCallerContext,
		CancellationReasonStreamConsumerGone,
		CancellationReasonWorkflowCancelled,
		CancellationReasonSystemShutdown,
		CancellationReasonRuntimeRequested:
		return true
	default:
		return false
	}
}

type ToolCancellationReason struct {
	Code               ToolCancellationReasonCode `json:"code"`
	OriginInvocationID string                     `json:"originInvocationId,omitempty"`
}

type CancellationExternalScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	SessionID      string
}

func (s CancellationExternalScope) Key() string {
	return s.UserID + "|" + s.CharacterID + "|" + s.ConversationID + "|" + s.SessionID
}

type WorkflowCancelFunc func(ctx context.Context, invocationID string, reason string) error
