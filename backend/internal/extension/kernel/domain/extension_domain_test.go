package domain

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestExtensionDomainValues(t *testing.T) {
	if string(ExtensionDomainGeneral) != "general" {
		t.Errorf("expected general, got %s", ExtensionDomainGeneral)
	}
	if string(ExtensionDomainGame) != "game" {
		t.Errorf("expected game, got %s", ExtensionDomainGame)
	}
	if string(ExtensionDomainDesktopPet) != "desktop_pet" {
		t.Errorf("expected desktop_pet, got %s", ExtensionDomainDesktopPet)
	}
}

func TestDefaultExtensionDomain(t *testing.T) {
	if d := DefaultExtensionDomain(); d != ExtensionDomainGeneral {
		t.Errorf("expected general, got %s", d)
	}
}

func TestIsValidExtensionDomain(t *testing.T) {
	valid := []ExtensionDomain{ExtensionDomainGeneral, ExtensionDomainGame, ExtensionDomainDesktopPet}
	for _, d := range valid {
		if !IsValidExtensionDomain(d) {
			t.Errorf("expected %s to be valid", d)
		}
	}
	invalid := []ExtensionDomain{"", "unknown", "minecraft", "pet", "channel", "provider", "foobar"}
	for _, d := range invalid {
		if IsValidExtensionDomain(d) {
			t.Errorf("expected %s to be invalid", d)
		}
	}
}

func TestNormalizeExtensionDomain(t *testing.T) {
	if d := NormalizeExtensionDomain(""); d != ExtensionDomainGeneral {
		t.Errorf("expected empty to normalize to general, got %s", d)
	}
	if d := NormalizeExtensionDomain(ExtensionDomainGeneral); d != ExtensionDomainGeneral {
		t.Errorf("expected general to stay general, got %s", d)
	}
	if d := NormalizeExtensionDomain(ExtensionDomainGame); d != ExtensionDomainGame {
		t.Errorf("expected game to stay game, got %s", d)
	}
	if d := NormalizeExtensionDomain(ExtensionDomainDesktopPet); d != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet to stay desktop_pet, got %s", d)
	}
	if d := NormalizeExtensionDomain("foobar"); d != "foobar" {
		t.Errorf("expected foobar to stay foobar, got %s", d)
	}
}

func TestExtensionDomainHelpers(t *testing.T) {
	if !ExtensionDomainGeneral.IsGeneral() {
		t.Errorf("expected general.IsGeneral() to be true")
	}
	if ExtensionDomainGeneral.IsGame() {
		t.Errorf("expected general.IsGame() to be false")
	}
	if ExtensionDomainGeneral.IsDesktopPet() {
		t.Errorf("expected general.IsDesktopPet() to be false")
	}
	if !ExtensionDomainGame.IsGame() {
		t.Errorf("expected game.IsGame() to be true")
	}
	if ExtensionDomainGame.IsGeneral() {
		t.Errorf("expected game.IsGeneral() to be false")
	}
	if !ExtensionDomainDesktopPet.IsDesktopPet() {
		t.Errorf("expected desktop_pet.IsDesktopPet() to be true")
	}
	if ExtensionDomainDesktopPet.IsGeneral() {
		t.Errorf("expected desktop_pet.IsGeneral() to be false")
	}
}

func TestExtensionDefinitionDefaultDomainGeneral(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Test"},
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/test", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if def.Domain != ExtensionDomainGeneral {
		t.Errorf("expected default domain to be general, got %s", def.Domain)
	}
}

func TestExtensionDefinitionValidGameDomain(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/game-test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Game Test"},
		Domain:          ExtensionDomainGame,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/game-test", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if def.Domain != ExtensionDomainGame {
		t.Errorf("expected game domain, got %s", def.Domain)
	}
}

func TestExtensionDefinitionValidDesktopPetDomain(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/pet-test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Pet Test"},
		Domain:          ExtensionDomainDesktopPet,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/pet-test", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if def.Domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet domain, got %s", def.Domain)
	}
}

func TestExtensionDefinitionInvalidDomain(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/invalid",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Invalid"},
		Domain:          "minecraft",
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/invalid", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err == nil {
		t.Errorf("expected error for invalid domain 'minecraft'")
	}
}

func TestExtensionDefinitionPreservesGameDomainOnCopy(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/game-test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Game"},
		Domain:          ExtensionDomainGame,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/game-test", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	copied := def
	if copied.Domain != ExtensionDomainGame {
		t.Errorf("expected game domain preserved on copy, got %s", copied.Domain)
	}
}

func TestExtensionDefinitionPreservesDesktopPetDomainOnCopy(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/pet-test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Pet"},
		Domain:          ExtensionDomainDesktopPet,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/pet-test", Type: ModuleTypeBuiltin}},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	copied := def
	if copied.Domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet domain preserved on copy, got %s", copied.Domain)
	}
}

func TestExtensionDomainJSONSerialization(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Test"},
		Domain:          ExtensionDomainGame,
		Modules:         []ModuleDefinition{{ID: "main", ExtensionID: "com.example/test", Type: ModuleTypeBuiltin}},
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(data, []byte(`"domain":"game"`)) {
		t.Errorf("expected domain:game in JSON, got: %s", string(data))
	}
}

func TestExtensionDomainJSONDeserialization(t *testing.T) {
	data := []byte(`{
		"id": "com.example/test",
		"name": {"default": "Test"},
		"version": {"major": 1, "minor": 0, "patch": 0},
		"manifestVersion": 2,
		"domain": "desktop_pet",
		"modules": [{"id": "main", "type": "builtin"}]
	}`)
	var def ExtensionDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if def.Domain != ExtensionDomainDesktopPet {
		t.Errorf("expected desktop_pet, got %s", def.Domain)
	}
}

func TestOnlyThreeDomainsAllowed(t *testing.T) {
	domains := []ExtensionDomain{ExtensionDomainGeneral, ExtensionDomainGame, ExtensionDomainDesktopPet}
	for _, d := range domains {
		if !IsValidExtensionDomain(d) {
			t.Errorf("expected %s to be valid", d)
		}
	}
	extra := []ExtensionDomain{"channel", "provider", "skill", "mcp", "workflow", "agent"}
	for _, d := range extra {
		if IsValidExtensionDomain(d) {
			t.Errorf("expected %s to be invalid (not part of domain model)", d)
		}
	}
}
