//go:build !linux || android

package kernel

import "github.com/u-ai/backend/internal/extension/kernel/capability"

func registerAndroidLinuxAdapter(registry *capability.RuntimeAdapterRegistry, provider interface{}) {
}
