package tool

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type MemorySummaryEntry struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	MemoryType  string `json:"memoryType"`
	Importance  int    `json:"importance"`
	Confidence  int    `json:"confidence"`
	Scope       string `json:"scope"`
	LastUsed    string `json:"lastUsed,omitempty"`
	MatchedOnce bool   `json:"matchedOnce"`
}

var toolRawDB *sql.DB

func SetRawDB(db *sql.DB) {
	toolRawDB = db
}

func init() {
	RegisterMemory(Tool{
		Type: "function",
		Function: Function{
			Name:        "summarize_memories",
			Description: "按主题或关键词汇总相关记忆，返回已按重要性和相关性排序的摘要列表。适合在需要回顾用户信息、偏好或历史时调用。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"topic": {
						Type:        "string",
						Description: "汇总主题或关键词，如爱好、工作、家庭、健康、旅行计划等",
					},
					"character_id": {
						Type:        "string",
						Description: "角色ID，为空时使用当前角色",
					},
					"limit": {
						Type:        "integer",
						Description: "返回最大条数，默认 20，最大 50",
					},
					"min_importance": {
						Type:        "integer",
						Description: "最低重要程度过滤 1-10，默认 1",
					},
				},
				Required: []string{"topic"},
			},
		},
	}, summarizeMemories)
}

func summarizeMemories(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx

	topic, _ := args["topic"].(string)
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ErrorResult("invalid_args", "ERROR: topic is required")
	}

	characterID, _ := args["character_id"].(string)
	if characterID == "" {
		characterID = execCtx.CharacterID
	}
	characterID = strings.TrimSpace(characterID)

	limitRaw, _ := args["limit"].(float64)
	limit := int(limitRaw)
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	minImportance, _ := args["min_importance"].(float64)
	minImp := int(minImportance)
	if minImp <= 0 {
		minImp = 1
	}
	if minImp > 10 {
		minImp = 10
	}

	likePattern := "%" + topic + "%"

	if toolDB == nil {
		return ErrorResult("database_not_initialized", "ERROR: database not initialized")
	}

	rows, err := toolDB.Query(
		"SELECT id, key, value, memory_type, importance, confidence, scope, last_used_at FROM memories WHERE character_id = ? AND (key LIKE ? OR value LIKE ?) AND importance >= ? ORDER BY importance DESC, last_used_at DESC LIMIT ?",
		characterID, likePattern, likePattern, minImp, limit,
	)
	if err != nil {
		return ErrorResult("query_failed", fmt.Sprintf("ERROR: %s", err.Error()))
	}
	defer rows.Close()

	entries := make([]MemorySummaryEntry, 0, limit)
	seen := make(map[string]bool)
	for rows.Next() {
		var id, key, value, memoryType, scope string
		var importance, confidence int
		var lastUsed sql.NullString
		if err := rows.Scan(&id, &key, &value, &memoryType, &importance, &confidence, &scope, &lastUsed); err != nil {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		matched := strings.Contains(strings.ToLower(key), strings.ToLower(topic)) ||
			strings.Contains(strings.ToLower(value), strings.ToLower(topic))

		summary := buildEntrySummary(key, value, memoryType, importance)

		entry := MemorySummaryEntry{
			Key:         key,
			Summary:     summary,
			MemoryType:  memoryType,
			Importance:  importance,
			Confidence:  confidence,
			Scope:       scope,
			MatchedOnce: matched,
		}
		if lastUsed.Valid {
			entry.LastUsed = lastUsed.String
		}
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		return ErrorResult("row_iteration_error", fmt.Sprintf("ERROR: %s", rows.Err().Error()))
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Importance != entries[j].Importance {
			return entries[i].Importance > entries[j].Importance
		}
		if entries[i].MatchedOnce != entries[j].MatchedOnce {
			return entries[i].MatchedOnce
		}
		return entries[i].Key < entries[j].Key
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	if len(entries) == 0 {
		return TextResult(fmt.Sprintf("未找到与「%s」相关的记忆摘要", topic))
	}

	var lines []string
	for _, e := range entries {
		tag := ""
		if e.MatchedOnce {
			tag = " ✓"
		}
		lines = append(lines, fmt.Sprintf("[%s] (重要性%d/置信度%d)%s %s", e.MemoryType, e.Importance, e.Confidence, tag, e.Summary))
	}

	summaryText := fmt.Sprintf("找到 %d 条与「%s」相关的记忆摘要：\n%s", len(entries), topic, strings.Join(lines, "\n"))

	now := time.Now().UTC().Format(time.RFC3339)
	result := TextResult(summaryText)
	result.Audit = map[string]interface{}{
		"topic":        topic,
		"character_id": characterID,
		"total":        len(entries),
		"created_at":   now,
	}
	return result
}

func buildEntrySummary(key, value, memoryType string, importance int) string {
	if importance >= 8 {
		return fmt.Sprintf("%s: %s", key, value)
	}
	if len(value) > 80 {
		return fmt.Sprintf("%s: %s...", key, value[:80])
	}
	return fmt.Sprintf("%s: %s", key, value)
}
