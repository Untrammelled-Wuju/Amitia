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
	UnreadCompletedCount int64
	ActivityLabel        string
	ActiveToolName       string
	ActiveTaskTitle      string
	LastMessagePreview   string
	LastMessageType      string
	LastMessageRole      string
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
		conversations[index].HasUnread = row.UnreadCompletedCount > 0
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
		parsed[index]["unreadCompletedCount"] = row.UnreadCompletedCount
		parsed[index]["hasUnread"] = row.UnreadCompletedCount > 0
		if row.ActivityLabel != "" {
			parsed[index]["activityLabel"] = row.ActivityLabel
			parsed[index]["activeToolName"] = row.ActiveToolName
			parsed[index]["activeTaskTitle"] = row.ActiveTaskTitle
		}
		if row.LastMessagePreview != "" {
			parsed[index]["lastMessagePreview"] = row.LastMessagePreview
			parsed[index]["lastMessage"] = row.LastMessagePreview
			parsed[index]["lastMessageType"] = row.LastMessageType
			parsed[index]["lastMessageRole"] = row.LastMessageRole
		}
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
	terminalRows := make([]struct {
		ConversationID string `gorm:"column:conversation_id"`
		Sequence       int64  `gorm:"column:sequence"`
	}, 0)
	if err := db.Table("assistant_turns").
		Select("conversation_id, sequence").
		Where("conversation_id IN ? AND status IN ?", unique, terminalConversationTurnStatuses).
		Scan(&terminalRows).Error; err != nil {
		return nil, err
	}
	for _, row := range terminalRows {
		if row.Sequence <= lastRead[row.ConversationID] {
			continue
		}
		value := result[row.ConversationID]
		value.UnreadCompletedCount++
		result[row.ConversationID] = value
	}
	activeTurns := make([]AssistantTurn, 0)
	if err := db.Table("assistant_turns").
		Select("id, conversation_id, sequence, status").
		Where("conversation_id IN ? AND status IN ?", unique, activeConversationTurnStatuses).
		Order("conversation_id, sequence DESC").
		Scan(&activeTurns).Error; err != nil {
		return nil, err
	}
	activeByConversation := make(map[string]AssistantTurn, len(activeTurns))
	activeTurnIDs := make([]string, 0, len(activeTurns))
	for _, turn := range activeTurns {
		if _, exists := activeByConversation[turn.ConversationID]; exists {
			continue
		}
		activeByConversation[turn.ConversationID] = turn
		activeTurnIDs = append(activeTurnIDs, turn.ID)
	}
	activeItems := make([]AssistantTurnItem, 0)
	if len(activeTurnIDs) > 0 {
		if err := db.Table("assistant_turn_items").
			Select("turn_id, conversation_id, sequence, item_type, status, tool_name, content, arguments_json").
			Where("turn_id IN ?", activeTurnIDs).
			Order("turn_id, sequence DESC").
			Scan(&activeItems).Error; err != nil {
			return nil, err
		}
	}
	itemByTurn := make(map[string]AssistantTurnItem, len(activeItems))
	for _, item := range activeItems {
		if _, exists := itemByTurn[item.TurnID]; !exists {
			itemByTurn[item.TurnID] = item
		}
	}
	for conversationID, turn := range activeByConversation {
		value := result[conversationID]
		item, hasItem := itemByTurn[turn.ID]
		if hasItem {
			value.ActiveToolName = strings.TrimSpace(item.ToolName)
			value.ActiveTaskTitle = conversationActivityTaskTitle(item)
			value.ActivityLabel = conversationActivityLabel(item)
		} else {
			value.ActivityLabel = "正在生成回复"
		}
		result[conversationID] = value
	}
	latestMessages := make([]struct {
		ConversationID string `gorm:"column:conversation_id"`
		Content        string `gorm:"column:content"`
		MsgType        string `gorm:"column:msg_type"`
		Role           string `gorm:"column:role"`
		Sequence       int64  `gorm:"column:sequence"`
	}, 0)
	messageSubquery := db.Table("messages").
		Select("conversation_id, MAX(sequence) AS sequence").
		Where("conversation_id IN ? AND deleted_at IS NULL", unique).
		Group("conversation_id")
	if err := db.Table("messages AS m").
		Select("m.conversation_id, m.content, m.msg_type, m.role, m.sequence").
		Joins("JOIN (?) latest ON latest.conversation_id = m.conversation_id AND latest.sequence = m.sequence", messageSubquery).
		Where("m.deleted_at IS NULL").
		Scan(&latestMessages).Error; err != nil {
		return nil, err
	}
	for _, message := range latestMessages {
		value := result[message.ConversationID]
		value.LastMessagePreview = conversationMessagePreview(message.Content, message.MsgType)
		value.LastMessageType = strings.TrimSpace(message.MsgType)
		value.LastMessageRole = strings.TrimSpace(message.Role)
		result[message.ConversationID] = value
	}
	return result, nil
}

func conversationActivityLabel(item AssistantTurnItem) string {
	switch item.ItemType {
	case assistantTurnItemToolCall:
		if name := strings.TrimSpace(item.ToolName); name != "" {
			return "正在使用 " + name + " 工具"
		}
		return "正在使用工具"
	case assistantTurnItemToolResult:
		return "正在处理工具结果"
	case assistantTurnItemAgentTask:
		if title := conversationActivityTaskTitle(item); title != "" {
			return "正在执行 " + title
		}
		return "正在执行任务"
	case assistantTurnItemReasoning:
		return "正在思考"
	case assistantTurnItemText:
		if preview := conversationMessagePreview(item.Content, "text"); preview != "" {
			return preview
		}
		return "正在生成回复"
	default:
		return "正在执行"
	}
}

func conversationActivityTaskTitle(item AssistantTurnItem) string {
	for _, raw := range []string{item.Content, item.ArgumentsJSON} {
		value := strings.TrimSpace(raw)
		if value == "" || value == "{}" {
			continue
		}
		var payload map[string]any
		if json.Unmarshal([]byte(value), &payload) == nil {
			for _, key := range []string{"title", "name", "goal", "task", "objective"} {
				if text, _ := payload[key].(string); strings.TrimSpace(text) != "" {
					return conversationMessagePreview(text, "text")
				}
			}
		}
		return conversationMessagePreview(value, "text")
	}
	return ""
}

func conversationMessagePreview(content, messageType string) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if len([]rune(value)) > 120 {
		value = string([]rune(value)[:120]) + "…"
	}
	if value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(messageType)) {
	case "image":
		return "[图片]"
	case "video":
		return "[视频]"
	case "audio", "voice":
		return "[语音]"
	case "file":
		return "[文件]"
	default:
		return ""
	}
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
