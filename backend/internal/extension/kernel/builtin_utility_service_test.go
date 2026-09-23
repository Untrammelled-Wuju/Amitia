package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/browser"
)

type capabilityOnlyBrowserProvider struct {
	browser.BrowserProvider
	capabilities browser.BrowserCapabilities
}

func (p capabilityOnlyBrowserProvider) BrowserCapabilities() browser.BrowserCapabilities {
	return p.capabilities
}

func TestBuiltinUtilityBrowserToolsFollowProviderCapabilities(t *testing.T) {
	disabled := NewBuiltinUtilityService(BuiltinUtilityDeps{Browser: browser.NewDisabledProvider()})
	if disabled.Supports("visit_web") || disabled.Supports("browser_close_all") || disabled.Supports("browser_fill_form") {
		t.Fatal("disabled browser provider must not expose browser tools")
	}

	enabledProvider := capabilityOnlyBrowserProvider{
		capabilities: browser.BrowserCapabilities{
			SupportsNavigation:  true,
			SupportsDOM:         true,
			SupportsInteraction: true,
		},
	}
	enabled := NewBuiltinUtilityService(BuiltinUtilityDeps{Browser: enabledProvider})
	if !enabled.Supports("visit_web") || !enabled.Supports("browser_close_all") || !enabled.Supports("browser_fill_form") {
		t.Fatal("capable browser provider must expose browser tools")
	}
}
