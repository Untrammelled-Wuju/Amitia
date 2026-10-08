// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package vision

import (
	"errors"
	"github.com/u-ai/backend/internal/configwrite"
	"gorm.io/gorm"
)

type Repository interface {
	List() ([]VisionConfig, error)
	GetByID(id int) (*VisionConfig, error)
	Create(cfg *VisionConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*VisionConfig, error)
	ListProviders() []ProviderInfo
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]VisionConfig, error) {
	var configs []VisionConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []VisionConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*VisionConfig, error) {
	var cfg VisionConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *VisionConfig) error {
	return configwrite.Create(r.db, cfg, cfg.IsActive == 1, requireIndependentVisionConfiguration)
}

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return configwrite.Update[VisionConfig](r.db, id, updates, requireIndependentVisionConfiguration)
}

func (r *repository) Delete(id int) error {
	return configwrite.Delete[VisionConfig](r.db, id, requireIndependentVisionConfiguration)
}

func (r *repository) Activate(id int) error {
	return configwrite.Activate[VisionConfig](r.db, id, requireIndependentVisionConfiguration)
}

func requireIndependentVisionConfiguration(tx *gorm.DB) error {
	main, err := (&repository{db: tx}).mainVisionModel()
	if err != nil {
		return err
	}
	if main != nil {
		return errors.New(MainModelVisionNotice)
	}
	return nil
}

func (r *repository) GetActive() (*VisionConfig, error) {
	var cfg VisionConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "volcengine", Name: "火山引擎", DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3", DefaultModel: "doubao-seed-2-0-lite-260428"},
		{ID: "openai", Name: "OpenAI", DefaultBaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o"},
		{ID: "gemini", Name: "Google Gemini", DefaultBaseURL: "https://generativelanguage.googleapis.com", DefaultModel: "gemini-2.0-flash"},
		{ID: "qwen", Name: "通义千问VL", DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-vl-plus"},
		{ID: "zhipu", Name: "智谱GLM-4V", DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "glm-4v-plus"},
		{ID: "siliconflow", Name: "硅基流动", DefaultBaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen2-VL-72B-Instruct"},
	}
}
