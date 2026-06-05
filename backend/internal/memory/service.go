package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	RecordUse(id string) (*Memory, error)
	GetVectorStatus() map[string]interface{}
	GetTimeline(page, pageSize int, characterID, source, memoryType string) ([]map[string]interface{}, int64, error)
	GenerateCandidates(conversationID string) ([]MemoryCandidate, error)
	ListCandidates() []MemoryCandidate
	AcceptCandidate(id string) (*Memory, error)
	RejectCandidate(id string) error
	BatchAcceptCandidates(ids []string) ([]Memory, error)
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

type service struct {
	repo       Repository
	db         *gorm.DB
	candidates []MemoryCandidate
	mu         sync.Mutex
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{repo: repo, db: ctx.DB, candidates: []MemoryCandidate{}}
}

func (s *service) List(q MemoryListQuery) (*MemoryListResponse, error) {
	items, total, err := s.repo.List(q)
	if err != nil { return nil, err }
	return &MemoryListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *service) Create(req *CreateMemoryRequest) (*Memory, error) {
	if req.MemoryType == "" { req.MemoryType = "custom" }
	if req.Source == "" { req.Source = "manual" }
	if req.Importance < 0 { req.Importance = 0 }
	if req.Importance > 10 { req.Importance = 10 }
	m := &Memory{CharacterID: req.CharacterID, MemoryType: req.MemoryType, Source: req.Source, Key: req.Key, Value: req.Value, Importance: req.Importance}
	if err := s.repo.Create(m); err != nil { return nil, fmt.Errorf("创建失败: %w", err) }
	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) Update(id string, req *UpdateMemoryRequest) (*Memory, error) {
	updates := make(map[string]interface{})
	if req.Key != nil { updates["key"] = *req.Key }
	if req.Value != nil { updates["value"] = *req.Value }
	if req.MemoryType != nil { updates["memory_type"] = *req.MemoryType }
	if req.CharacterID != nil { updates["character_id"] = *req.CharacterID }
	if req.Importance != nil { updates["importance"] = *req.Importance }
	if len(updates) == 0 { return nil, fmt.Errorf("没有可更新的字段") }
	if err := s.repo.Update(id, updates); err != nil { return nil, fmt.Errorf("更新失败: %w", err) }
	m, _ := s.repo.FindByID(id)
	if m != nil { s.logEvent(m.ID, "memory_edited", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID) }
	return m, nil
}

func (s *service) Delete(id string) error {
	m, _ := s.repo.FindByID(id)
	if m != nil { s.logEvent(m.ID, "memory_deleted", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID) }
	return s.repo.Delete(id)
}
func (s *service) DeleteAll() error { return s.repo.DeleteAll() }

func (s *service) Search(req *SearchMemoryRequest) ([]Memory, error) {
	if req.Limit <= 0 { req.Limit = 10 }
	return s.repo.Search(req.Keyword, req.CharacterID, req.Limit)
}

func (s *service) RecordUse(id string) (*Memory, error) {
	if err := s.repo.RecordUse(id); err != nil { return nil, err }
	return s.repo.FindByID(id)
}

func (s *service) GetVectorStatus() map[string]interface{} {
	total, embedded := s.repo.VectorStatus()
	return map[string]interface{}{"totalMemories": total, "embeddedCount": embedded, "totalEmbeddings": embedded, "enabled": false, "providerName": "", "status": "ok"}
}

func (s *service) GenerateCandidates(conversationID string) ([]MemoryCandidate, error) {
	if conversationID == "" { return nil, fmt.Errorf("conversationId is required") }

	var msgs []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND role IN ('user','assistant')", conversationID).Order("created_at ASC").Limit(30).Find(&msgs)
	if len(msgs) == 0 { return nil, fmt.Errorf("没有可提取的消息") }

	var conversationText strings.Builder
	for _, msg := range msgs {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role == "user" { conversationText.WriteString("用户: ") } else { conversationText.WriteString("AI: ") }
		conversationText.WriteString(content)
		conversationText.WriteString("\n")
	}

	cfg := s.getActiveModel()
	if cfg == nil { return nil, fmt.Errorf("没有可用的模型配置") }

	prompt := conversationText.String()
	sysPrompt := `你是一个信息提取助手。从对话中提取关于用户的重要事实信息。
以JSON数组格式返回，每个事实包含：
- key: 简短标签（如姓名、职业、爱好、宠物）
- value: 具体内容
- memoryType: 类型（personal_info/hobby/preference/fact/plan）
- importance: 重要度1-10

只返回JSON数组，不要其他内容。`

	messages := []map[string]interface{}{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": prompt},
	}

	content, _, err := s.callLLM(cfg, messages)
	if err != nil { return nil, fmt.Errorf("LLM调用失败: %w", err) }

	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "["); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "]"); idx >= 0 {
		content = content[:idx+1]
	}

	var extracted []struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		MemoryType string `json:"memoryType"`
		Importance int    `json:"importance"`
	}
	if err := json.Unmarshal([]byte(content), &extracted); err != nil {
		return nil, fmt.Errorf("解析提取结果失败: %w", err)
	}

	s.mu.Lock()
	s.candidates = nil
	for _, e := range extracted {
		if e.Key == "" || e.Value == "" { continue }
		if e.MemoryType == "" { e.MemoryType = "fact" }
		if e.Importance < 1 { e.Importance = 5 }
		if e.Importance > 10 { e.Importance = 10 }
		s.candidates = append(s.candidates, MemoryCandidate{
			ID:             uuid.New().String(),
			Key:            e.Key,
			Value:          e.Value,
			MemoryType:     e.MemoryType,
			Importance:     e.Importance,
			SourceText:     conversationID[:8],
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
			m := &Memory{Key: c.Key, Value: c.Value, MemoryType: c.MemoryType, Importance: c.Importance, Source: "extracted"}
			if err := s.repo.Create(m); err != nil { return nil, err }
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
		if err == nil { accepted = append(accepted, *m) }
	}
	return accepted, nil
}

func (s *service) logEvent(memoryID, eventType, key, value, memoryType string, importance int, source, characterID string) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec("INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, memoryID, eventType, key, value, memoryType, importance, source, characterID, now)
}

func (s *service) GetTimeline(page, pageSize int, characterID, source, memoryType string) ([]map[string]interface{}, int64, error) {
	if page <= 0 { page = 1 }
	if pageSize <= 0 { pageSize = 30 }
	query := s.db.Table("memory_events")
	if characterID != "" { query = query.Where("character_id = ?", characterID) }
	if source != "" { query = query.Where("source = ?", source) }
	if memoryType != "" { query = query.Where("memory_type = ?", memoryType) }
	var total int64
	query.Count(&total)
	var events []map[string]interface{}
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error
	if events == nil { events = []map[string]interface{}{} }
	return events, total, err
}

func (s *service) getActiveModel() map[string]interface{} {
	var baseURL, apiKey, modelName string
	var temperature, maxTokens float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temperature, &maxTokens)
	if err != nil { return nil }
	return map[string]interface{}{"baseUrl": baseURL, "apiKey": apiKey, "modelName": modelName, "temperature": temperature, "maxTokens": int(maxTokens)}
}

func (s *service) callLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg["baseUrl"].(string), "/")
	reqBody := map[string]interface{}{"model": cfg["modelName"], "messages": messages, "temperature": cfg["temperature"], "max_tokens": cfg["maxTokens"], "stream": false}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"].(string))
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil { return "", 0, err }
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 { return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb)[:200]) }
	var result struct {
		Choices []struct{ Message struct{ Content string } }
		Usage   struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &result)
	if len(result.Choices) == 0 { return "", 0, fmt.Errorf("no choices") }
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}
