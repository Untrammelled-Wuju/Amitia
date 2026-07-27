package trusted_service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type DefinitionProvider interface {
	GetServiceDefinition(serviceID string) (*ServiceRuntimeDefinition, error)
}

type TrustedServiceFactory struct {
	mu         sync.RWMutex
	supervisor *ProcessSupervisor
	provider   DefinitionProvider
	rootDir    string
}

func NewTrustedServiceFactory(supervisor *ProcessSupervisor, provider DefinitionProvider, rootDir string) *TrustedServiceFactory {
	return &TrustedServiceFactory{
		supervisor: supervisor,
		provider:   provider,
		rootDir:    rootDir,
	}
}

func (f *TrustedServiceFactory) Type() domain.RuntimeType {
	return domain.RuntimeTypeService
}

func (f *TrustedServiceFactory) Validate(spec runtime_supervisor.InstanceSpec) error {
	if spec.DefinitionID == "" {
		return fmt.Errorf("trusted_service: definition id required")
	}
	if spec.ExtensionID == "" {
		return fmt.Errorf("trusted_service: extension id required")
	}
	if spec.Generation <= 0 {
		return fmt.Errorf("trusted_service: generation must be positive")
	}
	def, err := f.provider.GetServiceDefinition(string(spec.DefinitionID))
	if err != nil {
		return fmt.Errorf("trusted_service: lookup definition: %w", err)
	}
	if def == nil {
		return fmt.Errorf("trusted_service: definition %s not found", spec.DefinitionID)
	}
	if len(def.Executables) == 0 {
		return fmt.Errorf("trusted_service: no executables in definition")
	}
	return nil
}

func (f *TrustedServiceFactory) Create(ctx context.Context, spec runtime_supervisor.InstanceSpec) (runtime_supervisor.ManagedRuntime, error) {
	if err := f.Validate(spec); err != nil {
		return nil, err
	}
	def, err := f.provider.GetServiceDefinition(string(spec.DefinitionID))
	if err != nil {
		return nil, err
	}
	publisherTrust := TrustLevelOfficial
	if def.TrustLevel != "" {
		publisherTrust = TrustLevel(def.TrustLevel)
	}
	args := make(map[string]string)
	for k, v := range spec.Env {
		args[k] = v
	}
	return &managedTrustedService{
		supervisor: f.supervisor,
		def:        def,
		spec:       spec,
		startReq: StartRequest{
			ServiceID:      def.ServiceID,
			Generation:     spec.Generation,
			PublisherTrust: publisherTrust,
			BasePath:       f.rootDir,
			SessionToken:   "",
			LogLevel:       "info",
			Args:           args,
		},
	}, nil
}

type managedTrustedService struct {
	mu         sync.Mutex
	supervisor *ProcessSupervisor
	def        *ServiceRuntimeDefinition
	spec       runtime_supervisor.InstanceSpec
	startReq   StartRequest
	instanceID string
	started    bool
}

func (m *managedTrustedService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return fmt.Errorf("trusted_service: already started")
	}
	result, err := m.supervisor.Start(ctx, m.startReq)
	if err != nil {
		return err
	}
	m.instanceID = result.InstanceID
	m.started = true
	return nil
}

func (m *managedTrustedService) Invoke(ctx context.Context, request runtime_supervisor.InvocationRequest) runtime_supervisor.InvocationResult {
	if !m.started {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("trusted_service: not started"),
		}
	}
	timeout := time.Until(request.Deadline)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	result, err := m.supervisor.Invoke(ctx, m.def.ServiceID, request.Operation, request.Input, timeout)
	if err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        err,
			Duration:     time.Since(time.Now()),
		}
	}
	var output json.RawMessage
	if result != nil {
		output = result.Output
	}
	return runtime_supervisor.InvocationResult{
		InvocationID: request.InvocationID,
		Status:       "ok",
		Output:       output,
		Duration:     time.Since(time.Now()),
	}
}

func (m *managedTrustedService) Health(ctx context.Context) runtime_supervisor.HealthReport {
	if !m.started {
		return runtime_supervisor.HealthReport{
			Status: runtime_supervisor.HealthUnknown,
			Reason: "not started",
		}
	}
	result, err := m.supervisor.HealthCheck(ctx, m.def.ServiceID)
	if err != nil {
		return runtime_supervisor.HealthReport{
			Status: runtime_supervisor.HealthUnhealthy,
			Reason: err.Error(),
		}
	}
	status := runtime_supervisor.HealthUnknown
	switch result.Status {
	case "healthy", "ok":
		status = runtime_supervisor.HealthHealthy
	case "degraded":
		status = runtime_supervisor.HealthDegraded
	case "unhealthy":
		status = runtime_supervisor.HealthUnhealthy
	}
	return runtime_supervisor.HealthReport{
		Status:    status,
		Reason:    result.Status,
		CheckedAt: time.Now().UTC(),
		Metrics:   result.Details,
	}
}

func (m *managedTrustedService) Stop(ctx context.Context, reason runtime_supervisor.StopReason) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil
	}
	force := reason == runtime_supervisor.StopReasonCrash ||
		reason == runtime_supervisor.StopReasonQuarantine ||
		reason == runtime_supervisor.StopReasonUninstall
	_, err := m.supervisor.Stop(ctx, StopRequest{
		ServiceID: m.def.ServiceID,
		Reason:    string(reason),
		Force:     force,
	})
	m.started = false
	return err
}

var _ runtime_supervisor.RuntimeFactory = (*TrustedServiceFactory)(nil)
var _ runtime_supervisor.ManagedRuntime = (*managedTrustedService)(nil)
