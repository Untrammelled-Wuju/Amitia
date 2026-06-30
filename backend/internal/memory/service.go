// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type Service interface {
	List(q MemoryListQuery) (*MemoryListResponse, error)
	Create(req *CreateMemoryRequest) (*Memory, error)
	Update(id string, req *UpdateMemoryRequest) (*Memory, error)
	Delete(id string) error
	DeleteAll(characterID string) error
	Search(req *SearchMemoryRequest) ([]Memory, error)
	VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error)
	HybridSearch(req *VectorSearchRequest) ([]HybridSearchResult, error)
	RecordUse(id string) (*Memory, error)
	GetVectorStatus() map[string]interface{}
	GetTimeline(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error)
	GenerateCandidates(conversationID string) ([]MemoryCandidate, error)
	ListCandidates() []MemoryCandidate
	AcceptCandidate(id string) (*Memory, error)
	RejectCandidate(id string) error
	BatchAcceptCandidates(ids []string) ([]Memory, error)
	UpdateCandidate(id string, req *UpdateCandidateRequest) (*MemoryCandidate, error)
	DeleteCandidate(id string) error
	CheckConflict(req *CheckConflictRequest) (*CheckConflictResponse, error)
	ResolveConflict(req *ResolveConflictRequest) (*ResolveConflictResponse, error)
	AutoResolveConflict(key, value, characterID string, newConfidence int) (*ResolveConflictResponse, error)
	GetRankedMemories(characterID, query string, limit int) ([]RankedMemory, error)
	ExtractCandidates() ([]MemoryCandidate, error)
	RebuildIndex() (map[string]interface{}, error)
	RebuildEmbeddings() (map[string]interface{}, error)
	SyncEmbedding(memID, key, value, characterID, memoryType string) bool
	SyncGraphMemory(id string) bool
	BatchVerify(ids []string, status string) error
	BatchSetImportance(ids []string, importance int) error
	RetrieveStats() (map[string]interface{}, error)
}

type RankedMemory struct {
	Memory         Memory  `json:"memory"`
	FinalScore     float64 `json:"finalScore"`
	VectorScore    float64 `json:"vectorScore"`
	KeywordScore   float64 `json:"keywordScore"`
	ImportanceNorm float64 `json:"importanceNorm"`
}

type UpdateCandidateRequest struct {
	Key        *string `json:"key"`
	Value      *string `json:"value"`
	MemoryType *string `json:"memoryType"`
	Importance *int    `json:"importance"`
}

type CheckConflictRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	MemoryType  string `json:"memoryType"`
	Importance  int    `json:"importance"`
	CharacterID string `json:"characterId"`
}

type CheckConflictResponse struct {
	HasConflict bool           `json:"hasConflict"`
	Conflicts   []ConflictItem `json:"conflicts"`
}

type ConflictItem struct {
	Memory Memory `json:"memory"`
	Reason string `json:"reason"`
}

type ResolveConflictRequest struct {
	Action      string `json:"action"`
	NewKey      string `json:"newKey"`
	NewValue    string `json:"newValue"`
	NewType     string `json:"newType"`
	Importance  int    `json:"importance"`
	CharacterID string `json:"characterId"`
	ConflictID  string `json:"conflictId"`
}

type ResolveConflictResponse struct {
	Resolved bool   `json:"resolved"`
	MemoryID string `json:"memoryId"`
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
	Memory         Memory  `json:"memory"`
	Score          float32 `json:"score"`
	CollectionName string  `json:"collectionName"`
	MemoryLayer    string  `json:"memoryLayer"`
	MatchType      string  `json:"matchType,omitempty"`
}

type HybridSearchResult struct {
	Memory         Memory  `json:"memory"`
	Score          float64 `json:"score"`
	VectorScore    float64 `json:"vectorScore"`
	KeywordScore   float64 `json:"keywordScore"`
	MatchType      string  `json:"matchType"`
	CollectionName string  `json:"collectionName"`
	MemoryLayer    string  `json:"memoryLayer"`
}

type service struct {
	repo         Repository
	db           *gorm.DB
	embeddingSvc *embedding.Service
	graphSvc     graph.Service
}

func NewService(repo Repository, ctx *app.AppContext, graphSvc ...graph.Service) Service {
	var gs graph.Service
	if len(graphSvc) > 0 {
		gs = graphSvc[0]
	}
	return &service{
		repo:         repo,
		db:           ctx.DB,
		embeddingSvc: embedding.NewService(ctx.DB),
		graphSvc:     gs,
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
	if req.Confidence < 0 {
		req.Confidence = 0
	}
	if req.Confidence > 100 {
		req.Confidence = 100
	}
	if req.Confidence == 0 {
		req.Confidence = 50
	}
	if req.VerifiedStatus == "" {
		req.VerifiedStatus = "unverified"
	}
	if req.Scope == "" {
		req.Scope = "character"
	}

	resp, err := s.AutoResolveConflict(req.Key, req.Value, req.CharacterID, req.Confidence)
	if err == nil && resp != nil && resp.Resolved {
		return s.repo.FindByID(resp.MemoryID)
	}

	var expiresAt *string
	if req.ExpiresAt != "" {
		expiresAt = &req.ExpiresAt
	}

	m := &Memory{
		CharacterID:    req.CharacterID,
		MemoryType:     req.MemoryType,
		Source:         req.Source,
		Scope:          req.Scope,
		Key:            req.Key,
		Value:          req.Value,
		Importance:     req.Importance,
		Confidence:     req.Confidence,
		ExpiresAt:      expiresAt,
		EntityID:       req.EntityID,
		EntityType:     req.EntityType,
		SourceMsgID:    req.SourceMsgID,
		SourceConvID:   req.SourceConvID,
		VerifiedStatus: req.VerifiedStatus,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)

	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) Update(id string, req *UpdateMemoryRequest) (*Memory, error) {
	before, _ := s.repo.FindByID(id)
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
	if req.Confidence != nil {
		updates["confidence"] = *req.Confidence
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.EntityID != nil {
		updates["entity_id"] = *req.EntityID
	}
	if req.EntityType != nil {
		updates["entity_type"] = *req.EntityType
	}
	if req.VerifiedStatus != nil {
		updates["verified_status"] = *req.VerifiedStatus
	}
	if req.Scope != nil {
		updates["scope"] = *req.Scope
	}
	if len(updates) == 0 {
		return s.repo.FindByID(id)
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if before != nil && before.MemoryType != m.MemoryType {
		deleteVectorsFromCollections([]string{m.ID}, collectionNameForMemoryType(before.MemoryType))
	}
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	s.logEvent(m.ID, "memory_edited", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) Delete(id string) error {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	s.logEvent(id, "memory_deleted", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	deleteVectorsFromCollections([]string{id}, qdrantDB.CollectionNames()...)
	s.deleteGraph(m)
	return s.repo.Delete(id)
}

func (s *service) DeleteAll(characterID string) error {
	var ids []string
	query := s.db.Model(&Memory{})
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	query.Pluck("id", &ids)
	if characterID != "" {
		s.logEvent("", "memory_deleted_all", "", "", "", 0, "", characterID)
	}
	deleteVectorsFromCollections(ids, qdrantDB.CollectionNames()...)
	if s.graphSvc != nil {
		for _, id := range ids {
			_ = s.graphSvc.DeleteNode("memory:" + id)
		}
	}
	return s.repo.DeleteAll(characterID)
}

func (s *service) Search(req *SearchMemoryRequest) ([]Memory, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	return s.repo.Search(req.Keyword, req.CharacterID, limit)
}

func (s *service) VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error) {
	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("向量数据库未初始化")
	}
	queryText := req.Query
	if queryText == "" {
		queryText = req.Keyword
	}
	if queryText == "" {
		return nil, fmt.Errorf("缺少查询文本")
	}
	vector, err := s.embeddingSvc.Embed(queryText)
	if err != nil {
		return nil, fmt.Errorf("向量化失败: %w", err)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	results, err := qdrantDB.MultiSearch(vector, limit+1, nil)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Point.Score > results[j].Point.Score
	})
	var vsResults []VectorSearchResult
	seen := map[string]bool{}
	for _, r := range results {
		memID := ""
		if val, ok := r.Point.Payload["memory_id"]; ok {
			memID = val.GetStringValue()
		}
		if memID == "" || seen[memID] {
			continue
		}
		m, err := s.repo.FindByID(memID)
		if err != nil {
			continue
		}
		if req.CharacterID != "" && m.CharacterID != req.CharacterID {
			continue
		}
		seen[memID] = true
		vsResults = append(vsResults, VectorSearchResult{
			Memory:         *m,
			Score:          float32(r.Point.Score),
			CollectionName: r.CollectionName,
			MemoryLayer:    memoryLayerLabel(collectionKeyFromCollectionName(r.CollectionName)),
			MatchType:      "vector",
		})
		if len(vsResults) >= limit {
			break
		}
	}
	return vsResults, nil
}

func (s *service) HybridSearch(req *VectorSearchRequest) ([]HybridSearchResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	queryText := req.Query
	if queryText == "" {
		queryText = req.Keyword
	}
	if queryText == "" {
		return nil, fmt.Errorf("缺少查询文本")
	}

	vectorFetchLimit := limit * 2
	if vectorFetchLimit < 20 {
		vectorFetchLimit = 20
	}
	vectorResults, _ := s.VectorSearch(&VectorSearchRequest{
		Query:       queryText,
		CharacterID: req.CharacterID,
		Limit:       vectorFetchLimit,
	})

	scorer := &RetrievalScorer{}
	pipelineResults := scorer.Pipeline(vectorResults)

	merged := map[string]*struct {
		m              Memory
		vectorScore    float64
		keywordScore   float64
		collectionName string
		matchType      string
	}{}
	for _, pr := range pipelineResults {
		merged[pr.Memory.ID] = &struct {
			m              Memory
			vectorScore    float64
			keywordScore   float64
			collectionName string
			matchType      string
		}{m: pr.Memory, vectorScore: pr.VectorScore, collectionName: pr.CollectionName, matchType: pr.MatchType}
	}

	keywordResults, err := s.repo.Search(queryText, req.CharacterID, limit*2)
	if err != nil {
		keywordResults = nil
	}
	queryLower := strings.ToLower(queryText)
	for _, m := range keywordResults {
		item, exists := merged[m.ID]
		if !exists {
			item = &struct {
				m              Memory
				vectorScore    float64
				keywordScore   float64
				collectionName string
				matchType      string
			}{m: m, collectionName: collectionNameForMemoryType(m.MemoryType), matchType: "keyword"}
			merged[m.ID] = item
		}
		score := keywordMatchScore(queryLower, m.Key, m.Value)
		if score <= 0 {
			score = 0.5
		}
		if score > item.keywordScore {
			item.keywordScore = score
		}
		if item.matchType == "vector" {
			item.matchType = "hybrid"
		}
	}

	results := make([]HybridSearchResult, 0, len(merged))
	for _, item := range merged {
		score := item.vectorScore*0.6 + item.keywordScore*0.4
		if item.matchType == "hybrid" {
			score += 0.1
		}
		collectionKey := collectionKeyFromCollectionName(item.collectionName)
		results = append(results, HybridSearchResult{
			Memory:         item.m,
			Score:          math.Round(score*10000) / 10000,
			VectorScore:    math.Round(item.vectorScore*10000) / 10000,
			KeywordScore:   math.Round(item.keywordScore*10000) / 10000,
			MatchType:      item.matchType,
			CollectionName: item.collectionName,
			MemoryLayer:    memoryLayerLabel(collectionKey),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}

	memoryIDs := make([]string, len(results))
	for i, r := range results {
		memoryIDs[i] = r.Memory.ID
	}

	s.logRetrieval(req.CharacterID, queryText, memoryIDs, results)

	return results, nil
}

func (s *service) RecordUse(id string) (*Memory, error) {
	if err := s.repo.RecordUse(id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *service) GetVectorStatus() map[string]interface{} {
	totalMem, embedded := s.repo.VectorStatus()
	collections := make([]map[string]interface{}, 0)
	totalEmbeddings := uint64(0)
	enabled := qdrantDB.Client != nil
	for _, collectionName := range qdrantDB.CollectionNames() {
		count := uint64(0)
		status := "disabled"
		if enabled {
			if c, err := qdrantDB.GetVectorCount(collectionName); err == nil {
				count = c
				status = "ready"
				totalEmbeddings += c
			} else {
				status = "error"
			}
		}
		collectionKey := collectionKeyFromCollectionName(collectionName)
		collections = append(collections, map[string]interface{}{
			"key":             collectionKey,
			"name":            collectionName,
			"label":           memoryLayerLabel(collectionKey),
			"totalEmbeddings": count,
			"status":          status,
		})
	}
	if totalEmbeddings > 0 {
		embedded = int64(totalEmbeddings)
	}
	notEmbedded := totalMem - embedded
	if notEmbedded < 0 {
		notEmbedded = 0
	}
	return map[string]interface{}{
		"totalMemories":   totalMem,
		"totalEmbedded":   embedded,
		"notEmbedded":     notEmbedded,
		"enabled":         enabled,
		"providerName":    "Qdrant",
		"totalEmbeddings": totalEmbeddings,
		"collections":     collections,
	}
}

func (s *service) GenerateCandidates(conversationID string) ([]MemoryCandidate, error) {
	messages, err := s.repo.GetConversationMessages(conversationID)
	if err != nil || len(messages) == 0 {
		return nil, err
	}
	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("no active model")
	}
	conversationText := ""
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		conversationText += role + ": " + content + "\n"
	}
	systemPrompt := `你是一个记忆提取器。从对话中提取值得长期记忆的事实，返回JSON数组。
每条记忆包含：key(关键词标签)、value(记忆内容)、memoryType(类型：personal_info/hobby/preference/fact/plan/habit/relationship)、importance(1-10)、confidence(0-100)

只提取明确的事实信息，不确定的信息confidence低于50。如果没有值得记忆的内容，返回空数组[]。`

	messagesLLM := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": conversationText},
	}
	content, _, err := s.callLLM(cfg, messagesLLM)
	if err != nil {
		return nil, err
	}
	content = extractJSONArray(content)
	var candidates []MemoryCandidate
	if err := json.Unmarshal([]byte(content), &candidates); err != nil {
		return nil, nil
	}
	for i := range candidates {
		candidates[i].ID = uuid.New().String()
		candidates[i].SourceText = conversationText
		candidates[i].ConversationID = conversationID
		candidates[i].CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		model := &MemoryCandidateModel{
			ID: candidates[i].ID, Key: candidates[i].Key, Value: candidates[i].Value,
			MemoryType: candidates[i].MemoryType, Importance: candidates[i].Importance,
			SourceText: candidates[i].SourceText, ConversationID: candidates[i].ConversationID,
			CreatedAt: candidates[i].CreatedAt,
		}
		if err := s.repo.CreateCandidate(model); err != nil {
			log.Error("保存候选记忆失败:", err)
		}
	}
	return candidates, nil
}

func (s *service) ListCandidates() []MemoryCandidate {
	models, err := s.repo.ListCandidates()
	if err != nil || len(models) == 0 {
		return []MemoryCandidate{}
	}
	result := make([]MemoryCandidate, len(models))
	for i, m := range models {
		result[i] = MemoryCandidate{
			ID: m.ID, Key: m.Key, Value: m.Value,
			MemoryType: m.MemoryType, Importance: m.Importance,
			SourceText: m.SourceText, ConversationID: m.ConversationID,
			CreatedAt: m.CreatedAt,
		}
	}
	return result
}

func (s *service) AcceptCandidate(id string) (*Memory, error) {
	model, err := s.repo.GetCandidateByID(id)
	if err != nil || model == nil {
		return nil, fmt.Errorf("候选记忆不存在")
	}
	m := &Memory{
		CharacterID:  model.CharacterID,
		MemoryType:   model.MemoryType,
		Source:       "auto",
		Key:          model.Key,
		Value:        model.Value,
		Importance:   model.Importance,
		Confidence:   50,
		SourceConvID: model.ConversationID,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, err
	}
	s.repo.DeleteCandidate(id)
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) RejectCandidate(id string) error {
	return s.repo.DeleteCandidate(id)
}

func (s *service) BatchAcceptCandidates(ids []string) ([]Memory, error) {
	var memories []Memory
	for _, id := range ids {
		m, err := s.AcceptCandidate(id)
		if err != nil {
			continue
		}
		memories = append(memories, *m)
	}
	return memories, nil
}

func (s *service) UpdateCandidate(id string, req *UpdateCandidateRequest) (*MemoryCandidate, error) {
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
	if req.Importance != nil {
		updates["importance"] = *req.Importance
	}
	if err := s.repo.UpdateCandidate(id, updates); err != nil {
		return nil, err
	}
	model, err := s.repo.GetCandidateByID(id)
	if err != nil {
		return nil, err
	}
	return &MemoryCandidate{
		ID: model.ID, Key: model.Key, Value: model.Value,
		MemoryType: model.MemoryType, Importance: model.Importance,
		SourceText: model.SourceText, ConversationID: model.ConversationID,
		CreatedAt: model.CreatedAt,
	}, nil
}

func (s *service) DeleteCandidate(id string) error {
	return s.RejectCandidate(id)
}

func (s *service) CheckConflict(req *CheckConflictRequest) (*CheckConflictResponse, error) {
	existing, err := s.repo.SearchByKey(req.Key, req.CharacterID)
	if err != nil {
		return &CheckConflictResponse{HasConflict: false}, nil
	}

	var conflicts []ConflictItem

	for _, m := range existing {
		if m.ID == "" {
			continue
		}

		if m.Key == req.Key && m.Value == req.Value {
			conflicts = append(conflicts, ConflictItem{Memory: m, Reason: "exact_match"})
			continue
		}

		if m.Key == req.Key && m.Value != req.Value {
			sim := jaccardSimilarity(req.Value, m.Value)
			if sim > 0.85 {
				conflicts = append(conflicts, ConflictItem{Memory: m, Reason: fmt.Sprintf("semantic_similar(%.2f)", sim)})
				continue
			}
		}

		if m.Key == req.Key {
			isContradict, _ := s.llmCheckContradiction(req.Value, m.Value)
			if isContradict {
				conflicts = append(conflicts, ConflictItem{Memory: m, Reason: "llm_contradiction"})
			}
		}
	}

	return &CheckConflictResponse{
		HasConflict: len(conflicts) > 0,
		Conflicts:   conflicts,
	}, nil
}

func (s *service) ResolveConflict(req *ResolveConflictRequest) (*ResolveConflictResponse, error) {
	resp := &ResolveConflictResponse{Resolved: true}
	var existing *Memory
	if req.ConflictID != "" {
		if m, err := s.repo.FindByID(req.ConflictID); err == nil {
			existing = m
		}
	}
	newKey := req.NewKey
	if newKey == "" && existing != nil {
		newKey = existing.Key
	}
	newType := req.NewType
	if newType == "" {
		if existing != nil && existing.MemoryType != "" {
			newType = existing.MemoryType
		} else {
			newType = "custom"
		}
	}
	characterID := req.CharacterID
	if characterID == "" && existing != nil {
		characterID = existing.CharacterID
	}
	importance := req.Importance
	if importance == 0 && existing != nil {
		importance = existing.Importance
	}
	if importance < 0 {
		importance = 0
	}
	if importance > 10 {
		importance = 10
	}

	switch req.Action {
	case "replace", "replace_old":
		if req.ConflictID != "" {
			if existing != nil {
				s.deleteGraph(existing)
			}
			s.repo.Delete(req.ConflictID)
		}
		m := &Memory{
			Key: newKey, Value: req.NewValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
		resp.MemoryID = m.ID
		return resp, nil
	case "keep_existing", "keep_old":
		resp.MemoryID = req.ConflictID
		return resp, nil
	case "keep_both":
		m := &Memory{
			Key: newKey, Value: req.NewValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
		resp.MemoryID = m.ID
		return resp, nil
	case "merge":
		newValue := req.NewValue
		if existing != nil {
			newValue = existing.Value + "; " + req.NewValue
			updates := map[string]interface{}{
				"value":            newValue,
				"importance":       importance,
				"confidence":       maxInt(existing.Confidence, 50),
				"verified_status":  "user_verified",
				"last_verified_at": time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := s.repo.Update(existing.ID, updates); err != nil {
				return nil, err
			}
			go s.SyncEmbedding(existing.ID, newKey, newValue, characterID, newType)
			if updated, err := s.repo.FindByID(existing.ID); err == nil {
				s.syncGraph(updated)
			}
			resp.MemoryID = existing.ID
			return resp, nil
		}
		m := &Memory{
			Key: newKey, Value: newValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
		resp.MemoryID = m.ID
		return resp, nil
	default:
		return nil, fmt.Errorf("未知的冲突解决动作: %s", req.Action)
	}
}

func (s *service) AutoResolveConflict(key, value, characterID string, newConfidence int) (*ResolveConflictResponse, error) {
	existing, err := s.repo.SearchByKey(key, characterID)
	if err != nil || len(existing) == 0 {
		return &ResolveConflictResponse{Resolved: false}, nil
	}

	for _, m := range existing {
		if m.Key != key {
			continue
		}
		if m.Value == value {
			if newConfidence > m.Confidence+10 {
				s.repo.Update(m.ID, map[string]interface{}{
					"confidence": newConfidence,
					"updated_at": time.Now().Format("2006-01-02 15:04:05"),
				})
				if updated, err := s.repo.FindByID(m.ID); err == nil {
					s.syncGraph(updated)
				}
			}
			return &ResolveConflictResponse{Resolved: true, MemoryID: m.ID}, nil
		}
		confDiff := newConfidence - m.Confidence
		if confDiff >= 40 {
			s.deleteGraph(&m)
			s.repo.Delete(m.ID)
			return &ResolveConflictResponse{Resolved: false}, nil
		}
	}

	return &ResolveConflictResponse{Resolved: false}, nil
}

func (s *service) GetRankedMemories(characterID, query string, limit int) ([]RankedMemory, error) {
	if limit <= 0 {
		limit = 10
	}

	allMemories, _, err := s.repo.List(MemoryListQuery{
		CharacterID: characterID,
		PageSize:    200,
		Page:        1,
	})
	if err != nil {
		return nil, err
	}

	vectorScores := make(map[string]float64)
	if qdrantDB.Client != nil && query != "" {
		vector, err := s.embeddingSvc.Embed(query)
		if err == nil {
			results, err := qdrantDB.MultiSearch(vector, 50, nil)
			if err == nil {
				for _, r := range results {
					if val, ok := r.Point.Payload["memory_id"]; ok {
						rawMemID := val.GetStringValue()
						if float64(r.Point.Score) > vectorScores[rawMemID] {
							vectorScores[rawMemID] = float64(r.Point.Score)
						}
					}
				}
			}
		}
	}

	queryLower := strings.ToLower(query)
	var ranked []RankedMemory
	for _, m := range allMemories {
		vs := vectorScores[m.ID]
		ks := keywordMatchScore(queryLower, m.Key, m.Value)
		is := float64(m.Importance) / 10.0

		finalScore := vs*0.4 + ks*0.3 + is*0.3
		if finalScore > 0 {
			ranked = append(ranked, RankedMemory{
				Memory:         m,
				FinalScore:     math.Round(finalScore*10000) / 10000,
				VectorScore:    math.Round(vs*10000) / 10000,
				KeywordScore:   math.Round(ks*10000) / 10000,
				ImportanceNorm: math.Round(is*10000) / 10000,
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}

func (s *service) BatchVerify(ids []string, status string) error {
	for _, id := range ids {
		now := time.Now().Format("2006-01-02 15:04:05")
		s.repo.Update(id, map[string]interface{}{
			"verified_status":  status,
			"last_verified_at": now,
		})
	}
	return nil
}

func (s *service) BatchSetImportance(ids []string, importance int) error {
	for _, id := range ids {
		s.repo.Update(id, map[string]interface{}{"importance": importance})
	}
	return nil
}

func keywordMatchScore(query, key, value string) float64 {
	keyLower := strings.ToLower(key)
	valLower := strings.ToLower(value)

	if strings.Contains(keyLower, query) || strings.Contains(valLower, query) {
		return 1.0
	}

	queryWords := strings.Fields(query)
	matchCount := 0
	for _, w := range queryWords {
		if strings.Contains(keyLower, w) || strings.Contains(valLower, w) {
			matchCount++
		}
	}
	if len(queryWords) > 0 {
		return float64(matchCount) / float64(len(queryWords))
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func jaccardSimilarity(a, b string) float64 {
	wordsA := make(map[string]bool)
	wordsB := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(a)) {
		wordsA[w] = true
	}
	for _, w := range strings.Fields(strings.ToLower(b)) {
		wordsB[w] = true
	}
	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}
	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func (s *service) llmCheckContradiction(newVal, oldVal string) (bool, error) {
	cfg := s.getActiveModel()
	if cfg == nil {
		return false, nil
	}
	prompt := fmt.Sprintf(`判断以下两条记忆是否存在矛盾。存在矛盾返回true，不矛盾返回false。
记忆A: %s
记忆B: %s
只返回true或false。`, newVal, oldVal)

	messages := []map[string]interface{}{
		{"role": "user", "content": prompt},
	}
	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(content)), "true"), nil
}

func (s *service) ExtractCandidates() ([]MemoryCandidate, error) {
	return s.ListCandidates(), nil
}

func (s *service) RebuildIndex() (map[string]interface{}, error) {
	return s.RebuildEmbeddings()
}

func (s *service) RebuildEmbeddings() (map[string]interface{}, error) {
	totalMem, embedded := s.repo.VectorStatus()
	if qdrantDB.Client == nil {
		var memories []Memory
		s.db.Find(&memories)
		for _, m := range memories {
			s.syncGraph(&m)
		}
		return map[string]interface{}{
			"totalMemories": totalMem,
			"embedded":      embedded,
			"status":        "qdrant_not_available",
		}, nil
	}
	var memories []Memory
	s.db.Find(&memories)
	successCount := 0
	failCount := 0
	for _, m := range memories {
		if s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType) {
			successCount++
		} else {
			failCount++
		}
		s.syncGraph(&m)
	}
	status := "completed"
	if failCount > 0 {
		status = "partial_failed"
	}
	return map[string]interface{}{
		"totalMemories": totalMem,
		"embedded":      int64(successCount),
		"failed":        int64(failCount),
		"status":        status,
	}, nil
}

func (s *service) syncGraph(m *Memory) {
	if s.graphSvc == nil || m == nil {
		return
	}
	userID := "default"
	if m.Scope == "user" && m.CharacterID != "" {
		userID = m.CharacterID
	}
	label := strings.TrimSpace(m.Key)
	if label == "" {
		label = strings.TrimSpace(m.Value)
	}
	_ = s.graphSvc.SyncNode("memory", m.ID, label, map[string]interface{}{
		"key":             m.Key,
		"value":           m.Value,
		"memory_type":     m.MemoryType,
		"source":          m.Source,
		"scope":           m.Scope,
		"importance":      m.Importance,
		"confidence":      m.Confidence,
		"character_id":    m.CharacterID,
		"user_id":         userID,
		"entity_id":       m.EntityID,
		"entity_type":     m.EntityType,
		"source_msg_id":   m.SourceMsgID,
		"source_conv_id":  m.SourceConvID,
		"verified_status": m.VerifiedStatus,
		"created_at":      m.CreatedAt,
		"updated_at":      m.UpdatedAt,
	})
	_ = s.graphSvc.SyncNode("user", userID, userID, map[string]interface{}{"user_id": userID})
	_ = s.graphSvc.SyncEdge("user:"+userID, "memory:"+m.ID, "remembers", float64(m.Importance)/10.0)
	if m.CharacterID != "" {
		_ = s.graphSvc.SyncNode("character", m.CharacterID, m.CharacterID, map[string]interface{}{"character_id": m.CharacterID, "user_id": userID})
		_ = s.graphSvc.SyncEdge("character:"+m.CharacterID, "memory:"+m.ID, "has_memory", float64(m.Confidence)/100.0)
	}
	if m.EntityID != "" {
		entityType := strings.TrimSpace(m.EntityType)
		if entityType == "" {
			entityType = "entity"
		}
		_ = s.graphSvc.SyncNode(entityType, m.EntityID, m.EntityID, map[string]interface{}{"user_id": userID})
		_ = s.graphSvc.SyncEdge("memory:"+m.ID, entityType+":"+m.EntityID, "mentions", 1.0)
	}
}

func (s *service) deleteGraph(m *Memory) {
	if s.graphSvc == nil || m == nil {
		return
	}
	_ = s.graphSvc.DeleteNode("memory:" + m.ID)
	if m.CharacterID != "" {
		_ = s.graphSvc.DeleteNodeIfOrphan("character:" + m.CharacterID)
	}
	if m.EntityID != "" {
		entityType := strings.TrimSpace(m.EntityType)
		if entityType == "" {
			entityType = "entity"
		}
		_ = s.graphSvc.DeleteNodeIfOrphan(entityType + ":" + m.EntityID)
	}
}

func (s *service) SyncEmbedding(memID, key, value, characterID, memoryType string) bool {
	if qdrantDB.Client == nil {
		return false
	}
	text := key + " " + value
	vector, err := s.embeddingSvc.Embed(text)
	if err != nil {
		log.Error("生成嵌入失败:", memID, err)
		return false
	}

	payload := map[string]interface{}{
		"memory_id":    memID,
		"character_id": characterID,
		"memory_type":  memoryType,
		"key":          key,
		"value":        value,
	}
	collectionName := collectionNameForMemoryType(memoryType)
	err = qdrantDB.UpsertVectors([]qdrantDB.VectorPoint{
		{ID: memID, Vector: vector, Payload: payload},
	}, collectionName)
	if err != nil {
		log.Error("存储嵌入失败:", memID, err)
		return false
	}
	if err := s.repo.MarkEmbedded(memID); err != nil {
		log.Warn("标记嵌入状态失败:", memID, err)
	}
	return true
}

func (s *service) SyncGraphMemory(id string) bool {
	m, err := s.repo.FindByID(id)
	if err != nil || m == nil {
		return false
	}
	s.syncGraph(m)
	return true
}

func collectionNameForMemoryType(memoryType string) string {
	return qdrantDB.ResolveConfiguredCollection(collectionKeyForMemoryType(memoryType))
}

func collectionKeyForMemoryType(memoryType string) string {
	switch strings.ToLower(memoryType) {
	case "working_memory", "working", "summary", "current_summary":
		return "working_memory"
	case "profile", "user_profile", "personal_info", "hobby", "preference", "habit", "relationship", "nickname":
		return "user_profiles"
	case "episodic", "episode", "event", "moment", "scene":
		return "episodic_memories"
	default:
		return "memory_embeddings"
	}
}

func collectionKeyFromCollectionName(collectionName string) string {
	keys := []string{"memory_embeddings", "working_memory", "user_profiles", "episodic_memories"}
	for _, key := range keys {
		if qdrantDB.ResolveConfiguredCollection(key) == collectionName {
			return key
		}
	}
	return collectionName
}

func memoryLayerLabel(collectionKey string) string {
	switch collectionKey {
	case "working_memory":
		return "当前摘要"
	case "user_profiles":
		return "用户画像"
	case "episodic_memories":
		return "情景回忆"
	default:
		return "事实记忆"
	}
}

func deleteVectorsFromCollections(ids []string, collectionNames ...string) {
	if qdrantDB.Client == nil || len(ids) == 0 {
		return
	}
	if len(collectionNames) == 0 {
		collectionNames = qdrantDB.CollectionNames()
	}
	for _, collectionName := range collectionNames {
		if collectionName == "" {
			continue
		}
		if err := qdrantDB.DeleteVectors(ids, collectionName); err != nil {
			log.Warn("删除向量失败:", collectionName, err)
		}
	}
}

func (s *service) logEvent(memoryID, eventType, key, value, memoryType string, importance int, source, characterID string) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec(
		"INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, memoryID, eventType, key, value, memoryType, importance, source, characterID, now,
	)
}

func (s *service) GetTimeline(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}

	var allEvents []map[string]interface{}

	if timelineType == "" || timelineType == "memory" || timelineType == "structured" {
		query := s.db.Table("memory_events")
		if source != "" {
			query = query.Where("source = ?", source)
		}
		if memoryType != "" {
			query = query.Where("memory_type = ?", memoryType)
		}
		var events []map[string]interface{}
		err := query.Order("created_at DESC").Find(&events).Error
		if err != nil {
			return nil, 0, err
		}
		if events == nil {
			events = []map[string]interface{}{}
		}
		for _, e := range events {
			e["timelineType"] = "memory"
			allEvents = append(allEvents, e)
		}
	}

	if timelineType == "" || timelineType == "episodic" {
		var episodics []map[string]interface{}
		eq := s.db.Table("episodic_memories")
		if userID != "" {
			eq = eq.Where("user_id = ?", userID)
		}
		err := eq.Order("created_at DESC").Find(&episodics).Error
		if err != nil {
			return nil, 0, err
		}
		if episodics == nil {
			episodics = []map[string]interface{}{}
		}
		for _, e := range episodics {
			e["timelineType"] = "episodic"
			allEvents = append(allEvents, e)
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		ti, _ := allEvents[i]["created_at"].(string)
		tj, _ := allEvents[j]["created_at"].(string)
		if ti == "" {
			ti2, _ := allEvents[i]["createdAt"].(string)
			tj2, _ := allEvents[j]["createdAt"].(string)
			return ti2 > tj2
		}
		return ti > tj
	})

	total := int64(len(allEvents))
	start := (page - 1) * pageSize
	if start >= int(total) {
		return []map[string]interface{}{}, total, nil
	}
	end := start + pageSize
	if end > int(total) {
		end = int(total)
	}
	return allEvents[start:end], total, nil
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

func (s *service) Name() string { return "结构化事实" }

func (s *service) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	candidates, err := s.GenerateCandidates(convID)
	if err != nil || len(candidates) == 0 {
		return nil
	}
	existingKeys := make(map[string]bool)
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
		mem, err := s.AcceptCandidate(c.ID)
		if err == nil && mem != nil {
			s.SyncEmbedding(mem.ID, mem.Key, mem.Value, mem.CharacterID, mem.MemoryType)
		}
	}
	return nil
}

func (s *service) logRetrieval(characterID, queryText string, memoryIDs []string, results []HybridSearchResult) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	memIDsJSON, _ := json.Marshal(memoryIDs)
	scoringDetails := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		scoringDetails = append(scoringDetails, map[string]interface{}{
			"id":         r.Memory.ID,
			"score":      r.Score,
			"matchType":  r.MatchType,
			"memoryType": r.Memory.MemoryType,
			"layer":      r.MemoryLayer,
		})
	}
	detailsJSON, _ := json.Marshal(scoringDetails)
	s.db.Exec(
		"INSERT INTO retrieval_logs (id, conversation_id, query_text, retrieved_memory_ids, scoring_details, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, characterID, queryText, string(memIDsJSON), string(detailsJSON), now,
	)
}

func (s *service) RetrieveStats() (map[string]interface{}, error) {
	type logRow struct {
		QueryText          string `json:"queryText"`
		RetrievedMemoryIDs string `json:"retrievedMemoryIDs"`
		ScoringDetails     string `json:"scoringDetails"`
		CreatedAt          string `json:"createdAt"`
	}
	var rows []logRow
	s.db.Table("retrieval_logs").Order("created_at DESC").Limit(50).Find(&rows)
	if rows == nil {
		rows = []logRow{}
	}
	var total int64
	s.db.Table("retrieval_logs").Count(&total)
	return map[string]interface{}{
		"recentLogs": rows,
		"totalCount": total,
	}, nil
}
