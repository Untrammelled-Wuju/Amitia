package config

import "testing"

func TestValidateSearchRuntimeAllowsImplicitNativeProvider(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
	})
	if err != nil {
		t.Fatalf("native search provider should be available without an explicit provider entry: %v", err)
	}
}
