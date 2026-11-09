package plugincontract

import "testing"

func TestBuiltinAdapterRegistryContainsKernelAdapters(t *testing.T) {
	registry, err := NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatalf("create builtin adapter registry: %v", err)
	}
	tests := []struct {
		adapter string
		kind    string
	}{
		{adapter: "tool", kind: "tool"},
		{adapter: "ui_provider", kind: "ui_provider"},
		{adapter: "ui_slot", kind: "ui_slot"},
		{adapter: "game", kind: "gamex"},
		{adapter: "desktop_pet", kind: "petx"},
	}
	for _, test := range tests {
		descriptor, ok := registry.Get(test.adapter, 1)
		if !ok {
			t.Fatalf("adapter %s was not registered", test.adapter)
		}
		found := false
		for _, kind := range descriptor.SupportedKinds {
			if kind == test.kind {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("adapter %s does not support kind %s", test.adapter, test.kind)
		}
	}
}

func TestBuiltinAdapterRegistryRejectsCommunityRegistration(t *testing.T) {
	registry, err := NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatalf("create builtin adapter registry: %v", err)
	}
	err = registry.Register(AdapterDescriptor{
		ID:                 "community.runtime",
		ContractVersion:    1,
		SupportedKinds:     []string{"community"},
		ContributionSchema: builtinAdapterSchema,
		Trust:              TrustUntrusted,
	})
	if err == nil {
		t.Fatal("community adapter registration must be rejected")
	}
}
