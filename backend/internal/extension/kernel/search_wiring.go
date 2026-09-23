package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
	nativeengines "github.com/u-ai/backend/internal/search/native/engines"
	"github.com/u-ai/backend/internal/search/providers/brave"
	"github.com/u-ai/backend/internal/search/providers/exa"
	"github.com/u-ai/backend/internal/search/providers/tavily"
	"github.com/u-ai/backend/internal/webresearch"
	applog "github.com/u-ai/backend/log"
)

type SearchProviderRegistration struct {
	InstanceID    string
	Provider      search.Provider
	Priority      int
	CredentialRef string
	Kinds         []string
	Manifest      search.ProviderManifest
}

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

func (a *secretLeaseAdapter) Store(ctx context.Context, namespace string, value []byte) (string, error) {
	if a == nil || a.broker == nil {
		return "", fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := a.broker.Store(ctx, namespace, value)
	if err != nil {
		return "", err
	}
	return ref.String(), nil
}

func (a *secretLeaseAdapter) Resolve(ctx context.Context, rawRef string) ([]byte, error) {
	if a == nil || a.broker == nil {
		return nil, fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := secret.ParseRef(rawRef)
	if err != nil {
		return nil, err
	}
	lease, err := a.broker.Issue(ctx, secret.LeaseRequest{
		Ref: ref, Purpose: "search-engine-credential", RuntimeInstanceID: "search", TTL: 30 * time.Second, MaxUses: 1,
	})
	if err != nil {
		return nil, err
	}
	return a.broker.Consume(ctx, lease.ID, secret.LeaseUseContext{RuntimeInstanceID: "search"})
}

func (a *secretLeaseAdapter) Delete(ctx context.Context, rawRef string) error {
	if a == nil || a.broker == nil {
		return fmt.Errorf("search secret broker is unavailable")
	}
	ref, err := secret.ParseRef(rawRef)
	if err != nil {
		return err
	}
	return a.broker.Delete(ctx, ref)
}

func buildSearchService(config search.Config, broker *secret.Broker, db *sql.DB, external []SearchProviderRegistration) *search.Service {
	runtimeConfig := config
	runtimeConfig.Providers = cloneSearchProviderConfig(config.Providers)
	providers := search.NewProviderSet(runtimeConfig.DefaultProvider)
	if runtimeConfig.Enabled {
		buildSearchProviders(providers, runtimeConfig)
	}
	for _, registration := range external {
		id := strings.TrimSpace(registration.InstanceID)
		if id == "" || registration.Provider == nil {
			continue
		}
		provider := restrictSearchProviderKinds(registration.Provider, registration.Kinds)
		if err := providers.RegisterWithManifest(id, provider, registration.Priority, registration.Manifest); err != nil {
			applog.Warn("search provider registration rejected", "provider", id, "error", err)
			continue
		}
		runtimeConfig.Providers[id] = search.ProviderConfig{
			Type:          "plugin",
			CredentialRef: strings.TrimSpace(registration.CredentialRef),
			Enabled:       true,
			Priority:      registration.Priority,
			Kinds:         append([]string(nil), registration.Kinds...),
		}
	}
	svc := search.NewService(runtimeConfig, providers)
	if broker != nil {
		bridge := search.NewSecretBridge(newSecretLeaseAdapter(broker))
		svc.WithCredentialResolver(bridge.Resolve)
	}
	if db != nil {
		store := search.NewCredentialStore(db)
		if broker != nil {
			vault := newSecretLeaseAdapter(broker)
			store.WithVault(vault)
			migrationCtx, cancelMigration := context.WithTimeout(context.Background(), 15*time.Second)
			if err := store.MigrateLegacyCredentials(migrationCtx); err != nil {
				applog.Warn("search credential migration to SecretBroker failed", "error", err)
			}
			cancelMigration()
		}
		svc.WithEngineCredentialSourceFactory(store.EngineCredentialSourceFactory)
	}
	return svc
}

func cloneSearchProviderConfig(input map[string]search.ProviderConfig) map[string]search.ProviderConfig {
	out := make(map[string]search.ProviderConfig, len(input))
	for id, config := range input {
		copyConfig := config
		copyConfig.Kinds = append([]string(nil), config.Kinds...)
		copyConfig.EngineCredentials = cloneCredentialMap(config.EngineCredentials)
		if len(config.Native.Engines) > 0 {
			copyConfig.Native.Engines = make(map[string]search.NativeEngineRuntimeConfig, len(config.Native.Engines))
			for engineID, engineConfig := range config.Native.Engines {
				copyConfig.Native.Engines[engineID] = engineConfig
			}
		}
		out[id] = copyConfig
	}
	return out
}

func cloneCredentialMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
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
		case "native":
			provider := native.NewProvider(nativeengines.DefaultRegistry(nil), true, config.EffectiveMaxResponseBytes()).Configure(nativeProviderRuntimeConfig(pc.Native))
			p = provider
		case "exa":
			provider := exa.NewProvider("", pc.CredentialRef, pc.Endpoint, true)
			provider.SetMaxResponseBytes(config.EffectiveMaxResponseBytes())
			p = provider
		case "tavily":
			provider := tavily.NewProvider("", pc.CredentialRef, pc.Endpoint, true)
			provider.SetMaxResponseBytes(config.EffectiveMaxResponseBytes())
			p = provider
		}
		if p != nil {
			p = restrictSearchProviderKinds(p, pc.Kinds)
			set.RegisterWithPriority(id, p, pc.Priority)
		}
	}
	if _, ok := set.Get(native.ProviderID); !ok && nativeProviderEnabled(config) {
		nativeConfig := search.NativeProviderRuntimeConfig{}
		if configured, ok := config.Providers[native.ProviderID]; ok {
			nativeConfig = configured.Native
		}
		set.RegisterWithPriority(native.ProviderID, native.NewProvider(nativeengines.DefaultRegistry(nil), true, config.EffectiveMaxResponseBytes()).Configure(nativeProviderRuntimeConfig(nativeConfig)), 100)
	}
}

func nativeProviderRuntimeConfig(config search.NativeProviderRuntimeConfig) native.ProviderRuntimeConfig {
	result := native.DefaultProviderRuntimeConfig()
	if config.MaxEngines > 0 {
		result.MaxEngines = config.MaxEngines
	}
	if config.DefaultEngineTimeout > 0 {
		result.DefaultEngineTimeout = config.DefaultEngineTimeout
	}
	if config.DefaultRatePerMinute > 0 {
		result.DefaultRatePerMinute = config.DefaultRatePerMinute
	}
	if config.DefaultBurst > 0 {
		result.DefaultBurst = config.DefaultBurst
	}
	if len(config.Engines) > 0 {
		result.Engines = make(map[string]native.EngineRuntimeConfig, len(config.Engines))
		for id, engine := range config.Engines {
			result.Engines[strings.ToLower(strings.TrimSpace(id))] = native.EngineRuntimeConfig{
				Enabled: engine.Enabled, Timeout: engine.Timeout, RateLimitPerMinute: engine.RateLimitPerMinute, Burst: engine.Burst,
			}
		}
	}
	return result
}

func nativeProviderEnabled(config search.Config) bool {
	if !strings.EqualFold(strings.TrimSpace(config.DefaultProvider), native.ProviderID) {
		return false
	}
	provider, ok := config.Providers[native.ProviderID]
	if !ok {
		return true
	}
	return provider.Enabled
}

type configuredSearchProvider struct {
	search.Provider
	capabilities search.ProviderCapabilities
	allowedKinds map[search.SearchKind]struct{}
}

func (p *configuredSearchProvider) Capabilities() search.ProviderCapabilities {
	return p.capabilities
}

func (p *configuredSearchProvider) CapabilitiesForContext(ctx context.Context) search.ProviderCapabilities {
	caps := p.Provider.Capabilities()
	if contextual, ok := p.Provider.(search.ContextCapabilitiesProvider); ok {
		caps = contextual.CapabilitiesForContext(ctx)
	}
	return restrictSearchCapabilities(caps, p.allowedKinds)
}

func (p *configuredSearchProvider) ValidateSearchRequest(ctx context.Context, request search.SearchRequest) *search.Error {
	kind := search.NormalizeKind(request.Kind)
	if _, ok := p.allowedKinds[kind]; !ok {
		return search.NewError(search.SEARCH_KIND_UNSUPPORTED, p.ID(), false, nil)
	}
	if validator, ok := p.Provider.(search.RequestCapabilityProvider); ok {
		return validator.ValidateSearchRequest(ctx, request)
	}
	caps := p.CapabilitiesForContext(ctx)
	return search.ProviderSupportsFilter(caps, kind, request.Language != "", request.Country != "", request.SafeSearch != "", request.TimeRange != nil, len(request.Domains) > 0, len(request.ExcludeDomains) > 0)
}

func restrictSearchCapabilities(caps search.ProviderCapabilities, allowed map[search.SearchKind]struct{}) search.ProviderCapabilities {
	if len(allowed) == 0 {
		return caps
	}
	_, allowWeb := allowed[search.SearchKindWeb]
	caps.GeneralWeb = caps.GeneralWeb && allowWeb
	filtered := make([]search.SearchKind, 0, len(caps.SearchKinds))
	for _, kind := range caps.SearchKinds {
		if _, ok := allowed[kind]; ok {
			filtered = append(filtered, kind)
		}
	}
	caps.SearchKinds = filtered
	return caps
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
	caps := restrictSearchCapabilities(provider.Capabilities(), allowed)
	return &configuredSearchProvider{Provider: provider, capabilities: caps, allowedKinds: allowed}
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
	phase := strings.TrimSpace(event.Phase)
	progress := func(prefix string) string {
		if event.Total > 0 && event.Completed > 0 {
			return fmt.Sprintf("%s %d/%d", prefix, event.Completed, event.Total)
		}
		if event.Completed > 0 {
			return fmt.Sprintf("%s %d", prefix, event.Completed)
		}
		return prefix
	}
	switch phase {
	case "searching":
		if event.Total > 0 {
			return fmt.Sprintf("正在准备 %d 个搜索查询", event.Total)
		}
		return "正在搜索网络"
	case "search_query":
		if query := strings.TrimSpace(event.Query); query != "" {
			return "正在搜索：" + query
		}
		return "正在搜索来源"
	case "source_found":
		message := progress("已发现来源")
		if title := strings.TrimSpace(event.Title); title != "" {
			return message + " · " + title
		}
		return message
	case "opening":
		if title := strings.TrimSpace(event.Title); title != "" {
			return "正在阅读：" + title
		}
		return "正在阅读来源"
	case "opened":
		message := progress("已读取来源")
		if title := strings.TrimSpace(event.Title); title != "" {
			return message + " · " + title
		}
		return message
	case "found":
		if event.Completed > 0 {
			return fmt.Sprintf("已定位 %d 处相关内容", event.Completed)
		}
		return "已定位相关内容"
	case "research_round":
		if event.Total > 0 && event.Completed > 0 {
			return fmt.Sprintf("正在进行第 %d/%d 轮研究", event.Completed, event.Total)
		}
		return "正在进行深度研究"
	case "research_followup":
		if query := strings.TrimSpace(event.Query); query != "" {
			return "正在补充证据：" + query
		}
		return "正在补充证据"
	case "evidence_check":
		if event.Total > 0 {
			return fmt.Sprintf("正在交叉验证 %d 条证据", event.Total)
		}
		return "正在交叉验证证据"
	case "research_stop":
		return "研究已收敛 · " + researchStopReasonLabel(strings.TrimSpace(event.Message))
	case "screenshot":
		return "正在获取视觉证据"
	}
	if message := strings.TrimSpace(event.Message); message != "" {
		return message
	}
	if phase != "" {
		return strings.ReplaceAll(phase, "_", " ")
	}
	return "联网研究进行中"
}

func researchStopReasonLabel(reason string) string {
	switch reason {
	case "coverage_satisfied":
		return "关键问题已覆盖"
	case "max_rounds":
		return "达到研究轮次上限"
	case "max_search_calls":
		return "达到搜索调用上限"
	case "max_provider_cost":
		return "达到 Provider 成本上限"
	case "max_provider_credits":
		return "达到 Provider credits 上限"
	case "no_new_queries":
		return "没有新的有效查询"
	case "no_new_sources":
		return "没有发现新的有效来源"
	case "low_information_gain":
		return "新增信息已低于阈值"
	case "budget_satisfied":
		return "研究预算已满足"
	case "":
		return "完成"
	default:
		return reason
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
