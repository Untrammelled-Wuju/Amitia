package execution

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewResultValidator() *ResultValidator {
	return &ResultValidator{
		MaxOutputBytes: 1 << 20,
	}
}

type ResultValidator struct {
	MaxOutputBytes int64
}

func (v *ResultValidator) Validate(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if result.Error != nil {
		if result.Error.Code == "" {
			result.Error.Code = capability.ErrorCodeExecutionFailed
		}
		if result.Status == "" {
			result.Status = capability.ToolResultStatusFailed
		}
		return result
	}

	if result.Status == "" {
		result.Status = capability.ToolResultStatusSuccess
	}

	if result.Status == capability.ToolResultStatusSuccess && len(result.Content) == 0 && result.Structured == nil {
		result.Status = capability.ToolResultStatusFailed
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInvalidResult,
			Message: "empty result content",
		}
		return result
	}

	for _, c := range result.Content {
		if c.Type == "" {
			c.Type = capability.ToolContentText
		}
	}

	if v.MaxOutputBytes > 0 {
		totalSize := int64(len(result.Structured))
		for _, c := range result.Content {
			totalSize += int64(len(c.Text)) + int64(len(c.Data))
		}
		if totalSize > v.MaxOutputBytes {
			result.Status = capability.ToolResultStatusFailed
			result.Error = &capability.ToolError{
				Code:    capability.ErrorCodeInvalidResult,
				Message: fmt.Sprintf("result too large: %d bytes exceeds limit of %d", totalSize, v.MaxOutputBytes),
			}
			return result
		}
	}

	return result
}
