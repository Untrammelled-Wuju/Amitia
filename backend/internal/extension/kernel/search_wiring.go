package kernel

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/providers/brave"
	"github.com/u-ai/backend/internal/search/providers/searxng"
	"github.com/u-ai/backend/internal/webresearch"
	applog "github.com/u-ai/backend/log"
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
		switch strings.ToLower(strings.TrimSpace(pc.Type)) {
		case "brave":
			provider := brave.NewProvider("", pc.CredentialRef, pc.Endpoint, true)
			provider.SetMaxResponseBytes(config.EffectiveMaxResponseBytes())
			p = provider
		case "searxng":
			provider := searxng.NewProvider(pc.Endpoint, true, pc.AllowHTTP, pc.AllowPrivate)
			provider.SetMaxResponseBytes(config.EffectiveMaxResponseBytes())
			p = provider
		}
		if p != nil {
			p = restrictSearchProviderKinds(p, pc.Kinds)
			set.RegisterWithPriority(id, p, pc.Priority)
		}
	}
}

type configuredSearchProvider struct {
	search.Provider
	capabilities search.ProviderCapabilities
}

func (p *configuredSearchProvider) Capabilities() search.ProviderCapabilities {
	return p.capabilities
}

func restrictSearchProviderKinds(provider search.Provider, configured []string) search.Provider {
	if provider == nil || len(configured) == 0 {
		return provider
	}
	allowed := make(map[search.SearchKind]struct{}, len(configured))
	for _, raw := range configured {
		kind := search.SearchKind(strings.ToLower(strings.TrimSpace(raw)))
		if kind.Valid() {
			allowed[kind] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return provider
	}
	caps := provider.Capabilities()
	_, allowWeb := allowed[search.SearchKindWeb]
	caps.GeneralWeb = caps.GeneralWeb && allowWeb
	filtered := make([]search.SearchKind, 0, len(caps.SearchKinds))
	for _, kind := range caps.SearchKinds {
		if _, ok := allowed[kind]; ok {
			filtered = append(filtered, kind)
		}
	}
	caps.SearchKinds = filtered
	return &configuredSearchProvider{Provider: provider, capabilities: caps}
}

func buildWebResearchCallFunc(runtime *webresearch.Runtime) capability.SearchCallFunc {
	return func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation capability.ToolInvocationContext,
		input json.RawMessage,
	) (json.RawMessage, error) {
		return executeWebResearch(ctx, runtime, handlerName, invocation, input, nil)
	}
}

func buildWebResearchStreamCallFunc(runtime *webresearch.Runtime) capability.SearchStreamCallFunc {
	return func(
		ctx context.Context,
		providerID string,
		handlerName string,
		invocation capability.ToolInvocationContext,
		input json.RawMessage,
		emitter capability.ToolStreamEmitter,
	) (json.RawMessage, error) {
		progress := func(event webresearch.Progress) error {
			if emitter == nil {
				return nil
			}
			message := webResearchProgressMessage(event)
			metadata := map[string]any{
				"phase": event.Phase,
			}
			if event.Query != "" {
				metadata["query"] = event.Query
			}
			if event.RefID != "" {
				metadata["refId"] = event.RefID
			}
			if event.Title != "" {
				metadata["title"] = event.Title
			}
			if event.Completed > 0 {
				metadata["completed"] = event.Completed
			}
			if event.Total > 0 {
				metadata["total"] = event.Total
			}
			return emitter.Emit(ctx, capability.ToolStreamEmission{
				Type: capability.ToolStreamEventProgress,
				Progress: &capability.ToolStreamProgress{
					Fraction:      event.Fraction,
					Indeterminate: event.Indeterminate,
					Message:       message,
				},
				Metadata: metadata,
			})
		}
		return executeWebResearch(ctx, runtime, handlerName, invocation, input, progress)
	}
}

func webResearchProgressMessage(event webresearch.Progress) string {
	if message := strings.TrimSpace(event.Message); message != "" {
		return message
	}
	switch strings.TrimSpace(event.Phase) {
	case "searching":
		if query := strings.TrimSpace(event.Query); query != "" {
			return "Searching: " + query
		}
		return "Searching the web"
	case "source_found":
		if title := strings.TrimSpace(event.Title); title != "" {
			return "Found source: " + title
		}
		return "Found a relevant source"
	case "opening":
		if title := strings.TrimSpace(event.Title); title != "" {
			return "Reading: " + title
		}
		return "Reading source"
	case "opened":
		if title := strings.TrimSpace(event.Title); title != "" {
			return "Read source: " + title
		}
		return "Source read complete"
	case "found":
		return "Located matching content"
	case "research_round":
		return "Running research round"
	case "screenshot":
		return "Captured web evidence"
	default:
		if phase := strings.TrimSpace(event.Phase); phase != "" {
			return strings.ReplaceAll(phase, "_", " ")
		}
		return "Web research in progress"
	}
}

func executeWebResearch(
	ctx context.Context,
	runtime *webresearch.Runtime,
	handlerName string,
	invocation capability.ToolInvocationContext,
	input json.RawMessage,
	emit webresearch.ProgressFunc,
) (json.RawMessage, error) {
	trace := webResearchTrace(invocation)
	applog.TraceInfo(trace.WithStage("web_research_started"), applog.Fields{
		"invocation_id": invocation.InvocationID,
		"input_bytes":   len(input),
	}, "web research invocation started")
	if runtime == nil {
		return nil, noSearchServiceError()
	}
	if handlerName != "" && handlerName != "web.run" {
		return nil, &capability.ToolError{
			Code:        capability.ErrorCodeInvalidInput,
			Message:     "unsupported web runtime handler",
			DomainCode:  webresearch.ErrInvalidInput,
			UserVisible: true,
		}
	}
	turnID := invocation.RootID
	if turnID == "" {
		turnID = invocation.ParentID
	}
	if turnID == "" {
		turnID = invocation.InvocationID
	}
	conversationID := invocation.ConversationID
	if conversationID == "" {
		conversationID = "execution:" + turnID
	}
	result, webErr := runtime.Execute(ctx, webresearch.Scope{
		ConversationID: conversationID,
		TurnID:         turnID,
		InvocationID:   invocation.InvocationID,
	}, input, emit)
	if webErr != nil {
		applog.TraceError(trace.WithStage("web_research_failed"), applog.Fields{
			"invocation_id": invocation.InvocationID,
			"error_code":    webErr.Code,
			"retryable":     webErr.Retryable,
		}, webErr, "web research invocation failed")
		return nil, mapWebResearchToToolError(webErr)
	}
	partial := false
	stopReason := ""
	if result != nil && result.Research != nil {
		partial = result.Research.Partial
		stopReason = result.Research.StopReason
	}
	applog.TraceInfo(trace.WithStage("web_research_completed"), applog.Fields{
		"invocation_id":           invocation.InvocationID,
		"operation":               result.Operation,
		"mode":                    result.Mode,
		"search_calls":            result.Stats.SearchCalls,
		"fetch_calls":             result.Stats.FetchCalls,
		"browser_calls":           result.Stats.BrowserCalls,
		"search_cache_hits":       result.Stats.CacheHits,
		"page_cache_hits":         result.Stats.PageCacheHits,
		"duration_ms":             result.Stats.DurationMs,
		"search_results":          len(result.Search),
		"pages":                   len(result.Pages),
		"citations":               len(result.Citations),
		"screenshots":             len(result.Screenshots),
		"partial":                 partial,
		"stop_reason":             stopReason,
		"prompt_injection_signal": result.Security.PotentialPromptInjection,
	}, "web research invocation completed")
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, &capability.ToolError{
			Code:        capability.ErrorCodeInvalidResult,
			Message:     "failed to encode web runtime result",
			DomainCode:  "WEB_INVALID_RESULT",
			UserVisible: false,
			Cause:       err,
		}
	}
	return payload, nil
}

func webResearchTrace(invocation capability.ToolInvocationContext) applog.TraceFields {
	requestID := strings.TrimSpace(invocation.TraceID)
	if requestID == "" {
		requestID = strings.TrimSpace(invocation.InvocationID)
	}
	return applog.TraceFields{
		RequestID:     requestID,
		CorrelationID: strings.TrimSpace(invocation.CorrelationID),
		CausationID:   strings.TrimSpace(invocation.CausationID),
		Character:     strings.TrimSpace(invocation.CharacterID),
		Conversation:  strings.TrimSpace(invocation.ConversationID),
		Channel:       strings.TrimSpace(invocation.Channel),
		StateVersion:  "web-research-v1",
		Path:          "web.run",
	}
}

func mapWebResearchToToolError(webErr *webresearch.Error) *capability.ToolError {
	if webErr == nil {
		return nil
	}
	code := capability.ErrorCodeExecutionFailed
	switch webErr.Code {
	case webresearch.ErrInvalidInput:
		code = capability.ErrorCodeInvalidInput
	case webresearch.ErrNotConfigured, webresearch.ErrBrowserUnavailable:
		code = capability.ErrorCodeNotAvailable
	case webresearch.ErrReferenceNotFound:
		code = capability.ErrorCodeInvalidInput
	case webresearch.ErrReferenceScope:
		code = capability.ErrorCodeScopeDenied
	case webresearch.ErrFetchBlocked:
		code = capability.ErrorCodePermissionDenied
	case webresearch.ErrUnsupportedContent:
		code = capability.ErrorCodeNotAvailable
	case webresearch.ErrBudgetExhausted:
		code = capability.ErrorCodeResourceLimitExceeded
	case webresearch.ErrCancelled:
		code = capability.ErrorCodeCancelled
	}
	return &capability.ToolError{
		Code:        code,
		Message:     webErr.Error(),
		DomainCode:  webErr.Code,
		Retryable:   webErr.Retryable,
		UserVisible: true,
		Cause:       webErr,
	}
}

func buildSearchHealthFunc(svc *search.Service) capability.SearchHealthFunc {
	return func(ctx context.Context, providerID string) capability.HealthStatus {
		if svc == nil {
			return capability.HealthUnknown
		}
		providerID = strings.TrimSpace(providerID)
		if providerID == "search-runtime" || providerID == "default" {
			providerID = ""
		}
		health := svc.ProviderHealth(ctx, providerID)
		return mapSearchHealth(health)
	}
}

func mapSearchHealth(health search.ProviderHealth) capability.HealthStatus {
	switch health {
	case search.ProviderHealthReady:
		return capability.HealthReady
	case search.ProviderHealthDegraded:
		return capability.HealthDegraded
	case search.ProviderHealthDisabled:
		return capability.HealthUnhealthy
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
