package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/embedding"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	List(q MemoryListQuery) (*MemoryListResponse, error)
	Create(req *CreateMemoryRequest) (*Memory, error)
	Update(id string, req *UpdateMemoryRequest) (*Memory, error)
	Delete(id string) error
	DeleteAll() error
	Search(req *SearchMemoryRequest) ([]Memory, error)
	VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error)
	RecordUse(id string) (*Memory, error)
	GetVectorStatus() map[string]interface{}
	GetTimeline(page, pageSize int, characterID, source, memoryType string) ([]map[string]interface{}, int64, error)
	GenerateCandidates(conversationID string) ([]MemoryCandidate, error)
	ListCandidates() []MemoryCandidate
	AcceptCandidate(id string) (*Memory, error)
	RejectCandidate(id string) error
	BatchAcceptCandidates(ids []string) ([]Memory, error)
	RebuildEmbeddings() (map[string]interface{}, error)
	SyncEmbedding(memID, key, value, characterID, memoryType string)
}
type MemoryCandidate struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Value          string `json:"value"`
	MemoryType     string `json:"memoryType"`
	Importance     int    `json:"importance"`
	SourceText     string `json:"sourceText"`
	ConversationID string `json:"conversationId"`
	CreatedAt      string `json:"createdAt"`
}

type VectorSearchResult struct {
	Memory Memory  `json:"memory"`
	Score  float32 `json:"score"`
}

type service struct {
	repo         Repository
	db           *gorm.DB
	embeddingSvc *embedding.Service
	candidates   []MemoryCandidate
	mu           sync.Mutex
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{
		repo:         repo,
		db:           ctx.DB,
		embeddingSvc: embedding.NewService(ctx.DB),
		candidates:   []MemoryCandidate{},
	}
}

func (s *service) List(q MemoryListQuery) (*MemoryListResponse, error) {
	items, total, err := s.repo.List(q)
	if err != nil {
		return nil, err
	}
	return &MemoryListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *service) Create(req *CreateMemoryRequest) (*Memory, error) {
	if req.MemoryType == "" {
		req.MemoryType = "custom"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	if req.Importance < 0 {
		req.Importance = 0
	}
	if req.Importance > 10 {
		req.Importance = 10
	}
	m := &Memory{
		CharacterID: req.CharacterID,
		MemoryType:  req.MemoryType,
		Source:      req.Source,
		Key:         req.Key,
		Value:       req.Value,
		Importance:  req.Importance,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)

	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) Update(id string, req *UpdateMemoryRequest) (*Memory, error) {
	updates := make(map[string]interface{})
	if req.Key != nil {
		updates["key"] = *req.Key
	}
	if req.Value != nil {
		updates["value"] = *req.Value
	}
	if req.MemoryType != nil {
		updates["memory_type"] = *req.MemoryType
	}
	if req.CharacterID != nil {
		updates["character_id"] = *req.CharacterID
	}
	if req.Importance != nil {
		updates["importance"] = *req.Importance
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	m, _ := s.repo.FindByID(id)
	if m != nil {
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.logEvent(m.ID, "memory_edited", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	}
	return m, nil
}

func (s *service) Delete(id string) error {
	m, _ := s.repo.FindByID(id)
	if m != nil {
		s.logEvent(m.ID, "memory_deleted", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	}

	go func() {
		if qdrantDB.Client != nil {
			_ = qdrantDB.DeleteVectors([]string{id})
		}
	}()

	return s.repo.Delete(id)
}

func (s *service) DeleteAll() error {
	go func() {
		if qdrantDB.Client != nil {
			qdrantDB.Client.DeleteCollection(context.Background(), config.AppCfg.Qdrant.CollectionName)
			qdrantDB.EnsureCollection()
		}
	}()
	return s.repo.DeleteAll()
}

func (s *service) Search(req *SearchMemoryRequest) ([]Memory, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}

	if qdrantDB.Client != nil && req.Keyword != "" {
		results, err := s.VectorSearch(&VectorSearchRequest{
			Keyword:     req.Keyword,
			CharacterID: req.CharacterID,
			Limit:       req.Limit,
		})
		if err == nil && len(results) > 0 {
			memories := make([]Memory, len(results))
			for i, r := range results {
				memories[i] = r.Memory
			}
			return memories, nil
		}
	}

	return s.repo.Search(req.Keyword, req.CharacterID, req.Limit)
}

func (s *service) VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error) {
	if req.Limit <= 0 {
		req.Limit = config.AppCfg.Qdrant.Limit
	}

	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("向量数据库未就绪")
	}

	text := req.Keyword
	if text == "" {
		text = req.Query
	}
	if text == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	vector, err := s.embeddingSvc.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("生成嵌入向量失败: %w", err)
	}

	filter := make(map[string]interface{})
	if req.CharacterID != "" {
		filter["character_id"] = req.CharacterID
	}

	points, err := qdrantDB.SearchVectors(vector, req.Limit, filter)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	results := make([]VectorSearchResult, 0)
	for _, p := range points {
		payloadMap := p.Payload
		memIDVal, ok := payloadMap["memory_id"]
		if !ok {
			continue
		}
		memID := memIDVal.GetStringValue()
		m, err := s.repo.FindByID(memID)
		if err != nil || m == nil {
			continue
		}
		results = append(results, VectorSearchResult{
			Memory: *m,
			Score:  p.Score,
		})
	}
	return results, nil
}

func (s *service) RecordUse(id string) (*Memory, error) {
	if err := s.repo.RecordUse(id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *service) GetVectorStatus() map[string]interface{} {
	total, embedded := s.repo.VectorStatus()

	enabled := qdrantDB.Client != nil
	providerName := ""
	vectorCount := uint64(0)
	if enabled {
		providerName = "qdrant"
		count, err := qdrantDB.GetVectorCount()
		if err == nil {
			vectorCount = count
		}
	}

	return map[string]interface{}{
		"totalMemories":   total,
		"embeddedCount":   embedded,
		"totalEmbeddings": vectorCount,
		"enabled":         enabled,
		"providerName":    providerName,
		"status":          "ok",
	}
}

func (s *service) RebuildEmbeddings() (map[string]interface{}, error) {
	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("向量数据库未就绪")
	}

	var memories []Memory
	s.db.Find(&memories)

	successCount := 0
	failCount := 0
	for _, m := range memories {
		text := m.Key + " " + m.Value
		vector, err := s.embeddingSvc.Embed(text)
		if err != nil {
			failCount++
			log.Error("重建嵌入失败:", m.ID, err)
			continue
		}

		payload := map[string]interface{}{
			"memory_id":    m.ID,
			"character_id": m.CharacterID,
			"memory_type":  m.MemoryType,
			"key":          m.Key,
			"value":        m.Value,
		}
		err = qdrantDB.UpsertVectors([]qdrantDB.VectorPoint{
			{ID: m.ID, Vector: vector, Payload: payload},
		})
		if err != nil {
			failCount++
			log.Error("存储嵌入失败:", m.ID, err)
			continue
		}
		successCount++
		_ = s.repo.MarkEmbedded(m.ID)
	}

	return map[string]interface{}{
		"total":   len(memories),
		"success": successCount,
		"failed":  failCount,
	}, nil
}

func (s *service) GenerateCandidates(conversationID string) ([]MemoryCandidate, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("conversationId is required")
	}

	var msgs []map[string]interface{}
	s.db.Table("messages").
		Where("conversation_id = ? AND role IN ('user','assistant')", conversationID).
		Order("created_at ASC").Limit(30).Find(&msgs)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("没有可提取的消息")
	}

	var conversationText strings.Builder
	for _, msg := range msgs {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role == "user" {
			conversationText.WriteString("用户: ")
		} else {
			conversationText.WriteString("AI: ")
		}
		conversationText.WriteString(content)
		conversationText.WriteString("\n")
	}

	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	prompt := fmt.Sprintf("从以下对话中提取值得记忆的关键信息，以JSON数组格式返回，每个元素包含key/value/memoryType/importance字段。\nmemoryType可选值: fact(事实)/preference(偏好)/relationship(关系)/plan(计划)\nimportance: 1-10 重要程度\n\n%s", conversationText.String())

	apiMessages := []map[string]interface{}{
		{"role": "system", "content": "你是一个信息提取助手。只返回JSON数组，不要其他内容。"},
		{"role": "user", "content": prompt},
	}
	content, _, err := s.callLLM(cfg, apiMessages)
	if err != nil {
		return nil, err
	}

	var extracted []struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		MemoryType string `json:"memoryType"`
		Importance int    `json:"importance"`
	}
	content = extractJSONArray(content)
	if err := json.Unmarshal([]byte(content), &extracted); err != nil {
		return nil, fmt.Errorf("解析提取结果失败: %w", err)
	}

	s.mu.Lock()
	s.candidates = nil
	for _, e := range extracted {
		if e.Key == "" || e.Value == "" {
			continue
		}
		if e.MemoryType == "" {
			e.MemoryType = "fact"
		}
		if e.Importance < 1 {
			e.Importance = 5
		}
		if e.Importance > 10 {
			e.Importance = 10
		}
		convIDShort := conversationID
		if len(convIDShort) > 8 {
			convIDShort = convIDShort[:8]
		}
		s.candidates = append(s.candidates, MemoryCandidate{
			ID:             uuid.New().String(),
			Key:            e.Key,
			Value:          e.Value,
			MemoryType:     e.MemoryType,
			Importance:     e.Importance,
			SourceText:     convIDShort,
			ConversationID: conversationID,
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	result := make([]MemoryCandidate, len(s.candidates))
	copy(result, s.candidates)
	s.mu.Unlock()

	return result, nil
}

func (s *service) ListCandidates() []MemoryCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]MemoryCandidate, len(s.candidates))
	copy(result, s.candidates)
	return result
}

func (s *service) AcceptCandidate(id string) (*Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.candidates {
		if c.ID == id {
			m := &Memory{
				Key: c.Key, Value: c.Value, MemoryType: c.MemoryType,
				Importance: c.Importance, Source: "extracted",
			}
			if err := s.repo.Create(m); err != nil {
				return nil, err
			}
			go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
			s.logEvent(m.ID, "candidate_accepted", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
			s.candidates = append(s.candidates[:i], s.candidates[i+1:]...)
			return m, nil
		}
	}
	return nil, fmt.Errorf("候选记忆不存在")
}

func (s *service) RejectCandidate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.candidates {
		if c.ID == id {
			s.candidates = append(s.candidates[:i], s.candidates[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("候选记忆不存在")
}

func (s *service) BatchAcceptCandidates(ids []string) ([]Memory, error) {
	var accepted []Memory
	for _, id := range ids {
		m, err := s.AcceptCandidate(id)
		if err == nil {
			accepted = append(accepted, *m)
		}
	}
	return accepted, nil
}

func (s *service) SyncEmbedding(memID, key, value, characterID, memoryType string) {
	if qdrantDB.Client == nil {
		return
	}
	text := key + " " + value
	vector, err := s.embeddingSvc.Embed(text)
	if err != nil {
		log.Error("生成嵌入失败:", memID, err)
		return
	}

	payload := map[string]interface{}{
		"memory_id":    memID,
		"character_id": characterID,
		"memory_type":  memoryType,
		"key":          key,
		"value":        value,
	}
	err = qdrantDB.UpsertVectors([]qdrantDB.VectorPoint{
		{ID: memID, Vector: vector, Payload: payload},
	})
	if err != nil {
		log.Error("存储嵌入失败:", memID, err)
		return
	}
	_ = s.repo.MarkEmbedded(memID)
}

func (s *service) logEvent(memoryID, eventType, key, value, memoryType string, importance int, source, characterID string) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec(
		"INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, memoryID, eventType, key, value, memoryType, importance, source, characterID, now,
	)
}

func (s *service) GetTimeline(page, pageSize int, characterID, source, memoryType string) ([]map[string]interface{}, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	query := s.db.Table("memory_events")
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if memoryType != "" {
		query = query.Where("memory_type = ?", memoryType)
	}
	var total int64
	query.Count(&total)
	var events []map[string]interface{}
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error
	if events == nil {
		events = []map[string]interface{}{}
	}
	return events, total, err
}

func (s *service) getActiveModel() map[string]interface{} {
	var baseURL, apiKey, modelName string
	var temperature, maxTokens float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temperature, &maxTokens)
	if err != nil {
		return nil
	}
	return map[string]interface{}{
		"baseUrl":     baseURL,
		"apiKey":      apiKey,
		"modelName":   modelName,
		"temperature": temperature,
		"maxTokens":   int(maxTokens),
	}
}

func (s *service) callLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
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
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncateStr(string(rb), 200))
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

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
