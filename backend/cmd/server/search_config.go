package main

import (
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/webresearch"
)

func searchConfigFromAppConfig(appCfg *config.Config) search.Config {
	out := search.DefaultConfig()
	if appCfg == nil {
		return out
	}
	in := appCfg.Providers.Search
	out.Enabled = in.Enabled
	out.DefaultProvider = strings.TrimSpace(in.DefaultProvider)
	out.DefaultLimit = in.DefaultLimit
	out.MaxLimit = in.MaxLimit
	if in.TimeoutSec > 0 {
		out.Timeout = time.Duration(in.TimeoutSec) * time.Second
	}
	out.MaxResponseBytes = in.MaxResponseBytes
	if in.CacheTTLSec < 0 {
		out.CacheTTL = -1
	} else if in.CacheTTLSec > 0 {
		out.CacheTTL = time.Duration(in.CacheTTLSec) * time.Second
	}
	out.CacheMaxEntries = in.CacheMaxEntries
	if in.NegativeCacheTTLSec < 0 {
		out.NegativeCacheTTL = -1
	} else if in.NegativeCacheTTLSec > 0 {
		out.NegativeCacheTTL = time.Duration(in.NegativeCacheTTLSec) * time.Second
	}
	out.CircuitFailures = in.CircuitFailures
	if in.CircuitOpenSec > 0 {
		out.CircuitOpen = time.Duration(in.CircuitOpenSec) * time.Second
	}
	out.Providers = make(map[string]search.ProviderConfig, len(in.Providers))
	for id, provider := range in.Providers {
		nativeConfig := search.NativeProviderRuntimeConfig{
			MaxEngines:           provider.Native.MaxEngines,
			DefaultRatePerMinute: provider.Native.DefaultRatePerMinute,
			DefaultBurst:         provider.Native.DefaultBurst,
			Engines:              make(map[string]search.NativeEngineRuntimeConfig, len(provider.Native.Engines)),
		}
		if provider.Native.DefaultEngineTimeoutSec > 0 {
			nativeConfig.DefaultEngineTimeout = time.Duration(provider.Native.DefaultEngineTimeoutSec) * time.Second
		}
		for engineID, engine := range provider.Native.Engines {
			nativeConfig.Engines[engineID] = search.NativeEngineRuntimeConfig{
				Enabled:            engine.Enabled,
				Timeout:            time.Duration(engine.TimeoutSec) * time.Second,
				RateLimitPerMinute: engine.RateLimitPerMinute,
				Burst:              engine.Burst,
			}
		}
		out.Providers[id] = search.ProviderConfig{
			Type: provider.Type, Endpoint: provider.Endpoint, CredentialRef: provider.CredentialRef,
			Enabled: provider.Enabled, Priority: provider.Priority, Kinds: append([]string(nil), provider.Kinds...),
			AllowHTTP: provider.AllowHTTP, AllowPrivate: provider.AllowPrivate,
			EngineCredentials: cloneStringMap(provider.EngineCredentials), Native: nativeConfig,
		}
	}
	out.Routes = make(map[string]search.ProviderRouteConfig, len(in.Routes))
	for name, route := range in.Routes {
		out.Routes[name] = search.ProviderRouteConfig{Preferred: append([]string(nil), route.Preferred...), Fallback: append([]string(nil), route.Fallback...)}
	}
	return out
}

func webResearchConfigFromAppConfig(appCfg *config.Config) webresearch.Config {
	out := webresearch.DefaultConfig()
	if appCfg == nil {
		return out
	}
	searchCfg := appCfg.Providers.Search
	in := searchCfg.Research
	out.Enabled = searchCfg.Enabled
	out.DeepResearchEnabled = in.DeepResearchEnabled
	out.BrowserEnabled = in.BrowserEnabled
	out.PDFEnabled = in.PDFEnabled
	if in.PDFTextCommand != "" {
		out.PDFTextCommand = in.PDFTextCommand
	}
	if in.PDFInfoCommand != "" {
		out.PDFInfoCommand = in.PDFInfoCommand
	}
	if in.PDFTimeoutSec > 0 {
		out.PDFTimeout = time.Duration(in.PDFTimeoutSec) * time.Second
	}
	if in.MaxExecutionSec > 0 {
		out.MaxExecution = time.Duration(in.MaxExecutionSec) * time.Second
	}
	if in.MaxToolOutputChars > 0 {
		out.MaxToolOutputChars = in.MaxToolOutputChars
	}
	if in.MaxQueries > 0 {
		out.MaxQueries = in.MaxQueries
	}
	if in.MaxProvidersPerQuery > 0 {
		out.MaxProvidersPerQuery = in.MaxProvidersPerQuery
	}
	if in.MaxSearchResults > 0 {
		out.MaxSearchResults = in.MaxSearchResults
	}
	if in.MaxResultsPerDomain > 0 {
		out.MaxResultsPerDomain = in.MaxResultsPerDomain
	}
	if in.MaxOpenPages > 0 {
		out.MaxOpenPages = in.MaxOpenPages
	}
	if in.MaxDeepOpenPages > 0 {
		out.MaxDeepOpenPages = in.MaxDeepOpenPages
	}
	if in.MaxDeepRounds > 0 {
		out.MaxDeepRounds = in.MaxDeepRounds
	}
	maxCalls := in.MaxSearchCalls
	if maxCalls <= 0 {
		maxCalls = in.MaxDeepSearchCalls
	}
	if maxCalls > 0 {
		out.MaxSearchCalls = maxCalls
	}
	if in.MaxProviderCostUSD > 0 {
		out.MaxProviderCostUSD = in.MaxProviderCostUSD
	}
	if in.MaxProviderCredits > 0 {
		out.MaxProviderCredits = in.MaxProviderCredits
	}
	if in.MinDeepNewSources > 0 {
		out.MinDeepNewSources = in.MinDeepNewSources
	}
	if in.MaxParallelSearch > 0 {
		out.MaxParallelSearch = in.MaxParallelSearch
	}
	if in.MaxParallelFetch > 0 {
		out.MaxParallelFetch = in.MaxParallelFetch
	}
	if in.SearchTimeoutSec > 0 {
		out.SearchTimeout = time.Duration(in.SearchTimeoutSec) * time.Second
	}
	if in.FetchTimeoutSec > 0 {
		out.FetchTimeout = time.Duration(in.FetchTimeoutSec) * time.Second
	}
	if in.MaxFetchBytes > 0 {
		out.MaxFetchBytes = in.MaxFetchBytes
	}
	if in.MaxPageChars > 0 {
		out.MaxPageChars = in.MaxPageChars
	}
	if in.MaxEvidencePerPage > 0 {
		out.MaxEvidencePerPage = in.MaxEvidencePerPage
	}
	if in.MaxEvidenceChars > 0 {
		out.MaxEvidenceChars = in.MaxEvidenceChars
	}
	if in.ReferenceTTLHours > 0 {
		out.ReferenceTTL = time.Duration(in.ReferenceTTLHours) * time.Hour
	}
	if in.PageCacheTTLSec < 0 {
		out.PageCacheTTL = 0
	} else if in.PageCacheTTLSec > 0 {
		out.PageCacheTTL = time.Duration(in.PageCacheTTLSec) * time.Second
	}
	out.AutoBrowserEscalation = in.AutoBrowserEscalation
	if in.MinStaticContentChars > 0 {
		out.MinStaticContentChars = in.MinStaticContentChars
	}
	if in.MaxRedirects > 0 {
		out.MaxRedirects = in.MaxRedirects
	}
	if in.MaxBrowserPages > 0 {
		out.MaxBrowserPages = in.MaxBrowserPages
	}
	if in.MaxBrowserScrolls >= 0 {
		out.MaxBrowserScrolls = in.MaxBrowserScrolls
	}
	if in.MaxBrowserSeconds > 0 {
		out.MaxBrowserTime = time.Duration(in.MaxBrowserSeconds) * time.Second
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
