package generation

type ArtifactType string

const (
	ArtifactTypeSpriteSheetRaw   ArtifactType = "sprite_sheet_raw"
	ArtifactTypeKeyframeSheetRaw ArtifactType = "keyframe_sheet_raw"
	ArtifactTypeSingleFrameRaw   ArtifactType = "single_frame_raw"
	ArtifactTypeLegacyFrameRaw   ArtifactType = "legacy_frame_raw"
	ArtifactTypeProviderReceipt  ArtifactType = "provider_receipt"
	ArtifactTypeLayoutManifest   ArtifactType = "layout_manifest"
)

type ArtifactStatus string

const (
	ArtifactStatusPending       ArtifactStatus = "pending"
	ArtifactStatusStaging       ArtifactStatus = "staging"
	ArtifactStatusSaved         ArtifactStatus = "saved"
	ArtifactStatusVerified      ArtifactStatus = "verified"
	ArtifactStatusPersisted     ArtifactStatus = "persisted"
	ArtifactStatusFailed        ArtifactStatus = "failed"
	ArtifactStatusRejected      ArtifactStatus = "rejected"
	ArtifactStatusOrphaned      ArtifactStatus = "orphaned"
	ArtifactStatusPublishFailed ArtifactStatus = "publish_failed"
	ArtifactStatusOrphanedByCancel    ArtifactStatus = "orphaned_by_cancel"
	ArtifactStatusRetainedCandidate   ArtifactStatus = "retained_candidate"
)

type GenerationArtifact struct {
	ID                  string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskID              string `gorm:"column:task_id;type:text" json:"taskId"`
	TaskActionID        string `gorm:"column:task_action_id;type:text" json:"taskActionId"`
	AttemptID           string `gorm:"column:attempt_id;type:text" json:"attemptId"`
	ArtifactType        string `gorm:"column:artifact_type;type:text" json:"artifactType"`
	ArtifactRole        string `gorm:"column:artifact_role;type:text" json:"artifactRole"`
	SegmentIndex        int    `gorm:"column:segment_index;type:integer" json:"segmentIndex"`
	CandidateIndex      int    `gorm:"column:candidate_index;type:integer" json:"candidateIndex"`
	IsPrimary           int    `gorm:"column:is_primary;type:integer" json:"isPrimary"`
	Status              string `gorm:"column:status;type:text" json:"status"`
	RelativePath        string `gorm:"column:relative_path;type:text" json:"relativePath"`
	StorageKey          string `gorm:"column:storage_key;type:text" json:"storageKey"`
	StorageBackend      string `gorm:"column:storage_backend;type:text" json:"storageBackend"`
	ContentHash         string `gorm:"column:content_hash;type:text" json:"contentHash"`
	MIME                string `gorm:"column:mime;type:text" json:"mime"`
	Width               int    `gorm:"column:width;type:integer" json:"width"`
	Height              int    `gorm:"column:height;type:integer" json:"height"`
	Size                int64  `gorm:"column:size;type:integer" json:"size"`
	Hash                string `gorm:"column:hash;type:text" json:"hash"`
	ProviderRequestID   string `gorm:"column:provider_request_id;type:text" json:"providerRequestId"`
	ProviderOperationID string `gorm:"column:provider_operation_id;type:text" json:"providerOperationId"`
	LayoutJSON          string `gorm:"column:layout_json;type:text" json:"layoutJson"`
	MetadataJSON        string `gorm:"column:metadata_json;type:text" json:"metadataJson"`
	SourceReferenceAssetID string `gorm:"column:source_reference_asset_id;type:text" json:"sourceReferenceAssetId"`
	SourcePromptHash    string `gorm:"column:source_prompt_hash;type:text" json:"sourcePromptHash"`
	CreatedAt           string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt           string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (GenerationArtifact) TableName() string {
	return "desktop_pet_generation_artifacts"
}

func (a *GenerationArtifact) IsPrimaryArtifact() bool {
	return a.IsPrimary == 1 || a.ArtifactRole == string(ArtifactRolePrimary)
}

func (a *GenerationArtifact) IsImageArtifact() bool {
	switch ArtifactType(a.ArtifactType) {
	case ArtifactTypeSpriteSheetRaw,
		ArtifactTypeKeyframeSheetRaw,
		ArtifactTypeSingleFrameRaw,
		ArtifactTypeLegacyFrameRaw:
		return true
	default:
		return false
	}
}

type ArtifactLocation struct {
	StorageBackend string
	StorageKey     string
	RelativePath   string
	ContentHash    string
}

func (a *GenerationArtifact) Location() ArtifactLocation {
	backend := a.StorageBackend
	if backend == "" {
		backend = "local"
	}
	return ArtifactLocation{
		StorageBackend: backend,
		StorageKey:     a.StorageKey,
		RelativePath:   a.RelativePath,
		ContentHash:    a.Hash,
	}
}

func NewArtifact() *GenerationArtifact {
	return &GenerationArtifact{
		ID:        generateUUID(),
		Status:    string(ArtifactStatusPending),
		CreatedAt: nowRFC3339(),
		UpdatedAt: nowRFC3339(),
	}
}
