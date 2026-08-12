// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding_config

import "gorm.io/gorm"

type Repository interface {
	List() ([]EmbeddingConfig, error)
	GetByID(id int) (*EmbeddingConfig, error)
	Create(cfg *EmbeddingConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*EmbeddingConfig, error)
	ListProviders() []ProviderInfo
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]EmbeddingConfig, error) {
	var configs []EmbeddingConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []EmbeddingConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*EmbeddingConfig, error) {
	var cfg EmbeddingConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *EmbeddingConfig) error { return r.db.Create(cfg).Error }

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&EmbeddingConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&EmbeddingConfig{}).Error
}

func (r *repository) Activate(id int) error {
	r.db.Model(&EmbeddingConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&EmbeddingConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetActive() (*EmbeddingConfig, error) {
	var cfg EmbeddingConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "volcengine", Name: "火山引擎", DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3", DefaultModel: "doubao-embedding-vision-251215"},
		{ID: "openai", Name: "OpenAI", DefaultBaseURL: "https://api.openai.com/v1", DefaultModel: "text-embedding-3-large"},
		{ID: "qwen", Name: "通义千问", DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "text-embedding-v3"},
		{ID: "zhipu", Name: "智谱", DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "embedding-3"},
		{ID: "siliconflow", Name: "硅基流动", DefaultBaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "BAAI/bge-large-zh-v1.5"},
		{ID: "jina", Name: "Jina AI", DefaultBaseURL: "https://api.jina.ai/v1", DefaultModel: "jina-embeddings-v3"},
		{ID: "llama_cpp", Name: "llama.cpp (本地 GGUF)", DefaultBaseURL: "", DefaultModel: ""},
	}
}
