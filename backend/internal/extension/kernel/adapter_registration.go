package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
)

type AdapterRegistrationDeps struct {
	JSGlobalFactory     *javascript_main.RuntimeFactory
	WASMFactory         *wasm_runtime.WASMRuntimeFactory
	WASMModuleMgr       *wasm_runtime.ModuleManager
	Supervisor          runtime_supervisor.Supervisor
	TaskService         *task_runtime.TaskRuntimeService
	MCPCaller           capability.MCPCallFunc
	MCPHealth           capability.MCPHealthFunc
	WorkflowCaller      capability.WorkflowCallFunc
	WorkflowCancel      capability.WorkflowCancelFunc
	BuiltinDispatcher   capability.DispatchFunc
	DesktopProvider     capability.DesktopProvider
	AndroidLinuxProvider interface{}
	SearchCaller        capability.SearchCallFunc
	SearchHealth        capability.SearchHealthFunc
	BrowserCaller       capability.BrowserCallFunc
	BrowserHealth       capability.BrowserHealthFunc
}

func RegisterProductionAdapters(registry *capability.RuntimeAdapterRegistry, deps AdapterRegistrationDeps) error {
	if registry == nil {
		return fmt.Errorf("adapter registry is nil")
	}
	if deps.BuiltinDispatcher == nil {
		return fmt.Errorf("builtin dispatcher must not be nil (no noop allowed)")
	}
	if deps.JSGlobalFactory == nil {
		return fmt.Errorf("javascript runtime factory must not be nil")
	}
	if deps.WASMFactory == nil {
		return fmt.Errorf("wasm runtime factory must not be nil")
	}
	if deps.WASMModuleMgr == nil {
		return fmt.Errorf("wasm module manager must not be nil")
	}
	if deps.Supervisor == nil {
		return fmt.Errorf("runtime supervisor must not be nil")
	}
	if deps.TaskService == nil {
		return fmt.Errorf("task runtime service must not be nil")
	}
	if deps.WorkflowCaller == nil {
		return fmt.Errorf("workflow caller must not be nil (no noop allowed)")
	}

	builtinAdapter := capability.NewBuiltinRuntimeAdapter(deps.BuiltinDispatcher)
	registry.Register(capability.RuntimeTypeBuiltin, builtinAdapter)

	jsAdapter := capability.NewJavaScriptRuntimeAdapter(
		makeJSCallFunc(deps.JSGlobalFactory),
		makeJSHealthFunc(deps.JSGlobalFactory),
	)
	registry.Register(capability.RuntimeTypeJavaScript, jsAdapter)
	registry.Register(capability.RuntimeTypePluginJS, jsAdapter)

	wasmAdapter := capability.NewWASMRuntimeAdapter(
		makeWASMCallFunc(deps.WASMFactory, deps.WASMModuleMgr),
		makeWASMHealthFunc(deps.WASMModuleMgr),
	)
	registry.Register(capability.RuntimeTypeWASM, wasmAdapter)

	tsAdapter := capability.NewTrustedServiceRuntimeAdapter(
		makeTrustedServiceCallFunc(deps.Supervisor),
		makeTrustedServiceHealthFunc(deps.Supervisor),
	)
	registry.Register(capability.RuntimeTypeTrustedService, tsAdapter)
	registry.Register(capability.RuntimeTypePluginService, tsAdapter)

	taskAdapter := capability.NewTaskRuntimeAdapter(
		makeTaskEnqueueFunc(deps.TaskService),
		makeTaskStatusFunc(deps.TaskService),
	)
	registry.Register(capability.RuntimeTypeTask, taskAdapter)

	if deps.MCPCaller != nil {
		mcpAdapter := capability.NewMCPRuntimeAdapter(deps.MCPCaller, deps.MCPHealth)
		registry.Register(capability.RuntimeTypeMCP, mcpAdapter)
	}

	wfAdapter := capability.NewWorkflowRuntimeAdapter(deps.WorkflowCaller, deps.WorkflowCancel)
	registry.Register(capability.RuntimeTypeWorkflow, wfAdapter)

	if deps.DesktopProvider != nil {
		desktopAdapter := capability.NewDesktopRuntimeAdapter(deps.DesktopProvider)
		registry.Register(capability.RuntimeTypeDesktop_Extension, desktopAdapter)
	}

	if deps.AndroidLinuxProvider != nil {
		registerAndroidLinuxAdapter(registry, deps.AndroidLinuxProvider)
	}

	if deps.SearchCaller != nil {
		searchAdapter := capability.NewSearchRuntimeAdapter(deps.SearchCaller, deps.SearchHealth)
		registry.Register(capability.RuntimeTypeSearch, searchAdapter)
	}

	if deps.BrowserCaller != nil {
		browserAdapter := capability.NewBrowserRuntimeAdapter(deps.BrowserCaller, deps.BrowserHealth)
		registry.Register(capability.RuntimeTypeBrowser, browserAdapter)
	}

	return nil
}

func makeJSCallFunc(factory *javascript_main.RuntimeFactory) capability.JavaScriptCallFunc {
	return func(ctx context.Context, extensionID string, moduleID string, handlerName string, input json.RawMessage) (json.RawMessage, error) {
		if factory == nil {
			return nil, fmt.Errorf("javascript runtime factory not configured")
		}

		instanceID := fmt.Sprintf("%s/%s", extensionID, moduleID)
		if moduleID == "" {
			instanceID = extensionID
		}

		host, err := factory.Get(instanceID)
		if err != nil || host == nil {
			return nil, fmt.Errorf("javascript runtime instance not found for %s/%s (no cross-extension search)", extensionID, moduleID)
		}

		result, err := host.Invoke(ctx, handlerName, string(input))
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
}

func makeJSHealthFunc(factory *javascript_main.RuntimeFactory) capability.JavaScriptHealthFunc {
	return func(ctx context.Context, extensionID string, moduleID string) capability.HealthStatus {
		if factory == nil {
			return capability.HealthUnknown
		}
		instanceID := fmt.Sprintf("%s/%s", extensionID, moduleID)
		if moduleID == "" {
			instanceID = extensionID
		}
		_, err := factory.Get(instanceID)
		if err != nil {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}

func makeWASMCallFunc(factory *wasm_runtime.WASMRuntimeFactory, mgr *wasm_runtime.ModuleManager) capability.WASMCallFunc {
	return func(ctx context.Context, moduleHash string, exportName string, input json.RawMessage) (json.RawMessage, error) {
		if factory == nil {
			return nil, fmt.Errorf("wasm runtime factory not configured")
		}
		if mgr == nil {
			return nil, fmt.Errorf("wasm module manager not configured")
		}

		_, ok := mgr.Get(moduleHash)
		if !ok {
			return nil, fmt.Errorf("wasm module not found: %s", moduleHash)
		}

		result, err := factory.Invoke(ctx, moduleHash, input)
		if err != nil {
			return nil, fmt.Errorf("wasm invoke failed for export %s: %w", exportName, err)
		}

		if result == nil {
			return nil, fmt.Errorf("wasm invoke returned nil result for export %s", exportName)
		}

		output, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("wasm result marshal failed: %w", err)
		}
		return output, nil
	}
}

func makeWASMHealthFunc(mgr *wasm_runtime.ModuleManager) capability.WASMHealthFunc {
	return func(ctx context.Context, moduleHash string) capability.HealthStatus {
		if mgr == nil {
			return capability.HealthUnknown
		}
		_, ok := mgr.Get(moduleHash)
		if !ok {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}

func makeTrustedServiceCallFunc(supervisor runtime_supervisor.Supervisor) capability.TrustedServiceCallFunc {
	return func(ctx context.Context, serviceID string, handlerName string, input json.RawMessage) (json.RawMessage, error) {
		if supervisor == nil {
			return nil, fmt.Errorf("runtime supervisor not configured")
		}

		req := runtime_supervisor.InvocationRequest{
			InstanceID:   serviceID,
			Operation:    handlerName,
			Input:        input,
			InvocationID: fmt.Sprintf("ts-%s-%s", serviceID, uuid.NewString()),
		}

		result := supervisor.Invoke(ctx, req)
		if result.Error != nil {
			return nil, result.Error
		}
		return result.Output, nil
	}
}

func makeTrustedServiceHealthFunc(supervisor runtime_supervisor.Supervisor) capability.TrustedServiceHealthFunc {
	return func(ctx context.Context, serviceID string) capability.HealthStatus {
		if supervisor == nil {
			return capability.HealthUnknown
		}
		_, err := supervisor.GetInstance(ctx, serviceID)
		if err != nil {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}

func makeTaskEnqueueFunc(svc *task_runtime.TaskRuntimeService) capability.TaskEnqueueFunc {
	return func(ctx context.Context, request capability.TaskAdapterEnqueueRequest) (string, error) {
		if svc == nil {
			return "", fmt.Errorf("task runtime service not configured")
		}

		def, err := svc.GetTaskDefinition(ctx, request.TaskDefinitionID)
		if err != nil {
			return "", fmt.Errorf("task definition %s not found in repository: %w", request.TaskDefinitionID, err)
		}
		if def == nil {
			return "", fmt.Errorf("task definition %s returned nil from repository", request.TaskDefinitionID)
		}

		req := task_runtime.EnqueueTaskRequest{
			TaskDefinitionID: request.TaskDefinitionID,
			Input:            request.Input,
		}

		result, err := svc.Enqueue(ctx, req, def)
		if err != nil {
			return "", err
		}
		return result.TaskRunID, nil
	}
}

func makeTaskStatusFunc(svc *task_runtime.TaskRuntimeService) capability.TaskStatusFunc {
	return func(ctx context.Context, taskRunID string) (capability.TaskRunStatus, error) {
		if svc == nil {
			return capability.TaskRunStatus{}, fmt.Errorf("task runtime service not configured")
		}

		run, err := svc.GetTaskRun(ctx, taskRunID)
		if err != nil {
			return capability.TaskRunStatus{}, err
		}

		status := capability.TaskRunStatus{
			State:    string(run.Status),
			Finished: run.Status.IsTerminal(),
		}
		if run.ErrorMessage != nil {
			status.Error = *run.ErrorMessage
		}
		return status, nil
	}
}
