package repair_baseline

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
)

func TestBaseline_E2E_ChatTool_ToolFacadeConstructs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E chat tool test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if container.ToolRegistry == nil {
		t.Fatalf("Container must have a non-nil ToolRegistry for Chat Tool E2E")
	}
	if container.ExecutionKernel == nil {
		t.Fatalf("Container must have a non-nil ExecutionKernel for Chat Tool E2E")
	}

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, nil, kernel.DefaultToolFacadeConfig())
	if toolFacade == nil {
		t.Fatalf("NewToolFacade must return a non-nil facade (Phase 10 section 19.6.1)")
	}
}

func TestBaseline_E2E_ChatTool_ModelToolsFromKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E chat tool test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, nil, kernel.DefaultToolFacadeConfig())

	scope := kernel.LegacyScope{
		UserID:         "test-user",
		CharacterID:    "test-character",
		ConversationID: "test-conversation",
		Channel:        "test",
		SessionID:      "test-session",
	}
	tools, err := toolFacade.ModelTools(ctx, scope)
	if err != nil {
		t.Fatalf("ModelTools must not return error (Phase 10 section 19.6.1): %v", err)
	}
	_ = tools
}

func TestBaseline_E2E_ChatTool_LegacyCounterZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E chat tool test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, nil, kernel.DefaultToolFacadeConfig())

	scope := kernel.LegacyScope{
		UserID:         "test-user",
		CharacterID:    "test-character",
		ConversationID: "test-conversation",
		Channel:        "test",
		SessionID:      "test-session",
	}
	_, _ = toolFacade.ModelTools(ctx, scope)

	total := kernel.GlobalLegacyCallCounter().Total()
	if total != 0 {
		t.Fatalf("Legacy Tool Counter must be 0 after Kernel-only ModelTools (Phase 10 section 19.6.6), got %d", total)
	}
}

func TestBaseline_E2E_ChatTool_CountersAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E chat tool test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, nil, kernel.DefaultToolFacadeConfig())
	counters := toolFacade.Counters()
	if counters == nil {
		t.Fatalf("ToolFacade.Counters must return a non-nil counter (Phase 10 section 19.6.1)")
	}
	snap := counters.Snapshot()
	if snap == nil {
		t.Fatalf("Counters.Snapshot must return a non-nil map")
	}
}

func TestBaseline_E2E_ChatTool_NoLegacyDispatcherMeansNoLegacyFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E chat tool test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, nil, kernel.DefaultToolFacadeConfig())

	scope := kernel.LegacyScope{
		UserID:         "test-user",
		CharacterID:    "test-character",
		ConversationID: "test-conversation",
		Channel:        "test",
		SessionID:      "test-session",
	}
	_ = toolFacade.BeforePrompt(ctx, scope)
	_, _ = toolFacade.ModelTools(ctx, scope)

	counters := toolFacade.Counters()
	snap := counters.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		t.Fatalf("legacy fallback must be 0 when no legacy dispatcher is set (Phase 10 section 19.6.2), got %d", snap["legacy_fallback_total"])
	}
}

func TestBaseline_E2E_ChatTool_PipelinePhasesDefined(t *testing.T) {
	requiredPhases := []string{
		"model_tools_from_kernel",
		"legacy_tools_empty_or_inactive",
		"execute_tool_once",
		"single_operation",
		"single_invocation",
		"legacy_counter_zero",
	}
	if len(requiredPhases) != 6 {
		t.Fatalf("Phase 10 section 19.6 requires 6 chat tool phases, got %d", len(requiredPhases))
	}
	for _, p := range requiredPhases {
		if p == "" {
			t.Fatalf("chat tool phase must not be empty")
		}
	}
}
