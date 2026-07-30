// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imagegen

import "gorm.io/gorm"

type Repository interface {
	List() ([]ImageGenConfig, error)
	GetByID(id int) (*ImageGenConfig, error)
	Create(cfg *ImageGenConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*ImageGenConfig, error)
	ListProviders() []ProviderInfo
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]ImageGenConfig, error) {
	var configs []ImageGenConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []ImageGenConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*ImageGenConfig, error) {
	var cfg ImageGenConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *ImageGenConfig) error { return r.db.Create(cfg).Error }

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&ImageGenConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&ImageGenConfig{}).Error
}

func (r *repository) Activate(id int) error {
	r.db.Model(&ImageGenConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&ImageGenConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetActive() (*ImageGenConfig, error) {
	var cfg ImageGenConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "seedream", Name: "火山引擎 Seedream", DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3", DefaultModel: "doubao-seedream-5-0"},
		{ID: "openai", Name: "OpenAI DALL-E", DefaultBaseURL: "https://api.openai.com/v1", DefaultModel: "dall-e-3"},
		{ID: "stability", Name: "Stability AI", DefaultBaseURL: "https://api.stability.ai/v2beta", DefaultModel: "stable-image-core"},
		{ID: "tongyi", Name: "阿里通义万相", DefaultBaseURL: "https://dashscope.aliyuncs.com/api/v1", DefaultModel: "wanx2.1-t2i-turbo"},
		{ID: "cogview", Name: "智谱 CogView", DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "cogview-3-plus"},
		{ID: "siliconflow", Name: "硅基流动", DefaultBaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Kwai-Kolors/Kolors"},
	}
}
