package search

import (
	"context"
	"testing"
)

type engineCredentialProbeProvider struct {
	credential string
}

func (p *engineCredentialProbeProvider) ID() string {
	return "probe"
}

func (p *engineCredentialProbeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{GeneralWeb: true, MaxResults: 1}
}

func (p *engineCredentialProbeProvider) Search(ctx context.Context, request SearchRequest) (ProviderSearchResponse, error) {
	p.credential = EngineCredentialFromContext(ctx, "serper")
	return ProviderSearchResponse{}, nil
}

func (p *engineCredentialProbeProvider) Health(ctx context.Context) ProviderHealth {
	return ProviderHealthReady
}

func (p *engineCredentialProbeProvider) SetCredential(credential string) {}

func TestService_DynamicEngineCredentialResolver(t *testing.T) {
	provider := &engineCredentialProbeProvider{}
	config := DefaultConfig()
	config.Enabled = true
	config.Providers = map[string]ProviderConfig{
		"probe": {Type: "probe", Enabled: true},
	}
	providers := NewProviderSet("probe")
	providers.Register("probe", provider)
	service := NewService(config, providers).WithEngineCredentialResolver(
		func(ctx context.Context, providerID, invocation string) (map[string]string, func(), error) {
			return map[string]string{"serper": "dynamic-key"}, func() {}, nil
		},
	)
	if _, searchErr := service.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "test"); searchErr != nil {
		t.Fatalf("unexpected search error: %v", searchErr)
	}
	if provider.credential != "dynamic-key" {
		t.Fatalf("dynamic credential = %q", provider.credential)
	}
}

type lazyCredentialProbeProvider struct {
	credential string
}

func (p *lazyCredentialProbeProvider) ID() string { return "lazy-probe" }
func (p *lazyCredentialProbeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{GeneralWeb: true, MaxResults: 1}
}
func (p *lazyCredentialProbeProvider) Search(ctx context.Context, request SearchRequest) (ProviderSearchResponse, error) {
	credential, release, err := ResolveEngineCredential(ctx, "serper")
	if release != nil {
		defer release()
	}
	if err != nil {
		return ProviderSearchResponse{}, err
	}
	p.credential = credential
	return ProviderSearchResponse{}, nil
}
func (p *lazyCredentialProbeProvider) Health(context.Context) ProviderHealth {
	return ProviderHealthReady
}

func TestService_ConfigEngineCredentialRefsResolveLazily(t *testing.T) {
	provider := &lazyCredentialProbeProvider{}
	config := DefaultConfig()
	config.Enabled = true
	config.Providers = map[string]ProviderConfig{
		"lazy-probe": {
			Type:    "probe",
			Enabled: true,
			EngineCredentials: map[string]string{
				"serper":  "secret://search/serper",
				"youtube": "secret://search/youtube",
			},
		},
	}
	providers := NewProviderSet("lazy-probe")
	providers.Register("lazy-probe", provider)
	calls := map[string]int{}
	service := NewService(config, providers).WithCredentialResolver(
		func(ctx context.Context, providerID, invocation, credentialRef string) (string, func(), error) {
			calls[credentialRef]++
			switch credentialRef {
			case "secret://search/serper":
				return "serper-key", func() {}, nil
			case "secret://search/youtube":
				return "youtube-key", func() {}, nil
			default:
				return "", func() {}, nil
			}
		},
	)
	if _, searchErr := service.Search(context.Background(), GeneralSearchRequest{Query: "test"}, "inv-lazy"); searchErr != nil {
		t.Fatalf("unexpected search error: %v", searchErr)
	}
	if provider.credential != "serper-key" {
		t.Fatalf("resolved credential = %q", provider.credential)
	}
	if calls["secret://search/serper"] != 1 {
		t.Fatalf("serper resolver calls = %d", calls["secret://search/serper"])
	}
	if calls["secret://search/youtube"] != 0 {
		t.Fatalf("youtube credential was resolved eagerly: %d calls", calls["secret://search/youtube"])
	}
}
