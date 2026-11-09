package kernel

import (
	"fmt"
	"runtime"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/platformcompat"
)

func appendPackageAdapterIssues(registry *plugincontract.AdapterRegistry, pkg *amitiax.Package, preview *InstallPreview) {
	if registry == nil || pkg == nil || pkg.Manifest.Plugin.ID == "" || preview == nil {
		return
	}
	platform := platformcompat.Normalize(runtime.GOOS)
	for _, module := range pkg.Manifest.Modules {
		for index, adapter := range module.Dynamic.AllowedAdapters {
			descriptor, ok := registry.Latest(adapter)
			if !ok {
				preview.Issues = append(preview.Issues, PreviewIssue{
					Category: PreviewNotInstallable,
					Code:     "adapter_unavailable",
					Message:  fmt.Sprintf("module %s dynamic adapter %s is not registered", module.ID, adapter),
					Path:     fmt.Sprintf("modules[%s].dynamic.allowedAdapters[%d]", module.ID, index),
				})
				continue
			}
			if len(descriptor.Platforms) == 0 {
				continue
			}
			if !packagePlatformMatches(descriptor.Platforms, platform) {
				preview.Issues = append(preview.Issues, PreviewIssue{
					Category: PreviewPartialUnsupported,
					Code:     "adapter_platform_unsupported",
					Message:  fmt.Sprintf("module %s dynamic adapter %s does not support platform %s", module.ID, adapter, platform),
					Path:     fmt.Sprintf("modules[%s].dynamic.allowedAdapters[%d]", module.ID, index),
				})
			}
		}
	}
}
