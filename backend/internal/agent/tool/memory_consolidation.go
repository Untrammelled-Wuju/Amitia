package tool

import (
	"context"
	"fmt"
	"strings"

	memorysvc "github.com/u-ai/backend/internal/memory"
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

func requestMemoryConsolidation(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx

	characterID, _ := args["character_id"].(string)
	if characterID == "" {
		characterID = execCtx.CharacterID
	}
	characterID = strings.TrimSpace(characterID)

	maxOutputsRaw, _ := args["max_outputs"].(float64)
	maxOutputs := int(maxOutputsRaw)
	if maxOutputs <= 0 {
		maxOutputs = 5
	}
	if maxOutputs > 10 {
		maxOutputs = 10
	}

	source, _ := args["source"].(string)
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}

	includeConflict, _ := args["include_conflict"].(bool)

	if toolMemoryService == nil {
		return ErrorResult("service_not_initialized", "ERROR: memory service not initialized")
	}

	req := &memorysvc.ConsolidationRequest{
		CharacterID:     characterID,
		Source:          source,
		MaxOutputs:      maxOutputs,
		IncludeConflict: includeConflict,
		PolicyVersion:   "v1",
		PromptVersion:   "v1",
	}

	result, err := toolMemoryService.RunConsolidation(req)
	if err != nil {
		return ErrorResult("consolidation_failed", fmt.Sprintf("ERROR: %s", err.Error()))
	}

	if len(result.Candidates) == 0 {
		return TextResult("记忆整理完成：当前无需整理。")
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("记忆整理完成 (操作ID: %s)", result.OperationID))
	lines = append(lines, fmt.Sprintf("已提交: %d | 跳过重复: %d | 需审核: %d",
		result.CommittedCount, result.SkippedDuplicate, result.RequiresReview))

	if len(result.Candidates) > 0 {
		lines = append(lines, "\n候选建议:")
		for i, c := range result.Candidates {
			reason := c.Reason
			if reason == "" {
				reason = c.ProposedAction
			}
			lines = append(lines, fmt.Sprintf("%d. [%s] %s: %s → %s", i+1, c.CandidateKind, c.Key, c.Value, reason))
		}
	}

	if len(result.Errors) > 0 {
		lines = append(lines, "\n错误:")
		for _, e := range result.Errors {
			lines = append(lines, "- "+e)
		}
	}

	return TextResult(strings.Join(lines, "\n"))
}
