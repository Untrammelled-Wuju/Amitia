// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/prompt/textlib"
)

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

func (s *service) GenerateEpisode(dialogue, characterID, userID string) (map[string]interface{}, error) {
	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("no active model")
	}

	userMsg := fmt.Sprintf(textlib.MemoryEpisodeUserMsgTemplate, dialogue)
	messages := []map[string]interface{}{
		{"role": "system", "content": textlib.MemoryEpisodesystemPrompt},
		{"role": "user", "content": userMsg},
	}
	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		return nil, err
	}

	content = extractJSONObject(content)
	var episode struct {
		Summary         string   `json:"summary"`
		EmotionKeywords []string `json:"emotionKeywords"`
		KeyQuote        string   `json:"keyQuote"`
		TimeContext     string   `json:"timeContext"`
	}
	if err := json.Unmarshal([]byte(content), &episode); err != nil {
		return nil, fmt.Errorf("parse episode: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	id := uuid.New().String()
	emotionsJSON, _ := json.Marshal(episode.EmotionKeywords)
	s.db.Exec(
		"INSERT INTO episodic_memories (id, user_id, character_id, summary, emotion_keywords, key_quote, time_context, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, userID, characterID, episode.Summary, string(emotionsJSON), episode.KeyQuote, episode.TimeContext, now,
	)

	return map[string]interface{}{
		"id":              id,
		"summary":         episode.Summary,
		"emotionKeywords": episode.EmotionKeywords,
		"keyQuote":        episode.KeyQuote,
		"timeContext":     episode.TimeContext,
		"createdAt":       now,
	}, nil
}

func (s *service) InferUserDimensions(characterID string) (map[string]interface{}, error) {
	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("no active model")
	}

	var facts []struct {
		Value string
	}
	s.db.Table("memories").
		Select("value").
		Where("character_id = ? AND verified_status != 'replaced' AND verified_status != 'tombstone'", characterID).
		Order("importance DESC, created_at DESC").Limit(100).Find(&facts)

	if len(facts) == 0 {
		return map[string]interface{}{"E": nil, "A": nil, "D": nil, "P": nil, "N": nil, "O": nil}, nil
	}

	textLines := make([]string, 0, len(facts))
	for _, f := range facts {
		textLines = append(textLines, f.Value)
	}
	text := ""
	for _, line := range textLines {
		text += line + "\n"
	}
	charCount := len([]rune(text))

	userMsg := fmt.Sprintf(textlib.MemorySixDimensionUserMsgTemplate, charCount, text)
	messages := []map[string]interface{}{
		{"role": "system", "content": textlib.MemorySixDimensionSystemPrompt},
		{"role": "user", "content": userMsg},
	}
	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		return nil, err
	}

	content = extractJSONObject(content)
	var dimensions map[string]interface{}
	if err := json.Unmarshal([]byte(content), &dimensions); err != nil {
		return nil, fmt.Errorf("parse dimensions: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	dimsJSON, _ := json.Marshal(dimensions)
	s.db.Exec(
		"INSERT INTO user_dimensions (id, character_id, dimensions, created_at) VALUES (?, ?, ?, ?)",
		uuid.New().String(), characterID, string(dimsJSON), now,
	)

	return dimensions, nil
}
