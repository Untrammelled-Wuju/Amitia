//go:build linux

package kernel

import (
	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerAndroidLinuxAdapter(registry *capability.RuntimeAdapterRegistry, provider interface{}) {
	alProvider, ok := provider.(terminal.AndroidLinuxProvider)
	if !ok {
		return
	}
	androidLinuxAdapter := capability.NewAndroidLinuxRuntimeAdapter(alProvider)
	registry.Register(capability.RuntimeTypeAndroidLinux, androidLinuxAdapter)
}
