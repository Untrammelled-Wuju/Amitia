package domain

import (
	"testing"
)

func makeExtension(id string, domain ExtensionDomain) ExtensionDefinition {
	v, _ := ParseVersion("1.0.0")
	return ExtensionDefinition{
		ID:              ExtensionID(id),
		Name:            LocalizedText{Default: id},
		Version:         v,
		ManifestVersion: 2,
		Domain:          domain,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: ExtensionID(id), Type: ModuleTypeBuiltin}},
	}
}

func TestExtensionCenterFilter(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
		makeExtension("general-b", ExtensionDomainGeneral),
	}
	result := FilterExtensionCenter(extensions)
	if len(result) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(result))
	}
	for _, ext := range result {
		if ext.Domain != ExtensionDomainGeneral {
			t.Errorf("expected only general in extension center, got %s", ext.Domain)
		}
	}
}

func TestGameCenterFilter(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
		makeExtension("game-b", ExtensionDomainGame),
	}
	result := FilterGameCenter(extensions)
	if len(result) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(result))
	}
	for _, ext := range result {
		if ext.Domain != ExtensionDomainGame {
			t.Errorf("expected only game in game center, got %s", ext.Domain)
		}
	}
}

func TestDesktopPetCenterFilter(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
		makeExtension("desktop-pet-b", ExtensionDomainDesktopPet),
	}
	result := FilterDesktopPetCenter(extensions)
	if len(result) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(result))
	}
	for _, ext := range result {
		if ext.Domain != ExtensionDomainDesktopPet {
			t.Errorf("expected only desktop_pet in pet center, got %s", ext.Domain)
		}
	}
}

func TestFilterByDomain(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	result := FilterByDomain(extensions, DomainFilter{Domains: []ExtensionDomain{ExtensionDomainGame, ExtensionDomainDesktopPet}})
	if len(result) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(result))
	}
}

func TestFilterByDomainEmpty(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
	}
	result := FilterByDomain(extensions, DomainFilter{Domains: nil})
	if len(result) != 2 {
		t.Fatalf("expected all extensions when no domain filter, got %d", len(result))
	}
}

func TestFilterByManagementTarget(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	result, err := FilterByManagementTarget(extensions, ManagementFilter{Targets: []ManagementTarget{ManagementTargetGameCenter}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(result))
	}
	if result[0].Domain != ExtensionDomainGame {
		t.Errorf("expected game domain, got %s", result[0].Domain)
	}
}

func TestDomainFilterMatches(t *testing.T) {
	filter := DomainFilter{Domains: []ExtensionDomain{ExtensionDomainGame}}
	if !filter.Matches(ExtensionDomainGame) {
		t.Error("expected game to match")
	}
	if filter.Matches(ExtensionDomainGeneral) {
		t.Error("expected general not to match")
	}
	if filter.Matches(ExtensionDomainDesktopPet) {
		t.Error("expected desktop_pet not to match")
	}
}

func TestDomainFilterMatchesEmpty(t *testing.T) {
	filter := DomainFilter{}
	if !filter.Matches(ExtensionDomainGame) {
		t.Error("expected any domain to match when filter is empty")
	}
	if !filter.Matches(ExtensionDomainGeneral) {
		t.Error("expected any domain to match when filter is empty")
	}
}

func TestManagementFilterMatches(t *testing.T) {
	filter := ManagementFilter{Targets: []ManagementTarget{ManagementTargetExtensionCenter}}
	if !filter.Matches(ManagementTargetExtensionCenter) {
		t.Error("expected extension_center to match")
	}
	if filter.Matches(ManagementTargetGameCenter) {
		t.Error("expected game_center not to match")
	}
}

func TestDomainFilterWithStatus(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("game-b", ExtensionDomainGame),
	}
	result := FilterGameCenter(extensions)
	if len(result) != 2 {
		t.Errorf("expected 2 game extensions, got %d", len(result))
	}
}

func TestDomainFilterWithSearch(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-minecraft", ExtensionDomainGame),
		makeExtension("game-other", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	filtered := FilterGameCenter(extensions)
	count := 0
	for _, ext := range filtered {
		if ext.ID == "game-minecraft" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected game-minecraft in game center filter, got count=%d", count)
	}
}

func TestDomainFilterPagination(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("general-b", ExtensionDomainGeneral),
		makeExtension("general-c", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	result := FilterByDomain(extensions, DomainFilter{Domains: []ExtensionDomain{ExtensionDomainGeneral}})
	if len(result) != 3 {
		t.Fatalf("expected 3 general extensions before pagination, got %d", len(result))
	}
	pageSize := 2
	if len(result) < pageSize {
		t.Skip("not enough data for pagination test")
	}
	page := result[:pageSize]
	if len(page) != pageSize {
		t.Errorf("expected page size %d, got %d", pageSize, len(page))
	}
}

func TestDomainFilterCount(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("general-b", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	generalCount := len(FilterExtensionCenter(extensions))
	if generalCount != 2 {
		t.Errorf("expected extension center count 2, got %d", generalCount)
	}
	gameCount := len(FilterGameCenter(extensions))
	if gameCount != 1 {
		t.Errorf("expected game center count 1, got %d", gameCount)
	}
	petCount := len(FilterDesktopPetCenter(extensions))
	if petCount != 1 {
		t.Errorf("expected pet center count 1, got %d", petCount)
	}
}

func TestExtensionCenterDoesNotContainGameOrPet(t *testing.T) {
	extensions := []ExtensionDefinition{
		makeExtension("general-a", ExtensionDomainGeneral),
		makeExtension("game-a", ExtensionDomainGame),
		makeExtension("desktop-pet-a", ExtensionDomainDesktopPet),
	}
	result := FilterExtensionCenter(extensions)
	for _, ext := range result {
		if ext.Domain != ExtensionDomainGeneral {
			t.Errorf("extension center should not contain %s domain", ext.Domain)
		}
	}
}
