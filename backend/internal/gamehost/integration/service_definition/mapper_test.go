package service_definition

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestBuildServiceDefinitionID(t *testing.T) {
	id := BuildServiceDefinitionID("com.example.game", "bridge-service")
	expected := "com.example.game/bridge-service"
	if id != expected {
		t.Errorf("expected %s, got %s", expected, id)
	}
}

func TestParseServiceDefinitionID(t *testing.T) {
	extID, modID, err := ParseServiceDefinitionID("com.example.game/bridge-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if extID != "com.example.game" {
		t.Errorf("expected com.example.game, got %s", extID)
	}
	if modID != "bridge-service" {
		t.Errorf("expected bridge-service, got %s", modID)
	}
}

func TestParseServiceDefinitionID_Invalid(t *testing.T) {
	_, _, err := ParseServiceDefinitionID("invalid")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}

	_, _, err = ParseServiceDefinitionID("/module")
	if err == nil {
		t.Fatal("expected error for empty extension id")
	}

	_, _, err = ParseServiceDefinitionID("ext/")
	if err == nil {
		t.Fatal("expected error for empty module id")
	}
}

func TestIsValidServiceRuntimeType(t *testing.T) {
	if !IsValidServiceRuntimeType("service") {
		t.Error("expected service to be valid")
	}
	if !IsValidServiceRuntimeType("javascript") {
		t.Error("expected javascript to be valid")
	}
	if !IsValidServiceRuntimeType("go") {
		t.Error("expected go to be valid")
	}
	if IsValidServiceRuntimeType("wasm") {
		t.Error("expected wasm to be invalid")
	}
	if IsValidServiceRuntimeType("") {
		t.Error("expected empty to be invalid")
	}
}

func TestDefinitionMapper_MapToDefinition(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "com.example.game",
		ModuleID:    "bridge-service",
		RuntimeType: "service",
		Name:        "Example Game Bridge",
		Description: "Bridge service for Example Game",
		EntryPoint:  "./bin/bridge",
		Env: map[string]string{
			"PORT": "8080",
		},
		Enabled: true,
	}

	def, err := mapper.MapToDefinition(view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ServiceID != "com.example.game/bridge-service" {
		t.Errorf("unexpected ServiceID: %s", def.ServiceID)
	}
	if def.ExtensionID != "com.example.game" {
		t.Errorf("unexpected ExtensionID: %s", def.ExtensionID)
	}
	if def.ModuleID != "bridge-service" {
		t.Errorf("unexpected ModuleID: %s", def.ModuleID)
	}
	if def.Name != "Example Game Bridge" {
		t.Errorf("unexpected Name: %s", def.Name)
	}
	if len(def.Executables) != 1 {
		t.Errorf("expected 1 executable, got %d", len(def.Executables))
	}
	if def.Executables[0].Entry != "./bin/bridge" {
		t.Errorf("unexpected EntryPoint: %s", def.Executables[0].Entry)
	}
	if def.Protocol != "amitia-trusted-service/1" {
		t.Errorf("unexpected Protocol: %s", def.Protocol)
	}
}

func TestDefinitionMapper_MapToDefinition_EmptyExtensionID(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "",
		ModuleID:    "bridge",
		RuntimeType: "service",
		EntryPoint:  "./bin/bridge",
	}

	_, err := mapper.MapToDefinition(view)
	if err == nil {
		t.Fatal("expected error for empty extension id")
	}
	if !IsServiceDefinitionError(err, ErrDefinitionMappingFailed) {
		t.Errorf("expected definition_mapping_failed, got %v", err)
	}
}

func TestDefinitionMapper_MapToDefinition_EmptyModuleID(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "ext",
		ModuleID:    "",
		RuntimeType: "service",
		EntryPoint:  "./bin/bridge",
	}

	_, err := mapper.MapToDefinition(view)
	if err == nil {
		t.Fatal("expected error for empty module id")
	}
}

func TestDefinitionMapper_MapToDefinition_UnsupportedType(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "ext",
		ModuleID:    "mod",
		RuntimeType: "wasm",
		EntryPoint:  "module.wasm",
	}

	_, err := mapper.MapToDefinition(view)
	if err == nil {
		t.Fatal("expected error for unsupported runtime type")
	}
	if !IsServiceDefinitionError(err, ErrUnsupportedServiceKind) {
		t.Errorf("expected unsupported_service_kind, got %v", err)
	}
}

func TestDefinitionMapper_MapToDefinition_EmptyEntryPoint(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "ext",
		ModuleID:    "mod",
		RuntimeType: "service",
		EntryPoint:  "",
	}

	_, err := mapper.MapToDefinition(view)
	if err == nil {
		t.Fatal("expected error for empty entry point")
	}
}

func TestDefinitionMapper_Stability(t *testing.T) {
	mapper := NewDefinitionMapper()

	view := ServiceRuntimeView{
		ExtensionID: "com.example.test",
		ModuleID:    "svc-1",
		RuntimeType: "service",
		EntryPoint:  "./bin/svc",
		Env:         map[string]string{"A": "1", "B": "2"},
	}

	first, err := mapper.MapToDefinition(view)
	if err != nil {
		t.Fatalf("first mapping error: %v", err)
	}

	second, err := mapper.MapToDefinition(view)
	if err != nil {
		t.Fatalf("second mapping error: %v", err)
	}

	if first.ServiceID != second.ServiceID {
		t.Errorf("ServiceID mismatch: %s vs %s", first.ServiceID, second.ServiceID)
	}
	if first.ManifestHash != second.ManifestHash {
		t.Errorf("ManifestHash mismatch: %s vs %s", first.ManifestHash, second.ManifestHash)
	}
}

func TestDefinitionMapper_CrossExtensionDifferentModuleIDs(t *testing.T) {
	mapper := NewDefinitionMapper()

	view1 := ServiceRuntimeView{
		ExtensionID: "ext-a",
		ModuleID:    "bridge",
		RuntimeType: "service",
		EntryPoint:  "./bin/a",
	}
	view2 := ServiceRuntimeView{
		ExtensionID: "ext-b",
		ModuleID:    "bridge",
		RuntimeType: "service",
		EntryPoint:  "./bin/b",
	}

	def1, err := mapper.MapToDefinition(view1)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	def2, err := mapper.MapToDefinition(view2)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if def1.ServiceID == def2.ServiceID {
		t.Errorf("expected different DefinitionIDs for different extensions, got same: %s", def1.ServiceID)
	}
}

func TestDefinitionIDFromServiceRuntime(t *testing.T) {
	def := &trusted_service.ServiceRuntimeDefinition{
		ExtensionID: "com.example.test",
		ModuleID:    "svc-1",
	}
	id := DefinitionIDFromServiceRuntime(def)
	if id != "com.example.test/svc-1" {
		t.Errorf("expected com.example.test/svc-1, got %s", id)
	}
}

func TestDefinitionIDFromServiceRuntime_Nil(t *testing.T) {
	id := DefinitionIDFromServiceRuntime(nil)
	if id != "" {
		t.Errorf("expected empty string, got %s", id)
	}
}

func TestCanonicalizeEnv(t *testing.T) {
	env := map[string]string{
		"Z": "last",
		"A": "first",
		"M": "middle",
	}
	result := CanonicalizeEnv(env)
	expected := []string{"A=first", "M=middle", "Z=last"}
	if len(result) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, result[i])
		}
	}
}

func TestCanonicalizeEnv_Nil(t *testing.T) {
	result := CanonicalizeEnv(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestServiceRuntimeView_IsValidProcessService(t *testing.T) {
	view := ServiceRuntimeView{
		RuntimeType: "service",
		EntryPoint:  "./bin/svc",
	}
	if !view.IsValidProcessService() {
		t.Error("expected valid process service")
	}

	view.EntryPoint = ""
	if view.IsValidProcessService() {
		t.Error("expected invalid for empty entry point")
	}

	view.EntryPoint = "./bin/svc"
	view.RuntimeType = "javascript"
	if view.IsValidProcessService() {
		t.Error("expected invalid for non-service type")
	}
}

func TestServiceRuntimeView_ToDefinitionID(t *testing.T) {
	view := ServiceRuntimeView{
		ExtensionID: "ext",
		ModuleID:    "mod",
	}
	id := view.ToDefinitionID()
	if id != "ext/mod" {
		t.Errorf("expected ext/mod, got %s", id)
	}
}
