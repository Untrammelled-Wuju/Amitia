// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
)

type RecallIntent struct {
	ExplicitMemoryRequest bool
	Temporal              bool
	Profile               bool
	Episodic              bool
	Relationship          bool
}

type recallCandidate struct {
	Memory         Memory
	Layer          string
	SourceType     string
	CollectionName string
	MatchTypes     map[string]bool
	VectorScore    float64
	KeywordScore   float64
	RRFScore       float64
	IntentBoost    float64
}

type recallSourceItem struct {
	Key        string
	Candidate  recallCandidate
	Source     string
	SourceRank int
	Weight     float64
}

func analyzeRecallIntent(query string) RecallIntent {
	q := strings.ToLower(strings.TrimSpace(query))
	containsAny := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(q, strings.ToLower(w)) {
				return true
			}
		}
		return false
	}
	return RecallIntent{
		ExplicitMemoryRequest: containsAny("记得", "记不记得", "还记得", "之前", "以前", "上次", "曾经", "我说过", "跟你说过", "remember"),
		Temporal:              containsAny("昨天", "今天", "前天", "上周", "上个月", "去年", "前年", "那天", "什么时候", "当时", "之前", "以前", "刚才", "最近"),
		Profile:               containsAny("我喜欢", "我讨厌", "我的名字", "我是谁", "我的习惯", "我的偏好", "我的职业", "我的家", "我住", "关于我"),
		Episodic:              containsAny("发生", "那次", "那件事", "经历", "我们聊", "聊过", "当时", "上次", "之前", "以前"),
		Relationship:          containsAny("关系", "朋友", "同事", "家人", "对象", "伴侣", "认识", "和我", "跟我"),
	}
}

func (s *service) dynamicRecall(req *VectorSearchRequest) ([]HybridSearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("缺少检索请求")
	}
	queryText := strings.TrimSpace(req.Query)
	if queryText == "" {
		queryText = strings.TrimSpace(req.Keyword)
	}
	if queryText == "" {
		return nil, fmt.Errorf("缺少查询文本")
	}
	intent := analyzeRecallIntent(queryText)
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if intent.ExplicitMemoryRequest && limit < 12 {
		limit = 12
	}
	if limit > MaxMemoryInjectTotal {
		limit = MaxMemoryInjectTotal
	}

	fetchLimit := maxInt(limit*4, 30)
	var sourceItems []recallSourceItem
	blockedMemoryIDs := s.blockedMemoryIDs()
	policy := retrievalAuthorityPolicy{CharacterID: req.CharacterID, UserID: req.UserID, ProactiveMention: req.ProactiveMention, Now: time.Now()}

	vectorResults, _ := s.VectorSearch(&VectorSearchRequest{
		Query: queryText, CharacterID: req.CharacterID, UserID: req.UserID, Limit: fetchLimit,
		ConversationID: req.ConversationID, RequestID: req.RequestID, Channel: req.Channel, ProactiveMention: req.ProactiveMention,
	})
	for rank, r := range vectorResults {
		m := r.Memory
		s.maintainRetentionForMemory(&m, time.Now())
		if !recallMemoryAllowed(m, intent, policy, blockedMemoryIDs) {
			continue
		}
		sourceItems = append(sourceItems, recallSourceItem{Key: "memory:" + m.ID, Source: "vector", SourceRank: rank + 1, Weight: 1.0, Candidate: recallCandidate{
			Memory: m, Layer: memoryLayerLabel(collectionKeyFromCollectionName(r.CollectionName)), SourceType: "memory", CollectionName: r.CollectionName,
			MatchTypes: map[string]bool{"vector": true}, VectorScore: float64(r.Score),
		}})
	}

	keywordResults, _ := s.repo.Search(queryText, req.CharacterID, req.UserID, fetchLimit)
	qLower := strings.ToLower(queryText)
	for rank, m := range keywordResults {
		s.maintainRetentionForMemory(&m, time.Now())
		if !recallMemoryAllowed(m, intent, policy, blockedMemoryIDs) {
			continue
		}
		ks := keywordMatchScore(qLower, m.Key, m.Value)
		if ks <= 0 {
			continue
		}
		sourceItems = append(sourceItems, recallSourceItem{Key: "memory:" + m.ID, Source: "keyword", SourceRank: rank + 1, Weight: 0.9, Candidate: recallCandidate{
			Memory: m, Layer: "事实记忆", SourceType: "memory", CollectionName: collectionNameForMemoryType(m.MemoryType),
			MatchTypes: map[string]bool{"keyword": true}, KeywordScore: ks,
		}})
	}

	sourceItems = append(sourceItems, s.profileRecallItems(queryText, req, intent, fetchLimit)...)
	sourceItems = append(sourceItems, s.episodicRecallItems(queryText, req, intent, fetchLimit)...)
	sourceItems = append(sourceItems, s.graphRecallItems(queryText, req, intent, fetchLimit)...)

	merged := make(map[string]*recallCandidate)
	for _, item := range sourceItems {
		c, exists := merged[item.Key]
		if !exists {
			copyCandidate := item.Candidate
			if copyCandidate.MatchTypes == nil {
				copyCandidate.MatchTypes = map[string]bool{}
			}
			c = &copyCandidate
			merged[item.Key] = c
		} else {
			if item.Candidate.VectorScore > c.VectorScore {
				c.VectorScore = item.Candidate.VectorScore
			}
			if item.Candidate.KeywordScore > c.KeywordScore {
				c.KeywordScore = item.Candidate.KeywordScore
			}
			for mt := range item.Candidate.MatchTypes {
				c.MatchTypes[mt] = true
			}
			if c.SourceType != "memory" && item.Candidate.SourceType == "memory" {
				c.Memory = item.Candidate.Memory
				c.SourceType = "memory"
				c.Layer = item.Candidate.Layer
			}
		}
		c.RRFScore += item.Weight / float64(60+item.SourceRank)
	}

	if len(merged) == 0 {
		s.logRetrieval(req.ConversationID, req.CharacterID, req.RequestID, req.Channel, queryText, nil, nil)
		return []HybridSearchResult{}, nil
	}

	maxRRF := 0.0
	for _, c := range merged {
		if c.RRFScore > maxRRF {
			maxRRF = c.RRFScore
		}
	}
	if maxRRF <= 0 {
		maxRRF = 1
	}

	results := make([]HybridSearchResult, 0, len(merged))
	now := time.Now()
	for _, c := range merged {
		rrfNorm := c.RRFScore / maxRRF
		relevance := math.Max(c.VectorScore, c.KeywordScore)
		importance := clamp01(float64(c.Memory.Importance) / 10.0)
		confidence := clamp01(float64(c.Memory.Confidence) / 100.0)
		intentBoost := recallIntentBoost(intent, *c)
		retStrength := 1.0
		retFactor := 1.0
		if c.SourceType == "memory" || c.SourceType == "episodic" {
			retStrength = memoryEffectiveStrength(c.Memory, now)
			retFactor = retentionFactor(c.Memory, intent.ExplicitMemoryRequest, now)
		}
		score := (0.46*rrfNorm + 0.24*relevance + 0.10*importance + 0.08*confidence + 0.12*intentBoost) * retFactor
		results = append(results, HybridSearchResult{
			Memory: c.Memory, Score: round4(score), VectorScore: round4(c.VectorScore), KeywordScore: round4(c.KeywordScore),
			MatchType: matchTypeLabel(c.MatchTypes), CollectionName: c.CollectionName, MemoryLayer: c.Layer, SourceType: c.SourceType,
			RRFScore: round4(c.RRFScore), RetentionStrength: round4(retStrength),
		})
	}

	if s.temporalReranker != nil && len(results) > 0 {
		candidates := make([]temporal.MemoryScoreCandidate, 0, len(results))
		for _, result := range results {
			candidates = append(candidates, temporal.MemoryScoreCandidate{MemoryID: result.Memory.ID, BaseScore: result.Score, CreatedAt: result.Memory.CreatedAt, MemoryType: result.Memory.MemoryType})
		}
		if reranked, err := s.temporalReranker.RerankMemoryScores(context.Background(), queryText, candidates); err == nil {
			for i := range results {
				if score, ok := reranked[results[i].Memory.ID]; ok {
					results[i].Score = score.FinalScore
					results[i].TemporalBoost = score.TemporalBoost
					results[i].ValidityPenalty = score.ValidityPenalty
					results[i].TemporalReference = score.ReferenceSource
				}
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	results = dedupeRecallResults(results, 0.92)
	results = mmrRecallResults(results, limit, 0.78)

	allCanonical := make([]string, 0)
	for _, c := range merged {
		if c.SourceType == "memory" && c.Memory.ID != "" {
			allCanonical = append(allCanonical, c.Memory.ID)
		}
	}
	s.recordRecallCounters(allCanonical, results)

	memoryIDs := make([]string, 0, len(results))
	for _, r := range results {
		memoryIDs = append(memoryIDs, r.Memory.ID)
	}
	s.logRetrieval(req.ConversationID, req.CharacterID, req.RequestID, req.Channel, queryText, memoryIDs, results)
	return results, nil
}

func recallMemoryAllowed(m Memory, intent RecallIntent, policy retrievalAuthorityPolicy, blocked map[string]bool) bool {
	if blocked != nil && blocked[m.ID] {
		return false
	}
	if !memoryAllowedBySQLiteAuthority(m, policy) {
		return false
	}
	if strings.EqualFold(m.DecayState, DecayStateArchived) && !intent.ExplicitMemoryRequest {
		return false
	}
	return true
}

func (s *service) blockedMemoryIDs() map[string]bool {
	if s.dataLifecycleCoordinator == nil {
		return nil
	}
	ids := s.dataLifecycleCoordinator.BlockedEntityIDsByType("memory")
	if len(ids) == 0 {
		return nil
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

func (s *service) profileRecallItems(query string, req *VectorSearchRequest, intent RecallIntent, limit int) []recallSourceItem {
	if s.db == nil || strings.TrimSpace(req.CharacterID) == "" {
		return nil
	}
	weight := 0.55
	if intent.Profile || intent.Relationship {
		weight = 1.05
	}
	type row struct {
		ID               string
		Category         string
		AttributeName    string
		AttributeValue   string
		Confidence       int
		SourceMemoryID   string
		ProjectionStatus string
		CreatedAt        string
	}
	var rows []row
	like := "%" + query + "%"
	profileScopes := []string{strings.TrimSpace(req.CharacterID)}
	if userID := strings.TrimSpace(req.UserID); userID != "" && userID != strings.TrimSpace(req.CharacterID) {
		profileScopes = append(profileScopes, userID)
	}
	q := s.db.Table("user_profiles").Select("id, category, attribute_name, attribute_value, confidence, source_memory_id, projection_status, created_at").
		Where("(user_id IN ? OR character_id IN ?)", profileScopes, profileScopes).
		Where("(projection_status IS NULL OR projection_status = '' OR projection_status = 'active')")
	if strings.TrimSpace(query) != "" && !intent.Profile && !intent.Relationship {
		q = q.Where("attribute_name LIKE ? OR attribute_value LIKE ?", like, like)
	}
	if err := q.Order("confidence DESC, updated_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil
	}
	items := make([]recallSourceItem, 0, len(rows))
	for rank, r := range rows {
		if r.SourceMemoryID != "" {
			if m, err := s.repo.FindByID(r.SourceMemoryID); err == nil && m != nil {
				s.maintainRetentionForMemory(m, time.Now())
				policy := retrievalAuthorityPolicy{CharacterID: req.CharacterID, UserID: req.UserID, ProactiveMention: req.ProactiveMention, Now: time.Now()}
				if !recallMemoryAllowed(*m, intent, policy, s.blockedMemoryIDs()) {
					continue
				}
				ks := keywordMatchScore(strings.ToLower(query), m.Key, m.Value)
				items = append(items, recallSourceItem{Key: "memory:" + m.ID, Source: "profile", SourceRank: rank + 1, Weight: weight, Candidate: recallCandidate{
					Memory: *m, Layer: "用户画像", SourceType: "memory", CollectionName: collectionNameForMemoryType(m.MemoryType), MatchTypes: map[string]bool{"profile": true}, KeywordScore: ks,
				}})
				continue
			}
		}
		mt, ok := NormalizeMemoryType(r.Category)
		if !ok || mt == "" {
			mt = MemoryTypeFact
		}
		m := Memory{ID: "profile:" + r.ID, CharacterID: req.CharacterID, MemoryType: string(mt), MemorySubtype: strings.ToUpper(r.Category), Key: r.AttributeName, Value: r.AttributeValue,
			Importance: 7, Confidence: r.Confidence, Source: "legacy_profile", Scope: "character", RetentionLevel: RetentionL2, MemoryStrength: clamp01(float64(r.Confidence) / 100), DecayState: DecayStateActive, CreatedAt: r.CreatedAt}
		items = append(items, recallSourceItem{Key: "profile:" + r.ID, Source: "profile", SourceRank: rank + 1, Weight: weight, Candidate: recallCandidate{
			Memory: m, Layer: "用户画像", SourceType: "profile", CollectionName: "user_profiles", MatchTypes: map[string]bool{"profile": true}, KeywordScore: keywordMatchScore(strings.ToLower(query), r.AttributeName, r.AttributeValue),
		}})
	}
	return items
}

func (s *service) episodicRecallItems(query string, req *VectorSearchRequest, intent RecallIntent, limit int) []recallSourceItem {
	if s.db == nil {
		return nil
	}
	userScope := strings.TrimSpace(req.UserID)
	if userScope == "" {
		userScope = strings.TrimSpace(req.CharacterID)
	}
	if userScope == "" {
		return nil
	}
	weight := 0.45
	if intent.Episodic || intent.Temporal || intent.ExplicitMemoryRequest {
		weight = 1.15
	}
	type row struct {
		ID                string
		SceneType         string
		Title             string
		Content           string
		TriggerKeywords   string
		SentimentScore    int
		RetentionLevel    int
		MemoryStrength    float64
		StrengthUpdatedAt *string
		LastReinforcedAt  *string
		ReinforceCount    int
		DecayState        string
		MessageTimeStart  string
		CreatedAt         string
	}
	var rows []row
	q := s.db.Table("episodic_memories").Select("id, scene_type, title, content, trigger_keywords, sentiment_score, retention_level, memory_strength, strength_updated_at, last_reinforced_at, reinforce_count, decay_state, message_time_start, created_at").Where("user_id IN ?", uniqueRecallScopes(userScope, strings.TrimSpace(req.CharacterID)))
	if !intent.ExplicitMemoryRequest {
		q = q.Where("(decay_state IS NULL OR decay_state = '' OR decay_state != ?)", DecayStateArchived)
	}
	like := "%" + query + "%"
	if !intent.Episodic && !intent.Temporal && !intent.ExplicitMemoryRequest {
		q = q.Where("title LIKE ? OR content LIKE ? OR trigger_keywords LIKE ?", like, like, like)
	} else {
		q = q.Where("title LIKE ? OR content LIKE ? OR trigger_keywords LIKE ? OR 1=1", like, like, like)
	}
	if err := q.Order("COALESCE(NULLIF(message_time_start, ''), created_at) DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil
	}
	items := make([]recallSourceItem, 0, len(rows))
	for rank, r := range rows {
		level := normalizeRetentionLevel(r.RetentionLevel)
		strength := r.MemoryStrength
		if strength <= 0 {
			strength = defaultStrengthForLevel(level)
		}
		occurredAt := strings.TrimSpace(r.MessageTimeStart)
		if occurredAt == "" {
			occurredAt = r.CreatedAt
		}
		m := Memory{ID: "episodic:" + r.ID, CharacterID: req.CharacterID, MemoryType: "fact", MemorySubtype: "EPISODIC", Key: r.Title, Value: r.Content, Importance: 6,
			Confidence: 75, Source: "episodic", Scope: "character", RetentionLevel: level, MemoryStrength: strength, StrengthUpdatedAt: r.StrengthUpdatedAt,
			LastReinforcedAt: r.LastReinforcedAt, ReinforceCount: r.ReinforceCount, DecayState: r.DecayState, CreatedAt: occurredAt}
		s.maintainEpisodicRetention(r.ID, &m, time.Now())
		if strings.EqualFold(m.DecayState, DecayStateArchived) && !intent.ExplicitMemoryRequest {
			continue
		}
		ks := keywordMatchScore(strings.ToLower(query), r.Title+" "+r.TriggerKeywords, r.Content)
		items = append(items, recallSourceItem{Key: "episodic:" + r.ID, Source: "episodic", SourceRank: rank + 1, Weight: weight, Candidate: recallCandidate{
			Memory: m, Layer: "情景回忆", SourceType: "episodic", CollectionName: "episodic_memories", MatchTypes: map[string]bool{"episodic": true}, KeywordScore: ks,
		}})
	}
	return items
}

func (s *service) graphRecallItems(query string, req *VectorSearchRequest, intent RecallIntent, limit int) []recallSourceItem {
	if s.graphSvc == nil || (!intent.Relationship && !intent.ExplicitMemoryRequest) {
		return nil
	}
	userScope := strings.TrimSpace(req.UserID)
	if userScope == "" {
		userScope = strings.TrimSpace(req.CharacterID)
	}
	if userScope == "" {
		return nil
	}
	var nodes []map[string]interface{}
	for _, scope := range uniqueRecallScopes(userScope, strings.TrimSpace(req.CharacterID)) {
		scopeNodes, err := s.graphSvc.GetAllNodes(scope)
		if err != nil {
			continue
		}
		nodes = append(nodes, scopeNodes...)
	}
	if len(nodes) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	qLower := strings.ToLower(strings.TrimSpace(query))
	weight := 0.65
	if intent.Relationship {
		weight = 1.15
	}
	blocked := s.blockedMemoryIDs()
	policy := retrievalAuthorityPolicy{CharacterID: req.CharacterID, UserID: req.UserID, ProactiveMention: req.ProactiveMention, Now: time.Now()}
	capacity := limit
	if len(nodes) < capacity {
		capacity = len(nodes)
	}
	items := make([]recallSourceItem, 0, capacity)
	rank := 0
	for _, node := range nodes {
		props, _ := node["properties"].(map[string]interface{})
		if len(props) == 0 {
			continue
		}
		sourceMemoryID := graphStringProperty(props, "source_memory_id")
		if sourceMemoryID == "" {
			continue
		}
		label := strings.TrimSpace(fmt.Sprint(node["label"]))
		graphText := label + " " + fmt.Sprint(props)
		ks := keywordMatchScore(qLower, graphText, "")
		if ks <= 0 {
			continue
		}
		m, err := s.repo.FindByID(sourceMemoryID)
		if err != nil || m == nil {
			continue
		}
		s.maintainRetentionForMemory(m, time.Now())
		if !recallMemoryAllowed(*m, intent, policy, blocked) {
			continue
		}
		rank++
		items = append(items, recallSourceItem{
			Key:        "memory:" + m.ID,
			Source:     "graph",
			SourceRank: rank,
			Weight:     weight,
			Candidate: recallCandidate{
				Memory:         *m,
				Layer:          "关系图谱",
				SourceType:     "memory",
				CollectionName: collectionNameForMemoryType(m.MemoryType),
				MatchTypes:     map[string]bool{"graph": true},
				KeywordScore:   ks,
			},
		})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func uniqueRecallScopes(scopes ...string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		result = append(result, scope)
	}
	return result
}

func graphStringProperty(props map[string]interface{}, key string) string {
	if len(props) == 0 {
		return ""
	}
	value, ok := props[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func recallIntentBoost(intent RecallIntent, c recallCandidate) float64 {
	boost := 0.2
	if intent.Profile && (c.Layer == "用户画像" || c.Memory.MemoryType == "personal_info" || c.Memory.MemoryType == "preference" || c.Memory.MemoryType == "habit") {
		boost += 0.6
	}
	if (intent.Episodic || intent.Temporal || intent.ExplicitMemoryRequest) && c.Layer == "情景回忆" {
		boost += 0.7
	}
	if intent.Relationship && c.Memory.MemoryType == "relationship" {
		boost += 0.7
	}
	if intent.Relationship && c.MatchTypes["graph"] {
		boost += 0.55
	}
	if intent.Temporal && c.Memory.MemorySubtype == "EPISODIC" {
		boost += 0.3
	}
	return clamp01(boost)
}

func matchTypeLabel(types map[string]bool) string {
	if len(types) == 0 {
		return "recall"
	}
	order := []string{"vector", "keyword", "profile", "episodic", "graph"}
	parts := make([]string, 0, len(types))
	for _, key := range order {
		if types[key] {
			parts = append(parts, key)
		}
	}
	if len(parts) == 0 {
		return "recall"
	}
	return strings.Join(parts, "+")
}

func mmrRecallResults(results []HybridSearchResult, limit int, lambda float64) []HybridSearchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	if lambda <= 0 || lambda > 1 {
		lambda = 0.78
	}
	remaining := append([]HybridSearchResult(nil), results...)
	selected := make([]HybridSearchResult, 0, limit)
	for len(remaining) > 0 && len(selected) < limit {
		bestIndex := 0
		bestScore := math.Inf(-1)
		for i, candidate := range remaining {
			maxSimilarity := 0.0
			for _, chosen := range selected {
				similarity := recallTextSimilarity(candidate.Memory.Key+" "+candidate.Memory.Value, chosen.Memory.Key+" "+chosen.Memory.Value)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
			mmrScore := lambda*candidate.Score - (1-lambda)*maxSimilarity
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIndex = i
			}
		}
		selected = append(selected, remaining[bestIndex])
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	return selected
}

func dedupeRecallResults(results []HybridSearchResult, threshold float64) []HybridSearchResult {
	kept := make([]HybridSearchResult, 0, len(results))
	for _, candidate := range results {
		duplicate := false
		for _, existing := range kept {
			if recallTextSimilarity(candidate.Memory.Key+" "+candidate.Memory.Value, existing.Memory.Key+" "+existing.Memory.Value) >= threshold {
				duplicate = true
				break
			}
		}
		if !duplicate {
			kept = append(kept, candidate)
		}
	}
	return kept
}

func recallTextSimilarity(a, b string) float64 {
	makeSet := func(raw string) map[string]bool {
		runes := []rune(strings.ToLower(strings.TrimSpace(raw)))
		set := map[string]bool{}
		for _, word := range strings.Fields(string(runes)) {
			set[word] = true
		}
		for i := 0; i+1 < len(runes); i++ {
			if runes[i] == ' ' || runes[i+1] == ' ' {
				continue
			}
			set[string(runes[i:i+2])] = true
		}
		return set
	}
	left, right := makeSet(a), makeSet(b)
	return jaccardSetSimilarity(left, right)
}

func (s *service) recordRecallCounters(retrievedCanonical []string, _ []HybridSearchResult) {
	if s.db == nil {
		return
	}
	seen := map[string]bool{}
	for _, id := range retrievedCanonical {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		_ = s.db.Model(&Memory{}).Where("id = ?", id).UpdateColumn("retrieved_count", gorm.Expr("retrieved_count + 1")).Error
	}
}

func (s *service) maintainEpisodicRetention(id string, m *Memory, now time.Time) {
	if s == nil || s.db == nil || strings.TrimSpace(id) == "" || m == nil {
		return
	}
	strength := memoryEffectiveStrength(*m, now)
	state, level := decayStateFor(m.RetentionLevel, strength)
	if state == m.DecayState && level == normalizeRetentionLevel(m.RetentionLevel) {
		return
	}
	anchor := now.Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"retention_level":     level,
		"memory_strength":     strength,
		"strength_updated_at": anchor,
		"decay_state":         state,
	}
	if state == DecayStateArchived {
		updates["archived_at"] = anchor
	}
	_ = s.db.Table("episodic_memories").Where("id = ?", id).Updates(updates).Error
	m.RetentionLevel = level
	m.MemoryStrength = strength
	m.StrengthUpdatedAt = &anchor
	m.DecayState = state
	if state == DecayStateArchived {
		m.ArchivedAt = &anchor
		if s.graphSvc != nil {
			_ = s.graphSvc.DeleteNode("episodic:" + id)
		}
	}
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
