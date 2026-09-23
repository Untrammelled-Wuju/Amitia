package search

import "testing"

func TestHasProviderTreatsImplicitNativeAsConfigured(t *testing.T) {
	config := Config{DefaultProvider: ProviderNative}
	if !config.HasProvider() {
		t.Fatal("implicit native provider should satisfy HasProvider")
	}
	if !config.IsProviderEnabled(ProviderNative) {
		t.Fatal("implicit native provider should be enabled")
	}
}
