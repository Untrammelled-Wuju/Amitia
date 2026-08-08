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

	outputSchemaIsNonEmpty := len(bytes.TrimSpace(tool.OutputSchema)) > 0

	if len(result.Structured) > 0 {
		if !json.Valid(result.Structured) {
			result.Status = capability.ToolResultStatusFailed
			result.Error = &capability.ToolError{
				Code:    capability.ErrorCodeInvalidResult,
				Message: "structured result is not valid json",
			}
			return result
		}

		if outputSchemaIsNonEmpty {
			if err := v.schemas.Validate(tool.OutputSchema, result.Structured); err != nil {
				result.Status = capability.ToolResultStatusFailed
				result.Error = &capability.ToolError{
					Code:    capability.ErrorCodeInvalidResult,
					Message: "structured result does not match output schema",
				}
				return result
			}
		}
	}

	if outputSchemaIsNonEmpty {
		for _, c := range result.Content {
			if c.Type == capability.ToolContentStructured && len(c.Data) > 0 {
				if !json.Valid(c.Data) {
					result.Status = capability.ToolResultStatusFailed
					result.Error = &capability.ToolError{
						Code:    capability.ErrorCodeInvalidResult,
						Message: "structured content is not valid json",
					}
					return result
				}

				if err := v.schemas.Validate(tool.OutputSchema, c.Data); err != nil {
					result.Status = capability.ToolResultStatusFailed
					result.Error = &capability.ToolError{
						Code:    capability.ErrorCodeInvalidResult,
						Message: "structured content does not match output schema",
					}
					return result
				}
			}
		}
	}

	return result
}
