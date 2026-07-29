package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func setupRecoverTestContainer(t *testing.T, ctx context.Context, extID string) (*Container, *runtime_supervisor.FakeRuntime) {
	t.Helper()
	tempDir := t.TempDir()
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}

	fakeRT := runtime_supervisor.NewFakeRuntime()
	defaultSupervisor, ok := container.RuntimeSupervisor.(*runtime_supervisor.DefaultSupervisor)
	if !ok {
		t.Fatalf("expected *DefaultSupervisor, got %T", container.RuntimeSupervisor)
	}
	if err := defaultSupervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeGo, fakeRT)); err != nil {
		t.Fatalf("RegisterFactory must succeed: %v", err)
	}

	version := domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0}
	now := time.Now().UTC()
	inst := domain.ExtensionInstallation{
		InstallationID:    "inst-" + extID,
		ExtensionID:       domain.ExtensionID(extID),
		InstalledVersion:  version,
		PackageID:         "pkg-" + extID,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        1,
	}
	if err := container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		t.Fatalf("PutInstallation must succeed: %v", err)
	}

	mod := domain.ModuleDefinition{
		ID:          "main",
		ExtensionID: domain.ExtensionID(extID),
		Name:        domain.LocalizedText{Default: "main"},
		Type:        domain.ModuleTypeService,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeGo,
		},
	}
	if err := container.ModuleRepository.PutModule(ctx, mod); err != nil {
		t.Fatalf("PutModule must succeed: %v", err)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extID}
	_ = container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementEnabled)
	_ = container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStarted)

	return container, fakeRT
}

func TestRecover_TypedContributionsRestored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/recover-typed"
	container, fakeRT := setupRecoverTestContainer(t, ctx, extID)
	defer container.Close()

	toolDef := map[string]any{
		"toolId":      "recover-test-tool",
		"modelName":   "test-model",
		"inputSchema": json.RawMessage(`{"type":"object","properties":{}}`),
	}
	toolContrib := domain.ContributionDefinition{
		ID:          "recover-test-tool",
		ModuleID:    "main",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Test Tool"},
		Description: domain.LocalizedText{Default: "A test tool for recovery"},
		Version:     "1.0.0",
		Definition:  toolDef,
	}
	if err := container.ContributionRepository.PutContribution(ctx, toolContrib); err != nil {
		t.Fatalf("PutContribution tool must succeed: %v", err)
	}

	skillDef := agent_skill.AgentSkillDefinition{
		ID:          "recover-test-skill",
		ExtensionID: extID,
		ModuleID:    "main",
		Name:        "Test Skill",
		Description: "A test skill for recovery",
	}
	skillDefData, _ := json.Marshal(skillDef)
	skillContrib := domain.ContributionDefinition{
		ID:          "recover-test-skill",
		ModuleID:    "main",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindAgentSkill,
		Name:        domain.LocalizedText{Default: "Test Skill"},
		Description: domain.LocalizedText{Default: "A test skill for recovery"},
		Version:     "1.0.0",
		Definition:  map[string]any{},
	}
	_ = json.Unmarshal(skillDefData, &skillContrib.Definition)
	if err := container.ContributionRepository.PutContribution(ctx, skillContrib); err != nil {
		t.Fatalf("PutContribution agent_skill must succeed: %v", err)
	}

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("Recover must succeed: %v", err)
	}

	if container.ToolRegistry != nil {
		tool, ok := container.ToolRegistry.Get(ctx, "recover-test-tool")
		if !ok {
			t.Errorf("expected tool recover-test-tool to be recovered in registry")
		}
		if tool.ID != "recover-test-tool" {
			t.Errorf("expected tool ID recover-test-tool, got %s", tool.ID)
		}
	}

	if container.AgentSkillCatalog != nil {
		skill, ok := container.AgentSkillCatalog.Get(extID)
		if !ok {
			t.Errorf("expected agent skill to be recovered in catalog")
		}
		if skill.ID != "recover-test-skill" {
			t.Errorf("expected skill ID recover-test-skill, got %s", skill.ID)
		}
	}

	if fakeRT.StartCount() == 0 {
		t.Errorf("expected FakeRuntime.Start to be called during recovery")
	}
}

func TestRecover_MarksRequiresRecoveryOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/recover-fail"
	container, fakeRT := setupRecoverTestContainer(t, ctx, extID)
	defer container.Close()

	fakeRT.SetStartErr(runtime_supervisor.ErrFakeStart)

	toolContrib := domain.ContributionDefinition{
		ID:          "recover-fail-tool",
		ModuleID:    "main",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Fail Tool"},
		Description: domain.LocalizedText{Default: "A tool that should fail recovery"},
		Version:     "1.0.0",
		Definition: map[string]any{
			"toolId":      "recover-fail-tool",
			"modelName":   "test-model",
			"inputSchema": json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
	if err := container.ContributionRepository.PutContribution(ctx, toolContrib); err != nil {
		t.Fatalf("PutContribution must succeed: %v", err)
	}

	err := container.Recover(ctx)
	if err == nil {
		t.Fatalf("Recover must return error when runtime fails")
	}

	inst, getErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if getErr != nil {
		t.Fatalf("GetInstallation must succeed: %v", getErr)
	}
	if inst.EnablementState != domain.EnablementRequiresRecovery {
		t.Errorf("expected EnablementState=requires_recovery, got %s", inst.EnablementState)
	}
}

func TestRecover_IdempotentNoDuplication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extID := "com.amitia.test/recover-idempotent"
	container, _ := setupRecoverTestContainer(t, ctx, extID)
	defer container.Close()

	toolContrib := domain.ContributionDefinition{
		ID:          "recover-idempotent-tool",
		ModuleID:    "main",
		ExtensionID: domain.ExtensionID(extID),
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "Idempotent Tool"},
		Description: domain.LocalizedText{Default: "A tool for testing idempotent recovery"},
		Version:     "1.0.0",
		Definition: map[string]any{
			"toolId":      "recover-idempotent-tool",
			"modelName":   "test-model",
			"inputSchema": json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}
	if err := container.ContributionRepository.PutContribution(ctx, toolContrib); err != nil {
		t.Fatalf("PutContribution must succeed: %v", err)
	}

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("First Recover must succeed: %v", err)
	}

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("Second Recover must succeed: %v", err)
	}

	if container.ToolRegistry != nil {
		tool, ok := container.ToolRegistry.Get(ctx, "recover-idempotent-tool")
		if !ok {
			t.Errorf("expected tool to still be registered after double recovery")
		}
		if tool.ID != "recover-idempotent-tool" {
			t.Errorf("expected tool ID recover-idempotent-tool, got %s", tool.ID)
		}
	}
}

func TestMarkRequiresRecovery_UpdatesState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	extID := "com.amitia.test/mark-recovery"
	container, _ := setupRecoverTestContainer(t, ctx, extID)
	defer container.Close()

	inst, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		t.Fatalf("GetInstallation must succeed: %v", err)
	}
	if inst.EnablementState != domain.EnablementEnabled {
		t.Fatalf("expected initial state enabled, got %s", inst.EnablementState)
	}

	container.markRequiresRecovery(ctx, inst)

	updated, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extID))
	if err != nil {
		t.Fatalf("GetInstallation after mark must succeed: %v", err)
	}
	if updated.EnablementState != domain.EnablementRequiresRecovery {
		t.Errorf("expected EnablementState=requires_recovery after mark, got %s", updated.EnablementState)
	}
}
