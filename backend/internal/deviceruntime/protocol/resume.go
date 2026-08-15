package protocol

type ResumeCursor = SessionCursor

type ResumeRequest struct {
	Identity SessionIdentity `json:"identity"`
	Cursor   ResumeCursor    `json:"cursor"`
}

type ResumeDecision struct {
	Mode                  ResumeMode `json:"mode"`
	RequiredStateRevision int64      `json:"requiredStateRevision,omitempty"`
	ReplayCommandAfter    int64      `json:"replayCommandAfter,omitempty"`
	ReplayEventAfter      int64      `json:"replayEventAfter,omitempty"`
	Reason                string     `json:"reason,omitempty"`
}
