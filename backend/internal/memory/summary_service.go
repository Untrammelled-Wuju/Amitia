package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/log"
)

func buildEntrySummary(key, value, memoryType string, importance int) string {
	if importance >= 8 {
		return fmt.Sprintf("%s: %s", key, value)
	}
	if len(value) > 80 {
		return fmt.Sprintf("%s: %s...", key, value[:80])
	}
	return fmt.Sprintf("%s: %s", key, value)
}

func (s *service) SummarizeMemories(req *MemorySummaryRequest) (*MemorySummaryResult, error) {
	if req == nil || strings.TrimSpace(req.Topic) == "" {
		return nil, fmt.Errorf("topic is required")
	}

	mode := req.Mode
	if mode == "" {
		mode = MemorySummaryModeAuto
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	minImportance := req.MinImportance
	if minImportance <= 0 {
		minImportance = 1
	}

	searchReq := &SearchMemoryRequest{
		Keyword:     req.Topic,
		CharacterID: req.CharacterID,
		Layers:      req.Layers,
		Types:       req.Types,
		Time:        req.Time,
		Limit:       limit * 3,
	}

	mems, err := s.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	filtered := make([]Memory, 0, len(mems))
	for _, m := range mems {
		if m.Importance < minImportance {
			continue
		}
		if req.MinConfidence > 0 && m.Confidence < req.MinConfidence {
			continue
		}
		filtered = append(filtered, m)
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	truncated := len(filtered) >= limit

	evidence := make([]MemoryEvidenceRef, 0)
	if req.IncludeEvidence {
		for _, m := range filtered {
			layer := string(CanonicalLayer(collectionKeyForMemoryType(m.MemoryType)))
			evidence = append(evidence, MemoryEvidenceRef{
				ID:    m.ID,
				Key:   m.Key,
				Layer: layer,
			})
		}
	}

	layers := uniqueLayersFromMemories(filtered)
	types := uniqueTypesFromMemories(filtered)

	var generatedBy string
	var warnings []string

	switch mode {
	case MemorySummaryModeDeterministic:
		generatedBy = "deterministic"
	case MemorySummaryModeModel:
		generatedBy = "model"
	case MemorySummaryModeAuto:
		if len(filtered) <= 5 {
			generatedBy = "deterministic"
		} else {
			generatedBy = "model"
		}
	}

	summary := s.renderSummary(filtered, req.Topic, generatedBy, &warnings)

	return &MemorySummaryResult{
		Summary:       summary,
		EvidenceCount: len(filtered),
		Evidence:      evidence,
		Topic:         req.Topic,
		Layers:        layers,
		Types:         types,
		GeneratedBy:   generatedBy,
		Truncated:     truncated,
		Warnings:      warnings,
	}, nil
}

func (s *service) renderSummary(mems []Memory, topic string, mode string, warnings *[]string) string {
	if len(mems) == 0 {
		return ""
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if mode == "deterministic" {
		var lines []string
		for i, m := range mems {
			matched := strings.Contains(strings.ToLower(m.Key), strings.ToLower(topic)) ||
				strings.Contains(strings.ToLower(m.Value), strings.ToLower(topic))
			tag := ""
			if matched {
				tag = " ✓"
			}
			summary := buildEntrySummary(m.Key, m.Value, m.MemoryType, m.Importance)
			lines = append(lines, fmt.Sprintf("[%d] %s (重要性%d/置信度%d)%s %s", i+1, m.MemoryType, m.Importance, m.Confidence, tag, summary))
		}
		return fmt.Sprintf("记忆摘要报告（主题: %s）\n生成时间: %s\n来源数量: %d\n\n%s", topic, now, len(mems), strings.Join(lines, "\n"))
	}

	var factLines []string
	for i, m := range mems {
		summary := buildEntrySummary(m.Key, m.Value, m.MemoryType, m.Importance)
		factLines = append(factLines, fmt.Sprintf("%d. [%s] %s", i+1, m.MemoryType, summary))
	}

	cfg := s.getActiveModel()
	if cfg == nil {
		*warnings = append(*warnings, "model unavailable")
		var simpleLines []string
		for i, m := range mems {
			summary := buildEntrySummary(m.Key, m.Value, m.MemoryType, m.Importance)
			simpleLines = append(simpleLines, fmt.Sprintf("%d. %s", i+1, summary))
		}
		return fmt.Sprintf("关于「%s」的记忆摘要（%d条）:\n%s", topic, len(mems), strings.Join(simpleLines, "\n"))
	}

	systemPrompt := "你是一个记忆助手。根据提供的结构化记忆条目，生成一段简洁、客观的自然语言摘要。不要编造信息。只基于给定事实。"

	userMsg := fmt.Sprintf("主题: %s\n\n记忆条目:\n%s\n\n请生成一段简洁的中文摘要，概括这些记忆的核心信息。", topic, strings.Join(factLines, "\n"))

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userMsg},
	}

	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		log.Warn("摘要模型调用失败:", err)
		*warnings = append(*warnings, "模型调用失败，回退到deterministic")
		var simpleLines []string
		for i, m := range mems {
			summary := buildEntrySummary(m.Key, m.Value, m.MemoryType, m.Importance)
			simpleLines = append(simpleLines, fmt.Sprintf("%d. %s", i+1, summary))
		}
		return fmt.Sprintf("关于「%s」的记忆摘要（%d条）:\n%s", topic, len(mems), strings.Join(simpleLines, "\n"))
	}

	content = strings.TrimSpace(content)
	if content == "" {
		*warnings = append(*warnings, "模型返回为空")
		return fmt.Sprintf("关于「%s」的记忆: 找到 %d 条相关条目，但未生成有效摘要。", topic, len(mems))
	}
	return content
}

func uniqueLayersFromMemories(mems []Memory) []MemoryLayer {
	seen := make(map[MemoryLayer]bool)
	var result []MemoryLayer
	for _, m := range mems {
		layer := CanonicalLayer(collectionKeyForMemoryType(m.MemoryType))
		if !seen[layer] {
			seen[layer] = true
			result = append(result, layer)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueTypesFromMemories(mems []Memory) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range mems {
		if !seen[m.MemoryType] {
			seen[m.MemoryType] = true
			result = append(result, m.MemoryType)
		}
	}
	sort.Strings(result)
	return result
}
