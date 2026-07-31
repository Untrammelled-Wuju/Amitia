package contracts

const RevisionManifestSchemaVersion = 1

const (
	RevisionStatusPreparing      = "preparing"
	RevisionStatusValidated      = "validated"
	RevisionStatusFilesPublished = "files_published"
	RevisionStatusDBCommitted    = "db_committed"
	RevisionStatusActive         = "active"
	RevisionStatusFailed         = "failed"
	RevisionStatusCancelled      = "cancelled"
	RevisionStatusCommitted      = "committed"
	RevisionStatusCorrupted      = "corrupted"
	RevisionStatusArchived       = "archived"
)

const (
	RevisionStatusLegacyDBCommitted = "db_committed"
	RevisionStatusLegacyActive      = "active"
)

const (
	PublishStagePreparing      = "preparing"
	PublishStageValidated      = "validated"
	PublishStageFilesPublished = "files_published"
	PublishStageDBCommitted    = "db_committed"
)

const (
	PublishStatusPending = "pending"
	PublishStatusDone    = "done"
	PublishStatusFailed  = "failed"
)

const (
	ArtifactKindCellSource  = "cell_source"
	ArtifactKindForeground  = "foreground"
	ArtifactKindMask        = "mask"
	ArtifactKindNormalized  = "normalized"
	ArtifactKindFrame       = "frame"
	ArtifactKindMeasurement = "measurement"
	ArtifactKindTransform   = "transform"
	ArtifactKindManifest    = "manifest"
)

type ProcessedActionRevision struct {
	ID                          string `json:"id"`
	ProcessingTaskID            string `json:"processingTaskId"`
	ProcessingActionID          string `json:"processingActionId"`
	ProcessingAttemptID         string `json:"processingAttemptId,omitempty"`
	RevisionNumber              int    `json:"revisionNumber"`
	SourceAttemptID             string `json:"sourceAttemptId"`
	SourceCandidate             int    `json:"sourceCandidate"`
	SourceManifestID            string `json:"sourceManifestId,omitempty"`
	SourceGenerationAttemptID   string `json:"sourceGenerationAttemptId,omitempty"`
	SourceGenerationArtifactID  string `json:"sourceGenerationArtifactId,omitempty"`
	SourceArtifactContentHash   string `json:"sourceArtifactContentHash,omitempty"`
	Status                      string `json:"status"`
	ConfigSnapshot              string `json:"configSnapshot"`
	ConfigHash                  string `json:"configHash"`
	PipelineVersion             string `json:"pipelineVersion"`
	FrameCount                  int    `json:"frameCount"`
	RootRelativePath            string `json:"rootRelativePath"`
	RootStorageKey              string `json:"rootStorageKey,omitempty"`
	RevisionHash                string `json:"revisionHash"`
	ContentRootHash             string `json:"contentRootHash,omitempty"`
	CommitID                    string `json:"commitId,omitempty"`
	Active                      bool   `json:"active"`
	ErrorCode                   string `json:"errorCode,omitempty"`
	ErrorMessage                string `json:"errorMessage,omitempty"`
	CreatedAt                   string `json:"createdAt"`
	PublishedAt                 string `json:"publishedAt,omitempty"`
	CommittedAt                 string `json:"committedAt,omitempty"`
	UpdatedAt                   string `json:"updatedAt"`
}

type ProcessingArtifact struct {
	ID               string `json:"id"`
	RevisionID       string `json:"revisionId"`
	FrameIndex       *int   `json:"frameIndex,omitempty"`
	ArtifactKind     string `json:"artifactKind"`
	Stage            string `json:"stage"`
	RelativePath     string `json:"relativePath"`
	MimeType         string `json:"mimeType"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	ByteSize         int64  `json:"byteSize"`
	ContentHash      string `json:"contentHash"`
	SourceArtifactID string `json:"sourceArtifactId,omitempty"`
	SourceCellIndex  *int   `json:"sourceCellIndex,omitempty"`
	MetadataJSON     string `json:"metadataJson,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

type ProcessingTransform struct {
	ID               string `json:"id"`
	RevisionID       string `json:"revisionId"`
	FrameIndex       int    `json:"frameIndex"`
	SequenceNumber   int    `json:"sequenceNumber"`
	FromSpace        string `json:"fromSpace"`
	ToSpace          string `json:"toSpace"`
	TransformType    string `json:"transformType"`
	MatrixJSON       string `json:"matrixJson"`
	ParametersJSON   string `json:"parametersJson,omitempty"`
	AlgorithmVersion string `json:"algorithmVersion"`
	CreatedAt        string `json:"createdAt"`
}

type FrameMeasurement struct {
	ID                       string  `json:"id"`
	RevisionID               string  `json:"revisionId"`
	FrameIndex               int     `json:"frameIndex"`
	MeasurementSchemaVersion int     `json:"measurementSchemaVersion"`
	SubjectBoxJSON           string  `json:"subjectBoxJson"`
	SourceAnchorJSON         string  `json:"sourceAnchorJson"`
	TargetAnchorJSON         string  `json:"targetAnchorJson"`
	AlphaCoverage            float64 `json:"alphaCoverage"`
	ComponentCount           int     `json:"componentCount"`
	EdgeContactJSON          string  `json:"edgeContactJson,omitempty"`
	ClippingJSON             string  `json:"clippingJson,omitempty"`
	TrajectoryJSON           string  `json:"trajectoryJson,omitempty"`
	MeasurementJSON          string  `json:"measurementJson,omitempty"`
	CreatedAt                string  `json:"createdAt"`
	UpdatedAt                string  `json:"updatedAt"`
}

type PublishJournalEntry struct {
	RevisionID string `json:"revisionId"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	Timestamp  string `json:"timestamp"`
}

type ManifestSource struct {
	AttemptID   string `json:"attemptId"`
	Candidate   int    `json:"candidate"`
	ArtifactID  string `json:"artifactId"`
	ContentHash string `json:"contentHash"`
	MimeType    string `json:"mimeType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type ManifestFrame struct {
	Index       int    `json:"index"`
	File        string `json:"file"`
	Mask        string `json:"mask,omitempty"`
	FileHash    string `json:"fileHash"`
	PixelHash   string `json:"pixelHash"`
	Measurement string `json:"measurement,omitempty"`
	Transform   string `json:"transform,omitempty"`
}

type RevisionManifest struct {
	SchemaVersion      int             `json:"schemaVersion"`
	RevisionID         string          `json:"revisionId"`
	ProcessingTaskID   string          `json:"processingTaskId"`
	ProcessingActionID string          `json:"processingActionId"`
	ActionKey          string          `json:"actionKey"`
	Source             ManifestSource  `json:"source"`
	PipelineVersion    string          `json:"pipelineVersion"`
	ConfigHash         string          `json:"configHash"`
	FrameCount         int             `json:"frameCount"`
	Frames             []ManifestFrame `json:"frames"`
	RevisionHash       string          `json:"revisionHash"`
}
