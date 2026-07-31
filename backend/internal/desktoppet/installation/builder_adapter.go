package installation

import "github.com/u-ai/backend/internal/desktoppet/packageformat"

type ProcessingTaskInfo struct {
	ID                string
	GenerationTaskID  string
	ProcessingVersion int
	OutputWidth       int
	OutputHeight      int
	DefaultFPS        int
	CharacterID       string
	PackageName       string
	UserID            string
}

type BuilderActionInfo struct {
	ActionKey           string
	ActionNameSnapshot  string
	Status              string
	Excluded            bool
	LoopType            string
	FPS                 int
	FrameDurationMS     int
	SupportsDefaultIdle bool
}

type ActiveRevisionDetail struct {
	RevisionID     string
	RevisionNumber int
	Status         string
	FrameCount     int
	DurationMS     int
	DefaultFPS     int
	LoopType       string
	ReturnAction   string
	Interruptible  bool
	QualityVerdict string
	Frames         []BuilderFrameInfo
}

type BuilderFrameInfo struct {
	FrameID      string
	LogicalIndex int
	AssetID      string
	ContentHash  string
	DurationMS   int
	Width        int
	Height       int
	MimeType     string
	StoragePath  string
}

type RevisionSource interface {
	GetProcessingTaskInfo(processingTaskID string) (*ProcessingTaskInfo, error)
	ListProcessingActions(processingTaskID string) ([]BuilderActionInfo, error)
	GetActiveRevisionDetail(processingTaskID, actionKey string) (*ActiveRevisionDetail, error)
	GetAssetPath(assetID string) (string, error)
	GetPackagePreviewPath(processingTaskID string, processingVersion int) (string, error)
}

type BuildReleaseRequest struct {
	ProcessingTaskID string
	UserID           string
	PetID            string
	Version          string
	DefaultAction    string
	IncludedActions  []string
	IdempotencyKey   string
}

type BuildReleaseResult struct {
	Release    *PackageRelease
	Manifest   *packageformat.Manifest
	Validation *packageformat.ValidationReport
}

type ImportPackageRequest struct {
	UserID          string
	CharacterID     string
	PackageName     string
	LegacyPackageID string
	LegacyVersion   int
	ImportStagingID string
	DefaultAction   string
	CanvasWidth     int
	CanvasHeight    int
	IncludedActions []string
	IdempotencyKey  string
}

type ImportPackageResult struct {
	Release  *PackageRelease
	Identity *PetIdentity
}
