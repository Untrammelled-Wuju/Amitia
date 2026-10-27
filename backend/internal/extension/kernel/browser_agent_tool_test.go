package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBrowserAgentToolIsNotRegisteredWithoutRuntimeCapabilities(t *testing.T) {
	if browserAgentAvailable(browser.NewDisabledProvider()) {
		t.Fatal("disabled browser provider must not expose browser agent")
	}
	registry := capability.NewToolRegistry()
	if err := registerBrowserAgentTool(context.Background(), registry, browser.NewDisabledProvider()); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.GetByModelName(context.Background(), "browser.agent.run"); ok {
		t.Fatal("browser.agent.run must not be exposed when browser runtime is unavailable")
	}
}
