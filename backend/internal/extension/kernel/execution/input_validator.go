package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewInputValidator(caches ...*capability.JSONSchemaCache) *InputValidator {
	cache := (*capability.JSONSchemaCache)(nil)
	if len(caches) > 0 {
		cache = caches[0]
	}
	if cache == nil {
		cache = capability.NewJSONSchemaCache()
	}

	return &InputValidator{
		schemas: cache,
	}
}

type InputValidator struct {
	MaxInputBytes int64

	schemas *capability.JSONSchemaCache
}

func (v *InputValidator) Validate(ctx context.Context, tool capability.ToolDefinition, input json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if v.MaxInputBytes > 0 && int64(len(input)) > v.MaxInputBytes {
		return fmt.Errorf("input too large: %d bytes exceeds limit of %d", len(input), v.MaxInputBytes)
	}

	if len(bytes.TrimSpace(tool.InputSchema)) == 0 {
		if len(bytes.TrimSpace(input)) == 0 {
			return nil
		}

		if !json.Valid(input) {
			return &capability.SchemaContractError{
				ToolID: tool.ID,
				Role:   capability.ToolSchemaInput,
				Kind:   "invalid_json",
			}
		}

		return nil
	}

	if err := v.schemas.Validate(tool.InputSchema, input); err != nil {
		return &capability.SchemaContractError{
			ToolID: tool.ID,
			Role:   capability.ToolSchemaInput,
			Kind:   "instance_mismatch",
			Cause:  err,
		}
	}

	return nil
}
