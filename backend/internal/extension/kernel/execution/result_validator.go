package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewResultValidator(caches ...*capability.JSONSchemaCache) *ResultValidator {
	cache := (*capability.JSONSchemaCache)(nil)
	if len(caches) > 0 {
		cache = caches[0]
	}
	if cache == nil {
		cache = capability.NewJSONSchemaCache()
	}

	return &ResultValidator{
		MaxOutputBytes: 1 << 20,
		schemas:        cache,
	}
}

type ResultValidator struct {
	MaxOutputBytes int64

	schemas *capability.JSONSchemaCache
}

func (v *ResultValidator) Validate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	result = result.Clone()

	if result.InvocationID == "" {
		result.InvocationID = inv.InvocationID
	} else if result.InvocationID != inv.InvocationID {
		return v.invalidResult(inv.InvocationID, tool, "invocation_id mismatch")
	}

	if result.ToolID == "" {
		result.ToolID = string(tool.ID)
	} else if result.ToolID != string(tool.ID) {
		return v.invalidResult(inv.InvocationID, tool, "tool_id mismatch")
	}

	if !result.Status.Valid() {
		return v.invalidResult(inv.InvocationID, tool, "invalid result status")
	}

	switch result.Status {
	case capability.ToolResultStatusSuccess:
		return v.validateSuccess(inv, tool, result)
	case capability.ToolResultStatusFailed:
		return v.validateFailed(inv, tool, result)
	case capability.ToolResultStatusCancelled:
		return v.validateCancelled(inv, result)
	case capability.ToolResultStatusTimedOut:
		return v.validateTimedOut(inv, tool, result)
	}

	return result
}

func (v *ResultValidator) validateSuccess(inv capability.ToolInvocationContext, tool capability.ToolDefinition, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if result.Error != nil {
		result.Status = capability.ToolResultStatusFailed
		result.Error = capability.NormalizeToolError(result.Error)
		return result
	}

	if len(result.Content) == 0 && len(result.Structured) == 0 {
		return v.invalidResult(inv.InvocationID, tool, "empty result content")
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
			return v.invalidResult(inv.InvocationID, tool, fmt.Sprintf("result too large: %d bytes exceeds limit of %d", totalSize, v.MaxOutputBytes))
		}
	}

	if len(result.Structured) > 0 {
		if !json.Valid(result.Structured) {
			return v.invalidResult(inv.InvocationID, tool, "structured result is not valid json")
		}

		if v.hasOutputSchema(tool) {
			if err := v.schemas.Validate(tool.OutputSchema, result.Structured); err != nil {
				return v.invalidResult(inv.InvocationID, tool, "structured result does not match output schema")
			}
		}
	}

	if v.hasOutputSchema(tool) {
		for _, c := range result.Content {
			if c.Type == capability.ToolContentStructured && len(c.Data) > 0 {
				if !json.Valid(c.Data) {
					return v.invalidResult(inv.InvocationID, tool, "structured content is not valid json")
				}
				if err := v.schemas.Validate(tool.OutputSchema, c.Data); err != nil {
					return v.invalidResult(inv.InvocationID, tool, "structured content does not match output schema")
				}
			}
		}
	}

	return result
}

func (v *ResultValidator) validateFailed(inv capability.ToolInvocationContext, tool capability.ToolDefinition, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if result.Error == nil {
		result.Error = &capability.ToolError{
			Code:     capability.ErrorCodeExecutionFailed,
			Category: capability.ToolErrorCategoryRuntime,
			Message:  "execution failed",
		}
		return result
	}
	result.Error = capability.NormalizeToolError(result.Error)
	return result
}

func (v *ResultValidator) validateCancelled(inv capability.ToolInvocationContext, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if result.Error == nil {
		result.Error = &capability.ToolError{
			Code:      capability.ErrorCodeCancelled,
			Category:  capability.ToolErrorCategoryCancellation,
			Message:   "execution was cancelled",
			Retryable: false,
		}
		return result
	}
	if result.Error.Code == "" {
		result.Error.Code = capability.ErrorCodeCancelled
	}
	if result.Error.Category == "" {
		result.Error.Category = capability.ToolErrorCategoryCancellation
	}
	result.Error.Retryable = false
	return result
}

func (v *ResultValidator) validateTimedOut(inv capability.ToolInvocationContext, tool capability.ToolDefinition, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if result.Error == nil {
		result.Error = &capability.ToolError{
			Code:     capability.ErrorCodeTimeout,
			Category: capability.ToolErrorCategoryTimeout,
			Message:  "execution timed out",
		}
		return result
	}
	if result.Error.Code == "" {
		result.Error.Code = capability.ErrorCodeTimeout
	}
	if result.Error.Category == "" {
		result.Error.Category = capability.ToolErrorCategoryTimeout
	}
	return result
}

func (v *ResultValidator) hasOutputSchema(tool capability.ToolDefinition) bool {
	return len(bytes.TrimSpace(tool.OutputSchema)) > 0
}

func (v *ResultValidator) invalidResult(invocationID string, tool capability.ToolDefinition, message string) capability.UnifiedToolResult {
	return capability.UnifiedToolResult{
		InvocationID: invocationID,
		ToolID:       string(tool.ID),
		Status:       capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:     capability.ErrorCodeInvalidResult,
			Category: capability.ToolErrorCategoryValidation,
			Message:  message,
		},
	}
}
