package search

import (
	"context"
	"encoding/json"
	"time"
)

type CredentialResolver func(ctx context.Context, providerID, invocation, credentialRef string) (credential string, release func(), err error)

type Service struct {
	providers          *ProviderSet
	config             Config
	normalizer         *Normalizer
	credentialResolver CredentialResolver
}

func NewService(config Config, providers *ProviderSet) *Service {
	return &Service{
		providers:          providers,
		config:             config,
		normalizer:         NewNormalizer(),
		credentialResolver: noopResolver,
	}
}

func (s *Service) WithCredentialResolver(r CredentialResolver) *Service {
	if r != nil {
		s.credentialResolver = r
	}
	return s
}

func noopResolver(ctx context.Context, providerID, invocation, credentialRef string) (string, func(), error) {
	return "", func() {}, nil
}

func (s *Service) Search(ctx context.Context, req GeneralSearchRequest, invocation string) (*GeneralSearchResponse, *Error) {
	if !s.config.Enabled {
		return nil, NewError(SEARCH_DISABLED, "", false, nil)
	}
	if _, verr := sanitizeQuery(req.Query); verr != nil {
		return nil, NewError(SEARCH_INVALID_QUERY, "", false, verr)
	}
	provider, serr := s.resolveProvider()
	if serr != nil {
		return nil, serr
	}
	credRef := s.config.ProviderCredentialRef(provider.ID())
	releaseCred := func() {}
	if s.credentialResolver != nil && credRef != "" {
		cred, release, rerr := s.credentialResolver(ctx, provider.ID(), invocation, credRef)
		if rerr != nil {
			return nil, NewError(SEARCH_PROVIDER_AUTH_FAILED, provider.ID(), false, rerr)
		}
		if cred != "" {
			applyCredential(provider, cred)
			releaseCred = release
		}
	}
	defer releaseCred()
	start := time.Now()
	raw, perr := provider.Search(ctx, req)
	if perr != nil {
		if searchErr, ok := perr.(*Error); ok {
			return nil, searchErr
		}
		return nil, NewError(SEARCH_PROVIDER_UNAVAILABLE, provider.ID(), true, perr)
	}
	results, _ := s.normalizer.NormalizeResults(raw.Results, provider.ID())
	s.normalizer.AssignRanks(results)
	resp := &GeneralSearchResponse{
		Query:       req.Query,
		Provider:    provider.ID(),
		Results:     results,
		Returned:    len(results),
		Offset:      req.Offset,
		HasMore:     raw.HasMore,
		RetrievedAt: time.Now().UTC(),
		DurationMs:  time.Since(start).Milliseconds(),
	}
	return resp, nil
}

func (s *Service) resolveProvider() (Provider, *Error) {
	provider, ok := s.providers.Default()
	if ok {
		return provider, nil
	}
	if s.config.DefaultProvider != "" {
		provider, ok = s.providers.Get(s.config.DefaultProvider)
		if ok {
			return provider, nil
		}
		return nil, NewError(SEARCH_PROVIDER_NOT_CONFIGURED, s.config.DefaultProvider, false, nil)
	}
	return nil, NewError(SEARCH_PROVIDER_NOT_CONFIGURED, "", false, nil)
}

func (s *Service) Health(ctx context.Context) map[string]ProviderHealth {
	result := make(map[string]ProviderHealth)
	for id, p := range s.providers.All() {
		result[id] = p.Health(ctx)
	}
	return result
}

func (s *Service) DefaultProviderHealth(ctx context.Context) (string, ProviderHealth) {
	provider, ok := s.providers.Default()
	if !ok {
		return "", ProviderHealthMisconfigured
	}
	return provider.ID(), provider.Health(ctx)
}

func (s *Service) ExecuteFromJSON(ctx context.Context, input json.RawMessage, invocation string) (*GeneralSearchResponse, *Error) {
	var toolInput ToolInput
	if err := json.Unmarshal(input, &toolInput); err != nil {
		return nil, NewError(SEARCH_INVALID_QUERY, "", false, err)
	}
	validated, verr := validateAndNormalize(&toolInput)
	if verr != nil {
		return nil, verr
	}
	req := toolInput.ToRequest()
	req.Query = validated.query
	req.Limit = validated.limit
	req.Offset = validated.offset
	req.Language = validated.language
	req.Country = validated.country
	req.SafeSearch = validated.safeSearch
	return s.Search(ctx, req, invocation)
}

type credentialBearer interface {
	SetCredential(string)
}

func applyCredential(p Provider, cred string) {
	if cb, ok := p.(credentialBearer); ok {
		cb.SetCredential(cred)
	}
}
