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
