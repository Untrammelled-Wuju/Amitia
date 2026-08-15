package artifact

import (
	"time"
)

type ID string

type BlobDigest string

type Kind string

const (
	KindImage      Kind = "image"
	KindAudio      Kind = "audio"
	KindVideo      Kind = "video"
	KindFile       Kind = "file"
	KindDocument   Kind = "document"
	KindArchive    Kind = "archive"
	KindGenerated  Kind = "generated"
	KindToolOutput Kind = "tool_output"
)

type Status string

const (
	StatusReady       Status = "ready"
	StatusDeleted     Status = "deleted"
	StatusQuarantined Status = "quarantined"
)

type Source string

const (
	SourceUserUpload Source = "user_upload"
	SourceChatUpload Source = "chat_upload"
	SourceToolOutput Source = "tool_output"
	SourceGenerated  Source = "generated"
	SourceImport     Source = "import"
	SourceExtension  Source = "extension"
)

type Artifact struct {
	ID          ID         `gorm:"column:artifact_id;primaryKey" json:"artifactId"`
	OwnerUserID string     `gorm:"column:owner_user_id;not null;index" json:"ownerUserId"`
	WorkspaceID string     `gorm:"column:workspace_id;not null;default:''" json:"workspaceId"`
	Kind        Kind       `gorm:"column:kind;not null" json:"kind"`
	BlobDigest  BlobDigest `gorm:"column:blob_digest;not null;index" json:"blobDigest"`
	SizeBytes   int64      `gorm:"column:size_bytes;not null" json:"sizeBytes"`
	MIMEType    string     `gorm:"column:mime_type;not null" json:"mimeType"`
	Filename    string     `gorm:"column:filename;not null;default:''" json:"filename"`
	Extension   string     `gorm:"column:file_extension;not null;default:''" json:"fileExtension"`
	Status      Status     `gorm:"column:status;not null;index" json:"status"`
	Source      Source     `gorm:"column:source;not null" json:"source"`
	Width       int        `gorm:"column:width;not null;default:0" json:"width"`
	Height      int        `gorm:"column:height;not null;default:0" json:"height"`
	DurationMS  int64      `gorm:"column:duration_ms;not null;default:0" json:"durationMs"`
	Revision    int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;index" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt   *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (Artifact) TableName() string { return "artifacts" }

type ArtifactReference struct {
	ArtifactID    ID        `gorm:"column:artifact_id;not null"`
	ReferenceType string    `gorm:"column:reference_type;not null"`
	ReferenceID   string    `gorm:"column:reference_id;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
}

func (ArtifactReference) TableName() string { return "artifact_references" }
