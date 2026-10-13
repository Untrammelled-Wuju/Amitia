package tool

import (
	"context"
)

func init() {
	RegisterMemory(Tool{
		Type: "function",
		Function: Function{
			Name:        "request_memory_consolidation",
			Description: "请求对记忆进行整理合并，系统会分析现有记忆中的重复、冲突和关联，生成整理建议。适合在记忆数量较多或需要清理时使用。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"character_id": {
						Type:        "string",
						Description: "角色ID，为空时使用当前角色",
					},
					"max_outputs": {
						Type:        "integer",
						Description: "最大产出条数，默认 5，最大 10",
					},
					"source": {
						Type:        "string",
						Description: "来源标识，如 reflection、manual、auto",
					},
					"include_conflict": {
						Type:        "boolean",
						Description: "是否包含冲突检测，默认 false",
					},
				},
				Required: []string{},
			},
		},
	}, requestMemoryConsolidation)
}

func requestMemoryConsolidation(ctx context.Context, execution ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	return executeOwnedMemoryTool(ctx, execution, "request_memory_consolidation", args)
}
