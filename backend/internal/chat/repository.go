// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	ListConversations(q ConversationQuery) ([]Conversation, int64, error)
	GetConversation(id string) (*Conversation, error)
	CreateConversation(c *Conversation) error
	UpdateConversation(id string, updates map[string]interface{}) error
	DeleteConversation(id string) error
	DeleteAllConversations() error
	GetMessages(convID string, page, pageSize int) ([]Message, int64, error)
	CreateMessage(m *Message) error
	DeleteMessage(id string) error
	DeleteMessagesByConv(convID string) error
	SearchMessages(q MessageSearchQuery) ([]Message, int64, error)
	GetActiveModel() (*ModelConfig, error)
	GetModelByID(id int) (*ModelConfig, error)
	ListModels() ([]ModelConfig, error)
	CountModels() (int64, error)
	CreateModel(cfg *ModelConfig) error
	UpdateModel(id int, updates map[string]interface{}) error
	DeleteModel(id int) error
	ActivateModel(id int) error
	GetModelRoutes() ([]map[string]interface{}, error)
	UpdateModelRoutes(routes []map[string]interface{}) error
	GetConversationByChannel(channel string) (*Conversation, error)
	CountMessagesByConv(convID string) int64
	ListProviders() []ProviderInfo
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) ListConversations(q ConversationQuery) ([]Conversation, int64, error) {
	query := r.db.Model(&Conversation{})
	if q.Channel != "" {
		query = query.Where("channel = ?", q.Channel)
	}
	if q.Source != "" {
		query = query.Where("source = ?", q.Source)
	}
	if q.CharacterID != "" {
		query = query.Where("character_id = ?", q.CharacterID)
	}
	if q.Keyword != "" {
		query = query.Where("title LIKE ?", "%"+q.Keyword+"%")
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	var convs []Conversation
	err := query.Order("updated_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&convs).Error
	if convs == nil {
		convs = []Conversation{}
	}
	return convs, total, err
}

func (r *repository) GetConversation(id string) (*Conversation, error) {
	var c Conversation
	err := r.db.Where("id = ?", id).First(&c).Error
	return &c, err
}

func (r *repository) CreateConversation(c *Conversation) error {
	c.ID = c.ID
	return r.db.Create(c).Error
}

func (r *repository) UpdateConversation(id string, updates map[string]interface{}) error {
	return r.db.Model(&Conversation{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) DeleteConversation(id string) error {
	r.db.Where("conversation_id = ?", id).Delete(&Message{})
	return r.db.Where("id = ?", id).Delete(&Conversation{}).Error
}

func (r *repository) DeleteAllConversations() error {
	r.db.Where("1=1").Delete(&Message{})
	return r.db.Where("1=1").Delete(&Conversation{}).Error
}

func (r *repository) GetMessages(convID string, page, pageSize int) ([]Message, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	var total int64
	r.db.Model(&Message{}).Where("conversation_id = ?", convID).Count(&total)
	var msgs []Message
	err := r.db.Where("conversation_id = ?", convID).Order("created_at ASC").Limit(pageSize).Offset(offset).Find(&msgs).Error
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, total, err
}

func (r *repository) CreateMessage(m *Message) error {
	return r.db.Create(m).Error
}

func (r *repository) DeleteMessage(id string) error {
	return r.db.Where("id = ?", id).Delete(&Message{}).Error
}

func (r *repository) DeleteMessagesByConv(convID string) error {
	return r.db.Where("conversation_id = ?", convID).Delete(&Message{}).Error
}

func (r *repository) SearchMessages(q MessageSearchQuery) ([]Message, int64, error) {
	query := r.db.Model(&Message{}).Where("content LIKE ?", "%"+q.Keyword+"%")
	if q.ConversationID != "" {
		query = query.Where("conversation_id = ?", q.ConversationID)
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 50 {
		q.PageSize = 50
	}
	var msgs []Message
	err := query.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&msgs).Error
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, total, err
}

func (r *repository) GetActiveModel() (*ModelConfig, error) {
	var cfg ModelConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}

func (r *repository) GetModelByID(id int) (*ModelConfig, error) {
	var cfg ModelConfig
	err := r.db.First(&cfg, id).Error
	return &cfg, err
}

func (r *repository) ListModels() ([]ModelConfig, error) {
	var cfgs []ModelConfig
	err := r.db.Order("id").Find(&cfgs).Error
	if cfgs == nil {
		cfgs = []ModelConfig{}
	}
	return cfgs, err
}

func (r *repository) CountModels() (int64, error) {
	var count int64
	err := r.db.Table("model_configs").Count(&count).Error
	return count, err
}

func (r *repository) CreateModel(cfg *ModelConfig) error {
	return r.db.Create(cfg).Error
}

func (r *repository) UpdateModel(id int, updates map[string]interface{}) error {
	return r.db.Model(&ModelConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) DeleteModel(id int) error {
	return r.db.Delete(&ModelConfig{}, id).Error
}

func (r *repository) ActivateModel(id int) error {
	r.db.Model(&ModelConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&ModelConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetModelRoutes() ([]map[string]interface{}, error) {
	var routes []map[string]interface{}
	r.db.Table("model_scenario_routes").Find(&routes)
	if routes == nil {
		routes = []map[string]interface{}{}
	}
	return routes, nil
}

func (r *repository) UpdateModelRoutes(routes []map[string]interface{}) error {
	r.db.Exec("DELETE FROM model_scenario_routes")
	for _, route := range routes {
		r.db.Exec("INSERT INTO model_scenario_routes (scenario, model_config_id) VALUES (?, ?)",
			route["scenario"], route["modelConfigId"])
	}
	return nil
}

func (r *repository) GetConversationByChannel(channel string) (*Conversation, error) {
	var c Conversation
	err := r.db.Where("channel = ? AND source = ?", channel, "system").First(&c).Error
	return &c, err
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "openai-compatible", Name: "OpenAI Compatible"},
		{ID: "ollama", Name: "Ollama"},
		{ID: "deepseek-compatible", Name: "DeepSeek Compatible"},
		{ID: "qwen-compatible", Name: "Qwen Compatible"},
		{ID: "custom-http", Name: "Custom HTTP"},
	}
}

func (r *repository) CountMessagesByConv(convID string) int64 {
	var count int64
	r.db.Table("messages").Where("conversation_id = ?", convID).Count(&count)
	return count
}
