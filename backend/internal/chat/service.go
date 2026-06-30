// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	applog "github.com/u-ai/backend/log"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qdrant"
	visioncfg "github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

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
	ProcessMessage(req *ProcessMessageRequest) (*ProcessMessageResponse, error)
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
	return &service{repo: repo, db: ctx.DB, memorySvc: memSvc, profileSvc: profSvc, episodicSvc: epiSvc, worldBookSvc: wbSvc, wmCache: NewWorkingMemoryCache(30 * time.Minute), compressor: comp, pipeline: p}
}

func (s *service) ListConversations(q ConversationQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	convs, total, err := s.repo.ListConversations(q)
	if err != nil {
		return nil, err
	}
	for i := range convs {
		convs[i].MessageCount = int(s.repo.CountMessagesByConv(convs[i].ID))
	}
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return &ConversationListResponse{Items: convs, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) GetConversation(id string) (*Conversation, error) {
	c, err := s.repo.GetConversation(id)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	return c, nil
}

func (s *service) CreateConversation(req *CreateConversationRequest) (*Conversation, error) {
	if req.Title == "" {
		req.Title = "New Chat"
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	c := &Conversation{CharacterID: req.CharacterID, Title: req.Title, Channel: req.Channel, Source: req.Source, PeerID: req.PeerID}
	if err := s.repo.CreateConversation(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) EnsureChannelConversation(channel string) (*Conversation, error) {
	title := "微信对话"
	if channel == "qq" {
		title = "QQ对话"
	}
	convID := "channel-" + channel
	c, err := s.repo.GetConversationByChannel(channel)
	if err == nil && c != nil && c.ID != "" {
		if c.ID != convID {
			s.db.Exec("UPDATE conversations SET id = ? WHERE id = ?", convID, c.ID)
			c.ID = convID
		}
		return c, nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	c = &Conversation{
		ID:          convID,
		CharacterID: "",
		Title:       title,
		Channel:     channel,
		Source:      "system",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateConversation(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) RecalculateMessageCounts() (int64, error) {
	result := s.db.Exec("UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE messages.conversation_id = conversations.id)")
	return result.RowsAffected, result.Error
}

func (s *service) DeleteConversation(id string) error {
	return s.repo.DeleteConversation(id)
}

func (s *service) DeleteAllConversations() error {
	return s.repo.DeleteAllConversations()
}

func (s *service) GetMessages(convID string, page, pageSize int) ([]Message, int64, error) {
	return s.repo.GetMessages(convID, page, pageSize)
}

func (s *service) DeleteMessages(convID string) error {
	return s.repo.DeleteMessagesByConv(convID)
}

func (s *service) DeleteSingleMessage(id string) error {
	msgs, _, err := s.repo.GetMessages("", 0, 0)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		if m.ID == id {
			return s.repo.DeleteMessage(id)
		}
	}
	return fmt.Errorf("消息不存在")
}

func (s *service) SearchMessages(q MessageSearchQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	_, total, err := s.repo.SearchMessages(q)
	if err != nil {
		return nil, err
	}
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	items := make([]Conversation, 0)
	return &ConversationListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) ChangeCharacter(convID, charID string) (*Conversation, error) {
	s.db.Exec("UPDATE conversations SET character_id = ?, updated_at = ? WHERE id = ?", charID, time.Now().Format("2006-01-02 15:04:05"), convID)
	return s.repo.GetConversation(convID)
}

func (s *service) GetStats() (*ChatStatsResponse, error) {
	var todayMessages int64
	s.db.Table("messages").Where("date(created_at) = date('now', 'localtime')").Count(&todayMessages)
	var totalConvs int64
	s.db.Table("conversations").Count(&totalConvs)
	return &ChatStatsResponse{TodayMessages: todayMessages, TotalConversations: totalConvs}, nil
}

func (s *service) Chat(req *ChatRequest) (*ChatResponse, error) {
	var charID, charName, identity, systemPrompt string
	if req.CharacterID != "" {
		err := s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Where("id = ?", req.CharacterID).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		if err != nil {
			return nil, fmt.Errorf("角色不存在")
		}
	} else {
		s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Where("is_default = 1").Limit(1).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		if charID == "" {
			s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Limit(1).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		}
		if charID == "" {
			return nil, fmt.Errorf("没有可用角色")
		}
	}
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	systemParts := []string{systemNoEmojiInstruction}
	if identity == "" {
		identity = "一个AI伙伴"
	}
	systemParts = append(systemParts, fmt.Sprintf("你是%s，%s。", charName, identity))
	if systemPrompt != "" {
		systemParts = append(systemParts, systemPrompt)
	}
	apiMessages := []map[string]interface{}{}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": strings.Join(systemParts, "\n\n")})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": systemFormatInstruction})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": req.Message})

	content, tokens, err := s.callLLM(cfg, apiMessages)
	if err != nil {
		return nil, err
	}

	convID := req.ConversationID
	if convID == "" {
		convID = uuid.New().String()
		s.repo.CreateConversation(&Conversation{ID: convID, CharacterID: charID, Title: req.Message, Channel: req.Channel})
	}

	s.repo.CreateMessage(&Message{ID: uuid.New().String(), ConversationID: convID, Role: "user", Content: req.Message})
	aiMsgID := uuid.New().String()
	s.repo.CreateMessage(&Message{ID: aiMsgID, ConversationID: convID, Role: "assistant", Content: content, Tokens: tokens})
	s.db.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), convID, convID)

	return &ChatResponse{ConversationID: convID, Message: &MessageItem{ID: aiMsgID, ConversationID: convID, Role: "assistant", Content: content, Tokens: tokens}}, nil
}

func (s *service) trimContextWindow(convID string) {
	maxRounds := config.AppCfg.Chat.ContextWindowMaxRounds
	if maxRounds <= 0 {
		maxRounds = 20
	}
	var ids []string
	s.db.Table("messages").Select("id").Where("conversation_id = ? AND role IN ('user','assistant') AND include_in_context = 1", convID).Order("created_at DESC").Limit(maxRounds*2+100).Pluck("id", &ids)
	if len(ids) <= maxRounds*2 {
		return
	}
	cutoff := ids[maxRounds*2-1]
	s.db.Exec("UPDATE messages SET include_in_context = 0 WHERE conversation_id = ? AND include_in_context = 1 AND created_at < (SELECT created_at FROM messages WHERE id = ?)", convID, cutoff)
}

func (s *service) loadHistory(convID string) []map[string]string {
	var messages []Message
	s.db.Where("conversation_id = ? AND include_in_context = 1", convID).Order("created_at ASC").Find(&messages)
	if messages == nil {
		messages = []Message{}
	}
	history := make([]map[string]string, len(messages))
	for i, m := range messages {
		history[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	return history
}

func (s *service) ProcessMessage(req *ProcessMessageRequest) (*ProcessMessageResponse, error) {
	fmt.Printf("[DIAG-ProcessMessage] channel=%s voiceMessage=%v msg=%s audioUrl=%s imageUrlLen=%d\n", req.Channel, req.VoiceMessage, req.Message, req.AudioUrl, len(req.ImageUrl))
	var charID, charName, identity, systemPrompt string
	if req.CharacterID != "" {
		err := s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Where("id = ?", req.CharacterID).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		if err != nil {
			return nil, fmt.Errorf("角色不存在")
		}
	} else {
		s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Where("is_default = 1").Limit(1).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		if charID == "" {
			s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Limit(1).Row().Scan(&charID, &charName, &identity, &systemPrompt)
		}
		if charID == "" {
			return nil, fmt.Errorf("没有可用角色")
		}
	}

	channel := req.Channel
	source := req.Source
	if source == "" {
		source = "manual"
	}
	convID := req.ConversationID
	if convID == "" {
		var existing struct{ ID string }
		err := s.db.Table("conversations").Select("id").Where("character_id = ? AND channel = ?", charID, channel).Order("updated_at DESC").Limit(1).Row().Scan(&existing.ID)
		if err == nil && existing.ID != "" {
			convID = existing.ID
		} else {
			convID = uuid.New().String()
			s.repo.CreateConversation(&Conversation{ID: convID, CharacterID: charID, Title: req.Message, Channel: channel})
		}
	}

	userMsgID := uuid.New().String()
	s.repo.CreateMessage(&Message{ID: userMsgID, ConversationID: convID, Role: "user", Content: req.Message, MsgType: "text", Source: source, AudioUrl: req.AudioUrl, AudioDuration: req.AudioDuration, ImageUrl: req.ImageUrl, VideoUrl: req.VideoUrl})

	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		s.db.Exec("DELETE FROM messages WHERE id = ?", userMsgID)
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	sys1Parts := s.sys1Builder(charName, identity, systemPrompt, req.Message)
	history := s.loadHistory(convID)
	sys2Parts := s.sys2Builder(convID, charID, req.Message)

	messages := []map[string]interface{}{}
	if len(sys1Parts) > 0 {
		messages = append(messages, map[string]interface{}{"role": "system", "content": strings.Join(sys1Parts, "\n\n")})
	}
	for _, m := range history {
		messages = append(messages, map[string]interface{}{"role": m["role"], "content": m["content"]})
	}
	if len(sys2Parts) > 0 {
		messages = append(messages, map[string]interface{}{"role": "system", "content": strings.Join(sys2Parts, "\n\n")})
	}
	userContent := req.Message
	if req.ImageContext != "" {
		applog.Info(fmt.Sprintf("[Process] Injecting ImageContext len=%d into prompt", len(req.ImageContext)))
		userContent = req.ImageContext + "\n\n用户问：" + req.Message
	} else {
		applog.Info("[Process] No ImageContext to inject")
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": userContent})
	toolDefs := tool.GetAll()
	var reply string
	seenTools := map[string]bool{}

	tool.SetCurrentConversationID(convID)
	tool.SetCurrentCharacterID(charID)
	for round := 0; round < 3; round++ {
		aiContent, reasoning, toolCalls, _, llmErr := s.callLLMWithTools(cfg, messages, toolDefs)
		applog.Info(fmt.Sprintf("[ToolLoop] round=%d toolCalls=%d aiContentLen=%d", round, len(toolCalls), len(aiContent)))
		if llmErr != nil {
			applog.Warn(fmt.Sprintf("[ToolLoop] LLM error: %v", llmErr))
			s.db.Exec("DELETE FROM messages WHERE id = ?", userMsgID)
			return nil, fmt.Errorf("AI 调用失败: %w", llmErr)
		}
		if len(toolCalls) == 0 {
			reply = aiContent
			break
		}
		assistantToolCall := map[string]interface{}{
			"role":       "assistant",
			"content":    aiContent,
			"tool_calls": toolCalls,
		}
		if reasoning != "" {
			assistantToolCall["reasoning_content"] = reasoning
		}
		messages = append(messages, assistantToolCall)
		for _, tc := range toolCalls {
			name, _ := tc["function"].(map[string]interface{})["name"].(string)
			args, _ := tc["function"].(map[string]interface{})["arguments"].(string)
			if name == "create_schedule" {
				dedupKey := name + "|" + args
				if seenTools[dedupKey] {
					continue
				}
				seenTools[dedupKey] = true
			}
			if name == "create_schedule" {
				var toolArgs map[string]interface{}
				json.Unmarshal([]byte(args), &toolArgs)
				toolArgs["conversation_id"] = convID
				toolArgs["character_id"] = charID
				if channel == "web" {
					toolArgs["channel"] = "all"
				} else if channel != "" {
					toolArgs["channel"] = channel
				}
				newArgs, _ := json.Marshal(toolArgs)
				args = string(newArgs)
			}
			result, ok := tool.Execute(name, args)
			applog.Info(fmt.Sprintf("[ToolLoop] Execute %s ok=%v result=%s", name, ok, result[:min(len(result), 60)]))
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": tc["id"], "content": result})
		}
	}
	if reply == "" {
		applog.Warn("[ToolLoop] reply empty, fallback to text")
		reply = "操作已完成"
	} else {
		applog.Info(fmt.Sprintf("[ToolLoop] final reply len=%d", len(reply)))
	}
	lines_ := strings.Split(strings.TrimSpace(reply), "\n")
	var realLines []string
	for _, line := range lines_ {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		realLines = append(realLines, line)
	}
	if len(realLines) == 0 {
		realLines = []string{reply}
	}
	var msgIDs []string
	var audioUrls []string
	for _, text := range realLines {
		aiMsgID := uuid.New().String()
		s.repo.CreateMessage(&Message{ID: aiMsgID, ConversationID: convID, Role: "assistant", Content: text, MsgType: "text", Source: source})
		msgIDs = append(msgIDs, aiMsgID)
	}
	s.db.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), convID, convID)

	if s.wmCache != nil {
		s.wmCache.UpdateSummary(convID, reply)
	}
	pipelineMessages := make([]map[string]string, 0, len(history)+2)
	pipelineMessages = append(pipelineMessages, history...)
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "user", "content": req.Message})
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "assistant", "content": reply})
	go s.pipeline.Execute(context.Background(), convID, pipelineMessages, reply)
	go s.trimContextWindow(convID)
	go s.moodRecoveryCheck(convID, charID, source)
	if s.compressor != nil {
		go s.compressor.MaybeCompress(convID)
	}

	fv := tool.GetForceVoice()
	fmt.Printf("[DIAG-ProcessMessage] 返回: replyLen=%d forceVoice=%v\n", len(reply), fv)
	return &ProcessMessageResponse{
		ConversationID: convID,
		Reply:          reply,
		CharacterID:    charID,
		CharacterName:  charName,
		MessageIDs:     msgIDs,
		ForceVoice:     fv,
		AudioUrls:      audioUrls,
		UserMessage:    &MessageItem{ID: userMsgID, ConversationID: convID, Role: "user", Content: req.Message, Source: source, CreatedAt: time.Now().Format("2006-01-02 15:04:05")},
		UserMessageID:  userMsgID,
	}, nil
}
func (s *service) sys1Builder(charName, identity, systemPrompt, userMessage string) []string {
	parts := []string{systemNoEmojiInstruction}
	if identity == "" {
		identity = "一个AI伙伴"
	}
	parts = append(parts, fmt.Sprintf("你是%s，%s。", charName, identity))
	if systemPrompt != "" {
		parts = append(parts, systemPrompt)
	}
	if s.profileSvc != nil {
		profilePrompt := s.profileSvc.ToSystemPrompt("default")
		if profilePrompt != "" {
			parts = append(parts, profilePrompt)
		}
	}
	if s.episodicSvc != nil {
		epiPrompt := s.episodicSvc.ToSystemPrompt("default")
		if epiPrompt != "" {
			parts = append(parts, epiPrompt)
		}
	}
	if s.worldBookSvc != nil {
		wbPrompt := s.worldBookSvc.ToSystemPrompt(userMessage, "")
		if wbPrompt != "" {
			parts = append(parts, wbPrompt)
		}
	}
	return parts
}

func (s *service) rewriteQueryForSearch(userMessage string) string {
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return userMessage
	}
	prompt := []map[string]interface{}{
		{"role": "system", "content": "把用户输入转成用于记忆检索的简洁关键词，去除语气词和寒暄，直接输出关键词不要解释。如果输入已经是简短的关键词则原样返回。"},
		{"role": "user", "content": userMessage},
	}
	rewritten, _, err := s.callLLM(cfg, prompt)
	if err != nil || rewritten == "" {
		return userMessage
	}
	rewritten = strings.TrimSpace(rewritten)
	if len([]rune(rewritten)) < 2 {
		return userMessage
	}
	return rewritten
}

func shouldRetrieveMemory(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if len([]rune(trimmed)) < 4 {
		return false
	}
	greetings := []string{"嗯", "好", "哦", "啊", "哈", "嗨", "喂", "在吗", "在不在", "好的", "好吧", "行", "可以", "知道了", "明白了", "懂了", "嗯嗯", "哈哈", "呵呵", "嘿嘿", "谢谢", "多谢", "再见", "拜拜", "晚安", "早安", "早上好", "晚上好", "ok", "OK", "Ok", "hi", "Hi", "hello", "Hello", "bye", "Bye"}
	lower := strings.ToLower(trimmed)
	for _, g := range greetings {
		if lower == strings.ToLower(g) {
			return false
		}
	}
	return true
}

func (s *service) sys2Builder(convID, charID, userMessage string) []string {
	parts := []string{systemFormatInstruction}
	if s.wmCache != nil {
		wm := s.wmCache.Get(convID)
		if wm != nil && wm.State != nil && wm.State.Summary != "" {
			parts = append(parts, "【工作记忆】\n"+wm.State.Summary)
		}
	}
	if s.compressor != nil {
		status := s.compressor.GetCompressionStatus(convID)
		if summary, ok := status["latestSummary"].(string); ok && summary != "" {
			parts = append(parts, "【对话历史摘要】\n"+summary)
		}
	}
	if s.memorySvc != nil && userMessage != "" {
		results, err := s.memorySvc.HybridSearch(&memory.VectorSearchRequest{Query: userMessage, CharacterID: charID, Limit: 8})
		if err == nil && len(results) > 0 {
			layerLines := map[string][]string{}
			layerOrder := []string{"当前摘要", "用户画像", "情景回忆", "事实记忆"}
			for _, r := range results {
				layer := r.MemoryLayer
				if layer == "" {
					layer = "事实记忆"
				}
				typeLabel := r.Memory.MemoryType
				if typeLabel == "" {
					typeLabel = "fact"
				}
				line := fmt.Sprintf("- [%s %s %.0f%% 置信%d%%] %s", typeLabel, r.MatchType, r.Score*100, r.Memory.Confidence, r.Memory.Value)
				layerLines[layer] = append(layerLines[layer], line)
				go s.memorySvc.RecordUse(r.Memory.ID)
			}
			for _, layer := range layerOrder {
				if lines := layerLines[layer]; len(lines) > 0 {
					parts = append(parts, "【"+layer+"】\n"+strings.Join(lines, "\n"))
				}
			}
		}
	}
	return parts
}

func (s *service) detectEpisodicMoment(convID string) {
	if s.episodicSvc == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.episodicSvc.ExtractFromConversation("default", convID, messages)
}
func (s *service) extractProfile(convID string) {
	if s.profileSvc == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.profileSvc.ExtractFromConversation("default", convID, messages)
}
func (s *service) autoExtractMemories(convID, charID string) {
	if s.memorySvc == nil {
		return
	}
	candidates, err := s.memorySvc.GenerateCandidates(convID)
	if err != nil || len(candidates) == 0 {
		return
	}

	existingKeys := map[string]bool{}
	var existingMemories []struct {
		Key   string
		Value string
	}
	s.db.Table("memories").Select("key, value").Find(&existingMemories)
	for _, m := range existingMemories {
		existingKeys[m.Key+"|"+m.Value] = true
	}

	for _, c := range candidates {
		if c.Importance < 7 {
			continue
		}
		if existingKeys[c.Key+"|"+c.Value] {
			continue
		}
		existingKeys[c.Key+"|"+c.Value] = true
		s.memorySvc.AcceptCandidate(c.ID)
	}
}

func (s *service) GetCompressionStatus(convID string) map[string]interface{} {
	if s.compressor == nil {
		return map[string]interface{}{}
	}
	return s.compressor.GetCompressionStatus(convID)
}

func (s *service) GetPipelineStatus() interface{} {
	if s.pipeline == nil {
		return nil
	}
	return s.pipeline.LastRun()
}

func (s *service) callLLM(cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	jsonBody, _ := json.Marshal(reqBody)
	url := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, truncateStr(string(respBytes), 200))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("API 未返回有效回复")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *service) ListModels() ([]ModelConfig, error) {
	return s.repo.ListModels()
}

func (s *service) CreateModel(cfg *ModelConfig) (*ModelConfig, error) {
	count, err := s.repo.CountModels()
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	if count == 0 {
		cfg.IsActive = 1
	}
	if err := s.repo.CreateModel(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return cfg, nil
}

func (s *service) UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error) {
	if err := s.repo.UpdateModel(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.repo.GetModelByID(id)
}

func (s *service) DeleteModel(id int) error {
	return s.repo.DeleteModel(id)
}

func (s *service) ActivateModel(id int) (*ModelConfig, error) {
	if err := s.repo.ActivateModel(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	return s.repo.GetModelByID(id)
}

func (s *service) GetModelRoutes() ([]map[string]interface{}, error) {
	return s.repo.GetModelRoutes()
}

func (s *service) UpdateModelRoutes(routes []map[string]interface{}) error {
	return s.repo.UpdateModelRoutes(routes)
}

type ModelDetectItem struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

func (s *service) DetectModels(baseURL, apiKey string) ([]ModelDetectItem, error) {
	base := strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("GET", base+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
	}
	var r struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		var r2 struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(rb, &r2) == nil {
			items := make([]ModelDetectItem, len(r2.Models))
			for i, m := range r2.Models {
				items[i] = ModelDetectItem{ID: m.Name}
			}
			return items, nil
		}
		return nil, fmt.Errorf("解析响应失败")
	}
	items := make([]ModelDetectItem, len(r.Data))
	for i, m := range r.Data {
		items[i] = ModelDetectItem{ID: m.ID, OwnedBy: m.OwnedBy}
	}
	return items, nil
}

func (s *service) ListProviders() []ProviderInfo {
	return s.repo.ListProviders()
}

func (s *service) callLLMWithTools(cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqMap := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	if len(tools) > 0 {
		reqMap["tools"] = tools
	}
	reqBody, _ := json.Marshal(reqMap)
	req, _ := http.NewRequest("POST", base+"/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncateStr(string(rb), 200))
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			}
		}
		Usage struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &r)
	if len(r.Choices) == 0 {
		return "", "", nil, 0, fmt.Errorf("no choices")
	}
	choice := r.Choices[0]
	var toolCalls []map[string]interface{}
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, map[string]interface{}{
			"id": tc.ID, "type": "function",
			"function": map[string]interface{}{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
		})
	}
	return choice.Message.Content, choice.Message.ReasoningContent, toolCalls, r.Usage.TotalTokens, nil
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func analyzeImageInternal(imageUrl string) (string, string) {
	cfg, err := getVisionModelConfig()
	if err != nil {
		return "", err.Error()
	}
	imageData := imageUrl
	if strings.HasPrefix(imageUrl, "/images/") {
		ext := filepath.Ext(imageUrl)
		mimeType := "image/png"
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".bmp":
			mimeType = "image/bmp"
		}
		filePath := filepath.Join(config.AppCfg.Storage.DataDir, "images", filepath.Base(imageUrl))
		data, err := os.ReadFile(filePath)
		if err == nil {
			imageData = "data:" + mimeType + ";base64," + base64Encode(data)
		}
	}
	content := []map[string]interface{}{
		{"type": "input_image", "image_url": imageData},
		{"type": "input_text", "text": "请详细描述这张图片的内容，包括场景、物体、人物、文字、表情、氛围等所有可见信息，严禁描述不存在于图片中的信息"},
	}
	return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
}

func analyzeVideoInternal(videoUrl string) (string, string) {
	cfg, err := getVisionModelConfig()
	if err != nil {
		return "", err.Error()
	}
	if strings.HasPrefix(videoUrl, "data:video/") {
		content := []map[string]interface{}{
			{"type": "input_video", "video_url": videoUrl},
			{"type": "input_text", "text": "请详细描述这段视频的内容，包括场景、人物动作、事件发展、关键画面等所有可见信息，严禁描述不存在于视频中的信息"},
		}
		return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
	}
	if strings.HasPrefix(videoUrl, "/videos/") {
		filePath := filepath.Join(config.AppCfg.Storage.DataDir, "videos", filepath.Base(videoUrl))
		fileID, err := uploadFileToArk(cfg.BaseUrl, cfg.ApiKey, filePath)
		time.Sleep(5 * time.Second)
		if err != nil {
			return "", fmt.Sprintf("视频上传失败: %s", err.Error())
		}
		content := []map[string]interface{}{
			{"type": "input_video", "file_id": fileID},
			{"type": "input_text", "text": "请详细描述这段视频的内容，包括场景、人物动作、事件发展、关键画面等所有可见信息，严禁描述不存在于视频中的信息"},
		}
		return callDoubaoVision(cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
	}
	return "", fmt.Sprintf("不支持的视频URL格式: %s", videoUrl[:min(len(videoUrl), 100)])
}

func uploadFileToArk(baseURL, apiKey, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	_ = writer.WriteField("purpose", "user_data")

	fileName := filepath.Base(filePath)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	writer.Close()

	req, _ := http.NewRequest("POST", strings.TrimRight(baseURL, "/")+"/files", &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("文件上传失败 (status=%d): %s", resp.StatusCode, string(body[:min(len(body), 300)]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析上传响应失败: %w", err)
	}
	fileID, _ := result["id"].(string)
	if fileID == "" {
		return "", fmt.Errorf("未获取到file_id: %s", string(body[:min(len(body), 300)]))
	}
	return fileID, nil
}

func callDoubaoVision(baseURL, apiKey, modelName string, content []map[string]interface{}) (string, string) {
	reqBody := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{{
			"role":    "user",
			"content": content,
		}},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", strings.TrimRight(baseURL, "/")+"/responses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", string(body)
	}
	rawBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", string(rawBody)
	}
	output, _ := result["output"].([]interface{})
	for _, item := range output {
		m, _ := item.(map[string]interface{})
		if m["type"] == "message" {
			contentArr, _ := m["content"].([]interface{})
			var texts []string
			for _, c := range contentArr {
				cm, _ := c.(map[string]interface{})
				if cm["type"] == "output_text" {
					texts = append(texts, fmt.Sprint(cm["text"]))
				}
			}
			resultText := strings.Join(texts, "")
			return resultText, ""
		}
	}
	return "", string(rawBody)
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func SaveImageFromDataURI(imageUrl string) string {
	if !strings.HasPrefix(imageUrl, "data:") {
		return imageUrl
	}
	imgDir := filepath.Join(config.AppCfg.Storage.DataDir, "images")
	os.MkdirAll(imgDir, 0755)
	idx := strings.Index(imageUrl, ";base64,")
	if idx <= 0 {
		return imageUrl
	}
	mimePart := imageUrl[5:idx]
	ext := ".png"
	if strings.Contains(mimePart, "jpeg") || strings.Contains(mimePart, "jpg") {
		ext = ".jpg"
	}
	fname := uuid.New().String() + ext
	data, err := base64.StdEncoding.DecodeString(imageUrl[idx+8:])
	if err != nil {
		return imageUrl
	}
	os.WriteFile(filepath.Join(imgDir, fname), data, 0644)
	return "/images/" + fname
}
