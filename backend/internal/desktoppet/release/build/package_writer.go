package build

import (
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type StagedAction struct {
	ActionKey           string
	DisplayName         string
	RevisionID          string
	QualityVerdict      string
	LoopType            string
	FPS                 int
	FrameCount          int
	SupportsDefaultIdle bool
	FrameEntries        []StagedFrameEntry
}

type StagedFrameEntry struct {
	FrameID     string
	Index       int
	File        string
	DurationMs  int
	AssetID     string
	ContentHash string
}

type StagedPackage struct {
	StagingDir      string
	Actions         []StagedAction
	PreviewName     string
	ManifestData    []byte
	ManifestHash    string
	ContentRootHash string
	TotalBytes      int64
	FileCount       int
	Files           []StagedFile
}

type StagedFile struct {
	Path      string
	SHA256    string
	Bytes     int64
	MediaType string
	Role      string
	ActionKey string
	FrameID   string
}

type ValidationResult struct {
	Verdict      string
	FileCount    int
	ErrorCount   int
	WarningCount int
	FindingsJSON string
}

type PackageWriter interface {
	StagePackage(snapshot *release.ReleaseBuildSnapshot, actionSnapshots []release.ReleaseActionSnapshot, taskInfo *release.TaskInfo, identity *release.PetIdentityData, previewArtifactID string, defaultActionKey string) (*StagedPackage, error)
	ValidatePackage(staged *StagedPackage) (*ValidationResult, error)
	WriteManifest(staged *StagedPackage) error
	BuildArchive(publishedDir string, petID, releaseID string) error
	MoveStagingToPublished(petID, releaseID string, stagingDir string) error
	RemoveStagingDir(stagingDir string) error
	RemovePublishedDir(petID, releaseID string) error
	PublishedDir(petID, releaseID string) string
	PublishedStorageKey(petID, releaseID string) string
	ArchiveStorageKey(petID, releaseID string) string
}
