package processing

type ProcessingRevision struct {
	ID                 string `gorm:"column:id;primaryKey" json:"id"`
	ProcessingTaskID   string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ProcessingActionID string `gorm:"column:processing_action_id" json:"processingActionId"`
	RevisionNumber     int    `gorm:"column:revision_number" json:"revisionNumber"`
	SourceAttemptID    string `gorm:"column:source_attempt_id" json:"sourceAttemptId"`
	SourceCandidateIdx int    `gorm:"column:source_candidate_index" json:"sourceCandidateIndex"`
	Status             string `gorm:"column:status" json:"status"`
	ConfigSnapshot     string `gorm:"column:config_snapshot" json:"configSnapshot"`
	ConfigHash         string `gorm:"column:config_hash" json:"configHash"`
	PipelineVersion    string `gorm:"column:pipeline_version" json:"pipelineVersion"`
	FrameCount         int    `gorm:"column:frame_count" json:"frameCount"`
	RootRelativePath   string `gorm:"column:root_relative_path" json:"rootRelativePath"`
	RevisionHash       string `gorm:"column:revision_hash" json:"revisionHash"`
	Active             int    `gorm:"column:active" json:"active"`
	ErrorCode          string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage       string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt          string `gorm:"column:created_at" json:"createdAt"`
	PublishedAt        string `gorm:"column:published_at" json:"publishedAt"`
	UpdatedAt          string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ProcessingRevision) TableName() string { return "desktop_pet_processing_revisions" }

type ProcessingArtifactRecord struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	RevisionID       string `gorm:"column:revision_id" json:"revisionId"`
	FrameIndex       *int   `gorm:"column:frame_index" json:"frameIndex,omitempty"`
	ArtifactKind     string `gorm:"column:artifact_kind" json:"artifactKind"`
	Stage            string `gorm:"column:stage" json:"stage"`
	RelativePath     string `gorm:"column:relative_path" json:"relativePath"`
	MimeType         string `gorm:"column:mime_type" json:"mimeType"`
	Width            int    `gorm:"column:width" json:"width"`
	Height           int    `gorm:"column:height" json:"height"`
	ByteSize         int64  `gorm:"column:byte_size" json:"byteSize"`
	ContentHash      string `gorm:"column:content_hash" json:"contentHash"`
	SourceArtifactID string `gorm:"column:source_artifact_id" json:"sourceArtifactId"`
	SourceCellIndex  *int   `gorm:"column:source_cell_index" json:"sourceCellIndex,omitempty"`
	MetadataJSON     string `gorm:"column:metadata_json" json:"metadataJson"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
}

func (ProcessingArtifactRecord) TableName() string { return "desktop_pet_processing_artifacts" }

type ProcessingTransformRecord struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	RevisionID       string `gorm:"column:revision_id" json:"revisionId"`
	FrameIndex       int    `gorm:"column:frame_index" json:"frameIndex"`
	SequenceNumber   int    `gorm:"column:sequence_number" json:"sequenceNumber"`
	FromSpace        string `gorm:"column:from_space" json:"fromSpace"`
	ToSpace          string `gorm:"column:to_space" json:"toSpace"`
	TransformType    string `gorm:"column:transform_type" json:"transformType"`
	MatrixJSON       string `gorm:"column:matrix_json" json:"matrixJson"`
	ParametersJSON   string `gorm:"column:parameters_json" json:"parametersJson"`
	AlgorithmVersion string `gorm:"column:algorithm_version" json:"algorithmVersion"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
}

func (ProcessingTransformRecord) TableName() string { return "desktop_pet_processing_transforms" }

type FrameMeasurementRecord struct {
	ID                       string  `gorm:"column:id;primaryKey" json:"id"`
	RevisionID               string  `gorm:"column:revision_id" json:"revisionId"`
	FrameIndex               int     `gorm:"column:frame_index" json:"frameIndex"`
	MeasurementSchemaVersion int     `gorm:"column:measurement_schema_version" json:"measurementSchemaVersion"`
	SubjectBoxJSON           string  `gorm:"column:subject_box_json" json:"subjectBoxJson"`
	SourceAnchorJSON         string  `gorm:"column:source_anchor_json" json:"sourceAnchorJson"`
	TargetAnchorJSON         string  `gorm:"column:target_anchor_json" json:"targetAnchorJson"`
	AlphaCoverage            float64 `gorm:"column:alpha_coverage" json:"alphaCoverage"`
	ComponentCount           int     `gorm:"column:component_count" json:"componentCount"`
	EdgeContactJSON          string  `gorm:"column:edge_contact_json" json:"edgeContactJson"`
	ClippingJSON             string  `gorm:"column:clipping_json" json:"clippingJson"`
	TrajectoryJSON           string  `gorm:"column:trajectory_json" json:"trajectoryJson"`
	MeasurementJSON          string  `gorm:"column:measurement_json" json:"measurementJson"`
	CreatedAt                string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (FrameMeasurementRecord) TableName() string { return "desktop_pet_frame_measurements" }
