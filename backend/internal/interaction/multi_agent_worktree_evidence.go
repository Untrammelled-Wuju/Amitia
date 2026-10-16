package interaction

type CodingTerminalDiagnostics struct {
	State            string `json:"state"`
	VerificationKind string `json:"verificationKind"`
	Total            int    `json:"total"`
	Exited           int    `json:"exited"`
	Closed           int    `json:"closed"`
	Revoked          int    `json:"revoked"`
	UnverifiedActive int    `json:"unverifiedActive"`
	LastExitCode     *int   `json:"lastExitCode,omitempty"`
	LatestState      string `json:"latestState,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
}

type CodingWorktreeReview struct {
	VerificationGate   *CodingWorkerVerificationGate `json:"verificationGate,omitempty"`
	Terminal           *CodingTerminalDiagnostics    `json:"terminal,omitempty"`
	State              string                        `json:"state"`
	WorktreeID         string                        `json:"worktreeId,omitempty"`
	WorkspaceURI       string                        `json:"workspaceUri,omitempty"`
	BaselineHead       string                        `json:"baselineHead,omitempty"`
	CurrentHead        string                        `json:"currentHead,omitempty"`
	FilesChanged       int                           `json:"filesChanged"`
	Insertions         int                           `json:"insertions"`
	Deletions          int                           `json:"deletions"`
	UntrackedCount     int                           `json:"untrackedCount"`
	ConflictedCount    int                           `json:"conflictedCount"`
	ReviewIncomplete   bool                          `json:"reviewIncomplete"`
	RequiresApproval   bool                          `json:"requiresApproval"`
	ErrorCode          string                        `json:"errorCode,omitempty"`
	CheckpointRevision string                        `json:"checkpointRevision,omitempty"`
	TestState          string                        `json:"testState,omitempty"`
	TestProfile        string                        `json:"testProfile,omitempty"`
	TestExitCode       int                           `json:"testExitCode,omitempty"`
	TestOutputDigest   string                        `json:"testOutputDigest,omitempty"`
	TestStartedAt      string                        `json:"testStartedAt,omitempty"`
	ApprovalState      string                        `json:"approvalState,omitempty"`
	MergeState         string                        `json:"mergeState,omitempty"`
	MergeTargetBranch  string                        `json:"mergeTargetBranch,omitempty"`
	MergeAppliedHead   string                        `json:"mergeAppliedHead,omitempty"`
}
