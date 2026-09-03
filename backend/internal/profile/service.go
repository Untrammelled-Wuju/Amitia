// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

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
	List(q ProfileListQuery) (*ProfileListResponse, error)
	Create(req *CreateProfileRequest) (*UserProfile, error)
	Update(id string, req *UpdateProfileRequest) (*UserProfile, error)
	Delete(id string) error
	GetByUserID(userID string, characterID ...string) ([]UserProfile, error)
	ExtractFromConversation(userID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPrompt(userID string, characterID ...string) string
	UpsertFromTool(userID, category, attrName, attrValue string, confidence int, convID string, characterID ...string) (*UserProfile, error)
	SyncGraphProfile(id string) bool
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

func (s *service) List(q ProfileListQuery) (*ProfileListResponse, error) {
	if s.dataLifecycleCoordinator != nil && q.CharacterID != "" && s.dataLifecycleCoordinator.IsRetrievalBlocked(q.CharacterID) {
		return &ProfileListResponse{Items: []UserProfile{}, Total: 0, Page: 1, PageSize: 20, TotalPages: 1}, nil
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
		pageSize = 50
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return &ProfileListResponse{Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

func (s *service) Create(req *CreateProfileRequest) (*UserProfile, error) {
	if req.Category == "" {
		return nil, fmt.Errorf("category不能为空")
	}
	userID := s.profileUserScope("", req.UserID, req.CharacterID)
	if userID == "" {
		return nil, fmt.Errorf("user scope required")
	}
	req.Confidence = clampProfileConfidence(req.Confidence)
	p := &UserProfile{
		UserID:         userID,
		CharacterID:    req.CharacterID,
		Category:       req.Category,
		AttributeName:  req.AttributeName,
		AttributeValue: req.AttributeValue,
		Confidence:     req.Confidence,
		Source:         req.Source,
		SourceConvID:   req.SourceConvID,
	}
	result, err := s.repo.UpsertConfidence(p)
	if err == nil && result != nil {
		s.syncGraph(result)
	}
	return result, err
}

func (s *service) Update(id string, req *UpdateProfileRequest) (*UserProfile, error) {
	updates := map[string]interface{}{}
	if req.AttributeValue != nil {
		updates["attribute_value"] = *req.AttributeValue
	}
	if req.Confidence != nil {
		updates["confidence"] = clampProfileConfidence(*req.Confidence)
	}
	if req.Verified != nil && *req.Verified {
		updates["verified_at"] = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	result, err := s.repo.FindByID(id)
	if err == nil && result != nil {
		s.syncGraph(result)
	}
	return result, err
}

func (s *service) Delete(id string) error {
	p, _ := s.repo.FindByID(id)
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if s.graphSvc != nil && p != nil {
		userID := s.profileUserScope(p.SourceConvID, p.UserID, p.CharacterID)
		if userID == "" {
			return nil
		}
		nodeID := userID + ":" + p.Category + ":" + p.AttributeName
		if p.CharacterID != "" {
			nodeID = userID + ":" + p.CharacterID + ":" + p.Category + ":" + p.AttributeName
		}
		_ = s.graphSvc.DeleteNode("profile:" + nodeID)
		_ = s.graphSvc.DeleteNodeIfOrphan("user:" + userID)
	}
	return nil
}

func (s *service) GetByUserID(userID string, characterID ...string) ([]UserProfile, error) {
	if s.dataLifecycleCoordinator != nil && len(characterID) > 0 && characterID[0] != "" && s.dataLifecycleCoordinator.IsRetrievalBlocked(characterID[0]) {
		return []UserProfile{}, nil
	}
	if len(characterID) > 0 && characterID[0] != "" {
		return s.repo.GetScopedByUserID(userID, characterID[0])
	}
	return s.repo.GetByUserID(userID)
}

func (s *service) UpsertFromTool(userID, category, attrName, attrValue string, confidence int, convID string, characterID ...string) (*UserProfile, error) {
	scope := s.profileScope(convID, characterID...)
	userID = s.profileUserScope(convID, userID, scope)
	if userID == "" {
		return nil, fmt.Errorf("user scope required")
	}
	if category == "" {
		category = "personal_info"
	}
	if confidence < 1 {
		confidence = 50
	}
	confidence = clampProfileConfidence(confidence)
	p := &UserProfile{
		UserID:         userID,
		CharacterID:    scope,
		Category:       category,
		AttributeName:  attrName,
		AttributeValue: attrValue,
		Confidence:     confidence,
		SourceConvID:   convID,
	}
	result, err := s.repo.UpsertConfidence(p)
	if err == nil && result != nil {
		s.syncGraph(result)
	}
	return result, err
}

func (s *service) SyncGraphProfile(id string) bool {
	p, err := s.repo.FindByID(id)
	if err != nil || p == nil {
		return false
	}
	s.syncGraph(p)
	return true
}

func (s *service) ExtractFromConversation(userID, convID string, messages []map[string]string, characterID ...string) error {
	if len(messages) == 0 {
		return nil
	}
	cfg := s.getActiveModel()
	if cfg == nil {
		return fmt.Errorf("no active model")
	}
	scope := s.profileScope(convID, characterID...)
	userID = s.profileUserScope(convID, userID, scope)
	if userID == "" {
		return nil
	}
	conversationText := ""
	for _, m := range messages {
		conversationText += m["role"] + ": " + m["content"] + "\n"
	}
	systemPrompt := `你是一个用户画像提取器。从对话中提取关于用户的个人事实，返回JSON数组。
每个事实包含：
- category: personal_info/preference/habit/fear/relationship/health/plan
- attribute_name: 简短属性名如"姓名"、"爱好"、"恐惧"、"职业"
- attribute_value: 属性值如"张三"、"喜欢摄影"
- confidence: 置信度0-100，根据信息明确程度打分

规则：
1. 只提取对话中明确出现的事实，不要推测
2. 如果同一个事实出现多次，confidence应该更高
3. 对于已经出现在["相关记忆"]中的事实，不需要重复提取
4. 最多返回5个事实
5. 如果没有值得提取的事实，返回空数组[]

返回格式：严格JSON数组，不要有额外解释。`

	messages_llm := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": conversationText},
	}
	content, _, err := s.callLLM(cfg, messages_llm)
	if err != nil {
		return err
	}
	content = extractJSONArray(content)
	var facts []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &facts); err != nil {
		return nil
	}
	for _, f := range facts {
		cat, _ := f["category"].(string)
		name, _ := f["attribute_name"].(string)
		val, _ := f["attribute_value"].(string)
		conf, _ := f["confidence"].(float64)
		if cat == "" || name == "" || val == "" {
			continue
		}
		result, err := s.repo.UpsertConfidence(&UserProfile{
			UserID:         userID,
			CharacterID:    scope,
			Category:       cat,
			AttributeName:  name,
			AttributeValue: val,
			Confidence:     int(conf),
			SourceConvID:   convID,
		})
		if err == nil && result != nil {
			s.syncGraph(result)
		}
	}
	return nil
}

func (s *service) ToSystemPrompt(userID string, characterID ...string) string {
	requestedUserID := strings.TrimSpace(userID)
	scope := firstScope(characterID...)
	userID = cleanUserScope(userID)
	if userID == "" {
		userID = scope
	}
	if userID == "" {
		return ""
	}
	profiles, err := s.repo.GetUserFactSummary(userID, scope)
	if (err != nil || len(profiles) == 0) && requestedUserID == "default" && scope != "" {
		profiles, err = s.legacyDefaultCharacterProfiles(scope)
	}
	if err != nil || len(profiles) == 0 {
		return ""
	}
	categoryGroups := map[string][]string{}
	coreCount := 0
	for _, p := range profiles {
		if coreCount >= 8 || !s.isCoreProfileEntry(p) {
			continue
		}
		label := categoryLabel(p.Category)
		line := fmt.Sprintf("- %s: %s (置信度%d%%)", p.AttributeName, p.AttributeValue, p.Confidence)
		categoryGroups[label] = append(categoryGroups[label], line)
		coreCount++
	}
	var parts []string
	order := []string{"个人信息", "偏好", "习惯", "恐惧", "关系", "健康", "计划"}
	for _, cat := range order {
		if lines, ok := categoryGroups[cat]; ok {
			parts = append(parts, "【"+cat+"】\n"+strings.Join(lines, "\n"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "【用户画像】\n" + strings.Join(parts, "\n\n")
}

func (s *service) isCoreProfileEntry(p UserProfile) bool {
	if strings.EqualFold(strings.TrimSpace(p.ProjectionStatus), "archived") {
		return false
	}
	if p.SourceMemoryID != "" && s.db != nil {
		var row struct {
			RetentionLevel int
			DecayState     string
			Pinned         bool
		}
		if err := s.db.Table("memories").Select("retention_level, decay_state, pinned").Where("id = ?", p.SourceMemoryID).Scan(&row).Error; err == nil {
			if strings.EqualFold(strings.TrimSpace(row.DecayState), "archived") {
				return false
			}
			return row.Pinned || (row.RetentionLevel >= 1 && row.RetentionLevel <= 2)
		}
	}
	// Legacy profiles remain available as a compact compatibility kernel only
	// when they are high-confidence; ordinary preferences are dynamically recalled.
	return p.Confidence >= 80
}

func (s *service) legacyDefaultCharacterProfiles(scope string) ([]UserProfile, error) {
	if s.db == nil || scope == "" {
		return []UserProfile{}, nil
	}
	var items []UserProfile
	err := s.db.Where("user_id = ? AND character_id = ? AND confidence >= 50", "default", scope).Order("confidence DESC").Limit(20).Find(&items).Error
	if items == nil {
		items = []UserProfile{}
	}
	return items, err
}

func categoryLabel(cat string) string {
	switch cat {
	case "personal_info":
		return "个人信息"
	case "preference":
		return "偏好"
	case "habit":
		return "习惯"
	case "fear":
		return "恐惧"
	case "relationship":
		return "关系"
	case "health":
		return "健康"
	case "plan":
		return "计划"
	default:
		return "其他"
	}
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

func (s *service) Name() string { return "用户画像" }

func (s *service) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	pending, maxSequence, err := pipelinecheckpoint.New(s.db).PendingRange(convID, "profile", 0)
	if err != nil || len(pending) == 0 {
		return err
	}
	return pipelinecheckpoint.New(s.db).Advance(convID, "profile", maxSequence, fmt.Sprintf("profile-projection:%s:%d", convID, maxSequence))
}

func (s *service) profileScope(convID string, characterID ...string) string {
	if scope := firstScope(characterID...); scope != "" {
		return scope
	}
	if s.db == nil || convID == "" {
		return ""
	}
	var scope string
	if err := s.db.Table("conversations").Select("character_id").Where("id = ?", convID).Row().Scan(&scope); err != nil {
		return ""
	}
	return scope
}

func (s *service) profileUserScope(convID, userID string, characterID ...string) string {
	if scope := cleanUserScope(userID); scope != "" {
		return scope
	}
	if scope := firstScope(characterID...); scope != "" {
		return scope
	}
	return s.profileScope(convID)
}

func firstScope(characterID ...string) string {
	if len(characterID) == 0 {
		return ""
	}
	return cleanUserScope(characterID[0])
}

func cleanUserScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "default" {
		return ""
	}
	return scope
}

func (s *service) syncGraph(p *UserProfile) {
	if s.graphSvc == nil || p == nil {
		return
	}
	userID := s.profileUserScope(p.SourceConvID, p.UserID, p.CharacterID)
	if userID == "" {
		return
	}
	p.UserID = userID
	nodeID := p.UserID + ":" + p.Category + ":" + p.AttributeName
	if p.CharacterID != "" {
		nodeID = p.UserID + ":" + p.CharacterID + ":" + p.Category + ":" + p.AttributeName
	}
	_ = s.graphSvc.SyncNode("user", p.UserID, p.UserID, map[string]interface{}{"user_id": p.UserID})
	_ = s.graphSvc.SyncNode("profile", nodeID, p.AttributeValue, map[string]interface{}{
		"category":       p.Category,
		"character_id":   p.CharacterID,
		"confidence":     p.Confidence,
		"user_id":        p.UserID,
		"source_conv_id": p.SourceConvID,
	})
	_ = s.graphSvc.SyncEdge("user:"+p.UserID, "profile:"+nodeID, "has_profile", float64(p.Confidence)/100.0)
}
