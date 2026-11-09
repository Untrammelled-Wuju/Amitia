package domain

import "testing"

func TestResolveGeneralDomain(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindTool, Adapter: ContributionAdapterTool},
		{Kind: ContributionKindWorkflow, Adapter: ContributionAdapterWorkflow},
		{Kind: ContributionKindProvider, Adapter: ContributionAdapterProvider},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGeneral {
		t.Errorf("expected general, got %s", domain)
	}
}

func TestResolveGameDomain(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGame {
		t.Errorf("expected game, got %s", domain)
	}
}

func TestResolveDesktopPetDomain(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet, got %s", domain)
	}
}

func TestMultipleGameContributionsRemainGame(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGame {
		t.Errorf("expected game, got %s", domain)
	}
}

func TestMultipleDesktopPetContributionsRemainDesktopPet(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet, got %s", domain)
	}
}

func TestGameDesktopPetDomainConflict(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
	}
	_, err := ResolveExtensionDomain(contribs)
	if err == nil {
		t.Error("expected error for game + desktop_pet conflict")
	}
}

func TestGeneralContributionsDoNotChangeGameDomain(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
		{Kind: ContributionKindTool, Adapter: ContributionAdapterTool},
		{Kind: ContributionKindProvider, Adapter: ContributionAdapterProvider},
		{Kind: ContributionKindUIPage, Adapter: ContributionAdapterUI},
		{Kind: ContributionKindMCPServer, Adapter: ContributionAdapterMCPServer},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGame {
		t.Errorf("expected game, got %s", domain)
	}
}

func TestGeneralContributionsDoNotChangeDesktopPetDomain(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
		{Kind: ContributionKindTool, Adapter: ContributionAdapterTool},
		{Kind: ContributionKindWorkflow, Adapter: ContributionAdapterWorkflow},
		{Kind: ContributionKindHook, Adapter: ContributionAdapterHook},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet, got %s", domain)
	}
}

func TestEmptyContributionsDefaultGeneral(t *testing.T) {
	domain, err := ResolveExtensionDomain(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGeneral {
		t.Errorf("expected general for empty contributions, got %s", domain)
	}
}

func TestResolveDomainFromKinds(t *testing.T) {
	domain, err := ResolveDomainFromKinds([]ContributionKind{ContributionKindTool, ContributionKindProvider})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGeneral {
		t.Errorf("expected general, got %s", domain)
	}

	domain, err = ResolveDomainFromKinds([]ContributionKind{ContributionKindGamePlugin, ContributionKindTool})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGame {
		t.Errorf("expected game, got %s", domain)
	}
}

func TestProviderRemainsGeneral(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindProvider},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGeneral {
		t.Errorf("expected general for provider, got %s", domain)
	}
}

func TestSkillMCPWorkflowRemainGeneral(t *testing.T) {
	skillContribs := []ContributionDefinition{{Kind: ContributionKindAgentSkill}}
	if domain, err := ResolveExtensionDomain(skillContribs); err != nil || domain != ExtensionDomainGeneral {
		t.Errorf("expected general for skill, got %s, err=%v", domain, err)
	}

	mcpContribs := []ContributionDefinition{{Kind: ContributionKindMCPServer}}
	if domain, err := ResolveExtensionDomain(mcpContribs); err != nil || domain != ExtensionDomainGeneral {
		t.Errorf("expected general for mcp_server, got %s, err=%v", domain, err)
	}

	workflowContribs := []ContributionDefinition{{Kind: ContributionKindWorkflow}}
	if domain, err := ResolveExtensionDomain(workflowContribs); err != nil || domain != ExtensionDomainGeneral {
		t.Errorf("expected general for workflow, got %s, err=%v", domain, err)
	}
}

func TestIsExclusiveContributionKind(t *testing.T) {
	if !IsExclusiveContributionKind(ContributionKindGamePlugin) {
		t.Error("expected game_plugin to be exclusive")
	}
	if !IsExclusiveContributionKind(ContributionKindDesktopPetPlugin) {
		t.Error("expected desktop_pet_plugin to be exclusive")
	}
	if IsExclusiveContributionKind(ContributionKindTool) {
		t.Error("expected tool to not be exclusive")
	}
	if IsExclusiveContributionKind(ContributionKindProvider) {
		t.Error("expected provider to not be exclusive")
	}
	if IsExclusiveContributionKind(ContributionKindWorkflow) {
		t.Error("expected workflow to not be exclusive")
	}
}

func TestClassifyExtension(t *testing.T) {
	classification, err := ClassifyExtension([]ContributionDefinition{{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if classification.Domain != ExtensionDomainGame {
		t.Errorf("expected game domain, got %s", classification.Domain)
	}
	if classification.ManagementTarget != ManagementTargetGameCenter {
		t.Errorf("expected game_center target, got %s", classification.ManagementTarget)
	}
	if len(classification.ExclusiveContributions) != 1 || classification.ExclusiveContributions[0] != ContributionKindGamePlugin {
		t.Errorf("expected exclusive contributions [game_plugin], got %v", classification.ExclusiveContributions)
	}
}

func TestClassifyExtensionConflict(t *testing.T) {
	classification, err := ClassifyExtension([]ContributionDefinition{
		{Kind: ContributionKindGamePlugin, Adapter: ContributionAdapterGame},
		{Kind: ContributionKindDesktopPetPlugin, Adapter: ContributionAdapterDesktopPet},
	})
	if err == nil {
		t.Error("expected error for conflict")
	}
	if classification.Domain != ExtensionDomainGeneral {
		t.Errorf("expected general domain for conflict, got %s", classification.Domain)
	}
}

func TestOldExtensionDefaultsToGeneral(t *testing.T) {
	contribs := []ContributionDefinition{
		{Kind: ContributionKindTool},
		{Kind: ContributionKindProvider},
	}
	domain, err := ResolveExtensionDomain(contribs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain != ExtensionDomainGeneral {
		t.Errorf("expected general for old extension, got %s", domain)
	}
	target, err := ManagementTargetForDomain(domain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != ManagementTargetExtensionCenter {
		t.Errorf("expected extension_center, got %s", target)
	}
}
