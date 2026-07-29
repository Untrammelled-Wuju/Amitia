package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestDiscardCandidateContributions_PreservesStableTool(t *testing.T) {
	registry := capability.NewToolRegistry()
	ctx := context.Background()

	stableTool := capability.ToolDefinition{
		ID:          "ext-a/tool-stable",
		ModelName:   "stable",
		ExtensionID: "ext-a",
		Source:      capability.ToolSourcePlugin,
		Enabled:     true,
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable tool: %v", err)
	}

	candidateTool := capability.ToolDefinition{
		ID:          "ext-a/tool-candidate",
		ModelName:   "candidate",
		ExtensionID: "ext-a",
		Source:      capability.ToolSourcePlugin,
		Enabled:     false,
	}
	if err := registry.Register(ctx, candidateTool); err != nil {
		t.Fatalf("register candidate tool: %v", err)
	}

	installer := &TypedContributionInstaller{
		container: &Container{
			ToolRegistry: registry,
		},
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:         "ext-a/tool-candidate",
			ExtensionID: "ext-a",
			Kind:        domain.ContributionKindTool,
			Definition: map[string]any{
				"toolId": "ext-a/tool-candidate",
			},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-a", 2, candidateContribs, nil); err != nil {
		t.Fatalf("DiscardCandidateContributions: %v", err)
	}

	_, stableExists := registry.Get(ctx, "ext-a/tool-stable")
	if !stableExists {
		t.Fatal("stable tool should still exist after candidate discard")
	}

	_, candidateExists := registry.Get(ctx, "ext-a/tool-candidate")
	if candidateExists {
		t.Fatal("candidate tool should be removed after discard")
	}
}

func TestDiscardCandidateContributions_Idempotent(t *testing.T) {
	registry := capability.NewToolRegistry()
	ctx := context.Background()

	tool := capability.ToolDefinition{
		ID:          "ext-a/tool-1",
		ModelName:   "test",
		ExtensionID: "ext-a",
		Source:      capability.ToolSourcePlugin,
		Enabled:     false,
	}
	_ = registry.Register(ctx, tool)

	installer := &TypedContributionInstaller{
		container: &Container{
			ToolRegistry: registry,
		},
	}

	contribs := []domain.ContributionDefinition{
		{
			ID:         "ext-a/tool-1",
			ExtensionID: "ext-a",
			Kind:        domain.ContributionKindTool,
			Definition: map[string]any{
				"toolId": "ext-a/tool-1",
			},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-a", 2, contribs, nil); err != nil {
		t.Fatalf("first discard: %v", err)
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-a", 2, contribs, nil); err != nil {
		t.Fatalf("second discard should be idempotent: %v", err)
	}

	_, exists := registry.Get(ctx, "ext-a/tool-1")
	if exists {
		t.Fatal("tool should not exist after discard")
	}
}

func TestDiscardCandidateContributions_MultipleTypes(t *testing.T) {
	registry := capability.NewToolRegistry()
	ctx := context.Background()

	stableTool := capability.ToolDefinition{
		ID:          "ext-b/stable",
		ModelName:   "stable",
		ExtensionID: "ext-b",
		Source:      capability.ToolSourcePlugin,
		Enabled:     true,
	}
	_ = registry.Register(ctx, stableTool)

	candidateTool := capability.ToolDefinition{
		ID:          "ext-b/candidate",
		ModelName:   "candidate",
		ExtensionID: "ext-b",
		Source:      capability.ToolSourcePlugin,
		Enabled:     false,
	}
	_ = registry.Register(ctx, candidateTool)

	installer := &TypedContributionInstaller{
		container: &Container{
			ToolRegistry: registry,
		},
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:         "ext-b/candidate",
			ExtensionID: "ext-b",
			Kind:        domain.ContributionKindTool,
			Definition: map[string]any{
				"toolId": "ext-b/candidate",
			},
		},
		{
			ID:         "ext-b/unknown-kind",
			ExtensionID: "ext-b",
			Kind:        domain.ContributionKind("unknown"),
			Definition:  map[string]any{},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-b", 2, candidateContribs, nil); err != nil {
		t.Fatalf("discard with multiple types: %v", err)
	}

	if _, exists := registry.Get(ctx, "ext-b/stable"); !exists {
		t.Fatal("stable tool should exist")
	}
	if _, exists := registry.Get(ctx, "ext-b/candidate"); exists {
		t.Fatal("candidate tool should be removed")
	}
}

func TestDiscardCandidateContributions_NilContainer(t *testing.T) {
	installer := &TypedContributionInstaller{container: nil}
	err := installer.DiscardCandidateContributions(context.Background(), "ext-a", 1, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil container")
	}
}

func TestDiscardCandidateContributions_EmptyContribs(t *testing.T) {
	registry := capability.NewToolRegistry()
	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}
	err := installer.DiscardCandidateContributions(context.Background(), "ext-a", 1, nil, nil)
	if err != nil {
		t.Fatalf("empty contribs should not error: %v", err)
	}
}

func TestDiscardSingleContribution_ToolUsesFallbackID(t *testing.T) {
	registry := capability.NewToolRegistry()
	ctx := context.Background()

	tool := capability.ToolDefinition{
		ID:          "ext-c/my-contrib",
		ModelName:   "test",
		ExtensionID: "ext-c",
		Source:      capability.ToolSourcePlugin,
		Enabled:     false,
	}
	_ = registry.Register(ctx, tool)

	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}

	contrib := domain.ContributionDefinition{
		ID:         "ext-c/my-contrib",
		ExtensionID: "ext-c",
		Kind:        domain.ContributionKindTool,
		Definition:  map[string]any{},
	}

	if err := installer.discardSingleContribution(ctx, contrib, nil); err != nil {
		t.Fatalf("discardSingleContribution: %v", err)
	}

	if _, exists := registry.Get(ctx, "ext-c/my-contrib"); exists {
		t.Fatal("tool should be removed when using fallback ID")
	}
}

func TestDiscardCandidateContributions_HookNilService(t *testing.T) {
	registry := capability.NewToolRegistry()
	ctx := context.Background()

	stableTool := capability.ToolDefinition{
		ID:          "ext-d/stable",
		ModelName:   "stable",
		ExtensionID: "ext-d",
		Source:      capability.ToolSourcePlugin,
		Enabled:     true,
	}
	_ = registry.Register(ctx, stableTool)

	installer := &TypedContributionInstaller{
		container: &Container{
			ToolRegistry: registry,
		},
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:         "ext-d/hook-candidate",
			ExtensionID: "ext-d",
			Kind:        domain.ContributionKindHook,
			Definition: map[string]any{
				"contributionId": "ext-d/hook-candidate",
				"event":          "on_message",
				"handler":        "handle",
			},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-d", 2, candidateContribs, nil); err != nil {
		t.Fatalf("discard with nil hook service should not error: %v", err)
	}

	if _, exists := registry.Get(ctx, "ext-d/stable"); !exists {
		t.Fatal("stable tool should still exist after hook discard")
	}
}
