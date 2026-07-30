package editing

import (
	"context"
	"image"
)

type GenerationAdapter interface {
	GenerateSingleFrame(ctx context.Context, req SingleFrameGenerationRequest) (*SingleFrameGenerationResult, error)
	GenerateFullAction(ctx context.Context, req FullActionGenerationRequest) (*FullActionGenerationResult, error)
	GetGenerationArtifacts(ctx context.Context, generationTaskID, actionKey string, attemptNumber int) ([]GenerationArtifactInfo, error)
}

type SingleFrameGenerationRequest struct {
	GenerationTaskID  string
	ActionKey         string
	TargetFrameID     string
	FrameIndex        int
	TotalFrames       int
	AdjacentFrames    []AdjacentFrameContext
	FixIntent         string
	UserID            string
}

type AdjacentFrameContext struct {
	FrameID    string
	FrameIndex int
	ImagePath  string
}

type SingleFrameGenerationResult struct {
	ProviderAttemptID  string
	ImagePath          string
	Width              int
	Height             int
	CostActual         any
}

type FullActionGenerationRequest struct {
	GenerationTaskID string
	ActionKey        string
	UserID           string
}

type FullActionGenerationResult struct {
	ProviderAttemptID string
	CandidateRevisionID string
	FrameCount        int
	FramePaths        []string
}

type GenerationArtifactInfo struct {
	ArtifactID  string
	FrameIndex  int
	ImagePath   string
	Width       int
	Height      int
	ContentHash string
	AttemptID   string
}

type ProcessingAdapter interface {
	GetProcessingAction(ctx context.Context, processingTaskID, actionKey string) (*ProcessingActionInfo, error)
	GetProcessedFrames(ctx context.Context, processingActionID string) ([]ProcessedFrameInfo, error)
	GetProcessingRevisionFrames(ctx context.Context, revisionID string) ([]ProcessedFrameInfo, error)
	ImportAsBaselineRevision(ctx context.Context, processingTaskID, actionKey string) (*BaselineRevisionImport, error)
}

type ProcessingActionInfo struct {
	ProcessingActionID string
	ProcessingTaskID   string
	GenerationTaskID   string
	ActionKey          string
	ActionNameSnapshot string
	SourceAttemptNumber int
	Status             string
	LoopType           string
	FPS                int
	FrameDurationMS    int
	Excluded           bool
}

type ProcessedFrameInfo struct {
	FrameID         string
	FrameIndex      int
	ProcessedPath   string
	SourcePath      string
	Width           int
	Height          int
	ContentHash     string
	AnchorX         float64
	AnchorY         float64
	QualityFlags    string
}

type BaselineRevisionImport struct {
	ProcessingActionID string
	Frames             []ProcessedFrameInfo
	LoopType           string
	FPS                int
	FrameDurationMS    int
}

type QualityAdapter interface {
	EvaluateRevision(ctx context.Context, revisionID string) (string, error)
	GetLatestEvaluation(ctx context.Context, revisionID string) (*QualityEvaluationInfo, error)
	GetFindings(ctx context.Context, revisionID string) ([]QualityFindingInfo, error)
	IsGatePassed(ctx context.Context, revisionID string) (bool, string, error)
}

type QualityEvaluationInfo struct {
	EvaluationID string
	RevisionID   string
	Verdict      string
	Status       string
	OverallScore float64
}

type QualityFindingInfo struct {
	FindingID    string
	Severity     string
	Dimension    string
	FrameID      string
	FramePairID  string
	Description  string
	Stale        bool
}

type RevisionAssetStore interface {
	WriteAsset(ctx context.Context, data []byte, mimeType string, sourceType string, sourceRefID string) (*FrameAsset, error)
	GetAssetPath(assetID string) (string, error)
	GetAsset(assetID string) (*FrameAsset, error)
	ComputeHash(data []byte) string
	ReadImage(assetID string) (image.Image, string, error)
	WriteMaskData(ctx context.Context, sessionID string, data []byte) (string, error)
	EnsureRevisionDir(processingTaskID, actionKey, revisionID string) (string, error)
	WriteManifest(revisionID string, manifest *RevisionManifest) (string, string, error)
	ReadManifest(revisionID string) (*RevisionManifest, error)
	GetRevisionDir(processingTaskID, actionKey, revisionID string) string
	GetAssetStoreDir() string
}
