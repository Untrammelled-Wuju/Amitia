package native

import (
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

// EngineRuntimeConfig controls one native engine without exposing provider
// implementation details to the model/tool schema.
type EngineRuntimeConfig struct {
	Enabled            *bool
	Timeout            time.Duration
	RateLimitPerMinute int
	Burst              int
}

type ProviderRuntimeConfig struct {
	MaxEngines           int
	DefaultEngineTimeout time.Duration
	DefaultRatePerMinute int
	DefaultBurst         int
	Engines              map[string]EngineRuntimeConfig
}

func DefaultProviderRuntimeConfig() ProviderRuntimeConfig {
	return ProviderRuntimeConfig{
		MaxEngines:           6,
		DefaultEngineTimeout: 8 * time.Second,
		DefaultRatePerMinute: 60,
		DefaultBurst:         6,
		Engines:              map[string]EngineRuntimeConfig{},
	}
}

type builtinPolicy struct {
	credentialIDs   []string
	defaultDisabled bool
	commercial      bool
	group           string
	timeout         time.Duration
	ratePerMinute   int
	burst           int
	usage           search.ProviderUsage
}

// Native intentionally does not contain the same commercial provider twice.
// Brave/Exa/Tavily live at the top-level provider layer, where their billing,
// credential and SLA semantics can be tracked accurately. The policies below
// cover credentialed vertical/native-only engines.
var builtinPolicies = map[string]builtinPolicy{
	"google_cse":  {credentialIDs: []string{"google_cse"}, group: "general", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"serper":      {credentialIDs: []string{"serper"}, commercial: true, group: "general", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"youtube":     {credentialIDs: []string{"youtube"}, group: "video", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"vimeo":       {credentialIDs: []string{"vimeo"}, group: "video", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"pexels":      {credentialIDs: []string{"pexels"}, group: "image", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"unsplash":    {credentialIDs: []string{"unsplash"}, group: "image", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"ebay_browse": {credentialIDs: []string{"ebay_browse"}, group: "product", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"500px":       {credentialIDs: []string{"500px"}, group: "image", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"genius":      {credentialIDs: []string{"genius"}, group: "music", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"soundcloud":  {credentialIDs: []string{"soundcloud"}, group: "music", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"pinterest":   {credentialIDs: []string{"pinterest"}, group: "image", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
	"wordnik":     {credentialIDs: []string{"wordnik"}, group: "dictionary", timeout: 8 * time.Second, ratePerMinute: 30, burst: 3, usage: search.ProviderUsage{Credits: 1}},
}

func normalizeDescriptor(descriptor EngineDescriptor) EngineDescriptor {
	descriptor.ID = strings.ToLower(strings.TrimSpace(descriptor.ID))
	if descriptor.Weight <= 0 {
		descriptor.Weight = 1
	}
	policy, ok := builtinPolicies[descriptor.ID]
	if !ok {
		return descriptor
	}
	if len(descriptor.CredentialIDs) == 0 {
		descriptor.CredentialIDs = append([]string(nil), policy.credentialIDs...)
	}
	if descriptor.Group == "" {
		descriptor.Group = policy.group
	}
	if descriptor.Timeout <= 0 {
		descriptor.Timeout = policy.timeout
	}
	if descriptor.RateLimitPerMinute <= 0 {
		descriptor.RateLimitPerMinute = policy.ratePerMinute
	}
	if descriptor.Burst <= 0 {
		descriptor.Burst = policy.burst
	}
	descriptor.Commercial = descriptor.Commercial || policy.commercial
	descriptor.DefaultDisabled = descriptor.DefaultDisabled || policy.defaultDisabled
	if descriptor.EstimatedUsage == (search.ProviderUsage{}) {
		descriptor.EstimatedUsage = policy.usage
	}
	return descriptor
}

func engineRuntimeConfig(config ProviderRuntimeConfig, engineID string) EngineRuntimeConfig {
	if config.Engines == nil {
		return EngineRuntimeConfig{}
	}
	return config.Engines[strings.ToLower(strings.TrimSpace(engineID))]
}

func engineEnabled(config ProviderRuntimeConfig, descriptor EngineDescriptor) bool {
	override := engineRuntimeConfig(config, descriptor.ID)
	if override.Enabled != nil {
		return *override.Enabled
	}
	return !descriptor.DefaultDisabled
}

func effectiveEngineTimeout(config ProviderRuntimeConfig, descriptor EngineDescriptor) time.Duration {
	override := engineRuntimeConfig(config, descriptor.ID)
	if override.Timeout > 0 {
		return override.Timeout
	}
	if descriptor.Timeout > 0 {
		return descriptor.Timeout
	}
	if config.DefaultEngineTimeout > 0 {
		return config.DefaultEngineTimeout
	}
	return 8 * time.Second
}

func effectiveEngineRate(config ProviderRuntimeConfig, descriptor EngineDescriptor) (int, int) {
	override := engineRuntimeConfig(config, descriptor.ID)
	rate := override.RateLimitPerMinute
	burst := override.Burst
	if rate <= 0 {
		rate = descriptor.RateLimitPerMinute
	}
	if burst <= 0 {
		burst = descriptor.Burst
	}
	if rate <= 0 {
		rate = config.DefaultRatePerMinute
	}
	if burst <= 0 {
		burst = config.DefaultBurst
	}
	if rate <= 0 {
		rate = 60
	}
	if burst <= 0 {
		burst = 6
	}
	return rate, burst
}
