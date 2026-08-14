package protocol

type ResumeCursor struct {
	ConnectionGeneration         int64  `json:"connectionGeneration"`
	LastAppliedStateRevision     int64  `json:"lastAppliedStateRevision"`
	LastProcessedCommandSequence int64  `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64  `json:"lastEventSequence"`
	ActualStateHash              string `json:"actualStateHash,omitempty"`
}

func (c ResumeCursor) IsZero() bool {
	return c.ConnectionGeneration == 0 && c.LastAppliedStateRevision == 0 &&
		c.LastProcessedCommandSequence == 0 && c.LastEventSequence == 0 && c.ActualStateHash == ""
}

func (c ResumeCursor) Normalize() ResumeCursor {
	if c.ConnectionGeneration < 0 {
		c.ConnectionGeneration = 0
	}
	if c.LastAppliedStateRevision < 0 {
		c.LastAppliedStateRevision = 0
	}
	if c.LastProcessedCommandSequence < 0 {
		c.LastProcessedCommandSequence = 0
	}
	if c.LastEventSequence < 0 {
		c.LastEventSequence = 0
	}
	return c
}

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
