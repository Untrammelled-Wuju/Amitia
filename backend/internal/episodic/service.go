// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	Name() string
	Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error
	List(q EpisodicListQuery) (*EpisodicListResponse, error)
	Create(req *CreateEpisodicRequest) (*EpisodicMemory, error)
	Delete(id string) error
	GetByUserID(userID string, characterID ...string) ([]EpisodicMemory, error)
	GetDetail(id string) (*EpisodicMemory, []map[string]interface{}, error)
	ExtractFromConversation(userID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPrompt(userID string, characterID ...string) string
	SaveFromTool(userID, sceneType, title, content string, sentimentScore int, convID, msgStart, msgEnd string, characterID ...string) (*EpisodicMemory, error)
	SyncGraphEpisodic(id string) bool
}

type service struct {
	repo                     Repository
	db                       *gorm.DB
	graphSvc                 graph.Service
	dataLifecycleCoordinator *mindruntime.DataLifecycleCoordinator
}

func NewService(repo Repository, ctx *app.AppContext, graphSvc graph.Service) Service {
	return &service{repo: repo, db: ctx.DB, graphSvc: graphSvc}
}

func (s *service) SetDataLifecycleCoordinator(coordinator *mindruntime.DataLifecycleCoordinator) {
	s.dataLifecycleCoordinator = coordinator
}

func (s *service) List(q EpisodicListQuery) (*EpisodicListResponse, error) {
	if s.dataLifecycleCoordinator != nil && q.CharacterID != "" && s.dataLifecycleCoordinator.IsRetrievalBlocked(q.CharacterID) {
		return &EpisodicListResponse{Items: []EpisodicMemory{}, Total: 0, Page: 1, PageSize: 20, TotalPages: 1}, nil
	}
	if strings.TrimSpace(q.CharacterID) != "" {
		q.UserID = strings.TrimSpace(q.CharacterID)
	}
	items, total, err := s.repo.List(q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	pageSize := q.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return &EpisodicListResponse{Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

func (s *service) Create(req *CreateEpisodicRequest) (*EpisodicMemory, error) {
	if req.SceneType == "" {
		return nil, fmt.Errorf("sceneType不能为空")
	}
	scope := s.episodicScope(req.SourceConvID, req.CharacterID, req.UserID)
	if scope == "" {
		return nil, fmt.Errorf("character scope required")
	}
	m := &EpisodicMemory{
		UserID:           scope,
		SceneType:        req.SceneType,
		Title:            req.Title,
		Content:          req.Content,
		ContextBefore:    req.ContextBefore,
		ContextAfter:     req.ContextAfter,
		TriggerKeywords:  req.TriggerKeywords,
		SentimentScore:   req.SentimentScore,
		MessageIDStart:   req.MessageIDStart,
		MessageIDEnd:     req.MessageIDEnd,
		SourceConvID:     req.SourceConvID,
		MessageTimeStart: s.resolveMessageTime(req.SourceConvID, req.MessageIDStart),
		MessageTimeEnd:   s.resolveMessageTime(req.SourceConvID, req.MessageIDEnd),
	}
	if err := s.repo.Create(m); err != nil {
		return nil, err
	}
	s.syncGraph(m)
	return m, nil
}

func (s *service) Delete(id string) error {
	memory, _ := s.repo.FindByID(id)
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if s.graphSvc != nil {
		_ = s.graphSvc.DeleteNode("episodic:" + id)
		if memory != nil && memory.UserID != "" {
			_ = s.graphSvc.DeleteNodeIfOrphan("user:" + memory.UserID)
		}
	}
	return nil
}

func (s *service) GetByUserID(userID string, characterID ...string) ([]EpisodicMemory, error) {
	if scope := firstScope(characterID...); scope != "" {
		userID = scope
	}
	return s.repo.GetByUserID(userID, 0)
}

func (s *service) GetDetail(id string) (*EpisodicMemory, []map[string]interface{}, error) {
	return s.repo.GetDetailWithMessages(id, s.db)
}

func (s *service) SaveFromTool(userID, sceneType, title, content string, sentimentScore int, convID, msgStart, msgEnd string, characterID ...string) (*EpisodicMemory, error) {
	scope := s.episodicScope(convID, characterID...)
	if scope == "" {
		return nil, fmt.Errorf("character scope required")
	}
	m := &EpisodicMemory{
		UserID:           scope,
		SceneType:        sceneType,
		Title:            title,
		Content:          content,
		SentimentScore:   sentimentScore,
		SourceConvID:     convID,
		MessageTimeStart: s.resolveMessageTime(convID, msgStart),
		MessageTimeEnd:   s.resolveMessageTime(convID, msgEnd),
		MessageIDStart:   msgStart,
		MessageIDEnd:     msgEnd,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, err
	}
	s.syncGraph(m)
	return m, nil
}

func (s *service) SyncGraphEpisodic(id string) bool {
	m, _, err := s.repo.GetDetailWithMessages(id, s.db)
	if err != nil || m == nil {
		return false
	}
	s.syncGraph(m)
	return true
}

func (s *service) resolveMessageTime(convID, messageID string) string {
	if convID == "" || messageID == "" {
		return ""
	}
	var createdAt string
	s.db.Table("messages").Select("created_at").Where("id = ? AND conversation_id = ?", messageID, convID).Row().Scan(&createdAt)
	return createdAt
}

func (s *service) ExtractFromConversation(userID, convID string, messages []map[string]string, characterID ...string) error {
	if len(messages) == 0 {
		return nil
	}
	scope := s.episodicScope(convID, characterID...)
	if scope == "" {
		return nil
	}
	cfg := s.getActiveModel()
	if cfg == nil {
		return fmt.Errorf("no active model")
	}
	conversationText := ""
	for _, m := range messages {
		conversationText += m["role"] + ": " + m["content"] + "\n"
	}
	systemPrompt := `你是一个情景记忆检测器。从对话中检测值得长期记忆的情景时刻，返回JSON数组。
每个情景包含：
- scene_type: insight(感悟)/joke(笑话)/milestone(里程碑)/emotional_peak(情感峰值)/confession(坦白)
- title: 简短标题，不超过20字
- content: 情景描述，为什么值得记忆
- sentiment_score: 情感分值 -10到+10，负值表示负面情感，正值表示正面
- trigger_keywords: 逗号分隔的关键触发词

规则：
1. 只检测明显的情感转折、重要感悟、里程碑式对话
2. 普通闲聊不需要记录
3. 最多返回2个情景
4. 如果没有值得记录的情景，返回空数组[]

返回格式：严格JSON数组。`

	messages_llm := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": conversationText},
	}
	content, _, err := s.callLLM(cfg, messages_llm)
	if err != nil {
		return err
	}
	content = extractJSONArray(content)
	var scenes []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &scenes); err != nil {
		return nil
	}
	for _, sc := range scenes {
		st, _ := sc["scene_type"].(string)
		title, _ := sc["title"].(string)
		desc, _ := sc["content"].(string)
		score, _ := sc["sentiment_score"].(float64)
		keywords, _ := sc["trigger_keywords"].(string)
		if st == "" || title == "" || desc == "" {
			continue
		}
		m := &EpisodicMemory{
			UserID:          scope,
			SceneType:       st,
			Title:           title,
			Content:         desc,
			SentimentScore:  int(score),
			TriggerKeywords: keywords,
			SourceConvID:    convID,
		}
		if err := s.repo.Create(m); err == nil {
			s.syncGraph(m)
		}
	}
	return nil
}

func (s *service) ToSystemPrompt(userID string, characterID ...string) string {
	scope := firstScope(characterID...)
	if scope == "" {
		scope = cleanScope(userID)
	}
	memories, err := s.repo.GetRecentScoped(scope, 3)
	if err != nil || len(memories) == 0 {
		return ""
	}
	var lines []string
	for _, m := range memories {
		emoji := sentimentEmoji(m.SentimentScore)
		lines = append(lines, fmt.Sprintf("- %s %s: %s", emoji, m.Title, m.Content))
	}
	return "【情景记忆-最近】\n" + strings.Join(lines, "\n")
}

func sentimentEmoji(score int) string {
	if score >= 5 {
		return "😊"
	}
	if score >= 1 {
		return "🙂"
	}
	if score >= -4 {
		return "😐"
	}
	return "😢"
}

func (s *service) getActiveModel() map[string]interface{} {
	var baseURL, apiKey, modelName, apiType string
	var temperature, maxTokens float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens, api_type").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temperature, &maxTokens, &apiType)
	if err != nil {
		return nil
	}
	return map[string]interface{}{
		"baseUrl":     baseURL,
		"apiKey":      apiKey,
		"modelName":   modelName,
		"temperature": temperature,
		"maxTokens":   int(maxTokens),
		"apiType":     apiType,
	}
}

func (s *service) callLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
	if apiType, _ := cfg["apiType"].(string); apiType == "ollama" {
		return s.callOllamaLLM(cfg, messages)
	}
	baseURL := strings.TrimRight(cfg["baseUrl"].(string), "/")
	reqBody := map[string]interface{}{
		"model":       cfg["modelName"],
		"messages":    messages,
		"temperature": cfg["temperature"],
		"max_tokens":  cfg["maxTokens"],
		"stream":      false,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"].(string))
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb[:minInt(len(rb), 200)]))
	}
	var result struct {
		Choices []struct{ Message struct{ Content string } }
		Usage   struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &result)
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *service) callOllamaLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg["baseUrl"].(string), "/")
	reqBody := map[string]interface{}{
		"model":    cfg["modelName"],
		"messages": messages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": cfg["temperature"],
			"num_ctx":     cfg["maxTokens"],
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb[:minInt(len(rb), 200)]))
	}
	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		EvalCount       int `json:"eval_count"`
		PromptEvalCount int `json:"prompt_eval_count"`
	}
	if err := json.Unmarshal(rb, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	total := result.EvalCount + result.PromptEvalCount
	return result.Message.Content, total, nil
}

func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "]"); idx >= 0 {
		s = s[:idx+1]
	}
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *service) Name() string { return "情景记忆" }

func (s *service) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	manager := pipelinecheckpoint.New(s.db)
	leaseOwner := fmt.Sprintf("episodic:%s:%d", convID, time.Now().UTC().UnixNano())
	pending, maxSequence, acquired, err := manager.AcquirePendingRange(convID, "episodic", 4, leaseOwner, 10*time.Minute)
	if err != nil || !acquired || len(pending) == 0 {
		return err
	}
	if err := s.ExtractFromConversation("", convID, pending); err != nil {
		return err
	}
	return manager.AdvanceLeased(convID, "episodic", maxSequence, fmt.Sprintf("episodic:%s:%d", convID, maxSequence), leaseOwner)
}

func (s *service) episodicScope(convID string, scopes ...string) string {
	for _, scope := range scopes {
		if cleaned := cleanScope(scope); cleaned != "" {
			return cleaned
		}
	}
	if s.db == nil || strings.TrimSpace(convID) == "" {
		return ""
	}
	var scope string
	if err := s.db.Table("conversations").Select("character_id").Where("id = ?", convID).Row().Scan(&scope); err != nil {
		return ""
	}
	return cleanScope(scope)
}

func firstScope(scopes ...string) string {
	for _, scope := range scopes {
		if cleaned := cleanScope(scope); cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func cleanScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "default" {
		return ""
	}
	return scope
}

func (s *service) syncGraph(m *EpisodicMemory) {
	if s.graphSvc == nil || m == nil {
		return
	}
	if m.UserID == "" {
		return
	}
	_ = s.graphSvc.SyncNode("user", m.UserID, m.UserID, map[string]interface{}{"user_id": m.UserID})
	_ = s.graphSvc.SyncNode("episodic", m.ID, m.Title, map[string]interface{}{
		"sceneType":       m.SceneType,
		"sentimentScore":  m.SentimentScore,
		"sourceConvID":    m.SourceConvID,
		"triggerKeywords": m.TriggerKeywords,
		"user_id":         m.UserID,
	})
	_ = s.graphSvc.SyncEdge("user:"+m.UserID, "episodic:"+m.ID, "experienced", 1.0)
}
