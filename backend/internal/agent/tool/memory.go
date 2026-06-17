package tool

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func init() {
	RegisterMemory(Tool{
		Type: "function",
		Function: Function{
			Name:        "save_memory",
			Description: "保存关于用户的重要信息到记忆库。当用户在对话中分享了个人信息、偏好、习惯、计划等值得记住的内容时调用。可以创建新记忆或更新已有记忆。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"key": {
						Type:        "string",
						Description: "记忆关键词，简短标签如'姓名'、'爱好'、'职业'、'宠物'、'计划'等",
					},
					"value": {
						Type:        "string",
						Description: "记忆具体内容，如'张三'、'喜欢爬山和摄影'",
					},
					"memoryType": {
						Type:        "string",
						Description: "记忆类型：personal_info(个人信息)、hobby(爱好)、preference(偏好)、fact(事实)、plan(计划)、habit(习惯)、relationship(关系)",
					},
					"importance": {
						Type:        "integer",
						Description: "重要程度 1-10，10为最重要。个人信息如姓名通常为9-10，爱好为7-8，一般事实为5-6",
					},
				},
				Required: []string{"key", "value"},
			},
		},
	}, saveMemory)
}

func saveMemory(args map[string]interface{}) string {
	if toolDB == nil {
		return "ERROR: database not initialized"
	}

	key, _ := args["key"].(string)
	value, _ := args["value"].(string)
	memoryType, _ := args["memoryType"].(string)
	importance, _ := args["importance"].(float64)

	if key == "" || value == "" {
		return "ERROR: key and value are required"
	}
	if memoryType == "" {
		memoryType = "fact"
	}
	if importance < 1 {
		importance = 5
	}
	if importance > 10 {
		importance = 10
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	characterID := CurrentCharacterID

	var existingID string
	if characterID != "" {
		row := toolDB.QueryRow("SELECT id FROM memories WHERE key = ? AND character_id = ? LIMIT 1", key, characterID)
		row.Scan(&existingID)
	} else {
		row := toolDB.QueryRow("SELECT id FROM memories WHERE key = ? LIMIT 1", key)
		row.Scan(&existingID)
	}

	id := uuid.New().String()

	if existingID != "" {
		_, err := toolDB.Exec("UPDATE memories SET value = ?, memory_type = ?, importance = ?, character_id = ?, updated_at = datetime('now', 'localtime') WHERE id = ?",
			value, memoryType, int(importance), characterID, existingID)
		if err != nil {
			return fmt.Sprintf("ERROR: %s", err.Error())
		}
		toolDB.Exec("INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, 'memory_edited', ?, ?, ?, ?, 'auto', ?, datetime('now', 'localtime'))",
			uuid.New().String(), existingID, key, value, memoryType, int(importance), characterID)

		if OnMemorySaved != nil {
			OnMemorySaved(existingID, key, value, memoryType, characterID)
		}
		return fmt.Sprintf("OK (updated) %s: %s", key, value)
	}

	_, err := toolDB.Exec("INSERT INTO memories (id, key, value, memory_type, importance, character_id, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'auto', datetime('now', 'localtime'), datetime('now', 'localtime'))",
		id, key, value, memoryType, int(importance), characterID)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}

	toolDB.Exec("INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, 'memory_created', ?, ?, ?, ?, 'auto', ?, datetime('now', 'localtime'))",
		uuid.New().String(), id, key, value, memoryType, int(importance), characterID)

	if OnMemorySaved != nil {
		OnMemorySaved(id, key, value, memoryType, characterID)
	}
	return fmt.Sprintf("OK (created) %s: %s", key, value)
}