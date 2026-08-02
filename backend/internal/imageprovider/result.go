package imageprovider

type SubmissionState string

const (
	SubmissionAccepted   SubmissionState = "accepted"
	SubmissionProcessing SubmissionState = "processing"
	SubmissionSucceeded  SubmissionState = "succeeded"
	SubmissionFailed     SubmissionState = "failed"
	SubmissionUnknown    SubmissionState = "unknown"
)

type RemoteArtifactReceipt struct {
	URL          string
	ProviderData map[string]any
	ExpiresAt    string
}

type CandidateImage struct {
	Index         int
	Bytes         []byte
	MimeType      string
	Width         int
	Height        int
	RemoteURL     string
	RemoteReceipt *RemoteArtifactReceipt
	Metadata      map[string]any
}

type GenerationResult struct {
	SubmissionState SubmissionState
	OperationID     string
	RequestID       string
	Candidates      []CandidateImage
	Provider        string
	Model           string
	Usage           *GenerationUsage
	ErrorCode       string
	ErrorMessage    string
	RawMetadata     map[string]any
}

func (r *GenerationResult) HasCandidates() bool {
	return len(r.Candidates) > 0
}

func (r *GenerationResult) PrimaryCandidate() *CandidateImage {
	if len(r.Candidates) == 0 {
		return nil
	}
	return &r.Candidates[0]
}

func (r *GenerationResult) IsSucceeded() bool {
	return r.SubmissionState == SubmissionSucceeded && len(r.Candidates) > 0
}

func (r *GenerationResult) IsAsync() bool {
	return r.SubmissionState == SubmissionProcessing || r.SubmissionState == SubmissionAccepted
}

func (r *GenerationResult) IsFailed() bool {
	return r.SubmissionState == SubmissionFailed
}

type GenerationSubmissionV2 struct {
	State       SubmissionState
	OperationID string
	RequestID   string
	Result      *GenerationResult
}
