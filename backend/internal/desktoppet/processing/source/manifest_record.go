package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type ProcessingSourceManifestRecord struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	SchemaVersion         int    `gorm:"column:schema_version" json:"schemaVersion"`
	ProcessingTaskID      string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ProcessingActionID    string `gorm:"column:processing_action_id" json:"processingActionId"`
	GenerationTaskID      string `gorm:"column:generation_task_id" json:"generationTaskId"`
	GenerationActionID    string `gorm:"column:generation_action_id" json:"generationActionId"`
	ActionKey             string `gorm:"column:action_key" json:"actionKey"`
	GenerationMode        string `gorm:"column:generation_mode" json:"generationMode"`
	GenerationAttemptID   string `gorm:"column:generation_attempt_id" json:"generationAttemptId"`
	SourceArtifactID      string `gorm:"column:source_artifact_id" json:"sourceArtifactId"`
	ArtifactRole          string `gorm:"column:artifact_role" json:"artifactRole"`
	ArtifactKind          string `gorm:"column:artifact_kind" json:"artifactKind"`
	ArtifactContentHash   string `gorm:"column:artifact_content_hash" json:"artifactContentHash"`
	ArtifactStorageKey    string `gorm:"column:artifact_storage_key" json:"artifactStorageKey"`
	ArtifactRelativePath  string `gorm:"column:artifact_relative_path" json:"artifactRelativePath"`
	ArtifactWidth         int    `gorm:"column:artifact_width" json:"artifactWidth"`
	ArtifactHeight        int    `gorm:"column:artifact_height" json:"artifactHeight"`
	ArtifactMimeType      string `gorm:"column:artifact_mime_type" json:"artifactMimeType"`
	CandidateIndex        int    `gorm:"column:candidate_index" json:"candidateIndex"`
	ReferenceAssetID      string `gorm:"column:reference_asset_id" json:"referenceAssetId"`
	PromptDocumentID      string `gorm:"column:prompt_document_id" json:"promptDocumentId"`
	PromptContentHash     string `gorm:"column:prompt_content_hash" json:"promptContentHash"`
	ExpectedFrameCount    int    `gorm:"column:expected_frame_count" json:"expectedFrameCount"`
	SpriteSheetLayoutJSON string `gorm:"column:sprite_sheet_layout_json" json:"spriteSheetLayoutJson"`
	KeyframesJSON         string `gorm:"column:keyframes_json" json:"keyframesJson"`
	LegacyFramesJSON      string `gorm:"column:legacy_frames_json" json:"legacyFramesJson"`
	FramesJSON            string `gorm:"column:frames_json" json:"framesJson"`
	ActionSpecSnapshotJSON string `gorm:"column:action_spec_snapshot_json" json:"actionSpecSnapshotJson"`
	SourceConfigHash      string `gorm:"column:source_config_hash" json:"sourceConfigHash"`
	ManifestHash          string `gorm:"column:manifest_hash" json:"manifestHash"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
}

func (ProcessingSourceManifestRecord) TableName() string {
	return "desktop_pet_processing_source_manifests"
}

func (r *ProcessingSourceManifestRecord) ToDescriptor() (*ProcessingSourceDescriptor, error) {
	if r == nil {
		return nil, fmt.Errorf("source: manifest record is nil")
	}

	var layout *SpriteSheetLayoutSnapshot
	if r.SpriteSheetLayoutJSON != "" && r.SpriteSheetLayoutJSON != "{}" {
		layout = &SpriteSheetLayoutSnapshot{}
		if err := json.Unmarshal([]byte(r.SpriteSheetLayoutJSON), layout); err != nil {
			return nil, fmt.Errorf("source: unmarshal layout: %w", err)
		}
		if layout.Rows <= 0 || layout.Columns <= 0 {
			layout = nil
		}
	}

	var frames []ProcessingSourceFrame
	if r.FramesJSON != "" && r.FramesJSON != "[]" {
		if err := json.Unmarshal([]byte(r.FramesJSON), &frames); err != nil {
			return nil, fmt.Errorf("source: unmarshal frames: %w", err)
		}
	}
	if frames == nil {
		frames = []ProcessingSourceFrame{}
	}

	kind := SourceKind(r.GenerationMode)
	if kind == "" {
		kind = SourceLegacyFrame
	}

	descriptor := &ProcessingSourceDescriptor{
		SourceKind:       kind,
		ActionKey:        r.ActionKey,
		GenerationMode:   r.GenerationMode,
		SourceAttemptID:  r.GenerationAttemptID,
		CandidateIndex:   r.CandidateIndex,
		SourceConfigHash: r.SourceConfigHash,
		Artifact: GenerationArtifactDescriptor{
			ArtifactID:   r.SourceArtifactID,
			AttemptID:    r.GenerationAttemptID,
			RelativePath: r.ArtifactRelativePath,
			ContentHash:  r.ArtifactContentHash,
			Width:        r.ArtifactWidth,
			Height:       r.ArtifactHeight,
			MIMEType:     r.ArtifactMimeType,
			Primary:      true,
			Layout:       layout,
		},
		Frames: frames,
	}

	return descriptor, nil
}

func (r *ProcessingSourceManifestRecord) ComputeManifestHash() string {
	h := sha256.New()
	h.Write([]byte(r.ProcessingActionID))
	h.Write([]byte(r.GenerationAttemptID))
	h.Write([]byte(r.SourceArtifactID))
	h.Write([]byte(r.ArtifactContentHash))
	h.Write([]byte(r.GenerationMode))
	h.Write([]byte(r.SourceConfigHash))
	h.Write([]byte(r.FramesJSON))
	h.Write([]byte(r.ActionSpecSnapshotJSON))
	return hex.EncodeToString(h.Sum(nil))
}

func (r *ProcessingSourceManifestRecord) VerifyHash() bool {
	return r.ManifestHash != "" && r.ManifestHash == r.ComputeManifestHash()
}

type ActionSpecSnapshot struct {
	PlaybackMode  string `json:"playbackMode"`
	FPS           int    `json:"fps"`
	ReturnTo      string `json:"returnTo"`
	Interruptible bool   `json:"interruptible"`
	Anchor        string `json:"anchor"`
}

type ManifestStore interface {
	Create(ctx context.Context, record *ProcessingSourceManifestRecord) error
	GetByProcessingAction(ctx context.Context, processingActionID string) (*ProcessingSourceManifestRecord, error)
	GetByID(ctx context.Context, id string) (*ProcessingSourceManifestRecord, error)
	ListByProcessingTask(ctx context.Context, processingTaskID string) ([]ProcessingSourceManifestRecord, error)
}

type ManifestBuilder struct {
	now func() string
}

func NewManifestBuilder() *ManifestBuilder {
	return &ManifestBuilder{now: func() string { return time.Now().UTC().Format("2006-01-02 15:04:05") }}
}

func (b *ManifestBuilder) Build(req BuildManifestRequest) (*ProcessingSourceManifestRecord, error) {
	descriptor := req.Descriptor
	if descriptor == nil {
		return nil, fmt.Errorf("source: descriptor is nil")
	}

	layoutJSON := "{}"
	if descriptor.Artifact.Layout != nil {
		data, err := json.Marshal(descriptor.Artifact.Layout)
		if err != nil {
			return nil, fmt.Errorf("source: marshal layout: %w", err)
		}
		layoutJSON = string(data)
	}

	framesJSON := "[]"
	if len(descriptor.Frames) > 0 {
		data, err := json.Marshal(descriptor.Frames)
		if err != nil {
			return nil, fmt.Errorf("source: marshal frames: %w", err)
		}
		framesJSON = string(data)
	}

	actionSpecJSON := "{}"
	if req.ActionSpecSnapshot != nil {
		data, err := json.Marshal(req.ActionSpecSnapshot)
		if err != nil {
			return nil, fmt.Errorf("source: marshal action spec: %w", err)
		}
		actionSpecJSON = string(data)
	}

	record := &ProcessingSourceManifestRecord{
		ID:                     req.ID,
		SchemaVersion:          1,
		ProcessingTaskID:       req.ProcessingTaskID,
		ProcessingActionID:     req.ProcessingActionID,
		GenerationTaskID:       req.GenerationTaskID,
		GenerationActionID:     req.GenerationActionID,
		ActionKey:              descriptor.ActionKey,
		GenerationMode:         descriptor.GenerationMode,
		GenerationAttemptID:    descriptor.SourceAttemptID,
		SourceArtifactID:       descriptor.Artifact.ArtifactID,
		ArtifactRole:           "primary",
		ArtifactKind:           string(descriptor.SourceKind),
		ArtifactContentHash:    descriptor.Artifact.ContentHash,
		ArtifactStorageKey:     descriptor.Artifact.RelativePath,
		ArtifactRelativePath:   descriptor.Artifact.RelativePath,
		ArtifactWidth:          descriptor.Artifact.Width,
		ArtifactHeight:         descriptor.Artifact.Height,
		ArtifactMimeType:       descriptor.Artifact.MIMEType,
		CandidateIndex:         descriptor.CandidateIndex,
		ReferenceAssetID:       req.ReferenceAssetID,
		PromptDocumentID:       req.PromptDocumentID,
		PromptContentHash:      req.PromptContentHash,
		ExpectedFrameCount:     len(descriptor.Frames),
		SpriteSheetLayoutJSON:  layoutJSON,
		KeyframesJSON:           "[]",
		LegacyFramesJSON:        "[]",
		FramesJSON:             framesJSON,
		ActionSpecSnapshotJSON: actionSpecJSON,
		SourceConfigHash:       descriptor.SourceConfigHash,
		CreatedAt:              b.now(),
	}
	record.ManifestHash = record.ComputeManifestHash()

	return record, nil
}

type BuildManifestRequest struct {
	ID                  string
	ProcessingTaskID    string
	ProcessingActionID  string
	GenerationTaskID    string
	GenerationActionID  string
	Descriptor          *ProcessingSourceDescriptor
	ReferenceAssetID    string
	PromptDocumentID    string
	PromptContentHash   string
	ActionSpecSnapshot  *ActionSpecSnapshot
}
