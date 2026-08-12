package browser

import (
	"context"
	"path/filepath"
	"testing"
)

func TestExecutableResolverDisabled(t *testing.T) {
	config := BrowserConfig{Enabled: false}
	resolver := NewBrowserExecutableResolver(config)
	_, err := resolver.Resolve(context.Background())
	if err == nil {
		t.Fatal("expected error when browser is disabled")
	}
	if !IsBrowserDisabled(err) {
		t.Fatalf("expected browser_disabled error, got: %v", err)
	}
}

func TestClassifyExecutable(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/usr/bin/google-chrome", browserKindChrome},
		{"/usr/bin/google-chrome-stable", browserKindChrome},
		{"/usr/bin/chromium", browserKindChromium},
		{"/usr/bin/chromium-browser", browserKindChromium},
		{`C:\Program Files\Microsoft\Edge\Application\msedge.exe`, browserKindEdge},
		{`C:\Program Files\Google\Chrome\Application\chrome.exe`, browserKindChrome},
		{"/usr/bin/firefox", browserKindUnknown},
		{"/usr/bin/safari", browserKindUnknown},
		{"chrome", browserKindChrome},
		{"chromium", browserKindChromium},
		{"msedge", browserKindEdge},
		{"unknown-browser", browserKindUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := classifyExecutable(tc.path)
			if got != tc.want {
				t.Fatalf("classifyExecutable(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestBrowserConfigFromMap(t *testing.T) {
	m := map[string]any{
		"enabled":               true,
		"executablePath":        "/usr/bin/chrome",
		"headless":              false,
		"startupTimeout":        0,
		"shutdownTimeout":       0,
		"userDataRoot":          "/tmp/browser",
		"maxBrowserMemoryBytes": int64(512 * 1024 * 1024),
		"allowedSchemes":        []string{"http", "https"},
	}

	config := BrowserConfigFromMap(m)
	if !config.Enabled {
		t.Fatal("expected enabled")
	}
	if config.ExecutablePath != "/usr/bin/chrome" {
		t.Fatalf("unexpected ExecutablePath: %s", config.ExecutablePath)
	}
	if config.Headless {
		t.Fatal("expected headless=false")
	}
	if config.UserDataRoot != "/tmp/browser" {
		t.Fatalf("unexpected UserDataRoot: %s", config.UserDataRoot)
	}
	if config.MaxBrowserMemoryBytes != 512*1024*1024 {
		t.Fatalf("unexpected MaxBrowserMemoryBytes: %d", config.MaxBrowserMemoryBytes)
	}
	if len(config.AllowedSchemes) != 2 {
		t.Fatalf("unexpected AllowedSchemes length: %d", len(config.AllowedSchemes))
	}
}

func TestBrowserConfigFromMapDefaults(t *testing.T) {
	config := BrowserConfigFromMap(map[string]any{})
	if config.Enabled {
		t.Fatal("expected disabled by default")
	}
	if !config.Headless {
		t.Fatal("expected headless=true by default")
	}
	if len(config.AllowedSchemes) == 0 {
		t.Fatal("expected default allowed schemes")
	}
}

func TestNewBrowserConfigResolver(t *testing.T) {
	config := BrowserConfig{
		Enabled:        true,
		StartupTimeout: 0,
	}
	resolver := NewBrowserConfigResolver(config)
	resolved := resolver.Resolve()
	if resolved.StartupTimeout <= 0 {
		t.Fatal("expected default startup timeout to be set")
	}
}

func TestCollectPackagedPaths(t *testing.T) {
	config := BrowserConfig{UserDataRoot: "/tmp/test-browser"}
	paths := collectPackagedPaths(config)
	if len(paths) == 0 {
		t.Fatal("expected packaged paths to be generated")
	}
	found := false
	for _, p := range paths {
		if filepath.Base(p) == "chrome-headless-shell" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected chrome-headless-shell in packaged paths")
	}
}

func TestBrowserCapabilitiesDefaultFalse(t *testing.T) {
	provider := NewDisabledProvider()
	caps := provider.BrowserCapabilities()
	if caps.SupportsNavigation || caps.SupportsDOM || caps.SupportsInteraction ||
		caps.SupportsDownload || caps.SupportsUpload || caps.SupportsScreenshot {
		t.Fatal("disabled provider capabilities should all be false")
	}
}
