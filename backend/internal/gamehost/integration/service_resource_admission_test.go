package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/resource"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

type resourceAdmissionReader struct{}

func (resourceAdmissionReader) ResolveRuntime(runtimeID string) (string, string, string, error) {
	return "plugin-1", "ext-1", "starting", nil
}
func (resourceAdmissionReader) ResolveService(runtimeID, serviceID string) (string, string, string, error) {
	return "plugin-1", "ext-1", "created", nil
}
func (resourceAdmissionReader) CurrentGeneration(runtimeID string) (int64, error) { return 7, nil }
func (resourceAdmissionReader) ExtensionEnabled(extensionID string) bool          { return true }
func (resourceAdmissionReader) RuntimeIDsByExtension(extensionID string) []string {
	return []string{"rt-1"}
}

type recordingResourceGovernor struct {
	mu        sync.Mutex
	runtimeID string
	serviceID string
	limits    resource.ServiceResourceLimitsSet
}

func (g *recordingResourceGovernor) ConfigureResourceLimits(runtimeID, serviceID string, limits resource.ServiceResourceLimitsSet) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.runtimeID = runtimeID
	g.serviceID = serviceID
	g.limits = limits
	return nil
}

func (g *recordingResourceGovernor) ClearServiceResourceLimits(runtimeID, serviceID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.runtimeID == runtimeID && g.serviceID == serviceID {
		g.runtimeID = ""
		g.serviceID = ""
		g.limits = resource.ServiceResourceLimitsSet{}
	}
}
func (g *recordingResourceGovernor) ClearRuntimeResourceLimits(runtimeID string) {}
func (g *recordingResourceGovernor) ClearAllResourceLimits()                     {}

func TestServiceResourceAdmissionAdapterWiresPerServiceLimits(t *testing.T) {
	governor := &recordingResourceGovernor{}
	admission := resource.NewResourceAdmissionAdapter(
		resource.NewSubjectMapper(resourceAdmissionReader{}),
		nil,
		nil,
		governor,
	)
	adapter := NewServiceResourceAdmissionAdapter(admission)

	finish, err := adapter.PrepareServiceStart(context.Background(), ghruntime.ServiceExecutionContext{
		RuntimeID:  domain.RuntimeInstanceID("rt-1"),
		PluginID:   domain.PluginID("plugin-1"),
		ServiceID:  domain.ServiceID("svc-1"),
		Generation: 7,
	}, &trusted_service.ServiceRuntimeDefinition{Limits: trusted_service.ServiceResourceLimits{
		MaxMemoryMB:     384,
		MaxCPUPercent:   25,
		MaxSubprocesses: 5,
	}})
	if err != nil {
		t.Fatalf("PrepareServiceStart() error = %v", err)
	}
	if finish == nil {
		t.Fatal("expected startup finish callback")
	}
	defer finish(true)

	governor.mu.Lock()
	defer governor.mu.Unlock()
	if governor.runtimeID != "rt-1" || governor.serviceID != "svc-1" {
		t.Fatalf("wrong resource key: %s/%s", governor.runtimeID, governor.serviceID)
	}
	if governor.limits.MaxMemoryMB != 384 || governor.limits.MaxCPUPercent != 25 || governor.limits.MaxSubprocesses != 5 {
		t.Fatalf("wrong resource limits: %+v", governor.limits)
	}
}

func TestServiceResourceAdmissionAdapterReleaseServiceClearsLimits(t *testing.T) {
	governor := &recordingResourceGovernor{}
	admission := resource.NewResourceAdmissionAdapter(
		resource.NewSubjectMapper(resourceAdmissionReader{}),
		nil,
		nil,
		governor,
	)
	adapter := NewServiceResourceAdmissionAdapter(admission)

	finish, err := adapter.PrepareServiceStart(context.Background(), ghruntime.ServiceExecutionContext{
		RuntimeID:  domain.RuntimeInstanceID("rt-1"),
		PluginID:   domain.PluginID("plugin-1"),
		ServiceID:  domain.ServiceID("svc-1"),
		Generation: 7,
	}, &trusted_service.ServiceRuntimeDefinition{Limits: trusted_service.ServiceResourceLimits{MaxMemoryMB: 128}})
	if err != nil {
		t.Fatalf("PrepareServiceStart() error = %v", err)
	}
	finish(true)
	adapter.ReleaseService("rt-1", "svc-1")

	governor.mu.Lock()
	defer governor.mu.Unlock()
	if governor.runtimeID != "" || governor.serviceID != "" {
		t.Fatalf("resource limits were not released: %s/%s", governor.runtimeID, governor.serviceID)
	}
}
