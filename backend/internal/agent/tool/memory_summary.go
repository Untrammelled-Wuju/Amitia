package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	memorysvc "github.com/u-ai/backend/internal/memory"
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

	modeRaw, _ := args["mode"].(string)
	mode := memorysvc.MemorySummaryMode(strings.TrimSpace(modeRaw))

	if toolMemoryService == nil {
		return ErrorResult("service_not_initialized", "ERROR: memory service not initialized")
	}

	req := &memorysvc.MemorySummaryRequest{
		CharacterID:   characterID,
		Topic:         topic,
		MinImportance: minImp,
		Limit:         limit,
		IncludeEvidence: false,
		Mode:          mode,
	}

	result, err := toolMemoryService.SummarizeMemories(req)
	if err != nil {
		return ErrorResult("summary_failed", fmt.Sprintf("ERROR: %s", err.Error()))
	}

	if result.EvidenceCount == 0 {
		return TextResult(fmt.Sprintf("未找到与「%s」相关的记忆摘要", topic))
	}

	summaryText := result.Summary
	if summaryText == "" {
		summaryText = fmt.Sprintf("找到 %d 条与「%s」相关的记忆", result.EvidenceCount, topic)
	}

	if len(result.Evidence) > 0 {
		var eviLines []string
		for _, e := range result.Evidence {
			eviLines = append(eviLines, fmt.Sprintf("- [%s] %s", e.Layer, e.Key))
		}
		sort.Strings(eviLines)
		summaryText += "\n\n证据来源:\n" + strings.Join(eviLines, "\n")
	}

	if len(result.Warnings) > 0 {
		summaryText += "\n\n警告: " + strings.Join(result.Warnings, "; ")
	}

	summaryText += fmt.Sprintf("\n\n[生成方式: %s | 条数: %d/%d%s]",
		result.GeneratedBy, result.EvidenceCount, limit,
		map[bool]string{true: " 截断", false: ""}[result.Truncated])

	return TextResult(summaryText)
}
