package chat

import (
	"bytes"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"net/http"
	"strings"
	"time"

		"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/config"
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

const systemNoEmojiInstruction = "【系统规则】回复中不要使用任何emoji表情符号。"

type service struct {
	repo     Repository
	db       *gorm.DB
	memorySvc memory.Service
}

func NewService(repo Repository, ctx *app.AppContext, memSvc memory.Service) Service {
	return &service{repo: repo, db: ctx.DB, memorySvc: memSvc}
}

func (s *service) ListConversations(q ConversationQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 { q.Page = 1 }
	if q.PageSize <= 0 { q.PageSize = 20 }
	convs, total, err := s.repo.ListConversations(q)
	if err != nil { return nil, err }
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return &ConversationListResponse{Items: convs, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) GetConversation(id string) (*Conversation, error) {
	c, err := s.repo.GetConversation(id)
	if err != nil { return nil, fmt.Errorf("对话不存在") }
	return c, nil
}

func (s *service) CreateConversation(req *CreateConversationRequest) (*Conversation, error) {
	if req.Title == "" { req.Title = "New Chat" }
	if req.Channel == "" { req.Channel = "web" }
	if req.Source == "" { req.Source = "manual" }
	c := &Conversation{CharacterID: req.CharacterID, Title: req.Title, Channel: req.Channel, Source: req.Source, PeerID: req.PeerID}
	if err := s.repo.CreateConversation(c); err != nil { return nil, err }
	return c, nil
}

func (s *service) EnsureChannelConversation(channel string) (*Conversation, error) {
	title := "微信对话"
	if channel == "qq" {
		title = "QQ对话"
	}
	c, err := s.repo.GetConversationByChannel(channel)
	if err == nil && c != nil && c.ID != "" {
		return c, nil
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	c = &Conversation{
		ID:          uuid.New().String(),
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
	if err := s.repo.DeleteMessagesByConv(convID); err != nil { return err }
	s.repo.UpdateConversation(convID, map[string]interface{}{"message_count": 0})
	return nil
}

func (s *service) DeleteSingleMessage(id string) error {
	var msg Message
	if err := s.db.Where("id = ?", id).First(&msg).Error; err != nil { return fmt.Errorf("消息不存在") }
	if err := s.repo.DeleteMessage(id); err != nil { return err }
	s.db.Exec("UPDATE conversations SET message_count = MAX(0, message_count - 1), updated_at = ? WHERE id = ?", time.Now(), msg.ConversationID)
	return nil
}

func (s *service) SearchMessages(q MessageSearchQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 { q.Page = 1 }
	if q.PageSize <= 0 { q.PageSize = 20 }
	_, total, err := s.repo.SearchMessages(q)
	if err != nil { return nil, err }
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return &ConversationListResponse{Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) ChangeCharacter(convID, charID string) (*Conversation, error) {
	if err := s.repo.UpdateConversation(convID, map[string]interface{}{"character_id": charID}); err != nil { return nil, err }
	return s.repo.GetConversation(convID)
}

func (s *service) GetStats() (*ChatStatsResponse, error) {
	var todayMsgs, totalConvs int64
	s.db.Model(&Message{}).Where("created_at >= ?", time.Now().Format("2006-01-02")).Count(&todayMsgs)
	s.db.Model(&Conversation{}).Count(&totalConvs)
	return &ChatStatsResponse{TodayMessages: todayMsgs, TotalConversations: totalConvs}, nil
}

func (s *service) Chat(req *ChatRequest) (*ChatResponse, error) {
	channel := req.Channel
	if channel == "" { channel = "web" }
	pmReq := &ProcessMessageRequest{
		CharacterID:    req.CharacterID,
		Message:        req.Message,
		ConversationID: req.ConversationID,
		Channel:        channel,
		Source:         "manual",
	}
	result, err := s.ProcessMessage(pmReq)
	if err != nil { return nil, err }
	return &ChatResponse{
		ConversationID: result.ConversationID,
		Message: &MessageItem{
			ID:             uuid.New().String(),
			ConversationID: result.ConversationID,
			Role:           "assistant",
			Content:        result.Reply,
			Source:         "manual",
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		},
	}, nil
}


func (s *service) loadHistory(convID string) []map[string]string {
	rows, _ := s.db.Table("messages").Select("role, content").Where("conversation_id = ? AND (tool_call_id IS NULL OR tool_call_id = '') AND role IN ('user','assistant','system')", convID).Order("created_at ASC").Limit(20).Rows()
	defer rows.Close()
	var history []map[string]string
	for rows.Next() { var role, content string; rows.Scan(&role, &content); history = append(history, map[string]string{"role": role, "content": content}) }
	if len(history) > 0 { history = history[:len(history)-1] }
	return history
}

func (s *service) ProcessMessage(req *ProcessMessageRequest) (*ProcessMessageResponse, error) {
	var charID, charName, systemPrompt string
	if req.CharacterID != "" {
		err := s.db.Table("characters").Select("id, name, system_prompt").Where("id = ?", req.CharacterID).Row().Scan(&charID, &charName, &systemPrompt)
		if err != nil { return nil, fmt.Errorf("角色不存在") }
	} else {
		s.db.Table("characters").Select("id, name, system_prompt").Where("is_default = 1").Limit(1).Row().Scan(&charID, &charName, &systemPrompt)
		if charID == "" {
			s.db.Table("characters").Select("id, name, system_prompt").Limit(1).Row().Scan(&charID, &charName, &systemPrompt)
		}
		if charID == "" { return nil, fmt.Errorf("没有可用角色") }
	}
	modelCfg, err := s.repo.GetActiveModel()
	if err != nil { return nil, fmt.Errorf("没有可用的模型配置") }
	convID := req.ConversationID
	channel := req.Channel
	if channel == "" { channel = "web" }
	source := req.Source
	if source == "" { source = "manual" }
	if convID == "" {
		convID = uuid.New().String()
		title := req.Message
		if len([]rune(title)) > 30 { title = string([]rune(title)[:30]) }
		s.db.Exec("INSERT INTO conversations (id, character_id, title, channel, source) VALUES (?, ?, ?, ?, ?)", convID, charID, title, channel, source)
	} else {
		var existingChannel string
		s.db.Table("conversations").Select("channel").Where("id = ?", convID).Limit(1).Row().Scan(&existingChannel)
		if existingChannel == "" {
			s.db.Exec("INSERT OR IGNORE INTO conversations (id, character_id, title, channel, source) VALUES (?, ?, ?, ?, ?)", convID, charID, "新对话", channel, source)
		} else if existingChannel != channel {
			convID = uuid.New().String()
			title := req.Message
			if len([]rune(title)) > 30 { title = string([]rune(title)[:30]) }
			s.db.Exec("INSERT INTO conversations (id, character_id, title, channel, source) VALUES (?, ?, ?, ?, ?)", convID, charID, title, channel, source)
		}
		if channel == "qq" {
			s.db.Exec("UPDATE conversations SET title = ? WHERE id = ?", "QQ对话", convID)
		} else if channel == "wechat" {
			s.db.Exec("UPDATE conversations SET title = ? WHERE id = ?", "微信对话", convID)
		}
	}
	if req.VoiceMessage && (req.Message == "" || req.Message == "[语音]") {
		req.Message = "（用户发来一条语音，听不清内容）"
	}
	if charID != "" {
		s.db.Exec("UPDATE characters SET conversation_id = ?, updated_at = ? WHERE id = ? AND (conversation_id IS NULL OR conversation_id = '')", convID, time.Now().Format("2006-01-02 15:04:05"), charID)
	}
	userMsgID := uuid.New().String()
	msgType := "text"
	if req.AudioUrl != "" { msgType = "voice" }
	s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, msg_type, audio_url, audio_duration, image_url, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", userMsgID, convID, "user", req.Message, source, msgType, req.AudioUrl, req.AudioDuration, req.ImageUrl, time.Now().Format("2006-01-02 15:04:05"))
	if req.ImageUrl != "" && req.ImageContext == "" {
		desc, errDetail := analyzeImageInternal(req.ImageUrl)
		logPath := filepath.Join(config.AppCfg.Storage.DataDir, "image_recognition_log.txt")
		if absPath, err := filepath.Abs(logPath); err == nil { os.WriteFile(absPath, []byte(desc), 0644) }
		if desc != "" {
			req.ImageContext = "[图片描述：" + desc + "]"
		} else {
			return &ProcessMessageResponse{
				ConversationID: convID,
				Reply:          errDetail,
				CharacterID:    charID,
				CharacterName:  charName,
			}, nil
		}
	}
	history := s.loadHistory(convID)
	messages := []map[string]interface{}{}
	messages = append(messages, map[string]interface{}{"role": "system", "content": systemNoEmojiInstruction})
	if systemPrompt != "" { messages = append(messages, map[string]interface{}{"role": "system", "content": systemPrompt}) }
	messages = append(messages, map[string]interface{}{"role": "system", "content": systemFormatInstruction})
	if req.ImageContext != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": req.ImageContext})
	}
	for _, m := range history { messages = append(messages, map[string]interface{}{"role": m["role"], "content": m["content"]}) }
	messages = append(messages, map[string]interface{}{"role": "user", "content": req.Message})
	toolDefs := tool.GetAll()
	var reply string
	seenTools := map[string]bool{}
	for round := 0; round < 3; round++ {
		aiContent, reasoning, toolCalls, _, llmErr := s.callLLMWithTools(modelCfg, messages, toolDefs)
		if llmErr != nil {
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
					if channel == "web" { toolArgs["channel"] = "all" } else if channel != "" { toolArgs["channel"] = channel }
				newArgs, _ := json.Marshal(toolArgs)
				args = string(newArgs)
			}
			result, _ := tool.Execute(name, args)
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": tc["id"], "content": result})

			// tool 结果不入库，避免污染历史上下文

		}
	}
	if reply == "" {
		reply = "操作已完成"
	}
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	var realLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		realLines = append(realLines, line)
	}
	if len(realLines) == 0 {
		realLines = []string{reply}
	}
	var msgIDs []string
	for _, line := range realLines {
		id := uuid.New().String()
		msgIDs = append(msgIDs, id)
		s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, created_at) VALUES (?, ?, ?, ?, ?, ?)", id, convID, "assistant", line, source, time.Now().Format("2006-01-02 15:04:05"))
	}
	s.db.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), convID, convID)

	go s.autoExtractMemories(convID, charID)
	go s.moodRecoveryCheck(convID, charID, source)

	return &ProcessMessageResponse{
		ConversationID: convID,
		Reply:          reply,
		CharacterID:    charID,
		CharacterName:  charName,
		MessageIDs:     msgIDs,
		ForceVoice:     tool.GetForceVoice(),
	}, nil
}

func (s *service) autoExtractMemories(convID, charID string) {
	if s.memorySvc == nil { return }
	candidates, err := s.memorySvc.GenerateCandidates(convID)
	if err != nil || len(candidates) == 0 { return }

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
		if c.Importance < 7 { continue }
		if existingKeys[c.Key+"|"+c.Value] { continue }
		existingKeys[c.Key+"|"+c.Value] = true
		s.memorySvc.AcceptCandidate(c.ID)
	}
}

func (s *service) callLLM(cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	jsonBody, _ := json.Marshal(reqBody)
	url := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil { return "", 0, err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return "", 0, fmt.Errorf("请求失败: %w", err) }
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 { return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, truncateStr(string(respBytes), 200)) }
	var result struct {
		Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"`
		Usage   struct{ TotalTokens int `json:"total_tokens"` } `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil { return "", 0, fmt.Errorf("解析响应失败: %w", err) }
	if len(result.Choices) == 0 { return "", 0, fmt.Errorf("API 未返回有效回复") }
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *service) ListModels() ([]ModelConfig, error) {
	return s.repo.ListModels()
}

func (s *service) CreateModel(cfg *ModelConfig) (*ModelConfig, error) {
	if err := s.repo.CreateModel(cfg); err != nil { return nil, fmt.Errorf("创建失败: %w", err) }
	return cfg, nil
}

func (s *service) UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error) {
	if err := s.repo.UpdateModel(id, updates); err != nil { return nil, fmt.Errorf("更新失败: %w", err) }
	return s.repo.GetModelByID(id)
}

func (s *service) DeleteModel(id int) error {
	return s.repo.DeleteModel(id)
}

func (s *service) ActivateModel(id int) (*ModelConfig, error) {
	if err := s.repo.ActivateModel(id); err != nil { return nil, fmt.Errorf("激活失败: %w", err) }
	return s.repo.GetModelByID(id)
}

func (s *service) GetModelRoutes() ([]map[string]interface{}, error) {
	return s.repo.GetModelRoutes()
}

func (s *service) UpdateModelRoutes(routes []map[string]interface{}) error {
	return s.repo.UpdateModelRoutes(routes)
}

type ModelDetectItem struct {
	ID     string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

func (s *service) DetectModels(baseURL, apiKey string) ([]ModelDetectItem, error) {
	base := strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("GET", base+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil { return nil, fmt.Errorf("请求失败: %w", err) }
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 { return nil, fmt.Errorf("API 返回 %d", resp.StatusCode) }
	var r struct{ Data []struct{ ID string `json:"id"`; OwnedBy string `json:"owned_by"` } `json:"data"` }
	if err := json.Unmarshal(rb, &r); err != nil {
		var r2 struct{ Models []struct{ Name string `json:"name"` } `json:"models"` }
		if json.Unmarshal(rb, &r2) == nil {
			items := make([]ModelDetectItem, len(r2.Models))
			for i, m := range r2.Models { items[i] = ModelDetectItem{ID: m.Name} }
			return items, nil
		}
		return nil, fmt.Errorf("解析响应失败")
	}
	items := make([]ModelDetectItem, len(r.Data))
	for i, m := range r.Data { items[i] = ModelDetectItem{ID: m.ID, OwnedBy: m.OwnedBy} }
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
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil { return "", "", nil, 0, err }
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
	if len(r.Choices) == 0 { return "", "", nil, 0, fmt.Errorf("no choices") }
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
	if len(runes) <= n { return s }
	return string(runes[:n]) + "..."
}





const doubaoAPIKey = "ark-919cb2bc-dcd1-4ef9-b8b5-5f1b42488bf7-9bd5c"
const doubaoBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
const doubaoModel = "doubao-seed-2-0-lite-260428"

func analyzeImageInternal(imageUrl string) (string, string) {
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
	reqBody := map[string]interface{}{
		"model": doubaoModel,
		"input": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "input_image", "image_url": imageData},
				{"type": "input_text", "text": "请详细描述这张图片的内容，包括场景、物体、人物、文字、表情、氛围等所有可见信息"},
			},
		}},
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", doubaoBaseURL+"/responses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+doubaoAPIKey)
	client := &http.Client{Timeout: 60 * time.Second}
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
