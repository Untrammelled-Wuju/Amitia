package tool

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/business"
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
					"scope": {
						Type:        "string",
						Description: "记忆范围。默认当前角色；只有用户在当前消息中明确要求保存到全局记忆时才使用global",
						Enum:        []string{"character", "global"},
					},
				},
				Required: []string{"key", "value"},
			},
		},
	}, saveMemory)
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
					"scope": {
						Type:        "string",
						Description: "画像范围。默认当前角色；只有用户在当前消息中明确要求保存到全局记忆时才使用global",
						Enum:        []string{"character", "global"},
					},
				},
				Required: []string{"category", "attribute_name", "attribute_value"},
			},
		},
	}, saveProfile)
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
					"scope": {
						Type:        "string",
						Description: "情景范围。默认当前角色；只有用户在当前消息中明确要求保存到全局记忆时才使用global",
						Enum:        []string{"character", "global"},
					},
				},
				Required: []string{"scene_type", "title", "content"},
			},
		},
	}, saveEpisodicMemory)
}

func normalizeMemoryExpiresAt(raw string) (string, error) {
	return business.NormalizeMemoryExpiresAt(raw)
}

func normalizeMemoryEntityID(raw string) (string, error) {
	return business.NormalizeMemoryEntityID(raw)
}

func stringPtr(value string) *string { return &value }

func intPtr(value int) *int { return &value }

func saveMemory(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "save_memory", args)
}

func saveProfile(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "save_profile", args)
}

func saveEpisodicMemory(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "save_episodic_memory", args)
}
