package domain

import "testing"

func TestManagementTargetValues(t *testing.T) {
	if string(ManagementTargetExtensionCenter) != "extension_center" {
		t.Errorf("expected extension_center, got %s", ManagementTargetExtensionCenter)
	}
	if string(ManagementTargetGameCenter) != "game_center" {
		t.Errorf("expected game_center, got %s", ManagementTargetGameCenter)
	}
	if string(ManagementTargetDesktopPetCenter) != "desktop_pet_center" {
		t.Errorf("expected desktop_pet_center, got %s", ManagementTargetDesktopPetCenter)
	}
}

func TestIsValidManagementTarget(t *testing.T) {
	valid := []ManagementTarget{
		ManagementTargetExtensionCenter,
		ManagementTargetGameCenter,
		ManagementTargetDesktopPetCenter,
	}
	for _, target := range valid {
		if !IsValidManagementTarget(target) {
			t.Errorf("expected %s to be valid", target)
		}
	}
	invalid := []ManagementTarget{"", "plugin_center", "normal_center", "pet_center", "game", "extension", "provider_center", "channel_center"}
	for _, target := range invalid {
		if IsValidManagementTarget(target) {
			t.Errorf("expected %s to be invalid", target)
		}
	}
}

func TestRejectUnknownManagementTarget(t *testing.T) {
	if IsValidManagementTarget("unknown") {
		t.Error("expected unknown to be invalid")
	}
	if IsValidManagementTarget("plugin_center") {
		t.Error("expected plugin_center to be invalid")
	}
}

func TestManagementTargetForGeneral(t *testing.T) {
	target, err := ManagementTargetForDomain(ExtensionDomainGeneral)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != ManagementTargetExtensionCenter {
		t.Errorf("expected extension_center, got %s", target)
	}
}

func TestManagementTargetForGame(t *testing.T) {
	target, err := ManagementTargetForDomain(ExtensionDomainGame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != ManagementTargetGameCenter {
		t.Errorf("expected game_center, got %s", target)
	}
}

func TestManagementTargetForDesktopPet(t *testing.T) {
	target, err := ManagementTargetForDomain(ExtensionDomainDesktopPet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != ManagementTargetDesktopPetCenter {
		t.Errorf("expected desktop_pet_center, got %s", target)
	}
}

func TestManagementTargetForDomainUnknown(t *testing.T) {
	_, err := ManagementTargetForDomain("unknown")
	if err == nil {
		t.Error("expected error for unknown domain")
	}
	_, err = ManagementTargetForDomain("")
	if err == nil {
		t.Error("expected error for empty domain")
	}
}

func TestManagementTargetHelpers(t *testing.T) {
	if !ManagementTargetExtensionCenter.IsExtensionCenter() {
		t.Error("expected extension_center.IsExtensionCenter() to be true")
	}
	if ManagementTargetExtensionCenter.IsGameCenter() {
		t.Error("expected extension_center.IsGameCenter() to be false")
	}
	if ManagementTargetExtensionCenter.IsDesktopPetCenter() {
		t.Error("expected extension_center.IsDesktopPetCenter() to be false")
	}
	if !ManagementTargetGameCenter.IsGameCenter() {
		t.Error("expected game_center.IsGameCenter() to be true")
	}
	if ManagementTargetGameCenter.IsExtensionCenter() {
		t.Error("expected game_center.IsExtensionCenter() to be false")
	}
	if !ManagementTargetDesktopPetCenter.IsDesktopPetCenter() {
		t.Error("expected desktop_pet_center.IsDesktopPetCenter() to be true")
	}
	if ManagementTargetDesktopPetCenter.IsGameCenter() {
		t.Error("expected desktop_pet_center.IsGameCenter() to be false")
	}
}
