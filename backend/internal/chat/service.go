// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qdrant"
	visioncfg "github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

var ErrConversationScopeMismatch = errors.New("conversation scope mismatch")

type Service interface {
	ListConversations(q ConversationQuery) (*ConversationListResponse, error)
	GetConversation(id string) (*Conversation, error)
	CreateConversation(req *CreateConversationRequest) (*Conversation, error)
	DeleteConversation(id string) error
	DeleteAllConversations() error
	GetMessages(convID string, page, pageSize int) ([]Message, int64, error)
	DeleteMessages(convID string) error
	DeleteSingleMessage(id string) error
	SearchMessages(q MessageSearchQuery) (*ConversationListResponse, error)
	ChangeCharacter(convID, charID string) (*Conversation, error)
	GetStats() (*ChatStatsResponse, error)
	Chat(req *ChatRequest) (*ChatResponse, error)
	ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (*ProcessMessageResponse, error)
	ListModels() ([]ModelConfig, error)
	CreateModel(cfg *ModelConfig) (*ModelConfig, error)
	UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error)
	DeleteModel(id int) error
	ActivateModel(id int) (*ModelConfig, error)
	GetModelRoutes() ([]map[string]interface{}, error)
	UpdateModelRoutes(routes []map[string]interface{}) error
	DetectModels(baseURL, apiKey string) ([]ModelDetectItem, error)
	EnsureChannelConversation(channel string) (*Conversation, error)
	RecalculateMessageCounts() (int64, error)
	GetCompressionStatus(convID string) map[string]interface{}
	GetPipelineStatus() interface{}
	ListProviders() []ProviderInfo
}

// systemFormatInstruction is injected into every LLM call for WeChat-style line splitting.
const systemFormatInstruction = `【回复格式 - 系统固定规则】

每句话必须单独一行，用换行符分隔。
每句话尽量短，像微信连续消息一样。
能一句说完就一句，不要写长段落。
不要把多句话连成一段。
不要用句号连接多个意思。

【工具使用规则 - 严格遵守】
create_schedule 仅在用户明确要求"提醒"、"叫"、"通知"、"叫醒"、"定时"等场景时调用。
禁止在用户只问时间、闲聊、打招呼、问天气等日常对话中调用 create_schedule。
get_current_time 仅在用户明确询问当前时间时调用。
不要在用户没有明确要求的情况下自动创建任何提醒。
force_voice_reply 仅在用户明确要求"用语音回复"、"发语音"、"语音回答"、"说语音"、"讲语音"时调用。调用后本次回复会以语音形式发送。`

const systemNoEmojiInstruction = "【系统指令】回复中不要使用任何emoji表情符号。"

const WechatStylePrompt = "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。\\n" +
	"回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。\\n" +
	"用户随口聊，你就自然接话；用户认真问问题，你再认真回答。\\n" +
	"不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。\\n" +
	"回复格式要像微信连续消息：\\n" +
	"用户发一句话时，你可以回复 1 到 4 句短句。\\n" +

	"不要写成一整段长文。\\n" +
	"整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。\\n" +
	"回复中不要使用任何emoji表情符号。\\n" +
	"不能使用markdown格式。"

type service struct {
	repo         Repository
	charRepo     character.Repository
	db           *gorm.DB
	memorySvc    memory.Service
	profileSvc   profile.Service
	episodicSvc  episodic.Service
	worldBookSvc worldbook.Service
	wmCache      *WorkingMemoryCache
	compressor   *Compressor
	pipeline     *memory.Pipeline
}

var visionModelConfigProviderMu sync.RWMutex
var visionModelConfigProvider func() (*visioncfg.VisionConfig, error)

type ModelDetectItem struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

var _ interaction.MessageProcessor = (*service)(nil)

func SetVisionModelConfigProvider(provider func() (*visioncfg.VisionConfig, error)) {
	visionModelConfigProviderMu.Lock()
	visionModelConfigProvider = provider
	visionModelConfigProviderMu.Unlock()
}

func getVisionModelConfig() (*visioncfg.VisionConfig, error) {
	visionModelConfigProviderMu.RLock()
	provider := visionModelConfigProvider
	visionModelConfigProviderMu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("未配置可用的模型来源")
	}
	cfg, err := provider()
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.ApiKey == "" || cfg.BaseUrl == "" || cfg.ModelName == "" {
		return nil, fmt.Errorf("未找到可用的模型配置")
	}
	return cfg, nil
}

func NewService(repo Repository, ctx *app.AppContext, memSvc memory.Service, profSvc profile.Service, epiSvc episodic.Service, wbSvc worldbook.Service, comp *Compressor, visionSvc visioncfg.Service, graphSvc graph.Service) Service {
	if visionSvc != nil {
		SetVisionModelConfigProvider(visionSvc.GetActive)
	}
	graphLayer := graphSvc
	if graphLayer == nil {
		graphLayer = graph.NewStubService()
	}
	p := memory.NewPipeline(
		memory.NewWorkingMemoryService(),
		profSvc.(memory.PipelineLayer),
		epiSvc.(memory.PipelineLayer),
		memSvc.(memory.PipelineLayer),
		qdrant.NewQdrantClient(),
		graphLayer,
	)
	return &service{repo: repo, charRepo: character.NewRepository(ctx), db: ctx.DB, memorySvc: memSvc, profileSvc: profSvc, episodicSvc: epiSvc, worldBookSvc: wbSvc, wmCache: NewWorkingMemoryCache(30 * time.Minute), compressor: comp, pipeline: p}
}
