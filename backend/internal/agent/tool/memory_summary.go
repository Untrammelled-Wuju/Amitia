package tool

import (
	"context"
)

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
					"mode": {
						Type:        "string",
						Description: "摘要模式: deterministic、model、auto，默认 auto",
					},
				},
				Required: []string{"topic"},
			},
		},
	}, summarizeMemories)
}

func summarizeMemories(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "summarize_memories", args)
}
