package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type TaskSupervisorFactory struct {
	service *TaskRuntimeService
}

func NewTaskSupervisorFactory(service *TaskRuntimeService) *TaskSupervisorFactory {
	return &TaskSupervisorFactory{service: service}
}

func (f *TaskSupervisorFactory) Type() domain.RuntimeType {
	return domain.RuntimeTypeTask
}

func (f *TaskSupervisorFactory) Validate(spec runtime_supervisor.InstanceSpec) error {
	if spec.RuntimeType != domain.RuntimeTypeTask {
		return errors.New("task_runtime: runtime type must be task")
	}
	if spec.ExtensionID == "" {
		return errors.New("task_runtime: extension id required")
	}
	if spec.ModuleID == "" {
		return errors.New("task_runtime: module id required")
	}
	if spec.EntryPoint == "" {
		return errors.New("task_runtime: entry point required")
	}
	return nil
}

func (f *TaskSupervisorFactory) Create(_ context.Context, spec runtime_supervisor.InstanceSpec) (runtime_supervisor.ManagedRuntime, error) {
	if err := f.Validate(spec); err != nil {
		return nil, err
	}
	return &managedTaskRuntime{
		service:    f.service,
		spec:       spec,
		started:    false,
		instanceID: fmt.Sprintf("task-%s-%s-%d", spec.ExtensionID, spec.ModuleID, spec.Generation),
	}, nil
}

type managedTaskRuntime struct {
	mu         sync.Mutex
	service    *TaskRuntimeService
	spec       runtime_supervisor.InstanceSpec
	started    bool
	instanceID string
}

func (m *managedTaskRuntime) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errors.New("task_runtime: already started")
	}
	m.started = true
	return nil
}

func (m *managedTaskRuntime) Invoke(ctx context.Context, request runtime_supervisor.InvocationRequest) runtime_supervisor.InvocationResult {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        errors.New("task_runtime: not started"),
		}
	}
	m.mu.Unlock()

	input := request.Input
	if len(input) == 0 {
		input = []byte(`{}`)
	}

	def := &TaskDefinition{
		TaskID:      fmt.Sprintf("task-%s", m.spec.DefinitionID),
		ExtensionID: string(m.spec.ExtensionID),
		ModuleID:    string(m.spec.ModuleID),
		RuntimeType: string(domain.RuntimeTypeTask),
		Entry:       m.spec.EntryPoint,
		RetryPolicy: DefaultRetryPolicy(),
		TimeoutPolicy: TaskTimeoutPolicy{
			DefaultTimeout: 30 * time.Minute,
			MaxTimeout:     24 * time.Hour,
		},
	}

	if err := m.service.PutTaskDefinition(ctx, def); err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("task_runtime: put definition: %w", err),
		}
	}

	enqueueReq := EnqueueTaskRequest{
		TaskDefinitionID: def.TaskID,
		ExtensionID:      string(m.spec.ExtensionID),
		ModuleID:         string(m.spec.ModuleID),
		Input:            input,
		OperationID:      request.InvocationID,
	}

	result, err := m.service.Enqueue(ctx, enqueueReq, def)
	if err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        err,
		}
	}

	return runtime_supervisor.InvocationResult{
		InvocationID: request.InvocationID,
		Status:       "success",
		Output:       []byte(fmt.Sprintf(`{"taskRunId":"%s","status":"%s"}`, result.TaskRunID, result.Status)),
	}
}

func (m *managedTaskRuntime) Health(_ context.Context) runtime_supervisor.HealthReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := runtime_supervisor.HealthUnhealthy
	if m.started {
		status = runtime_supervisor.HealthHealthy
	}

	return runtime_supervisor.HealthReport{
		Status:    status,
		Reason:    "task runtime",
		CheckedAt: time.Now().UTC(),
		Metrics: map[string]any{
			"instanceId": m.instanceID,
		},
	}
}

func (m *managedTaskRuntime) Stop(_ context.Context, _ runtime_supervisor.StopReason) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = false
	return nil
}

var _ runtime_supervisor.RuntimeFactory = (*TaskSupervisorFactory)(nil)
var _ runtime_supervisor.ManagedRuntime = (*managedTaskRuntime)(nil)
