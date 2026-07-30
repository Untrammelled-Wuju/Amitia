// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
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
