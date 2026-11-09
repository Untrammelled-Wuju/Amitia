package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestPackageAdapterValidationRejectsMissingDynamicAdapter(t *testing.T) {
	registry, err := plugincontract.NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatal(err)
	}
	pkg := &amitiax.Package{
		Manifest: plugincontract.Manifest{
			Plugin: plugincontract.PluginMeta{ID: "com.example/plugin"},
			Modules: []plugincontract.Module{{
				ID: "main",
				Dynamic: plugincontract.DynamicCapability{
					AllowedAdapters:  []string{"community.missing"},
					MaxContributions: 1,
				},
			}},
		},
	}
	preview := InstallPreview{}
	appendPackageAdapterIssues(registry, pkg, &preview)
	if len(preview.Issues) != 1 || preview.Issues[0].Code != "adapter_unavailable" {
		t.Fatalf("unexpected adapter issues: %#v", preview.Issues)
	}
}
