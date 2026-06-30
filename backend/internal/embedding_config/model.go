// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding_config

type EmbeddingConfig struct {
	ID        int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string `gorm:"column:name;not null" json:"name"`
	ApiKey    string `gorm:"column:api_key" json:"apiKey"`
	ModelName string `gorm:"column:model_name;default:doubao-embedding-vision-251215" json:"modelName"`
	BaseUrl   string `gorm:"column:base_url;default:https://ark.cn-beijing.volces.com/api/v3" json:"baseUrl"`
	IsActive  int    `gorm:"column:is_active;default:0" json:"isActive"`
	HasApiKey bool   `gorm:"-" json:"hasApiKey"`
	CreatedAt string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt string `gorm:"column:updated_at" json:"updatedAt"`
}

func (EmbeddingConfig) TableName() string { return "embedding_configs" }

type CreateEmbeddingConfigRequest struct {
	Name      string `json:"name"`
	ApiKey    string `json:"apiKey"`
	ModelName string `json:"modelName"`
	BaseUrl   string `json:"baseUrl"`
	IsActive  int    `json:"isActive"`
}
