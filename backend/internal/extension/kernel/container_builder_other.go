//go:build !linux || android

package kernel

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimehost"
)

func registerTerminalTools(host runtimehost.RuntimeHost, provider interface{}, toolRegistry *capability.ToolRegistry) error {
	return nil
}
