package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func makeTestInstallerWithNamespace(t *testing.T) (*TypedContributionInstaller, *CandidateNamespace, *capability.ToolRegistry) {
	t.Helper()
	registry := capability.NewToolRegistry()
	ns := NewCandidateNamespace()
	installer := &TypedContributionInstaller{
		container:   &Container{ToolRegistry: registry},
		candidateNS: ns,
	}
	return installer, ns, registry
}

func makeToolContrib(id, extID string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID:          domain.ContributionID(id),
		ExtensionID: domain.ExtensionID(extID),
		ModuleID:    "mod-1",
		Kind:        domain.ContributionKindTool,
		Version:     "1.0.0",
		Name:        domain.LocalizedText{Default: id},
		Description: domain.LocalizedText{Default: "test tool"},
		Definition: map[string]any{
			"toolId":       id,
			"modelName":    id,
			"inputSchema":  map[string]any{"type": "object"},
			"outputSchema": map[string]any{"type": "object"},
		},
	}
}

func TestCandidateNamespace_RegisterIsolatesFromProduction(t *testing.T) {
	installer, ns, registry := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}

	if !ns.HasCandidate("cand-1") {
		t.Fatal("candidate should be in namespace after registration")
	}

	_, existsInProduction := registry.Get(ctx, "ext-a/candidate-tool")
	if existsInProduction {
		t.Fatal("candidate tool must NOT appear in production registry before promote")
	}

	entry, ok := ns.Load("cand-1")
	if !ok {
		t.Fatal("namespace entry should exist")
	}
	if len(entry.Contribs) != 1 {
		t.Fatalf("expected 1 contrib in namespace, got %d", len(entry.Contribs))
	}
	if len(entry.Keys) != 1 {
		t.Fatalf("expected 1 key in namespace, got %d", len(entry.Keys))
	}
	key := entry.Keys[0]
	if key.ExtensionID != "ext-a" || key.ContributionID != "ext-a/candidate-tool" || key.CandidateGeneration != 1 || key.DefinitionHash != "hash-1" {
		t.Fatalf("candidate key mismatch: %+v", key)
	}
}

func TestCandidateNamespace_ValidateMarksValidated(t *testing.T) {
	installer, ns, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}

	if ns.IsValidated("cand-1") {
		t.Fatal("candidate should not be validated before ValidateCandidateContributions")
	}

	if err := installer.ValidateCandidateContributions(ctx, "cand-1"); err != nil {
		t.Fatalf("ValidateCandidateContributions: %v", err)
	}

	if !ns.IsValidated("cand-1") {
		t.Fatal("candidate should be validated after ValidateCandidateContributions")
	}
}

func TestCandidateNamespace_PromoteRequiresValidation(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}

	if _, err := installer.PromoteCandidateContributions(ctx, "cand-1"); err == nil {
		t.Fatal("promote should fail when candidate has not been validated")
	}
}

func TestCandidateNamespace_PromoteAtomicallySwitches(t *testing.T) {
	installer, ns, registry := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool-1", "ext-a"),
		makeToolContrib("ext-a/candidate-tool-2", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}

	if err := installer.ValidateCandidateContributions(ctx, "cand-1"); err != nil {
		t.Fatalf("ValidateCandidateContributions: %v", err)
	}

	_, existsBefore := registry.Get(ctx, "ext-a/candidate-tool-1")
	if existsBefore {
		t.Fatal("candidate tool must NOT be in production before promote")
	}

	if _, err := installer.PromoteCandidateContributions(ctx, "cand-1"); err != nil {
		t.Fatalf("PromoteCandidateContributions: %v", err)
	}

	_, existsAfter1 := registry.Get(ctx, "ext-a/candidate-tool-1")
	if !existsAfter1 {
		t.Fatal("candidate tool-1 should be in production after promote")
	}
	_, existsAfter2 := registry.Get(ctx, "ext-a/candidate-tool-2")
	if !existsAfter2 {
		t.Fatal("candidate tool-2 should be in production after promote")
	}

	if err := installer.RemoveCandidateNamespaceAfterCommit(ctx, "cand-1"); err != nil {
		t.Fatalf("RemoveCandidateNamespaceAfterCommit: %v", err)
	}
	if ns.HasCandidate("cand-1") {
		t.Fatal("candidate should be removed from namespace after commit")
	}
}

func TestCandidateNamespace_DiscardPreservesProduction(t *testing.T) {
	installer, ns, registry := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	stableTool := capability.ToolDefinition{
		ID:           "ext-a/stable-tool",
		ModelName:    "stable",
		ExtensionID:  "ext-a",
		Source:       capability.ToolSourcePlugin,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable tool: %v", err)
	}

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}

	if err := installer.DiscardCandidateNamespace(ctx, "cand-1"); err != nil {
		t.Fatalf("DiscardCandidateNamespace: %v", err)
	}

	if ns.HasCandidate("cand-1") {
		t.Fatal("candidate should be removed from namespace after discard")
	}

	_, stableExists := registry.Get(ctx, "ext-a/stable-tool")
	if !stableExists {
		t.Fatal("stable tool must still exist in production after candidate discard")
	}

	_, candidateExists := registry.Get(ctx, "ext-a/candidate-tool")
	if candidateExists {
		t.Fatal("candidate tool must NOT exist in production after discard (was never promoted)")
	}
}

func TestCandidateNamespace_DiscardNamespaceIdempotent(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	if err := installer.DiscardCandidateNamespace(ctx, "nonexistent"); err != nil {
		t.Fatalf("discarding nonexistent candidate should be idempotent: %v", err)
	}
}

func TestCandidateNamespace_RegisterDuplicateFails(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate-tool", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err != nil {
		t.Fatalf("first RegisterCandidateContributions: %v", err)
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, contribs, "hash-1", ""); err == nil {
		t.Fatal("duplicate registration should fail")
	}
}

func TestCandidateNamespace_PromoteNonexistentFails(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	if _, err := installer.PromoteCandidateContributions(ctx, "nonexistent"); err == nil {
		t.Fatal("promoting nonexistent candidate should fail")
	}
}

func TestCandidateNamespace_ValidateNonexistentFails(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	if err := installer.ValidateCandidateContributions(ctx, "nonexistent"); err == nil {
		t.Fatal("validating nonexistent candidate should fail")
	}
}

func TestCandidateNamespace_RegisterInvalidContribFails(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	invalidContrib := domain.ContributionDefinition{
		ID:          domain.ContributionID("ext-a/bad"),
		ExtensionID: domain.ExtensionID("ext-a"),
		Kind:        domain.ContributionKind("unknown-kind"),
		Definition:  map[string]any{},
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 1, []domain.ContributionDefinition{invalidContrib}, "hash-1", ""); err == nil {
		t.Fatal("registering invalid contribution kind should fail")
	}
}

func TestCandidateNamespace_NilNamespaceSafe(t *testing.T) {
	registry := capability.NewToolRegistry()
	installer := &TypedContributionInstaller{
		container:   &Container{ToolRegistry: registry},
		candidateNS: nil,
	}
	ctx := context.Background()

	if err := installer.DiscardCandidateNamespace(ctx, "cand-1"); err != nil {
		t.Fatalf("DiscardCandidateNamespace with nil namespace should be safe: %v", err)
	}

	if installer.IsCandidateRegistered("cand-1") {
		t.Fatal("IsCandidateRegistered should return false for nil namespace")
	}

	if installer.IsCandidateValidated("cand-1") {
		t.Fatal("IsCandidateValidated should return false for nil namespace")
	}

	if list := installer.ListNamespaceCandidates(); list != nil {
		t.Fatal("ListNamespaceCandidates should return nil for nil namespace")
	}
}

func TestCandidateNamespace_StableGenerationPreservedOnPromoteFailure(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	ns := NewCandidateNamespace()

	stableTool := capability.ToolDefinition{
		ID:           "ext-a/stable",
		ModelName:    "stable",
		ExtensionID:  "ext-a",
		Source:       capability.ToolSourcePlugin,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable tool: %v", err)
	}

	installer := &TypedContributionInstaller{
		container:   &Container{ToolRegistry: registry},
		candidateNS: ns,
	}

	contribs := []domain.ContributionDefinition{
		makeToolContrib("ext-a/candidate", "ext-a"),
	}

	if err := installer.RegisterCandidateContributions(ctx, "cand-1", "ext-a", nil, "gen-1", 2, contribs, "hash-1", ""); err != nil {
		t.Fatalf("RegisterCandidateContributions: %v", err)
	}
	if err := installer.ValidateCandidateContributions(ctx, "cand-1"); err != nil {
		t.Fatalf("ValidateCandidateContributions: %v", err)
	}

	if _, err := installer.PromoteCandidateContributions(ctx, "cand-1"); err != nil {
		t.Fatalf("PromoteCandidateContributions: %v", err)
	}

	_, stableExists := registry.Get(ctx, "ext-a/stable")
	if !stableExists {
		t.Fatal("stable tool must still exist after candidate promote")
	}
	_, candidateExists := registry.Get(ctx, "ext-a/candidate")
	if !candidateExists {
		t.Fatal("candidate tool should exist in production after successful promote")
	}
	if err := installer.RemoveCandidateNamespaceAfterCommit(ctx, "cand-1"); err != nil {
		t.Fatalf("RemoveCandidateNamespaceAfterCommit: %v", err)
	}
	if ns.HasCandidate("cand-1") {
		t.Fatal("candidate should be removed from namespace after commit")
	}
}

func TestCandidateNamespace_ListByExtension(t *testing.T) {
	installer, _, _ := makeTestInstallerWithNamespace(t)
	ctx := context.Background()

	contribsA := []domain.ContributionDefinition{makeToolContrib("ext-a/tool", "ext-a")}
	contribsB := []domain.ContributionDefinition{makeToolContrib("ext-b/tool", "ext-b")}

	if err := installer.RegisterCandidateContributions(ctx, "cand-a", "ext-a", nil, "gen-a", 1, contribsA, "hash-a", ""); err != nil {
		t.Fatalf("register cand-a: %v", err)
	}
	if err := installer.RegisterCandidateContributions(ctx, "cand-b", "ext-b", nil, "gen-b", 1, contribsB, "hash-b", ""); err != nil {
		t.Fatalf("register cand-b: %v", err)
	}

	all := installer.ListNamespaceCandidates()
	if len(all) != 2 {
		t.Fatalf("expected 2 namespace candidates, got %d", len(all))
	}

	extAEntries := installer.candidateNS.ListByExtension("ext-a")
	if len(extAEntries) != 1 {
		t.Fatalf("expected 1 entry for ext-a, got %d", len(extAEntries))
	}
	if extAEntries[0].CandidateID != "cand-a" {
		t.Fatalf("expected cand-a, got %s", extAEntries[0].CandidateID)
	}
}

func TestCandidateNamespace_StoreAndRemove(t *testing.T) {
	ns := NewCandidateNamespace()

	entry := &CandidateNamespaceEntry{
		CandidateID: "cand-1",
		ExtensionID: "ext-a",
		Generation:  1,
	}
	if err := ns.Store(entry); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if ns.Count() != 1 {
		t.Fatalf("expected count 1, got %d", ns.Count())
	}

	ns.Remove("cand-1")
	if ns.HasCandidate("cand-1") {
		t.Fatal("candidate should be removed")
	}
	if ns.Count() != 0 {
		t.Fatalf("expected count 0 after remove, got %d", ns.Count())
	}
}

func TestCandidateNamespace_Clear(t *testing.T) {
	ns := NewCandidateNamespace()

	_ = ns.Store(&CandidateNamespaceEntry{CandidateID: "cand-1", ExtensionID: "ext-a"})
	_ = ns.Store(&CandidateNamespaceEntry{CandidateID: "cand-2", ExtensionID: "ext-b"})
	if ns.Count() != 2 {
		t.Fatalf("expected count 2, got %d", ns.Count())
	}

	ns.Clear()
	if ns.Count() != 0 {
		t.Fatalf("expected count 0 after clear, got %d", ns.Count())
	}
}
