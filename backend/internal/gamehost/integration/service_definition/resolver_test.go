package service_definition

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestNewDefinitionResolver_ValidateInput(t *testing.T) {
	_, err := NewDefinitionResolver(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestDefinitionResolver_HasDefinition(t *testing.T) {
	provider := newMockProvider()
	provider.registered["com.example.ext/bridge"] = &trusted_service.ServiceRuntimeDefinition{
		ServiceID: "com.example.ext/bridge",
	}

	resolver, err := NewDefinitionResolver(provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resolver.HasDefinition("com.example.ext/bridge") {
		t.Error("expected bridge service to exist")
	}

	if resolver.HasDefinition("com.example.ext/unknown") {
		t.Error("expected unknown service to not exist")
	}
}

func TestDefinitionResolver_Resolve_Found(t *testing.T) {
	provider := newMockProvider()
	provider.registered["com.example.ext/bridge"] = &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "com.example.ext/bridge",
		ExtensionID: "com.example.ext",
		ModuleID:    "bridge",
	}

	resolver, err := NewDefinitionResolver(provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	def, err := resolver.Resolve("com.example.ext/bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ServiceID != "com.example.ext/bridge" {
		t.Errorf("unexpected ServiceID: %s", def.ServiceID)
	}
}

func TestDefinitionResolver_Resolve_NotFound(t *testing.T) {
	provider := newMockProvider()

	resolver, _ := NewDefinitionResolver(provider)
	_, err := resolver.Resolve("com.example.ext/unknown")
	if err == nil {
		t.Fatal("expected error for missing definition")
	}
	if !IsServiceDefinitionError(err, ErrServiceDefinitionNotFound) {
		t.Errorf("expected service_definition_not_found, got %v", err)
	}
}

func TestDefinitionResolver_ResolveForService_Found(t *testing.T) {
	provider := newMockProvider()
	provider.getResult = &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "svc-123",
		ExtensionID: "com.example.ext",
		ModuleID:    "bridge",
	}

	resolver, _ := NewDefinitionResolver(provider)

	def, suggestions, err := resolver.ResolveForService("svc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def == nil {
		t.Fatal("expected definition, got nil")
	}
	if def.ServiceID != "svc-123" {
		t.Errorf("unexpected ServiceID: %s", def.ServiceID)
	}
	if len(suggestions) != 1 || suggestions[0] != "service:com.example.ext/bridge" {
		t.Errorf("unexpected suggestions: %v", suggestions)
	}
}

func TestDefinitionResolver_ResolveForService_NotFound(t *testing.T) {
	provider := newMockProvider()
	provider.getErr = trusted_service.ErrServiceNotFound

	resolver, _ := NewDefinitionResolver(provider)

	result, suggestions, err := resolver.ResolveForService("unknown-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil definition for missing service")
	}
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions, got %v", suggestions)
	}
}

func TestDefinitionResolver_ListDefinitions(t *testing.T) {
	provider := newMockProvider()
	provider.registered["ext1/bridge"] = &trusted_service.ServiceRuntimeDefinition{ServiceID: "ext1/bridge"}
	provider.registered["ext2/bridge"] = &trusted_service.ServiceRuntimeDefinition{ServiceID: "ext2/bridge"}

	resolver, _ := NewDefinitionResolver(provider)
	defs := resolver.ListDefinitions()

	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestDefinitionResolver_Count(t *testing.T) {
	provider := newMockProvider()
	provider.registered["svc-1"] = &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-1"}
	provider.registered["svc-2"] = &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-2"}
	provider.registered["svc-3"] = &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-3"}

	resolver, _ := NewDefinitionResolver(provider)
	if resolver.Count() != 3 {
		t.Errorf("expected count 3, got %d", resolver.Count())
	}
}

func TestDefinitionResolver_ExtensionIDs(t *testing.T) {
	provider := newMockProvider()
	provider.registered["com.ext1/bridge"] = &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "com.ext1/bridge",
		ExtensionID: "com.ext1",
	}
	provider.registered["com.ext2/bridge"] = &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "com.ext2/bridge",
		ExtensionID: "com.ext2",
	}

	resolver, _ := NewDefinitionResolver(provider)
	ids := resolver.ExtensionIDs()

	if len(ids) != 2 {
		t.Errorf("expected 2 extension ids, got %d", len(ids))
	}
}
