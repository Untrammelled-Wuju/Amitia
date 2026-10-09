package extension

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

func TestAcquisitionDiscoverySkillUsesPersistentService(t *testing.T) {
	ctx := context.Background()
	db := agentSkillTestDB(t)
	validator, _ := NewSchemaValidator()
	repository := NewRepository(db)
	service := NewAgentSkillService(repository, validator)
	adapter := NewAgentSkillAcquisitionAdapter(service)
	registry := capability.NewProviderRegistry()
	sources := acquisition.NewSourceRegistry()
	sources.Register(acquisition.NewExplicitSource())
	installers, err := acquisition.NewInstallerRegistry(&acquisition.InstallerRegistryOpts{
		PackageInstallPort: acquisition.NewPackagePortBridgeFromManager(nil),
		MCPInstallPort:     acquisition.NewMCPPortBridge(nil),
		SkillInstallPort:   adapter,
		EnableExistingPort: acquisition.NewEnableExistingPortBridge(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	acquirer, err := acquisition.NewAcquisitionService(acquisition.AcquisitionDependencies{
		CapabilityService: capability.NewCapabilityService(registry), ProviderRegistry: registry,
		SourceRegistry: sources, InstallerRegistry: installers, PolicyEngine: acquisition.NewPolicyEngine(), DeploymentPlanner: acquisition.NewDeploymentPlanner(),
	})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "review")
	os.MkdirAll(filepath.Join(directory, "references"), 0700)
	os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte("---\nname: review\ndescription: Review code. Use when a review is requested.\n---\nRead references/checklist.md."), 0600)
	os.WriteFile(filepath.Join(directory, "references", "checklist.md"), []byte("Check correctness."), 0600)
	bridge := acquisition.NewAgentCapabilityBridge(acquirer)
	found, err := bridge.FindCapabilities(ctx, acquisition.FindCapabilitiesInput{CapabilityID: "skill.review", SourceURI: directory, PreferredKind: "skill"}, "user-1")
	if err != nil || found.TotalFound != 1 {
		t.Fatalf("found=%+v err=%v", found, err)
	}
	output, err := bridge.AcquireCapability(ctx, acquisition.AcquireInput{CapabilityID: "skill.review", CandidateID: found.Candidates[0].ID, UserConfirmed: true}, "user-1", nil)
	if err != nil || !output.Success || !output.Installed || !output.Enabled || output.State != acquisition.StateReady {
		t.Fatalf("output=%+v err=%v", output, err)
	}
	listed, err := service.List(ctx, ExecutionScope{SpaceID: "user-1"}, AgentSkillFilter{})
	if err != nil || len(listed.Items) != 1 {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	id := listed.Items[0].ExtensionID
	_, _, files, err := repository.LoadAgentSkill(ctx, id)
	if err != nil || string(files["references/checklist.md"]) != "Check correctness." {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	restored := NewAgentSkillService(repository, validator)
	if err := restored.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := restored.Get(ctx, ExecutionScope{SpaceID: "user-1"}, id)
	if err != nil || !loaded.Enabled {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	if loaded.Scope != AgentSkillScopeGlobal {
		t.Fatalf("installed skill lost its configured global scope: %s", loaded.Scope)
	}
	candidates, err := adapter.Search(ctx, acquisition.AcquisitionRequest{CapabilityID: "skill.review", SpaceID: "user-1"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	if err := service.Disable(ctx, ExecutionScope{SpaceID: "user-1"}, id); err != nil {
		t.Fatal(err)
	}
	candidates, err = adapter.Search(ctx, acquisition.AcquisitionRequest{CapabilityID: "skill.review", SpaceID: "user-1"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("disabled skill not discoverable: %+v %v", candidates, err)
	}
}
