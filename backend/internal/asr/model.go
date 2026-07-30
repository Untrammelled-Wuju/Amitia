// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

type AsrConfig struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name       string `gorm:"column:name;not null" json:"name"`
	ApiType    string `gorm:"column:api_type;default:volcengine" json:"apiType"`
	ApiKey     string `gorm:"column:api_key" json:"apiKey"`
	BaseURL    string `gorm:"column:base_url" json:"baseUrl"`
	ResourceId string `gorm:"column:resource_id;default:volc.seedasr.auc" json:"resourceId"`
	IsActive   int    `gorm:"column:is_active;default:0" json:"isActive"`
	HasApiKey  bool   `gorm:"-" json:"hasApiKey"`
	CreatedAt  string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  string `gorm:"column:updated_at" json:"updatedAt"`
}

func (AsrConfig) TableName() string { return "asr_configs" }

type CreateAsrConfigRequest struct {
	Name       string `json:"name"`
	ApiType    string `json:"apiType"`
	ApiKey     string `json:"apiKey"`
	BaseURL    string `json:"baseUrl"`
	ResourceId string `json:"resourceId"`
	IsActive   int    `json:"isActive"`
}

type ProviderInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DefaultBaseURL string `json:"defaultBaseUrl"`
	DefaultModel   string `json:"defaultModel"`
}
