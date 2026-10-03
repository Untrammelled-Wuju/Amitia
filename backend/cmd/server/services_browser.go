package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/browser"
)

func browserConfigFromAppConfig(cfg *config.Config) browser.BrowserConfig {
	pc := cfg.Providers.Browser

	userDataRoot := strings.TrimSpace(pc.UserDataRoot)
	if userDataRoot == "" && strings.TrimSpace(cfg.Storage.DataDir) != "" {
		userDataRoot = filepath.Join(cfg.Storage.DataDir, "browser")
	}

	bc := browser.BrowserConfig{
		Enabled:               pc.Enabled,
		ExecutablePath:        pc.ExecutablePath,
		Headless:              pc.Headless,
		UserDataRoot:          userDataRoot,
		StartupTimeout:        time.Duration(pc.StartupTimeoutSec) * time.Second,
		ShutdownTimeout:       time.Duration(pc.ShutdownTimeoutSec) * time.Second,
		MaxBrowserMemoryBytes: pc.MaxBrowserMemoryBytes,
		AllowedSchemes:        append([]string(nil), pc.AllowedSchemes...),
		MaxSessions:           pc.MaxSessions,
		MaxTabsPerSession:     pc.MaxTabsPerSession,
		MaxTabsTotal:          pc.MaxTabsTotal,
		NavigationTimeout:     time.Duration(pc.NavigationTimeoutSec) * time.Second,
		MaxNavigationTimeout:  time.Duration(pc.MaxNavigationTimeoutSec) * time.Second,
	}

	return browser.NewBrowserConfigResolver(bc).Resolve()
}

func buildProductionBrowserProvider(cfg *config.Config, bootstrap *runtimeBootstrap) (browser.BrowserProvider, error) {
	browserConfig := browserConfigFromAppConfig(cfg)
	if bootstrap != nil && bootstrap.RuntimeHost() != nil {
		browser.SetGlobalRuntimeSupervisor(bootstrap.RuntimeHost().Processes())
	}
	factory := browser.NewBrowserEngineFactory()
	provider, err := browser.NewProductionProvider(browserConfig, factory)
	if err != nil {
		return nil, err
	}
	return provider, nil
}
