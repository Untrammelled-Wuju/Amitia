package service_definition

import (
	"errors"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type mockDefinitionProvider struct {
	registered  map[string]*trusted_service.ServiceRuntimeDefinition
	registerErr error
	removeErr   error
	listResult  []*trusted_service.ServiceRuntimeDefinition
	getResult   *trusted_service.ServiceRuntimeDefinition
	getErr      error
}

func newMockProvider() *mockDefinitionProvider {
	return &mockDefinitionProvider{
		registered: make(map[string]*trusted_service.ServiceRuntimeDefinition),
	}
}

func (m *mockDefinitionProvider) Register(def *trusted_service.ServiceRuntimeDefinition) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.registered[def.ServiceID] = def
	return nil
}

func (m *mockDefinitionProvider) Remove(definitionID string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.registered, definitionID)
	return nil
}

func (m *mockDefinitionProvider) HasDefinition(definitionID string) bool {
	_, ok := m.registered[definitionID]
	return ok
}

func (m *mockDefinitionProvider) GetForService(serviceID string) (*trusted_service.ServiceRuntimeDefinition, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if def, ok := m.registered[serviceID]; ok {
		return def, nil
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return nil, trusted_service.ErrServiceNotFound
}

func (m *mockDefinitionProvider) ListByExtension(extensionID string) []*trusted_service.ServiceRuntimeDefinition {
	var result []*trusted_service.ServiceRuntimeDefinition
	for _, def := range m.registered {
		if def.ExtensionID == extensionID {
			result = append(result, def)
		}
	}
	return result
}

func (m *mockDefinitionProvider) ListAll() []*trusted_service.ServiceRuntimeDefinition {
	if m.listResult != nil {
		return m.listResult
	}
	result := make([]*trusted_service.ServiceRuntimeDefinition, 0, len(m.registered))
	for _, def := range m.registered {
		result = append(result, def)
	}
	return result
}

type mockDefinitionSource struct {
	views map[string][]ServiceRuntimeView
	err   error
}

func (m *mockDefinitionSource) GetServiceViewsByExtension(extensionID string) ([]ServiceRuntimeView, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.views == nil {
		return nil, nil
	}
	return m.views[extensionID], nil
}

func (m *mockDefinitionSource) GetExtensionIDs() ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	ids := make([]string, 0, len(m.views))
	for id := range m.views {
		ids = append(ids, id)
	}
	return ids, nil
}

func newTestViews(extensionID string) []ServiceRuntimeView {
	return []ServiceRuntimeView{
		{
			ExtensionID: extensionID,
			ModuleID:    "bridge-service",
			RuntimeType: "service",
			Name:        "Bridge Service",
			Description: "Bridge for external service",
			EntryPoint:  "./bin/bridge",
			Env:         map[string]string{"PORT": "8080"},
			Enabled:     true,
		},
		{
			ExtensionID: extensionID,
			ModuleID:    "monitor-service",
			RuntimeType: "service",
			Name:        "Monitor Service",
			Description: "Monitoring service",
			EntryPoint:  "./bin/monitor",
			Env:         map[string]string{},
			Enabled:     true,
		},
	}
}

func newMockSource() *mockDefinitionSource {
	return &mockDefinitionSource{}
}

func newTestService() *DefinitionSyncService {
	return &DefinitionSyncService{
		source:         newMockSource(),
		provider:       newMockProvider(),
		mapper:         NewDefinitionMapper(),
		extensionLocks: make(map[string]*sync.Mutex),
	}
}

func NewTestDefinitionSyncService(source ServiceDefinitionSource, provider ServiceDefinitionBatchProvider, mapper *DefinitionMapper) (*DefinitionSyncService, error) {
	return NewDefinitionSyncService(source, provider, mapper)
}

func newMockSyncServiceWith(source ServiceDefinitionSource, provider ServiceDefinitionBatchProvider) *DefinitionSyncService {
	return &DefinitionSyncService{
		source:         source,
		provider:       provider,
		mapper:         NewDefinitionMapper(),
		extensionLocks: make(map[string]*sync.Mutex),
	}
}

func TestNewDefinitionSyncService_ValidateInput(t *testing.T) {
	_, err := NewDefinitionSyncService(nil, newMockProvider(), NewDefinitionMapper())
	if err == nil {
		t.Fatal("expected error for nil source")
	}

	_, err = NewDefinitionSyncService(newMockSource(), nil, NewDefinitionMapper())
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestDefinitionSyncService_FullSync_EmptySource(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{views: map[string][]ServiceRuntimeView{}}

	service := newMockSyncServiceWith(source, provider)
	report := service.FullSync()

	if len(report.Errors) != 0 {
		t.Errorf("expected no errors, got %v", report.Errors)
	}
	if report.TotalExtensions != 0 {
		t.Errorf("expected 0 extensions, got %d", report.TotalExtensions)
	}
}

func TestDefinitionSyncService_FullSync_Success(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": newTestViews("com.example.ext1"),
			"com.example.ext2": newTestViews("com.example.ext2"),
		},
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.FullSync()

	if len(report.Errors) != 0 {
		t.Errorf("expected no errors, got %v", report.Errors)
	}
	if report.TotalExtensions != 2 {
		t.Errorf("expected 2 extensions, got %d", report.TotalExtensions)
	}
	if report.SyncedDefinitions != 4 {
		t.Errorf("expected 4 synced definitions, got %d", report.SyncedDefinitions)
	}
}

func TestDefinitionSyncService_FullSync_SourceError(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		err: errors.New("source failed"),
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.FullSync()

	if len(report.Errors) == 0 {
		t.Error("expected errors")
	}
}

func TestDefinitionSyncService_ReconcileExtension_Success(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": newTestViews("com.example.ext1"),
		},
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.ReconcileExtension("com.example.ext1")

	if len(report.Errors) != 0 {
		t.Errorf("expected no errors, got %v", report.Errors)
	}
	if report.Added != 2 {
		t.Errorf("expected 2 added, got %d", report.Added)
	}
	if report.Updated != 0 {
		t.Errorf("expected 0 updated, got %d", report.Updated)
	}
	if report.Removed != 0 {
		t.Errorf("expected 0 removed, got %d", report.Removed)
	}
}

func TestDefinitionSyncService_ReconcileExtension_ProviderError(t *testing.T) {
	provider := newMockProvider()
	provider.registerErr = errors.New("register failed")
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": newTestViews("com.example.ext1"),
		},
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.ReconcileExtension("com.example.ext1")

	if len(report.Errors) == 0 {
		t.Error("expected errors")
	}
}

func TestDefinitionSyncService_ReconcileExtension_ViewError(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		err: errors.New("source failed"),
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.ReconcileExtension("com.example.ext1")

	if len(report.Errors) == 0 {
		t.Error("expected errors")
	}
}

func TestDefinitionSyncService_ReconcileExtension_SkipsInvalid(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": {
				{
					ExtensionID: "com.example.ext1",
					ModuleID:    "",
					RuntimeType: "service",
					EntryPoint:  "./bin/svc",
					Enabled:     true,
				},
			},
		},
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.ReconcileExtension("com.example.ext1")

	if report.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Skipped)
	}
	if len(report.DefinitionErrors) == 0 {
		t.Error("expected definition errors")
	}
}

func TestDefinitionSyncService_ReconcileExtension_SkipsDisabled(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": {
				{
					ExtensionID: "com.example.ext1",
					ModuleID:    "svc-1",
					RuntimeType: "service",
					EntryPoint:  "./bin/svc",
					Enabled:     false,
				},
			},
		},
	}

	service := newMockSyncServiceWith(source, provider)
	report := service.ReconcileExtension("com.example.ext1")

	if report.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Skipped)
	}
}

func TestDefinitionSyncService_RemoveExtension(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": newTestViews("com.example.ext1"),
		},
	}

	service := newMockSyncServiceWith(source, provider)
	service.ReconcileExtension("com.example.ext1")

	if len(provider.registered) != 2 {
		t.Fatalf("expected 2 registered, got %d", len(provider.registered))
	}

	report := service.RemoveExtension("com.example.ext1")

	if len(report.Errors) != 0 {
		t.Errorf("expected no errors, got %v", report.Errors)
	}
	if report.RemovedCount != 2 {
		t.Errorf("expected 2 removed, got %d", report.RemovedCount)
	}
	if len(provider.registered) != 0 {
		t.Errorf("expected 0 registered after removal, got %d", len(provider.registered))
	}
}

func TestDefinitionSyncService_RemoveExtension_ProviderError(t *testing.T) {
	provider := newMockProvider()
	source := &mockDefinitionSource{
		views: map[string][]ServiceRuntimeView{
			"com.example.ext1": newTestViews("com.example.ext1"),
		},
	}

	service := newMockSyncServiceWith(source, provider)
	service.ReconcileExtension("com.example.ext1")

	provider.removeErr = errors.New("remove failed")
	report := service.RemoveExtension("com.example.ext1")

	if len(report.Errors) == 0 {
		t.Error("expected errors")
	}
}

func TestDefinitionSyncService_DefinitionStub_Success(t *testing.T) {
	view := ServiceRuntimeView{
		ExtensionID: "com.example.ext1",
		ModuleID:    "bridge-service",
		RuntimeType: "service",
		Name:        "Bridge",
		EntryPoint:  "./bin/bridge",
		Enabled:     true,
	}

	stub, err := DefinitionStub(view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.ServiceID != "com.example.ext1/bridge-service" {
		t.Errorf("unexpected ServiceID: %s", stub.ServiceID)
	}
}

func TestDefinitionSyncService_DefinitionStub_Error(t *testing.T) {
	view := ServiceRuntimeView{
		ExtensionID: "",
		ModuleID:    "mod",
		RuntimeType: "service",
		EntryPoint:  "./bin/svc",
	}

	_, err := DefinitionStub(view)
	if err == nil {
		t.Fatal("expected error for invalid view")
	}
}

func TestDefinitionSyncService_ValidateView(t *testing.T) {
	tests := []struct {
		name    string
		view    ServiceRuntimeView
		wantErr bool
		errCode ServiceDefinitionErrorCode
	}{
		{
			name: "valid",
			view: ServiceRuntimeView{
				ExtensionID: "com.example.test",
				ModuleID:    "svc-1",
				RuntimeType: "service",
				EntryPoint:  "./bin/svc",
			},
			wantErr: false,
		},
		{
			name:    "empty extension id",
			view:    ServiceRuntimeView{ModuleID: "mod", RuntimeType: "service"},
			wantErr: true,
			errCode: ErrDefinitionMappingFailed,
		},
		{
			name:    "empty module id",
			view:    ServiceRuntimeView{ExtensionID: "com.test", RuntimeType: "service"},
			wantErr: true,
			errCode: ErrDefinitionMappingFailed,
		},
		{
			name:    "invalid runtime type",
			view:    ServiceRuntimeView{ExtensionID: "com.test", ModuleID: "mod", RuntimeType: "lua"},
			wantErr: true,
			errCode: ErrUnsupportedServiceKind,
		},
		{
			name:    "empty entry point",
			view:    ServiceRuntimeView{ExtensionID: "com.test", ModuleID: "mod", RuntimeType: "service"},
			wantErr: true,
			errCode: ErrDefinitionMappingFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateView(tc.view)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !IsServiceDefinitionError(err, tc.errCode) {
					t.Errorf("expected error code %v, got %v", tc.errCode, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
