package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSearchRuntimeIsEnabledByDefault(t *testing.T) {
	defaults := viper.New()
	setDefaults(defaults)
	if !defaults.GetBool("providers.search.enabled") {
		t.Fatal("native search should be enabled by default")
	}
	if defaults.GetString("providers.search.defaultProvider") != "native" {
		t.Fatalf("default search provider = %q, want native", defaults.GetString("providers.search.defaultProvider"))
	}
}

func TestValidateSearchRuntimeAllowsImplicitNativeProvider(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
	})
	if err != nil {
		t.Fatalf("native search provider should be available without an explicit provider entry: %v", err)
	}
}

func TestValidateSearchRuntimeRejectsPlaintextProviderCredential(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "brave",
		Providers: map[string]SearchProviderRuntimeConfig{
			"brave": {
				Type:          "brave",
				Enabled:       true,
				Endpoint:      "https://api.search.brave.com/res/v1/web/search",
				CredentialRef: "plaintext-api-key",
			},
		},
	})
	if err == nil {
		t.Fatal("expected plaintext credentialRef to be rejected")
	}
}

func TestValidateSearchRuntimeAcceptsSecretProviderCredential(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "brave",
		Providers: map[string]SearchProviderRuntimeConfig{
			"brave": {
				Type:          "brave",
				Enabled:       true,
				Endpoint:      "https://api.search.brave.com/res/v1/web/search",
				CredentialRef: "secret://search/brave",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected secret credentialRef to pass validation: %v", err)
	}
}

func TestValidateSearchRuntimeRejectsPlaintextEngineCredential(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
		Providers: map[string]SearchProviderRuntimeConfig{
			"native": {
				Type:    "native",
				Enabled: true,
				EngineCredentials: map[string]string{
					"serper": "plaintext-api-key",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected plaintext engine credential to be rejected")
	}
}

func TestValidateSearchRuntimeAcceptsSecretEngineCredential(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
		Providers: map[string]SearchProviderRuntimeConfig{
			"native": {
				Type:    "native",
				Enabled: true,
				EngineCredentials: map[string]string{
					"serper": "secret://search/serper",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected secret engine credential to pass validation: %v", err)
	}
}

func TestValidateSearchRuntimeRejectsUnknownNativeEngineOverride(t *testing.T) {
	enabled := true
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
		Providers: map[string]SearchProviderRuntimeConfig{
			"native": {
				Type:    "native",
				Enabled: true,
				Native: SearchNativeProviderRuntimeConfig{
					Engines: map[string]SearchNativeEngineRuntimeConfig{
						"not-a-real-engine": {Enabled: &enabled},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown native engine override to be rejected")
	}
}

func TestValidateSearchRuntimeRejectsUnknownEngineCredentialID(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "native",
		Providers: map[string]SearchProviderRuntimeConfig{
			"native": {
				Type:    "native",
				Enabled: true,
				EngineCredentials: map[string]string{
					"typo-engine": "secret://search/typo",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown engine credential id to be rejected")
	}
}

func TestValidateSearchRuntimeRejectsEngineCredentialsOnTopLevelProvider(t *testing.T) {
	err := validateSearchRuntimeConfig(SearchRuntimeProviderConfig{
		Enabled:         true,
		DefaultProvider: "brave",
		Providers: map[string]SearchProviderRuntimeConfig{
			"brave": {
				Type:          "brave",
				Enabled:       true,
				Endpoint:      "https://api.search.brave.com/res/v1/web/search",
				CredentialRef: "secret://search/brave",
				EngineCredentials: map[string]string{
					"serper": "secret://search/serper",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected engineCredentials on non-native provider to be rejected")
	}
}
