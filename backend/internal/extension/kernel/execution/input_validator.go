package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

type InputValidator struct {
	MaxInputBytes int64
}

func (v *InputValidator) Validate(ctx context.Context, tool capability.ToolDefinition, input json.RawMessage) error {
	if v.MaxInputBytes > 0 && int64(len(input)) > v.MaxInputBytes {
		return fmt.Errorf("input too large: %d bytes exceeds limit of %d", len(input), v.MaxInputBytes)
	}
	return nil
}
