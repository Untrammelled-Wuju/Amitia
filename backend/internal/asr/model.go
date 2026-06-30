// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

type AsrConfig struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name       string `gorm:"column:name;not null" json:"name"`
	ApiKey     string `gorm:"column:api_key" json:"apiKey"`
	ResourceId string `gorm:"column:resource_id;default:volc.seedasr.auc" json:"resourceId"`
	IsActive   int    `gorm:"column:is_active;default:0" json:"isActive"`
	HasApiKey  bool   `gorm:"-" json:"hasApiKey"`
	CreatedAt  string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  string `gorm:"column:updated_at" json:"updatedAt"`
}

func (AsrConfig) TableName() string { return "asr_configs" }

type CreateAsrConfigRequest struct {
	Name       string `json:"name"`
	ApiKey     string `json:"apiKey"`
	ResourceId string `json:"resourceId"`
	IsActive   int    `json:"isActive"`
}
