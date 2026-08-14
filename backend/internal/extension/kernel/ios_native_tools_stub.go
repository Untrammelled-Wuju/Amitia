//go:build !ios
// +build !ios

package kernel

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerIOSToolsIfPresent(toolRegistry *capability.ToolRegistry, provider capability.IOSProvider) error {
	return nil
}
