// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	episodicsvc "github.com/u-ai/backend/internal/episodic"
	memorysvc "github.com/u-ai/backend/internal/memory"
	profilesvc "github.com/u-ai/backend/internal/profile"
)

var toolMemoryService memorysvc.Service
var toolProfileService profilesvc.Service
var toolEpisodicService episodicsvc.Service

func SetMemoryService(svc memorysvc.Service) {
	toolMemoryService = svc
}

func SetProfileService(svc profilesvc.Service) {
	toolProfileService = svc
}

func SetEpisodicService(svc episodicsvc.Service) {
	toolEpisodicService = svc
}

type userScopedMemoryToolService interface {
	SearchForSpace(req *memorysvc.SearchMemoryRequest, spaceID string) ([]memorysvc.Memory, error)
	VectorSearchForSpace(req *memorysvc.VectorSearchRequest, spaceID string) ([]memorysvc.VectorSearchResult, error)
	HybridSearchForSpace(req *memorysvc.VectorSearchRequest, spaceID string) ([]memorysvc.HybridSearchResult, error)
	UpdateForSpace(id, spaceID string, req *memorysvc.UpdateMemoryRequest) (*memorysvc.Memory, error)
	CreateForSpace(req *memorysvc.CreateMemoryRequest, spaceID string) (*memorysvc.Memory, error)
}

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
						Description: "记忆类型",
						Enum:        []string{"personal_info", "hobby", "preference", "fact", "plan", "habit", "relationship", "custom"},
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

func saveMemory(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx
	if toolMemoryService == nil {
		result := ErrorResult("memory_service_not_initialized", "ERROR: memory service not initialized")
		result.Audit = map[string]interface{}{"service": "memory"}
		return result
	}
	spaceID, userErr := effectiveToolSpaceID(execCtx)
	if userErr != nil {
		return *userErr
	}
	scopedMemoryService, ok := toolMemoryService.(userScopedMemoryToolService)
	if !ok {
		return ErrorResult("memory_service_scope_unavailable", "ERROR: memory service does not support authenticated ownership")
	}

	key, _ := args["key"].(string)
	value, _ := args["value"].(string)
	memoryType, _ := args["memoryType"].(string)
	importance, _ := args["importance"].(float64)
	confidence, _ := args["confidence"].(float64)
	expiresAtRaw, _ := args["expiresAt"].(string)
	entityIDRaw, _ := args["entityId"].(string)

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		result := ErrorResult("invalid_args", "ERROR: key and value are required")
		result.Audit = map[string]interface{}{"missing_fields": []string{"key", "value"}}
		return result
	}
	if memoryType == "" {
		memoryType = "fact"
	}
	if normalized := memorysvc.CanonicalMemoryType(memoryType); normalized != "" {
		memoryType = string(normalized)
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

	expiresAt, err := normalizeMemoryExpiresAt(expiresAtRaw)
	if err != nil {
		result := ErrorResult("invalid_expires_at", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"field": "expiresAt", "value": expiresAtRaw}
		return result
	}
	entityID, err := normalizeMemoryEntityID(entityIDRaw)
	if err != nil {
		result := ErrorResult("invalid_entity_id", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"field": "entityId", "value": entityIDRaw}
		return result
	}

	characterID := execCtx.CharacterID
	if entityID != "" && characterID == "" {
		result := ErrorResult("invalid_entity_scope", "ERROR: entityId requires character scope")
		result.Audit = map[string]interface{}{"field": "entityId", "value": entityID}
		return result
	}
	searchResults, err := scopedMemoryService.SearchForSpace(&memorysvc.SearchMemoryRequest{
		Keyword:     key,
		CharacterID: characterID,
		Limit:       50,
	}, spaceID)
	if err != nil {
		result := ErrorResult("memory_service_error", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"operation": "search", "key": key, "character_id": characterID}
		return result
	}

	var existing *memorysvc.Memory
	for i := range searchResults {
		if strings.TrimSpace(searchResults[i].Key) != key {
			continue
		}
		if characterID != "" && searchResults[i].Scope != "user" && searchResults[i].CharacterID != characterID {
			continue
		}
		existing = &searchResults[i]
		break
	}

	if existing != nil {
		updateReq := &memorysvc.UpdateMemoryRequest{
			Value:          stringPtr(value),
			MemoryType:     stringPtr(memoryType),
			CharacterID:    stringPtr(characterID),
			Importance:     intPtr(int(importance)),
			Confidence:     intPtr(int(confidence)),
			VerifiedStatus: stringPtr("auto_confirmed"),
		}
		if expiresAt != "" {
			updateReq.ExpiresAt = stringPtr(expiresAt)
		}
		if entityID != "" {
			updateReq.EntityID = stringPtr(entityID)
		}
		updated, err := scopedMemoryService.UpdateForSpace(existing.ID, spaceID, updateReq)
		if err != nil {
			result := ErrorResult("memory_service_error", fmt.Sprintf("ERROR: %s", err.Error()))
			result.Audit = map[string]interface{}{"operation": "update", "memory_id": existing.ID, "key": key}
			return result
		}
		result := TextResult(fmt.Sprintf("OK (updated) %s: %s (confidence %d)", key, value, int(confidence)))
		result.ExternalOperationID = updated.ID
		result.SideEffects = []ToolSideEffect{{Type: "memory_update", TargetID: updated.ID, Confirmed: true}}
		result.Audit = map[string]interface{}{
			"key":          key,
			"memory_type":  memoryType,
			"character_id": characterID,
			"expires_at":   updated.ExpiresAt,
			"entity_id":    updated.EntityID,
		}
		return result
	}

	created, err := scopedMemoryService.CreateForSpace(&memorysvc.CreateMemoryRequest{
		CharacterID:    characterID,
		MemoryType:     memoryType,
		Key:            key,
		Value:          value,
		Importance:     int(importance),
		Confidence:     int(confidence),
		ExpiresAt:      expiresAt,
		EntityID:       entityID,
		VerifiedStatus: "auto_confirmed",
		Source:         "auto",
		Scope:          "character",
		SourceConvID:   execCtx.ConversationID,
		SourceMsgID:    execCtx.RequestID,
	}, spaceID)
	if err != nil {
		result := ErrorResult("memory_service_error", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"operation": "create", "key": key, "character_id": characterID}
		return result
	}
	result := TextResult(fmt.Sprintf("OK (created) %s: %s (confidence %d)", key, value, int(confidence)))
	result.ExternalOperationID = created.ID
	result.SideEffects = []ToolSideEffect{{Type: "memory_create", TargetID: created.ID, Confirmed: true}}
	result.Audit = map[string]interface{}{
		"key":          key,
		"memory_type":  memoryType,
		"character_id": characterID,
		"expires_at":   created.ExpiresAt,
		"entity_id":    created.EntityID,
	}
	return result
}

func normalizeMemoryExpiresAt(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" {
			return t.Format("2006-01-02"), nil
		}
		return t.Format("2006-01-02 15:04:05"), nil
	}
	return "", fmt.Errorf("invalid expiresAt, expected ISO date or datetime")
}

func normalizeMemoryEntityID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid entityId, expected UUID")
	}
	return id.String(), nil
}

func stringPtr(v string) *string {
	return &v
}

func intPtr(v int) *int {
	return &v
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

func saveProfile(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx
	if toolProfileService == nil {
		return ErrorResult("profile_service_not_initialized", "ERROR: profile service not initialized")
	}
	spaceID, userErr := effectiveToolSpaceID(execCtx)
	if userErr != nil {
		return *userErr
	}

	category, _ := args["category"].(string)
	attrName, _ := args["attribute_name"].(string)
	attrValue, _ := args["attribute_value"].(string)
	confidence, _ := args["confidence"].(float64)
	category = strings.TrimSpace(category)
	attrName = strings.TrimSpace(attrName)
	attrValue = strings.TrimSpace(attrValue)
	if category == "" || attrName == "" || attrValue == "" {
		return ErrorResult("invalid_args", "ERROR: category, attribute_name and attribute_value are required")
	}
	if confidence < 1 {
		confidence = 50
	}
	if confidence > 100 {
		confidence = 100
	}
	profileItem, err := toolProfileService.UpsertFromTool(
		spaceID, category, attrName, attrValue, int(confidence), execCtx.ConversationID, execCtx.CharacterID,
	)
	if err != nil {
		result := ErrorResult("profile_service_error", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"category": category, "attribute_name": attrName, "space_id": spaceID, "character_id": execCtx.CharacterID, "conversation_id": execCtx.ConversationID}
		return result
	}
	if OnProfileSaved != nil {
		OnProfileSaved(profileItem.ID)
	}
	result := TextResult(fmt.Sprintf("OK %s/%s: %s (confidence %d)", category, attrName, attrValue, profileItem.Confidence))
	result.ExternalOperationID = profileItem.ID
	result.SideEffects = []ToolSideEffect{{Type: "profile_upsert", TargetID: profileItem.ID, Confirmed: true}}
	result.Audit = map[string]interface{}{"category": category, "attribute_name": attrName, "conversation_id": execCtx.ConversationID, "character_id": execCtx.CharacterID, "space_id": spaceID}
	return result
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

func saveEpisodicMemory(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx
	if toolEpisodicService == nil {
		return ErrorResult("episodic_service_not_initialized", "ERROR: episodic service not initialized")
	}
	spaceID, userErr := effectiveToolSpaceID(execCtx)
	if userErr != nil {
		return *userErr
	}

	sceneType, _ := args["scene_type"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	score, _ := args["sentiment_score"].(float64)
	sceneType = strings.TrimSpace(sceneType)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if sceneType == "" || title == "" || content == "" {
		return ErrorResult("invalid_args", "ERROR: scene_type, title and content are required")
	}
	if score < -10 {
		score = -10
	}
	if score > 10 {
		score = 10
	}
	item, err := toolEpisodicService.SaveFromTool(
		spaceID, sceneType, title, content, int(score), execCtx.ConversationID, execCtx.RequestID, execCtx.RequestID, execCtx.CharacterID,
	)
	if err != nil {
		result := ErrorResult("episodic_service_error", fmt.Sprintf("ERROR: %s", err.Error()))
		result.Audit = map[string]interface{}{"scene_type": sceneType, "space_id": spaceID, "character_id": execCtx.CharacterID, "conversation_id": execCtx.ConversationID}
		return result
	}
	if OnEpisodicSaved != nil {
		OnEpisodicSaved(item.ID)
	}
	result := TextResult(fmt.Sprintf("OK (created) %s: %s (score %d)", sceneType, title, int(score)))
	result.ExternalOperationID = item.ID
	result.SideEffects = []ToolSideEffect{{Type: "episodic_memory_create", TargetID: item.ID, Confirmed: true}}
	result.Audit = map[string]interface{}{"scene_type": sceneType, "conversation_id": execCtx.ConversationID, "character_id": execCtx.CharacterID, "space_id": spaceID}
	return result
}
