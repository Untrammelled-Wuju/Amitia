package kernel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/providers/brave"
)

type secretLeaseAdapter struct {
	broker *secret.Broker
}

func newSecretLeaseAdapter(broker *secret.Broker) *secretLeaseAdapter {
	return &secretLeaseAdapter{broker: broker}
}

func (a *secretLeaseAdapter) Issue(ctx context.Context, ref string, purpose string) (string, error) {
	secretRef, err := secret.ParseRef(ref)
	if err != nil {
		return "", err
	}
	lease, err := a.broker.Issue(ctx, secret.LeaseRequest{
		Ref:               secretRef,
		Purpose:           purpose,
		RuntimeInstanceID: "search",
		TTL:               30 * time.Second,
		MaxUses:           1,
	})
	if err != nil {
		return "", err
	}
	return string(lease.ID), nil
}

func (a *secretLeaseAdapter) Consume(ctx context.Context, leaseID string) (string, error) {
	val, err := a.broker.Consume(ctx, secret.LeaseID(leaseID), secret.LeaseUseContext{
		RuntimeInstanceID: "search",
	})
	if err != nil {
		return "", err
	}
	defer zeroSecretBytes(val)
	return string(val), nil
}

func zeroSecretBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func buildSearchService(config search.Config, broker *secret.Broker) *search.Service {
	providers := search.NewProviderSet(config.DefaultProvider)
	if config.HasProvider() {
		buildSearchProviders(providers, config)
	}
	svc := search.NewService(config, providers)
	if broker != nil {
		bridge := search.NewSecretBridge(newSecretLeaseAdapter(broker))
		svc.WithCredentialResolver(bridge.Resolve)
	}
	return svc
}

func buildSearchProviders(set *search.ProviderSet, config search.Config) {
	for id, pc := range config.Providers {
		if !pc.Enabled {
			continue
		}
		var p search.Provider
		switch pc.Type {
		case "brave":
			p = brave.NewProvider("", pc.CredentialRef, pc.Endpoint, true)
		}
		if p != nil {
			set.Register(id, p)
		}
	}
}

func buildSearchCallFunc(svc *search.Service) capability.SearchCallFunc {
	return func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation capability.ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		if svc == nil {
			return nil, noSearchServiceError()
		}
		resp, serr := svc.ExecuteFromJSON(ctx, input, invocation.InvocationID)
		if serr != nil {
			return nil, mapSearchToToolError(serr)
		}
		out := search.ToolOutputFromResponse(resp)
		return json.Marshal(out)
	}
}

func buildSearchHealthFunc(svc *search.Service) capability.SearchHealthFunc {
	return func(ctx context.Context, providerID string) capability.HealthStatus {
		if svc == nil {
			return capability.HealthUnknown
		}
		_, health := svc.DefaultProviderHealth(ctx)
		return mapSearchHealth(health)
	}
}

func mapSearchToToolError(serr *search.Error) *capability.ToolError {
	code := capability.ErrorCodeExecutionFailed
	retryable := false
	switch serr.Code {
	case search.SEARCH_DISABLED:
		code = capability.ErrorCodeNotAvailable
	case search.SEARCH_PROVIDER_NOT_CONFIGURED:
		code = capability.ErrorCodeNotAvailable
	case search.SEARCH_PROVIDER_UNAVAILABLE:
		retryable = serr.Retryable
	case search.SEARCH_PROVIDER_AUTH_FAILED:
		code = capability.ErrorCodeNotAvailable
	case search.SEARCH_PROVIDER_RATE_LIMITED:
		code = capability.ErrorCodeRateLimited
	case search.SEARCH_PROVIDER_TIMEOUT:
		code = capability.ErrorCodeTimeout
		retryable = true
	case search.SEARCH_PROVIDER_REQUEST_FAILED:
		retryable = serr.Retryable
	case search.SEARCH_PROVIDER_REQUEST_REJECTED:
		code = capability.ErrorCodeInvalidInput
	case search.SEARCH_PROVIDER_INVALID_RESPONSE:
	case search.SEARCH_INVALID_QUERY, search.SEARCH_INVALID_LIMIT,
		search.SEARCH_INVALID_OFFSET, search.SEARCH_INVALID_LANGUAGE,
		search.SEARCH_INVALID_COUNTRY, search.SEARCH_INVALID_SAFE_SEARCH,
		search.SEARCH_INVALID_KIND, search.SEARCH_SPECIALIZED_OPTIONS_INVALID:
		code = capability.ErrorCodeInvalidInput
	case search.SEARCH_KIND_UNSUPPORTED, search.SEARCH_FILTER_UNSUPPORTED,
		search.SEARCH_TIME_RANGE_UNSUPPORTED, search.SEARCH_DOMAIN_FILTER_UNSUPPORTED:
		code = capability.ErrorCodeNotAvailable
	case search.SEARCH_CANCELLED:
		code = capability.ErrorCodeCancelled
	default:
		retryable = serr.Retryable
	}
	return &capability.ToolError{
		Code:        code,
		Message:     serr.Error(),
		DomainCode:  serr.Code,
		Retryable:   retryable,
		UserVisible: true,
	}
}

func mapSearchHealth(health search.ProviderHealth) capability.HealthStatus {
	switch health {
	case search.ProviderHealthReady:
		return capability.HealthReady
	case search.ProviderHealthDegraded:
		return capability.HealthDegraded
	case search.ProviderHealthMisconfigured,
		search.ProviderHealthCredentialMiss,
		search.ProviderHealthNetworkDown:
		return capability.HealthUnhealthy
	}
	return capability.HealthUnknown
}

func noSearchServiceError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:     "search service is not configured",
		DomainCode:  search.SEARCH_DISABLED,
		UserVisible: false,
	}
}

type SearchToolsDeps struct {
	Registry *capability.ToolRegistry
	Service  *search.Service
	Config   search.Config
}

func RegisterWebSearchTool(deps SearchToolsDeps) error {
	if deps.Registry == nil || deps.Service == nil {
		return nil
	}
	inputSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["query"],"properties":{"query":{"type":"string","minLength":1,"maxLength":2048},"kind":{"type":"string","enum":["web","news","academic","code","image","video","places","product"]},"limit":{"type":"integer","minimum":1,"maximum":20},"offset":{"type":"integer","minimum":0,"maximum":100},"language":{"type":"string"},"country":{"type":"string"},"safeSearch":{"type":"string","enum":["off","moderate","strict"]},"domains":{"type":"array","maxItems":16,"items":{"type":"string","maxLength":253}},"specialized":{"type":"object"}}}`)
	outputSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["query","provider","results","returned","retrievedAt"],"properties":{"query":{"type":"string"},"kind":{"type":"string"},"provider":{"type":"string"},"results":{"type":"array","items":{"type":"object"}},"returned":{"type":"integer"},"hasMore":{"type":"boolean"},"retrievedAt":{"type":"string","format":"date-time"},"citations":{"type":"array","items":{"type":"object"}}}}`)
	enabled := deps.Config.Enabled && deps.Config.HasProvider()
	definition := capability.ToolDefinition{
		ID:           "internal/search/web",
		ModelName:    "web_search",
		CapabilityID: "search.web",
		Source:       capability.ToolSourceInternal,
		Name:         "Web Search",
		Description:  "Search the web using the configured provider.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "network.request", Description: "Sends query to external search provider"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectExternal,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      true,
		Enabled:        enabled,
		TimeoutMS:      30000,
		Runtime: capability.RuntimeBinding{
			RuntimeType: "search",
			RuntimeID:   "default",
			HandlerName: "search.general",
		},
		RoutingMode: capability.RoutingModeProviderRequired,
		ProviderID:  "com.amitia.builtin.search.provider",
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:     30 * time.Second,
			Idempotent:  true,
			RetryPolicy: capability.RetryPolicy{MaxRetries: 1, BackoffBase: 1 * time.Second},
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 131072,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
	}
	return deps.Registry.Replace(context.Background(), definition)
}
