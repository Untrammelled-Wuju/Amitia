package search

import (
	"strings"
	"time"
)

const ProviderNative = "native"

type ProviderRouteConfig struct {
	Preferred []string `mapstructure:"preferred"`
	Fallback  []string `mapstructure:"fallback"`
}

type ProviderConfig struct {
	Type              string            `mapstructure:"type"`
	Endpoint          string            `mapstructure:"endpoint"`
	CredentialRef     string            `mapstructure:"credentialRef"`
	Enabled           bool              `mapstructure:"enabled"`
	Priority          int               `mapstructure:"priority"`
	Kinds             []string          `mapstructure:"kinds"`
	AllowHTTP         bool              `mapstructure:"allowHttp"`
	AllowPrivate      bool              `mapstructure:"allowPrivate"`
	EngineCredentials map[string]string `mapstructure:"engineCredentials"`
}

type Config struct {
	Enabled          bool                           `mapstructure:"enabled"`
	DefaultProvider  string                         `mapstructure:"defaultProvider"`
	DefaultLimit     int                            `mapstructure:"defaultLimit"`
	MaxLimit         int                            `mapstructure:"maxLimit"`
	Timeout          time.Duration                  `mapstructure:"timeout"`
	MaxResponseBytes int64                          `mapstructure:"maxResponseBytes"`
	CacheTTL         time.Duration                  `mapstructure:"cacheTtl"`
	CacheMaxEntries  int                            `mapstructure:"cacheMaxEntries"`
	NegativeCacheTTL time.Duration                  `mapstructure:"negativeCacheTtl"`
	CircuitFailures  int                            `mapstructure:"circuitFailures"`
	CircuitOpen      time.Duration                  `mapstructure:"circuitOpen"`
	Providers        map[string]ProviderConfig      `mapstructure:"providers"`
	Routes           map[string]ProviderRouteConfig `mapstructure:"routes"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		DefaultLimit:     8,
		MaxLimit:         20,
		Timeout:          10 * time.Second,
		MaxResponseBytes: 2 * 1024 * 1024,
		CacheTTL:         2 * time.Minute,
		CacheMaxEntries:  512,
		NegativeCacheTTL: 30 * time.Second,
		CircuitFailures:  3,
		CircuitOpen:      30 * time.Second,
		Providers:        map[string]ProviderConfig{},
		Routes:           map[string]ProviderRouteConfig{},
	}
}

func (c Config) EffectiveCircuitFailures() int {
	if c.CircuitFailures > 0 {
		return c.CircuitFailures
	}
	return 3
}

func (c Config) EffectiveCircuitOpen() time.Duration {
	if c.CircuitOpen > 0 {
		return c.CircuitOpen
	}
	return 30 * time.Second
}

func (c Config) EffectiveTimeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 10 * time.Second
}

func (c Config) EffectiveMaxResponseBytes() int64 {
	if c.MaxResponseBytes > 0 {
		return c.MaxResponseBytes
	}
	return 2 * 1024 * 1024
}

func (c Config) HasProvider() bool {
	if strings.EqualFold(strings.TrimSpace(c.DefaultProvider), ProviderNative) {
		provider, ok := c.Providers[ProviderNative]
		if !ok || provider.Enabled {
			return true
		}
	}
	for _, p := range c.Providers {
		if p.Enabled {
			return true
		}
	}
	return false
}

func (c Config) IsProviderEnabled(id string) bool {
	p, ok := c.Providers[id]
	if !ok && strings.EqualFold(strings.TrimSpace(id), ProviderNative) {
		return strings.EqualFold(strings.TrimSpace(c.DefaultProvider), ProviderNative)
	}
	if !ok {
		return false
	}
	return p.Enabled
}

func (c Config) ProviderCredentialRef(id string) string {
	p, ok := c.Providers[id]
	if !ok {
		return ""
	}
	return p.CredentialRef
}

func (c Config) ProviderEngineCredentials(id string) map[string]string {
	p, ok := c.Providers[id]
	if !ok || len(p.EngineCredentials) == 0 {
		return nil
	}
	return p.EngineCredentials
}

func (c Config) EffectiveCacheTTL() time.Duration {
	if c.CacheTTL < 0 {
		return 0
	}
	if c.CacheTTL > 0 {
		return c.CacheTTL
	}
	return 2 * time.Minute
}

func (c Config) EffectiveNegativeCacheTTL() time.Duration {
	if c.NegativeCacheTTL < 0 {
		return 0
	}
	if c.NegativeCacheTTL > 0 {
		return c.NegativeCacheTTL
	}
	return 30 * time.Second
}

func (c Config) EffectiveCacheMaxEntries() int {
	if c.CacheMaxEntries > 0 {
		return c.CacheMaxEntries
	}
	return 512
}
