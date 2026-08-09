package service_definition

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestNewDefinitionValidationService_ValidateInput(t *testing.T) {
	_, err := NewDefinitionValidationService(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestDefinitionValidationService_ValidateForRegistration_Valid(t *testing.T) {
	provider := newMockProvider()
	mapper := NewDefinitionMapper()
	vs, err := NewDefinitionValidationService(provider, mapper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	view := ServiceRuntimeView{
		ExtensionID: "com.example.test",
		ModuleID:    "svc-1",
		RuntimeType: "service",
		EntryPoint:  "./bin/svc",
	}

	if err := vs.ValidateForRegistration(view); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefinitionValidationService_ValidateForRegistration_Invalid(t *testing.T) {
	provider := newMockProvider()
	vs, _ := NewDefinitionValidationService(provider, NewDefinitionMapper())

	view := ServiceRuntimeView{
		ExtensionID: "",
		ModuleID:    "mod",
		RuntimeType: "service",
		EntryPoint:  "./bin/svc",
	}

	err := vs.ValidateForRegistration(view)
	if err == nil {
		t.Fatal("expected error for invalid view")
	}
	if !IsServiceDefinitionError(err, ErrDefinitionMappingFailed) {
		t.Errorf("expected definition_mapping_failed, got %v", err)
	}
}

func TestDefinitionValidationService_ValidateExisting_Found(t *testing.T) {
	provider := newMockProvider()
	provider.registered["test/svc"] = &trusted_service.ServiceRuntimeDefinition{
		ServiceID:   "test/svc",
		ExtensionID: "test",
		ModuleID:    "svc",
	}

	vs, _ := NewDefinitionValidationService(provider, NewDefinitionMapper())

	def, err := vs.ValidateExisting("test/svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ServiceID != "test/svc" {
		t.Errorf("unexpected ServiceID: %s", def.ServiceID)
	}
}

func TestDefinitionValidationService_ValidateExisting_EmptyDefinitionID(t *testing.T) {
	provider := newMockProvider()
	vs, _ := NewDefinitionValidationService(provider, NewDefinitionMapper())

	_, err := vs.ValidateExisting("")
	if err == nil {
		t.Fatal("expected error for empty definition ID")
	}
}

func TestDefinitionValidationService_ValidateExisting_NotFound(t *testing.T) {
	provider := newMockProvider()
	provider.getErr = trusted_service.ErrServiceNotFound
	vs, _ := NewDefinitionValidationService(provider, NewDefinitionMapper())

	_, err := vs.ValidateExisting("unknown/def")
	if err == nil {
		t.Fatal("expected error for missing definition")
	}
}

func TestDefinitionValidationService_ValidateAll_Empty(t *testing.T) {
	provider := newMockProvider()
	vs, _ := NewDefinitionValidationService(provider, NewDefinitionMapper())

	report, err := vs.ValidateAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.IsValid {
		t.Error("expected empty report to be valid")
	}
}

func TestDefinitionValidationService_ValidateAll_WithBatchProvider(t *testing.T) {
	mock := &batchMockProvider{
		defs: []*trusted_service.ServiceRuntimeDefinition{
			{
				ServiceID:   "ext/bridge",
				ExtensionID: "ext",
				ModuleID:    "bridge",
				Executables: []trusted_service.PlatformExecutable{makeExecutable("./bin/bridge")},
			},
			{
				ServiceID:   "",
				ExtensionID: "ext",
				ModuleID:    "broken",
				Executables: []trusted_service.PlatformExecutable{makeExecutable("./bin/broken")},
			},
			{
				ServiceID:   "ext/no-exec",
				ExtensionID: "ext",
				ModuleID:    "no-exec",
				Executables: []trusted_service.PlatformExecutable{},
			},
		},
	}

	vs, _ := NewDefinitionValidationService(mock, NewDefinitionMapper())

	report, err := vs.ValidateAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.IsValid {
		t.Error("expected report to be invalid due to bad definitions")
	}
	if report.ValidCount != 1 {
		t.Errorf("expected 1 valid, got %d", report.ValidCount)
	}
	if report.InvalidCount != 2 {
		t.Errorf("expected 2 invalid, got %d", report.InvalidCount)
	}
}

type batchMockProvider struct {
	defs []*trusted_service.ServiceRuntimeDefinition
}

func (b *batchMockProvider) Register(def *trusted_service.ServiceRuntimeDefinition) error {
	return nil
}

func (b *batchMockProvider) Remove(definitionID string) error {
	return nil
}

func (b *batchMockProvider) HasDefinition(definitionID string) bool {
	return false
}

func (b *batchMockProvider) GetForService(serviceID string) (*trusted_service.ServiceRuntimeDefinition, error) {
	return nil, nil
}

func (b *batchMockProvider) ListByExtension(extensionID string) []*trusted_service.ServiceRuntimeDefinition {
	return nil
}

func (b *batchMockProvider) ListAll() []*trusted_service.ServiceRuntimeDefinition {
	return b.defs
}

func makeExecutable(entry string) trusted_service.PlatformExecutable {
	return trusted_service.PlatformExecutable{
		Platform: "windows",
		Entry:    entry,
	}
}
