// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	List(q WorldBookListQuery) (*WorldBookListResponse, error)
	Create(req *CreateWorldBookRequest) (*WorldBookEntry, error)
	Update(id string, req *UpdateWorldBookRequest) (*WorldBookEntry, error)
	Delete(id string) error
	TestMatch(text string) (*TestMatchResponse, error)
	MatchAndCollect(userMessage, assistantReply string) []MatchResult
	ToSystemPrompt(userMessage, assistantReply string) string
	DeleteAll() error
}

type regexCacheEntry struct {
	re       *regexp.Regexp
	cachedAt time.Time
}

type service struct {
	repo       Repository
	db         *gorm.DB
	graphSvc   graph.Service
	mu         sync.RWMutex
	regexCache map[string]*regexCacheEntry
	rulesTTL   time.Time
	rulesCache []WorldBookEntry
}

func NewService(repo Repository, ctx *app.AppContext, graphSvc graph.Service) Service {
	return &service{
		repo:       repo,
		db:         ctx.DB,
		graphSvc:   graphSvc,
		regexCache: make(map[string]*regexCacheEntry),
	}
}

func (s *service) List(q WorldBookListQuery) (*WorldBookListResponse, error) {
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
	return &WorldBookListResponse{Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

func (s *service) Create(req *CreateWorldBookRequest) (*WorldBookEntry, error) {
	if req.MatchType == "" || req.MatchPattern == "" || req.InjectContent == "" {
		return nil, fmt.Errorf("matchType、matchPattern和injectContent不能为空")
	}
	if req.MatchScope == "" {
		req.MatchScope = "full_context"
	}
	e := &WorldBookEntry{
		MatchType:     req.MatchType,
		MatchPattern:  req.MatchPattern,
		MatchScope:    req.MatchScope,
		InjectContent: req.InjectContent,
		Priority:      req.Priority,
	}
	if err := s.repo.Create(e); err != nil {
		return nil, err
	}
	s.syncGraph(e)
	s.invalidateCache()
	return e, nil
}

func (s *service) Update(id string, req *UpdateWorldBookRequest) (*WorldBookEntry, error) {
	updates := map[string]interface{}{}
	if req.MatchType != nil {
		updates["match_type"] = *req.MatchType
	}
	if req.MatchPattern != nil {
		updates["match_pattern"] = *req.MatchPattern
	}
	if req.MatchScope != nil {
		updates["match_scope"] = *req.MatchScope
	}
	if req.InjectContent != nil {
		updates["inject_content"] = *req.InjectContent
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	s.invalidateCache()
	e, err := s.repo.FindByID(id)
	if err == nil {
		s.syncGraph(e)
	}
	return e, err
}

func (s *service) Delete(id string) error {
	err := s.repo.Delete(id)
	if err == nil {
		s.deleteGraph(id)
		s.invalidateCache()
	}
	return err
}

func (s *service) DeleteAll() error {
	rules := s.loadRules()
	err := s.repo.DeleteAll()
	if err == nil {
		for _, rule := range rules {
			s.deleteGraph(rule.ID)
		}
		s.invalidateCache()
	}
	return err
}

func (s *service) invalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regexCache = make(map[string]*regexCacheEntry)
	s.rulesTTL = time.Time{}
	s.rulesCache = nil
}

func (s *service) loadRules() []WorldBookEntry {
	s.mu.RLock()
	if time.Now().Before(s.rulesTTL) && s.rulesCache != nil {
		rules := s.rulesCache
		s.mu.RUnlock()
		return rules
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.rulesTTL) && s.rulesCache != nil {
		return s.rulesCache
	}
	rules, err := s.repo.GetAll()
	if err != nil {
		rules = []WorldBookEntry{}
	}
	s.rulesCache = rules
	s.rulesTTL = time.Now().Add(5 * time.Minute)
	return rules
}

func (s *service) getRegex(pattern string) (*regexp.Regexp, error) {
	s.mu.RLock()
	entry, exists := s.regexCache[pattern]
	s.mu.RUnlock()
	if exists && time.Since(entry.cachedAt) < 5*time.Minute {
		return entry.re, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists = s.regexCache[pattern]
	if exists && time.Since(entry.cachedAt) < 5*time.Minute {
		return entry.re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	s.regexCache[pattern] = &regexCacheEntry{re: re, cachedAt: time.Now()}
	return re, nil
}

func (s *service) TestMatch(text string) (*TestMatchResponse, error) {
	rules := s.loadRules()
	var results []MatchResult
	for _, rule := range rules {
		scopedText := s.getScopedText(text, text, text, rule.MatchScope)
		if matched, hitText := s.tryMatch(rule, scopedText); matched {
			results = append(results, MatchResult{
				Entry:      rule,
				MatchScope: rule.MatchScope,
				HitText:    hitText,
			})
		}
	}
	if results == nil {
		results = []MatchResult{}
	}
	return &TestMatchResponse{Matches: results}, nil
}

func (s *service) MatchAndCollect(userMessage, assistantReply string) []MatchResult {
	rules := s.loadRules()
	var results []MatchResult
	for _, rule := range rules {
		fullText := userMessage
		if assistantReply != "" {
			fullText = userMessage + "\n" + assistantReply
		}
		scopedText := s.getScopedText(userMessage, assistantReply, fullText, rule.MatchScope)
		if matched, hitText := s.tryMatch(rule, scopedText); matched {
			results = append(results, MatchResult{
				Entry:      rule,
				MatchScope: rule.MatchScope,
				HitText:    hitText,
			})
			go s.repo.IncrementHitCount(rule.ID)
		}
	}
	if s.graphSvc != nil {
		for _, r := range results {
			s.syncGraph(&r.Entry)
			triggerID := triggerNodeID(r.Entry.ID, r.HitText)
			s.graphSvc.SyncNode("worldbook_trigger", triggerID, r.HitText, map[string]interface{}{
				"worldbook_id": r.Entry.ID,
				"hit_text":     r.HitText,
				"match_scope":  r.MatchScope,
				"user_id":      "default",
			})
			s.graphSvc.SyncEdge("worldbook:"+r.Entry.ID, "worldbook_trigger:"+triggerID, "triggered_by", float64(r.Entry.Priority)/10.0)
		}
	}
	return results
}

func (s *service) syncGraph(e *WorldBookEntry) {
	if s.graphSvc == nil || e == nil {
		return
	}
	_ = s.graphSvc.SyncNode("worldbook", e.ID, e.MatchPattern, map[string]interface{}{
		"match_type":     e.MatchType,
		"match_pattern":  e.MatchPattern,
		"match_scope":    e.MatchScope,
		"inject_content": e.InjectContent,
		"priority":       e.Priority,
		"hit_count":      e.HitCount,
		"user_id":        "default",
	})
}

func (s *service) deleteGraph(id string) {
	if s.graphSvc == nil || id == "" {
		return
	}
	_ = s.graphSvc.DeleteNode("worldbook:" + id)
	_ = s.graphSvc.DeleteNodesByProperty("worldbook_trigger", "worldbook_id", id)
}

func triggerNodeID(entryID, hitText string) string {
	sum := sha1.Sum([]byte(entryID + ":" + hitText))
	return fmt.Sprintf("%x", sum)
}

func (s *service) ToSystemPrompt(userMessage, assistantReply string) string {
	matches := s.MatchAndCollect(userMessage, assistantReply)
	if len(matches) == 0 {
		return ""
	}
	sorted := make([]MatchResult, len(matches))
	copy(sorted, matches)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Entry.Priority < sorted[j].Entry.Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	var lines []string
	for _, m := range sorted {
		lines = append(lines, fmt.Sprintf("- %s", m.Entry.InjectContent))
	}
	return "【世界书】\n" + strings.Join(lines, "\n")
}

func (s *service) getScopedText(userMessage, assistantReply, fullText, scope string) string {
	switch scope {
	case "user_message":
		return userMessage
	case "assistant_reply":
		if assistantReply != "" {
			return assistantReply
		}
		return userMessage
	case "full_context":
		return fullText
	default:
		return fullText
	}
}

func (s *service) tryMatch(rule WorldBookEntry, text string) (bool, string) {
	switch rule.MatchType {
	case "regex":
		re, err := s.getRegex(rule.MatchPattern)
		if err != nil {
			return false, ""
		}
		loc := re.FindStringIndex(text)
		if loc != nil {
			start := loc[0] - 30
			if start < 0 {
				start = 0
			}
			end := loc[1] + 30
			if end > len(text) {
				end = len(text)
			}
			return true, text[start:end]
		}
		return false, ""
	case "exact":
		if strings.Contains(text, rule.MatchPattern) {
			idx := strings.Index(text, rule.MatchPattern)
			start := idx - 30
			if start < 0 {
				start = 0
			}
			end := idx + len(rule.MatchPattern) + 30
			if end > len(text) {
				end = len(text)
			}
			return true, text[start:end]
		}
		return false, ""
	case "keyword":
		keywords := strings.Split(rule.MatchPattern, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(text, kw) {
				idx := strings.Index(text, kw)
				start := idx - 30
				if start < 0 {
					start = 0
				}
				end := idx + len(kw) + 30
				if end > len(text) {
					end = len(text)
				}
				return true, text[start:end]
			}
		}
		return false, ""
	default:
		return false, ""
	}
}
