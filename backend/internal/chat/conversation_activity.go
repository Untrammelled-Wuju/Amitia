package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

var activeConversationTurnStatuses = []string{
	assistantTurnStatusQueued,
	assistantTurnStatusStarting,
	assistantTurnStatusRunning,
	assistantTurnStatusWaitingTool,
	"waiting_approval",
	"cancelling",
}

var terminalConversationTurnStatuses = []string{
	assistantTurnStatusCompleted,
	assistantTurnStatusFailed,
	assistantTurnStatusInterrupted,
}

type conversationActivityRow struct {
	ConversationID       string `gorm:"column:conversation_id"`
	IsGenerating         int64  `gorm:"column:is_generating"`
	LastTerminalSequence int64  `gorm:"column:last_terminal_sequence"`
}

type ConversationActivity struct {
	IsGenerating         bool
	LastTerminalSequence int64
}

func EnrichConversationActivity(db *gorm.DB, conversations []Conversation) error {
	if len(conversations) == 0 {
		return nil
	}
	ids := make([]string, 0, len(conversations))
	lastRead := make(map[string]int64, len(conversations))
	for index := range conversations {
		id := strings.TrimSpace(conversations[index].ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		lastRead[id] = conversations[index].LastReadTurnSequence
	}
	activity, err := loadConversationActivity(db, ids, lastRead)
	if err != nil {
		return err
	}
	for index := range conversations {
		row := activity[strings.TrimSpace(conversations[index].ID)]
		conversations[index].IsGenerating = row.IsGenerating
		conversations[index].LastTerminalTurnSequence = row.LastTerminalSequence
		conversations[index].HasUnread = row.LastTerminalSequence > conversations[index].LastReadTurnSequence
	}
	return nil
}

func EnrichConversationActivityBodies(db *gorm.DB, bodies []json.RawMessage) ([]json.RawMessage, error) {
	if len(bodies) == 0 {
		return bodies, nil
	}
	parsed := make([]map[string]any, len(bodies))
	ids := make([]string, 0, len(bodies))
	lastRead := make(map[string]int64, len(bodies))
	for index, raw := range bodies {
		if len(raw) == 0 {
			return nil, fmt.Errorf("conversation body is empty")
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		parsed[index] = value
		id, _ := value["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		lastRead[id] = jsonInt64(value["lastReadTurnSequence"])
	}
	activity, err := loadConversationActivity(db, ids, lastRead)
	if err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, len(bodies))
	for index := range parsed {
		id, _ := parsed[index]["id"].(string)
		id = strings.TrimSpace(id)
		row := activity[id]
		parsed[index]["isGenerating"] = row.IsGenerating
		parsed[index]["lastTerminalTurnSequence"] = row.LastTerminalSequence
		parsed[index]["hasUnread"] = row.LastTerminalSequence > jsonInt64(parsed[index]["lastReadTurnSequence"])
		encoded, err := json.Marshal(parsed[index])
		if err != nil {
			return nil, err
		}
		result[index] = encoded
	}
	return result, nil
}

func loadConversationActivity(db *gorm.DB, ids []string, lastRead map[string]int64) (map[string]ConversationActivity, error) {
	result := make(map[string]ConversationActivity, len(ids))
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return result, nil
	}
	if db == nil {
		return nil, fmt.Errorf("conversation activity database is unavailable")
	}
	for start := 0; start < len(unique); start += 400 {
		end := start + 400
		if end > len(unique) {
			end = len(unique)
		}
		rows := make([]conversationActivityRow, 0, end-start)
		query := db.Table("assistant_turns").
			Select("conversation_id, MAX(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS is_generating, MAX(CASE WHEN status IN ? THEN sequence ELSE 0 END) AS last_terminal_sequence", activeConversationTurnStatuses, terminalConversationTurnStatuses).
			Where("conversation_id IN ?", unique[start:end]).
			Group("conversation_id")
		if err := query.Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			result[row.ConversationID] = ConversationActivity{
				IsGenerating:         row.IsGenerating > 0,
				LastTerminalSequence: row.LastTerminalSequence,
			}
		}
	}
	return result, nil
}

func jsonInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}
