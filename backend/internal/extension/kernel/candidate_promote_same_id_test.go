package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/update"
)

func setupSameIDTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func setupSameIDTestContainer(t *testing.T, db *sql.DB) *Container {
	t.Helper()
	registry := capability.NewToolRegistry()
	contribRepo := sqlite.NewContributionRepository(db)
	instRepo := sqlite.NewInstallationRepository(db)
	return &Container{
		ToolRegistry:           registry,
		ContributionRepository: contribRepo,
		InstallationRepository: instRepo,
	}
}

func seedStableTool(t *testing.T, ctx context.Context, container *Container, extID string, toolID string, modelSuffix string) {
	t.Helper()
	stableTool := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    "stable-" + modelSuffix,
		ExtensionID:  extID,
		Source:       capability.ToolSourcePlugin,
		Name:         "Stable " + modelSuffix,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := container.ToolRegistry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable tool: %v", err)
	}

	stableContrib := domain.ContributionDefinition{
		ID:          domain.ContributionID(toolID),
		ModuleID:    "mod-1",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Stable " + modelSuffix},
		Version:     "1.0.0",
		Definition: map[string]any{
			"toolId":       toolID,
			"modelName":    "stable-" + modelSuffix,
			"inputSchema":  map[string]any{"type": "object"},
			"outputSchema": map[string]any{"type": "object"},
		},
	}
	if err := container.ContributionRepository.PutContribution(ctx, stableContrib); err != nil {
		t.Fatalf("put stable contribution: %v", err)
	}

	inst := domain.ExtensionInstallation{
		InstallationID:   "inst-" + extID,
		ExtensionID:      domain.ExtensionID(extID),
		InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		Generation:       1,
		EnablementState:  domain.EnablementEnabled,
		InstalledAt:      time.Now(),
	}
	if err := container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		t.Fatalf("put installation: %v", err)
	}
}

func buildCandidateToolContrib(extID string, toolID string, modelSuffix string) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID(toolID),
			ModuleID:    "mod-1",
			ExtensionID: domain.ExtensionID(extID),
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Candidate " + modelSuffix},
			Version:     "2.0.0",
			Definition: map[string]any{
				"toolId":       toolID,
				"modelName":    "candidate-" + modelSuffix,
				"inputSchema":  map[string]any{"type": "object"},
				"outputSchema": map[string]any{"type": "object"},
			},
		},
	}
}

func registerCandidateInManager(t *testing.T, ctx context.Context, mgr *CandidateContributionManager, extID string, generationID string, candidateGen int64, expectedStableGen int64, contribs []domain.ContributionDefinition) string {
	t.Helper()
	record := &CandidateRecord{
		CandidateID:              "cand-1",
		ExtensionID:              domain.ExtensionID(extID),
		GenerationID:             generationID,
		CandidateGeneration:      candidateGen,
		ExpectedStableGeneration: expectedStableGen,
		Contribs:                 contribs,
		DefinitionHash:           "hash-1",
	}
	if err := mgr.RegisterCandidate(ctx, record); err != nil {
		t.Fatalf("register candidate: %v", err)
	}
	if err := mgr.HealthCandidate(ctx, "cand-1"); err != nil {
		t.Fatalf("validate candidate: %v", err)
	}
	return "cand-1"
}

func TestSameID_PromoteTool_Success(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-same-id-tool"
	toolID := "ext-same-id-tool/chat"

	seedStableTool(t, ctx, container, extID, toolID, "chat")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}

	contribs := buildCandidateToolContrib(extID, toolID, "chat")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	candidateID := registerCandidateInManager(t, ctx, mgr, extID, candidateGen.GenerationID, int64(candidateGen.Generation), int64(stableGen.Generation), contribs)

	if err := mgr.PromoteCandidate(ctx, candidateID); err != nil {
		t.Fatalf("promote candidate: %v", err)
	}

	tool, exists := container.ToolRegistry.Get(ctx, toolID)
	if !exists {
		t.Fatal("tool should exist after successful promote")
	}
	if tool.ModelName != "candidate-chat" {
		t.Fatalf("expected candidate model name, got %s", tool.ModelName)
	}

	activeGen := genMgr.Active(ctx, extID)
	if activeGen == nil || activeGen.GenerationID != candidateGen.GenerationID {
		t.Fatal("candidate generation should be active after promote")
	}
}

func TestSameID_PromoteTool_TransitionFailure_RestoresStable(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-same-id-transfail"
	toolID := "ext-same-id-transfail/chat"

	seedStableTool(t, ctx, container, extID, toolID, "chat")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")

	contribs := buildCandidateToolContrib(extID, toolID, "chat")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	candidateID := registerCandidateInManager(t, ctx, mgr, extID, candidateGen.GenerationID, int64(candidateGen.Generation), int64(stableGen.Generation), contribs)

	err := mgr.PromoteCandidate(ctx, candidateID)
	if err == nil {
		t.Fatal("expected promote to fail due to invalid generation transition")
	}

	tool, exists := container.ToolRegistry.Get(ctx, toolID)
	if !exists {
		t.Fatal("stable tool should still exist after promote failure (restored from snapshot)")
	}
	if tool.ModelName != "stable-chat" {
		t.Fatalf("expected stable model name after restore, got %s", tool.ModelName)
	}

	record, ok := mgr.GetCandidate(candidateID)
	if !ok {
		t.Fatal("candidate record should still exist after failure")
	}
	if record.Status != CandidateStatusFailed {
		t.Fatalf("expected status failed, got %s", record.Status)
	}
}

func TestSameID_PromoteTool_CASFailure(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-same-id-cas"
	toolID := "ext-same-id-cas/chat"

	seedStableTool(t, ctx, container, extID, toolID, "chat")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	otherGen := genMgr.Prepare(ctx, extID, "1.5.0", "other-hash")
	if err := genMgr.Transition(ctx, extID, otherGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, otherGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, otherGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}

	contribs := buildCandidateToolContrib(extID, toolID, "chat")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)

	record := &CandidateRecord{
		CandidateID:              "cand-cas",
		ExtensionID:              domain.ExtensionID(extID),
		GenerationID:             candidateGen.GenerationID,
		CandidateGeneration:      int64(candidateGen.Generation),
		ExpectedStableGeneration: int64(stableGen.Generation),
		Contribs:                 contribs,
		DefinitionHash:           "hash-1",
	}
	if err := mgr.RegisterCandidate(ctx, record); err != nil {
		t.Fatalf("register candidate: %v", err)
	}

	err := mgr.PromoteCandidate(ctx, "cand-cas")
	if err == nil {
		t.Fatal("expected CAS check to fail")
	}

	tool, exists := container.ToolRegistry.Get(ctx, toolID)
	if !exists {
		t.Fatal("stable tool should still exist after CAS failure")
	}
	if tool.ModelName != "stable-chat" {
		t.Fatalf("expected stable model name, got %s", tool.ModelName)
	}
}

func TestSameID_DiscardCandidate_PreservesStableSameID(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()

	stableTool := capability.ToolDefinition{
		ID:           "ext-dup/tool",
		ModelName:    "stable",
		ExtensionID:  "ext-dup",
		Source:       capability.ToolSourcePlugin,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable: %v", err)
	}

	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:          "ext-dup/tool",
			ExtensionID: "ext-dup",
			Kind:        domain.ContributionKindTool,
			Definition: map[string]any{
				"toolId":      "ext-dup/tool",
				"modelName":   "candidate",
				"inputSchema": map[string]any{"type": "object"},
			},
		},
	}

	if err := installer.InstallContributions(ctx, candidateContribs, 2); err != nil {
		t.Fatalf("install candidate contributions (same ID): %v", err)
	}

	tool, exists := registry.Get(ctx, "ext-dup/tool")
	if !exists {
		t.Fatal("tool should exist after candidate install with same ID")
	}
	if tool.ModelName != "candidate" {
		t.Fatalf("expected candidate model name, got %s", tool.ModelName)
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-dup", 2, candidateContribs, nil); err != nil {
		t.Fatalf("discard candidate: %v", err)
	}

	_, exists = registry.Get(ctx, "ext-dup/tool")
	if exists {
		t.Fatal("candidate tool should be removed after discard (same ID as stable, both replaced)")
	}
}

func TestSameID_DiscardTwice_Idempotent(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()

	tool := capability.ToolDefinition{
		ID:          "ext-dup-idem/tool",
		ModelName:   "v1",
		ExtensionID: "ext-dup-idem",
		Source:      capability.ToolSourcePlugin,
		Enabled:     false,
	}
	_ = registry.Register(ctx, tool)

	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}

	contribs := []domain.ContributionDefinition{
		{
			ID:          "ext-dup-idem/tool",
			ExtensionID: "ext-dup-idem",
			Kind:        domain.ContributionKindTool,
			Definition:  map[string]any{"toolId": "ext-dup-idem/tool"},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-dup-idem", 2, contribs, nil); err != nil {
		t.Fatalf("first discard: %v", err)
	}
	if err := installer.DiscardCandidateContributions(ctx, "ext-dup-idem", 2, contribs, nil); err != nil {
		t.Fatalf("second discard should be idempotent: %v", err)
	}

	_, exists := registry.Get(ctx, "ext-dup-idem/tool")
	if exists {
		t.Fatal("tool should not exist after discard")
	}
}

func TestSameID_RestoreStableFromSnapshot(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-restore-snap"
	toolID := "ext-restore-snap/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)

	snap := mgr.captureStableSnapshot(ctx, domain.ExtensionID(extID))
	if snap == nil {
		t.Fatal("snapshot should not be nil")
	}
	if len(snap.Contributions) != 1 {
		t.Fatalf("expected 1 contribution in snapshot, got %d", len(snap.Contributions))
	}
	if snap.EnablementState != domain.EnablementEnabled {
		t.Fatalf("expected enablement state enabled, got %s", snap.EnablementState)
	}
	if snap.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", snap.Generation)
	}

	candidateTool := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    "candidate-tool",
		ExtensionID:  extID,
		Source:       capability.ToolSourcePlugin,
		Enabled:      false,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := container.ToolRegistry.Replace(ctx, candidateTool); err != nil {
		t.Fatalf("replace stable with candidate: %v", err)
	}

	tool, _ := container.ToolRegistry.Get(ctx, toolID)
	if tool.ModelName != "candidate-tool" {
		t.Fatalf("expected candidate model name, got %s", tool.ModelName)
	}

	if err := mgr.restoreStableFromSnapshot(ctx, "", domain.ExtensionID(extID), snap); err != nil {
		t.Fatalf("restore stable from snapshot: %v", err)
	}

	tool, exists := container.ToolRegistry.Get(ctx, toolID)
	if !exists {
		t.Fatal("stable tool should exist after restore")
	}
	if tool.ModelName != "stable-tool" {
		t.Fatalf("expected stable model name after restore, got %s", tool.ModelName)
	}
}

func TestSameID_PromoteHook_SameIDRecoverable(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}

	stableTool := capability.ToolDefinition{
		ID:           "ext-hook/stable",
		ModelName:    "stable",
		ExtensionID:  "ext-hook",
		Source:       capability.ToolSourcePlugin,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable: %v", err)
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:          "ext-hook/hook-1",
			ExtensionID: "ext-hook",
			Kind:        domain.ContributionKindHook,
			Definition: map[string]any{
				"contributionId": "ext-hook/hook-1",
				"event":          "on_message",
				"handler":        "handle",
			},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-hook", 2, candidateContribs, nil); err != nil {
		t.Fatalf("discard candidate hook: %v", err)
	}

	if _, exists := registry.Get(ctx, "ext-hook/stable"); !exists {
		t.Fatal("stable tool should still exist after hook candidate discard")
	}
}

func TestSameID_PromoteWorkflow_SameIDRecoverable(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	installer := &TypedContributionInstaller{
		container: &Container{ToolRegistry: registry},
	}

	stableTool := capability.ToolDefinition{
		ID:           "ext-wf/stable",
		ModelName:    "stable",
		ExtensionID:  "ext-wf",
		Source:       capability.ToolSourcePlugin,
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := registry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable: %v", err)
	}

	candidateContribs := []domain.ContributionDefinition{
		{
			ID:          "ext-wf/wf-1",
			ExtensionID: "ext-wf",
			Kind:        domain.ContributionKindWorkflow,
			Definition: map[string]any{
				"id": "ext-wf/wf-1",
			},
		},
	}

	if err := installer.DiscardCandidateContributions(ctx, "ext-wf", 2, candidateContribs, nil); err != nil {
		t.Fatalf("discard candidate workflow: %v", err)
	}

	if _, exists := registry.Get(ctx, "ext-wf/stable"); !exists {
		t.Fatal("stable tool should still exist after workflow candidate discard")
	}
}

func TestSameID_MarkRequiresRecovery(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-recovery"
	toolID := "ext-recovery/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)

	mgr.markRequiresRecovery(ctx, domain.ExtensionID(extID))

	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		t.Fatalf("get installation: %v", err)
	}
	if inst.EnablementState != domain.EnablementRequiresRecovery {
		t.Fatalf("expected enablement state requires_recovery, got %s", inst.EnablementState)
	}
}

func TestSameID_StableSnapshot_CapturesAllFields(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-snap-fields"
	toolID := "ext-snap-fields/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash-xyz")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	snap := mgr.captureStableSnapshot(ctx, domain.ExtensionID(extID))

	if snap == nil {
		t.Fatal("snapshot should not be nil")
	}
	if snap.GenerationID != stableGen.GenerationID {
		t.Fatalf("expected generation ID %s, got %s", stableGen.GenerationID, snap.GenerationID)
	}
	if snap.DefinitionHash != "stable-hash-xyz" {
		t.Fatalf("expected definition hash 'stable-hash-xyz', got %s", snap.DefinitionHash)
	}
	if snap.EnablementState != domain.EnablementEnabled {
		t.Fatalf("expected enablement state enabled, got %s", snap.EnablementState)
	}
	if len(snap.Contributions) != 1 {
		t.Fatalf("expected 1 contribution, got %d", len(snap.Contributions))
	}
	if string(snap.Contributions[0].ID) != toolID {
		t.Fatalf("expected contribution ID %s, got %s", toolID, snap.Contributions[0].ID)
	}
	if snap.CapturedAt.IsZero() {
		t.Fatal("captured at should not be zero")
	}
}

func TestSameID_PromoteTool_Success_ActivatesStable(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-promote-act"
	toolID := "ext-promote-act/tool"

	stableTool := capability.ToolDefinition{
		ID:           toolID,
		ModelName:    "stable",
		ExtensionID:  extID,
		Source:       capability.ToolSourcePlugin,
		Name:         "Stable",
		Enabled:      true,
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	if err := container.ToolRegistry.Register(ctx, stableTool); err != nil {
		t.Fatalf("register stable tool: %v", err)
	}

	stableContrib := domain.ContributionDefinition{
		ID:          domain.ContributionID(toolID),
		ModuleID:    "mod-1",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Stable"},
		Version:     "1.0.0",
		Definition: map[string]any{
			"toolId":      toolID,
			"modelName":   "stable",
			"inputSchema": map[string]any{"type": "object"},
		},
	}
	if err := container.ContributionRepository.PutContribution(ctx, stableContrib); err != nil {
		t.Fatalf("put stable contribution: %v", err)
	}

	inst := domain.ExtensionInstallation{
		InstallationID:   "inst-" + extID,
		ExtensionID:      domain.ExtensionID(extID),
		InstalledVersion: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		Generation:       1,
		EnablementState:  domain.EnablementEnabled,
		InstalledAt:      time.Now(),
	}
	if err := container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		t.Fatalf("put installation: %v", err)
	}

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}

	contribs := buildCandidateToolContrib(extID, toolID, "tool")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	candidateID := registerCandidateInManager(t, ctx, mgr, extID, candidateGen.GenerationID, int64(candidateGen.Generation), int64(stableGen.Generation), contribs)

	if err := mgr.PromoteCandidate(ctx, candidateID); err != nil {
		t.Fatalf("promote candidate: %v", err)
	}

	tool, exists := container.ToolRegistry.Get(ctx, toolID)
	if !exists {
		t.Fatal("tool should exist after successful promote")
	}
	if tool.ModelName != "candidate-tool" {
		t.Fatalf("expected candidate model name after promote, got %s", tool.ModelName)
	}

	activeGen := genMgr.Active(ctx, extID)
	if activeGen == nil || activeGen.GenerationID != candidateGen.GenerationID {
		t.Fatal("candidate generation should be active after promote")
	}

	oldGen, err := genMgr.Get(ctx, extID, stableGen.GenerationID)
	if err != nil {
		t.Fatalf("get stable generation: %v", err)
	}
	if oldGen.State != update.GenerationStateDraining {
		t.Fatalf("stable generation should be draining after promote, got %s", oldGen.State)
	}
}

func TestSameID_PromoteFailure_DoesNotDeleteCandidateRecord(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-fail-keep"
	toolID := "ext-fail-keep/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")

	contribs := buildCandidateToolContrib(extID, toolID, "tool")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	candidateID := registerCandidateInManager(t, ctx, mgr, extID, candidateGen.GenerationID, int64(candidateGen.Generation), int64(stableGen.Generation), contribs)

	err := mgr.PromoteCandidate(ctx, candidateID)
	if err == nil {
		t.Fatal("expected promote to fail")
	}

	record, ok := mgr.GetCandidate(candidateID)
	if !ok {
		t.Fatal("candidate record should still exist after failure")
	}
	if record.Status != CandidateStatusFailed {
		t.Fatalf("expected status failed, got %s", record.Status)
	}

	if tool, exists := container.ToolRegistry.Get(ctx, toolID); !exists || tool.ModelName != "stable-tool" {
		if !exists {
			t.Fatal("stable tool should exist after restore")
		}
		t.Fatalf("expected stable model name, got %s", tool.ModelName)
	}
}

func TestSameID_CAS_NoStableGeneration_SkipsCheck(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-no-stable-gen"
	toolID := "ext-no-stable-gen/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}

	contribs := buildCandidateToolContrib(extID, toolID, "tool")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)

	record := &CandidateRecord{
		CandidateID:              "cand-no-stable",
		ExtensionID:              domain.ExtensionID(extID),
		GenerationID:             candidateGen.GenerationID,
		CandidateGeneration:      int64(candidateGen.Generation),
		ExpectedStableGeneration: 0,
		Contribs:                 contribs,
		DefinitionHash:           "hash-1",
	}
	if err := mgr.RegisterCandidate(ctx, record); err != nil {
		t.Fatalf("register candidate: %v", err)
	}
	if err := mgr.HealthCandidate(ctx, "cand-no-stable"); err != nil {
		t.Fatalf("validate candidate: %v", err)
	}

	err := mgr.PromoteCandidate(ctx, "cand-no-stable")
	if err != nil {
		t.Fatalf("promote should succeed when ExpectedStableGeneration is 0 (no CAS check): %v", err)
	}

	activeGen := genMgr.Active(ctx, extID)
	if activeGen == nil || activeGen.GenerationID != candidateGen.GenerationID {
		t.Fatal("candidate generation should be active")
	}
}

func TestSameID_PromoteTool_StableGenDrainsAfterPromote(t *testing.T) {
	ctx := context.Background()
	db := setupSameIDTestDB(t)
	container := setupSameIDTestContainer(t, db)
	installer := NewTypedContributionInstaller(container)
	ns := NewCandidateNamespace()
	installer.SetCandidateNamespace(ns)
	genMgr := update.NewGenerationManager()

	extID := "ext-drain-test"
	toolID := "ext-drain-test/tool"

	seedStableTool(t, ctx, container, extID, toolID, "tool")

	stableGen := genMgr.Prepare(ctx, extID, "1.0.0", "stable-hash")
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, stableGen.GenerationID, update.GenerationStateActive); err != nil {
		t.Fatal(err)
	}

	candidateGen := genMgr.Prepare(ctx, extID, "2.0.0", "candidate-hash")
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateValidated); err != nil {
		t.Fatal(err)
	}
	if err := genMgr.Transition(ctx, extID, candidateGen.GenerationID, update.GenerationStateRuntimeReady); err != nil {
		t.Fatal(err)
	}

	contribs := buildCandidateToolContrib(extID, toolID, "tool")

	mgr := NewCandidateContributionManager(installer, genMgr, nil, nil, ns)
	candidateID := registerCandidateInManager(t, ctx, mgr, extID, candidateGen.GenerationID, int64(candidateGen.Generation), int64(stableGen.Generation), contribs)

	if err := mgr.PromoteCandidate(ctx, candidateID); err != nil {
		t.Fatalf("promote: %v", err)
	}

	oldGen, err := genMgr.Get(ctx, extID, stableGen.GenerationID)
	if err != nil {
		t.Fatalf("get old generation: %v", err)
	}
	if oldGen.State != update.GenerationStateDraining {
		t.Fatalf("old stable generation should be draining, got %s", oldGen.State)
	}

	newGen := genMgr.Active(ctx, extID)
	if newGen == nil || newGen.GenerationID != candidateGen.GenerationID {
		t.Fatal("candidate generation should be active")
	}
}
