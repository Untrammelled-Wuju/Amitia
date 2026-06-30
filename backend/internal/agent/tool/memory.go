// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
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
					"confidence": {
						Type:        "integer",
						Description: "置信度0-100。用户明确说出的80-100，推测的40-60，不确定的20-40",
					},
					"expiresAt": {
						Type:        "string",
						Description: "过期时间ISO格式，如'2026-12-31'。临时计划类记忆应设置过期时间",
					},
					"entityId": {
						Type:        "string",
						Description: "关联实体ID，用于关联到特定人物、地点、事件等",
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
	confidence, _ := args["confidence"].(float64)
	_, _ = args["expiresAt"].(string)
	_, _ = args["entityId"].(string)

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
	if confidence < 1 {
		confidence = 50
	}
	if confidence > 100 {
		confidence = 100
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
		_, err := toolDB.Exec("UPDATE memories SET value = ?, memory_type = ?, importance = ?, character_id = ?, confidence = ?, verified_status = 'auto_confirmed', updated_at = datetime('now', 'localtime') WHERE id = ?",
			value, memoryType, int(importance), characterID, int(confidence), existingID)
		if err != nil {
			return fmt.Sprintf("ERROR: %s", err.Error())
		}
		toolDB.Exec("INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, 'memory_edited', ?, ?, ?, ?, 'auto', ?, datetime('now', 'localtime'))",
			uuid.New().String(), existingID, key, value, memoryType, int(importance), characterID)

		if OnMemorySaved != nil {
			OnMemorySaved(existingID, key, value, memoryType, characterID)
		}
		return fmt.Sprintf("OK (updated) %s: %s (confidence %d)", key, value, int(confidence))
	}

	_, err := toolDB.Exec("INSERT INTO memories (id, key, value, memory_type, importance, character_id, source, confidence, verified_status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'auto', ?, 'auto_confirmed', datetime('now', 'localtime'), datetime('now', 'localtime'))",
		id, key, value, memoryType, int(importance), characterID, int(confidence))
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}

	toolDB.Exec("INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, 'memory_created', ?, ?, ?, ?, 'auto', ?, datetime('now', 'localtime'))",
		uuid.New().String(), id, key, value, memoryType, int(importance), characterID)

	if OnMemorySaved != nil {
		OnMemorySaved(id, key, value, memoryType, characterID)
	}
	return fmt.Sprintf("OK (created) %s: %s (confidence %d)", key, value, int(confidence))
}
func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "save_profile",
			Description: "保存用户画像信息。当用户在对话中分享了个人信息、偏好、习惯、恐惧、关系、健康状况、计划等值得记录的画像事实时调用。支持去重和自动提升置信度。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"category": {
						Type:        "string",
						Description: "画像类别：personal_info(个人信息)/preference(偏好)/habit(习惯)/fear(恐惧)/relationship(关系)/health(健康)/plan(计划)",
					},
					"attribute_name": {
						Type:        "string",
						Description: "属性名，如'姓名'、'年龄'、'爱好'、'最怕'、'近期目标'",
					},
					"attribute_value": {
						Type:        "string",
						Description: "属性值，如'张三'、'25'、'打篮球'、'蜘蛛'、'减肥5公斤'",
					},
					"confidence": {
						Type:        "integer",
						Description: "置信度0-100。用户明确说出的信息80-100，推测的信息40-60，模糊的信息20-40",
					},
				},
				Required: []string{"category", "attribute_name", "attribute_value"},
			},
		},
	}, saveProfile)
}

func saveProfile(args map[string]interface{}) string {
	if toolDB == nil {
		return "ERROR: database not initialized"
	}

	category, _ := args["category"].(string)
	attrName, _ := args["attribute_name"].(string)
	attrValue, _ := args["attribute_value"].(string)
	confidence, _ := args["confidence"].(float64)

	if category == "" || attrName == "" || attrValue == "" {
		return "ERROR: category, attribute_name and attribute_value are required"
	}

	if confidence < 1 {
		confidence = 50
	}
	if confidence > 100 {
		confidence = 100
	}

	userID := "default"
	convID := CurrentConversationID
	if convID == "" {
		convID = "unknown"
	}

	var existingID string
	var currentConf int
	row := toolDB.QueryRow(
		"SELECT id, confidence FROM user_profiles WHERE user_id = ? AND category = ? AND attribute_name = ?",
		userID, category, attrName)
	row.Scan(&existingID, &currentConf)

	newConf := int(confidence)
	if existingID != "" {
		newConf = currentConf + 10
		if newConf > 100 {
			newConf = 100
		}
		toolDB.Exec(
			"UPDATE user_profiles SET attribute_value = ?, confidence = ?, source_conv_id = ?, updated_at = datetime('now','localtime') WHERE id = ?",
			attrValue, newConf, convID, existingID)
		if OnProfileSaved != nil {
			OnProfileSaved(existingID)
		}
		return fmt.Sprintf("OK (updated) %s/%s: %s (confidence %d)", category, attrName, attrValue, newConf)
	}

	id := uuid.New().String()
	toolDB.Exec(
		"INSERT INTO user_profiles (id, user_id, category, attribute_name, attribute_value, confidence, source_conv_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, userID, category, attrName, attrValue, newConf, convID)
	if OnProfileSaved != nil {
		OnProfileSaved(id)
	}
	return fmt.Sprintf("OK (created) %s/%s: %s (confidence %d)", category, attrName, attrValue, newConf)
}
func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "save_episodic_memory",
			Description: "保存一段值得长期记忆的情景时刻。当对话中出现重要感悟、情感转折、里程碑事件、笑话、坦白等特殊时刻时调用。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"scene_type": {
						Type:        "string",
						Description: "情景类型：insight(感悟)/joke(笑话)/milestone(里程碑)/emotional_peak(情感峰值)/confession(坦白)",
					},
					"title": {
						Type:        "string",
						Description: "情景标题，简短概括如'用户分享了童年回忆'、'达成了重要目标'",
					},
					"content": {
						Type:        "string",
						Description: "情景详细描述，记录发生了什么、为什么值得记忆",
					},
					"sentiment_score": {
						Type:        "integer",
						Description: "情感分值-10到+10，负值负面正值正面",
					},
				},
				Required: []string{"scene_type", "title", "content"},
			},
		},
	}, saveEpisodicMemory)
}

func saveEpisodicMemory(args map[string]interface{}) string {
	if toolDB == nil {
		return "ERROR: database not initialized"
	}

	sceneType, _ := args["scene_type"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	score, _ := args["sentiment_score"].(float64)

	if sceneType == "" || title == "" || content == "" {
		return "ERROR: scene_type, title and content are required"
	}

	userID := "default"
	convID := CurrentConversationID
	if convID == "" {
		convID = "unknown"
	}

	id := uuid.New().String()
	toolDB.Exec(
		"INSERT INTO episodic_memories (id, user_id, scene_type, title, content, sentiment_score, source_conv_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now','localtime'), datetime('now','localtime'))",
		id, userID, sceneType, title, content, int(score), convID)
	if OnEpisodicSaved != nil {
		OnEpisodicSaved(id)
	}
	return fmt.Sprintf("OK (created) %s: %s (score %d)", sceneType, title, int(score))
}
