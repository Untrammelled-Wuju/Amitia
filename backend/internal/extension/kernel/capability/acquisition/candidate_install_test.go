package acquisition

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeEnableExistingPort struct {
	extensionID string
}

func (f *fakeEnableExistingPort) EnableExtension(_ context.Context, extID string) error {
	f.extensionID = extID
	return nil
}

func (*fakeEnableExistingPort) EnableSkill(context.Context, string) error {
	return nil
}

func (*fakeEnableExistingPort) EnableMCP(context.Context, string) error {
	return nil
}

func TestInstalledCandidateCarriesExtensionIdentity(t *testing.T) {
	registry := capability.NewProviderRegistry()
	provider := capability.CapabilityProviderDefinition{
		ID:           "com.amitia.builtin.browser.provider",
		CapabilityID: "browser.control",
		Kind:         capability.ProviderKindBuiltin,
		Placement:    capability.ProviderPlacementCore,
		ExtensionID:  "com.amitia.builtin.browser",
		ModuleID:     "browser-runtime",
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBrowser,
			RuntimeID:   "browser-runtime",
			HandlerName: "browser.tab.get",
		},
	}
	if err := registry.RegisterDefinition(provider); err != nil {
		t.Fatal(err)
	}
	service := capability.NewCapabilityService(registry)
	source := NewInstalledSource(service, registry)

	candidates, err := source.Search(context.Background(), AcquisitionRequest{CapabilityID: "browser.control"})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.ExtensionID != "com.amitia.builtin.browser" || candidate.PackageName != "com.amitia.builtin.browser" || candidate.ProviderID != string(provider.ID) {
		t.Fatalf("candidate identity = %#v", candidate)
	}

	port := &fakeEnableExistingPort{}
	installer := NewEnableExistingInstaller(port)
	if _, err := installer.Install(context.Background(), candidate, DeploymentTarget{}); err != nil {
		t.Fatal(err)
	}
	if port.extensionID != "com.amitia.builtin.browser" {
		t.Fatalf("enabled extension = %q", port.extensionID)
	}
}
