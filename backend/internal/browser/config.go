package browser

import (
	"time"
)

const (
	defaultBrowserStartupTimeout  = 30 * time.Second
	defaultBrowserShutdownTimeout = 5 * time.Second
	defaultMaxBrowserMemoryBytes  = 0
)

type BrowserConfigResolver interface {
	Resolve() BrowserConfig
}

type defaultConfigResolver struct {
	config BrowserConfig
}

func NewBrowserConfigResolver(config BrowserConfig) BrowserConfigResolver {
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = defaultBrowserStartupTimeout
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = defaultBrowserShutdownTimeout
	}
	if config.MaxBrowserMemoryBytes <= 0 {
		config.MaxBrowserMemoryBytes = defaultMaxBrowserMemoryBytes
	}
	if len(config.AllowedSchemes) == 0 {
		config.AllowedSchemes = []string{"http", "https", "about"}
	}
	if !config.Headless {
		config.Headless = true
	}
	return &defaultConfigResolver{config: config}
}

func (r *defaultConfigResolver) Resolve() BrowserConfig {
	return r.config
}

func BrowserConfigFromMap(m map[string]any) BrowserConfig {
	config := BrowserConfig{
		Enabled:        false,
		Headless:       true,
		StartupTimeout: defaultBrowserStartupTimeout,
		ShutdownTimeout: defaultBrowserShutdownTimeout,
		AllowedSchemes: []string{"http", "https", "about"},
	}

	if v, ok := m["enabled"].(bool); ok {
		config.Enabled = v
	}
	if v, ok := m["executablePath"].(string); ok {
		config.ExecutablePath = v
	}
	if v, ok := m["headless"].(bool); ok {
		config.Headless = v
	}
	if v, ok := m["startupTimeout"].(time.Duration); ok && v > 0 {
		config.StartupTimeout = v
	}
	if v, ok := m["shutdownTimeout"].(time.Duration); ok && v > 0 {
		config.ShutdownTimeout = v
	}
	if v, ok := m["userDataRoot"].(string); ok {
		config.UserDataRoot = v
	}
	if v, ok := m["maxBrowserMemoryBytes"].(int64); ok && v > 0 {
		config.MaxBrowserMemoryBytes = v
	}
	if v, ok := m["allowedSchemes"].([]string); ok && len(v) > 0 {
		config.AllowedSchemes = v
	}

	return config
}
