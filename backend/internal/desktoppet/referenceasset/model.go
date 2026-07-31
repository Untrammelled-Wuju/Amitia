package referenceasset

type ReferenceAsset struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskID           string `gorm:"column:task_id;type:text" json:"taskId"`
	SourcePath       string `gorm:"column:source_path;type:text" json:"sourcePath"`
	SourceHash       string `gorm:"column:source_hash;type:text" json:"sourceHash"`
	SourceMIME       string `gorm:"column:source_mime;type:text" json:"sourceMime"`
	SourceWidth      int    `gorm:"column:source_width;type:integer" json:"sourceWidth"`
	SourceHeight     int    `gorm:"column:source_height;type:integer" json:"sourceHeight"`
	NormalizedPath   string `gorm:"column:normalized_path;type:text" json:"normalizedPath"`
	NormalizedHash   string `gorm:"column:normalized_hash;type:text" json:"normalizedHash"`
	NormalizedMIME   string `gorm:"column:normalized_mime;type:text" json:"normalizedMime"`
	NormalizedWidth  int    `gorm:"column:normalized_width;type:integer" json:"normalizedWidth"`
	NormalizedHeight int    `gorm:"column:normalized_height;type:integer" json:"normalizedHeight"`
	ConfigHash       string `gorm:"column:config_hash;type:text" json:"configHash"`
	ContentHash      string `gorm:"column:content_hash;type:text" json:"contentHash"`
	NormalizerVersion string `gorm:"column:normalizer_version;type:text" json:"normalizerVersion"`
	SubjectBox       string `gorm:"column:subject_box;type:text" json:"subjectBox"`
	Anchor           string `gorm:"column:anchor;type:text" json:"anchor"`
	CoordinateSpace  string `gorm:"column:coordinate_space;type:text" json:"coordinateSpace"`
	CharacterID      string `gorm:"column:character_id;type:text" json:"characterId"`
	UserID           string `gorm:"column:user_id;type:text" json:"userId"`
	SourceArtifactID string `gorm:"column:source_artifact_id;type:text" json:"sourceArtifactId"`
	StoragePath      string `gorm:"column:storage_path;type:text" json:"storagePath"`
	CreatedAt        string `gorm:"column:created_at;type:text" json:"createdAt"`
}

func (ReferenceAsset) TableName() string { return "desktop_pet_reference_assets" }

type NormalizeConfig struct {
	TargetWidth     int
	TargetHeight    int
	TargetMIME      string
	MaxBytes        int64
	BackgroundColor string
}

type Rect struct {
	MinX int `json:"minX"`
	MinY int `json:"minY"`
	MaxX int `json:"maxX"`
	MaxY int `json:"maxY"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
