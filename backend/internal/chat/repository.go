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
	GetAllMessagesByConv(convID string) ([]Message, error)
	ListProviders() []ProviderInfo
	CreateMessageAttachment(attachment *MessageAttachment) error
	GetMessageAttachments(messageID string) ([]MessageAttachment, error)
	GetAttachmentByID(id string) (*MessageAttachment, error)
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
	err := r.db.Where("conversation_id = ?", convID).Order("sequence ASC").Limit(pageSize).Offset(offset).Find(&msgs).Error
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
	err := r.db.Where("channel = ?", channel).Order("CASE WHEN source = 'system' THEN 0 ELSE 1 END").First(&c).Error
	return &c, err
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "openai", Name: "OpenAI", Protocol: "openai", DefaultProtocol: "openai_responses", SupportedProtocols: []string{"openai_responses", "openai_chat"}, DefaultBaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", DocsURL: "https://platform.openai.com/docs"},
		{ID: "deepseek", Name: "DeepSeek", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.deepseek.com", DefaultModel: "deepseek-chat", DocsURL: "https://platform.deepseek.com/docs"},
		{ID: "qwen", Name: "通义千问 (Qwen)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-plus", DocsURL: "https://help.aliyun.com/zh/dashscope"},
		{ID: "zhipu", Name: "智谱AI (GLM)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4", DefaultModel: "glm-4-plus", DocsURL: "https://open.bigmodel.cn/dev/api"},
		{ID: "moonshot", Name: "月之暗面 (Kimi)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-8k", DocsURL: "https://platform.moonshot.cn/docs"},
		{ID: "baichuan", Name: "百川智能", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.baichuan-ai.com/v1", DefaultModel: "Baichuan4", DocsURL: "https://platform.baichuan-ai.com/docs"},
		{ID: "minimax", Name: "MiniMax", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.minimax.chat/v1", DefaultModel: "abab6.5s-chat", DocsURL: "https://platform.minimaxi.com/document"},
		{ID: "lingyi", Name: "零一万物 (Yi)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.lingyiwanwu.com/v1", DefaultModel: "yi-large", DocsURL: "https://platform.lingyiwanwu.com/docs"},
		{ID: "stepfun", Name: "阶跃星辰 (Step)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.stepfun.com/v1", DefaultModel: "step-2-16k", DocsURL: "https://platform.stepfun.com/docs"},
		{ID: "hunyuan", Name: "腾讯混元", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.hunyuan.cloud.tencent.com/v1", DefaultModel: "hunyuan-turbos-latest", DocsURL: "https://cloud.tencent.com/document/product/1729"},
		{ID: "ernie", Name: "百度文心 (ERNIE)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://qianfan.baidubce.com/v2", DefaultModel: "ernie-4.0-8k-latest", DocsURL: "https://cloud.baidu.com/doc/WENXINWORKSHOP"},
		{ID: "spark", Name: "讯飞星火 (Spark)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://spark-api-open.xf-yun.com/v1", DefaultModel: "generalv3.5", DocsURL: "https://www.xfyun.cn/doc/spark"},
		{ID: "siliconflow", Name: "硅基流动 (SiliconFlow)", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen2.5-72B-Instruct", DocsURL: "https://docs.siliconflow.cn"},
		{ID: "openrouter", Name: "OpenRouter", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://openrouter.ai/api/v1", DefaultModel: "openai/gpt-4o", DocsURL: "https://openrouter.ai/docs"},
		{ID: "groq", Name: "Groq", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.groq.com/openai/v1", DefaultModel: "llama-3.3-70b-versatile", DocsURL: "https://console.groq.com/docs"},
		{ID: "together", Name: "Together AI", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat"}, DefaultBaseURL: "https://api.together.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct-Turbo", DocsURL: "https://docs.together.ai"},
		{ID: "ollama", Name: "Ollama (本地)", Protocol: "ollama", DefaultProtocol: "ollama_chat", SupportedProtocols: []string{"ollama_chat"}, DefaultBaseURL: "http://127.0.0.1:11434", DefaultModel: "llama3.1", DocsURL: "https://ollama.com/library"},
		{ID: "mnn", Name: "MNN (本地)", Protocol: "mnn", DefaultProtocol: "mnn", SupportedProtocols: []string{"mnn"}, DefaultBaseURL: "", DefaultModel: "", DocsURL: "https://github.com/alibaba/MNN"},
		{ID: "llama_cpp", Name: "llama.cpp (本地 GGUF)", Protocol: "llama_cpp", DefaultProtocol: "llama_cpp", SupportedProtocols: []string{"llama_cpp"}, DefaultBaseURL: "", DefaultModel: "", DocsURL: "https://github.com/ggerganov/llama.cpp"},
		{ID: "anthropic", Name: "Anthropic (Claude)", Protocol: "anthropic", DefaultProtocol: "anthropic_messages", SupportedProtocols: []string{"anthropic_messages"}, DefaultBaseURL: "https://api.anthropic.com", DefaultModel: "claude-sonnet-4-20250514", DocsURL: "https://docs.anthropic.com"},
		{ID: "gemini", Name: "Google Gemini", Protocol: "gemini", DefaultProtocol: "gemini_generate_content", SupportedProtocols: []string{"gemini_generate_content"}, DefaultBaseURL: "https://generativelanguage.googleapis.com", DefaultModel: "gemini-2.0-flash", DocsURL: "https://ai.google.dev/gemini-api/docs"},
		{ID: "custom-http", Name: "自定义 HTTP", Protocol: "openai", DefaultProtocol: "openai_chat", SupportedProtocols: []string{"openai_chat", "openai_responses", "anthropic_messages", "gemini_generate_content", "ollama_chat"}, DefaultBaseURL: "", DefaultModel: "", DocsURL: ""},
	}
}

func (r *repository) CountMessagesByConv(convID string) int64 {
	var count int64
	r.db.Table("messages").Where("conversation_id = ?", convID).Count(&count)
	return count
}

func (r *repository) GetAllMessagesByConv(convID string) ([]Message, error) {
	var msgs []Message
	err := r.db.Where("conversation_id = ?", convID).Order("sequence ASC").Find(&msgs).Error
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, err
}

func (r *repository) CreateMessageAttachment(attachment *MessageAttachment) error {
	return r.db.Create(attachment).Error
}

func (r *repository) GetMessageAttachments(messageID string) ([]MessageAttachment, error) {
	var attachments []MessageAttachment
	err := r.db.Where("message_id = ?", messageID).Order("sequence ASC").Find(&attachments).Error
	if attachments == nil {
		attachments = []MessageAttachment{}
	}
	return attachments, err
}

func (r *repository) GetAttachmentByID(id string) (*MessageAttachment, error) {
	var attachment MessageAttachment
	err := r.db.Where("id = ?", id).First(&attachment).Error
	return &attachment, err
}
