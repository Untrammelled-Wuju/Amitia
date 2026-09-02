//go:build !linux || android

package kernel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func amitiaLinuxAvailable(provider interface{}) bool { return false }

func amitiaLinuxCall(ctx context.Context, provider interface{}, invocation capability.ToolInvocationContext, operation string, payload map[string]any) (json.RawMessage, error) {
	return nil, errors.New("Android Linux terminal tools are unavailable on this backend platform")
}
