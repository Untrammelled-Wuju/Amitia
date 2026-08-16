package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"image"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

type AdapterRegistrationDeps struct {
	JSGlobalFactory       *javascript_main.RuntimeFactory
	WASMFactory           *wasm_runtime.WASMRuntimeFactory
	WASMModuleMgr         *wasm_runtime.ModuleManager
	Supervisor            runtime_supervisor.Supervisor
	TaskService           *task_runtime.TaskRuntimeService
	MCPCaller             capability.MCPCallFunc
	MCPHealth             capability.MCPHealthFunc
	WorkflowCaller        capability.WorkflowCallFunc
	WorkflowCancel        capability.WorkflowCancelFunc
	BuiltinDispatcher     capability.DispatchFunc
	DesktopProvider       capability.DesktopProvider
	AndroidLinuxProvider  interface{}
	AndroidNativeProvider capability.AndroidProvider
	SearchCaller          capability.SearchCallFunc
	SearchHealth          capability.SearchHealthFunc
	BrowserCaller         capability.BrowserCallFunc
	BrowserHealth         capability.BrowserHealthFunc
	InternalDispatcher    capability.InternalCallFunc
	MediaCaller           capability.MediaCallFunc
	MediaHealth           capability.MediaHealthFunc
	WorkspaceCaller       capability.WorkspaceCallFunc
	WorkspaceHealth       capability.WorkspaceHealthFunc
	DeviceRuntimePort     capability.DeviceRuntimeInvocationPort
	BackgroundRemoval     backgroundremoval.Registry
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

	if deps.TaskService != nil {
		taskAdapter := capability.NewTaskRuntimeAdapter(
			makeTaskEnqueueFunc(deps.TaskService),
			makeTaskStatusFunc(deps.TaskService),
		)
		registry.Register(capability.RuntimeTypeTask, taskAdapter)
	}

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

	if deps.AndroidNativeProvider != nil {
		registry.Register(
			capability.RuntimeTypeAndroid_Native,
			capability.NewAndroidRuntimeAdapter(deps.AndroidNativeProvider),
		)
	}

	if deps.SearchCaller != nil {
		searchAdapter := capability.NewSearchRuntimeAdapter(deps.SearchCaller, deps.SearchHealth)
		registry.Register(capability.RuntimeTypeSearch, searchAdapter)
	}

	if deps.BrowserCaller != nil {
		browserAdapter := capability.NewBrowserRuntimeAdapter(deps.BrowserCaller, deps.BrowserHealth)
		registry.Register(capability.RuntimeTypeBrowser, browserAdapter)
	}

	if deps.InternalDispatcher != nil {
		internalAdapter := capability.NewInternalRuntimeAdapter(deps.InternalDispatcher)
		registry.Register(capability.RuntimeTypeInternal, internalAdapter)
	}

	if deps.MediaCaller != nil {
		mediaAdapter := capability.NewMediaRuntimeAdapter(deps.MediaCaller, deps.MediaHealth)
		registry.Register(capability.RuntimeTypeMedia, mediaAdapter)
	}

	if deps.WorkspaceCaller != nil {
		workspaceAdapter := capability.NewWorkspaceRuntimeAdapter(deps.WorkspaceCaller, deps.WorkspaceHealth)
		registry.Register(capability.RuntimeTypeWorkspace, workspaceAdapter)
	}

	if deps.DeviceRuntimePort != nil {
		deviceAdapter := capability.NewDeviceRuntimeAdapter(deps.DeviceRuntimePort)
		registry.RegisterDeviceAdapter(deviceAdapter)
	}

	if deps.BackgroundRemoval != nil {
		bgAdapter := capability.NewBackgroundRemovalRuntimeAdapter(
			makeBackgroundRemovalCallFunc(deps.BackgroundRemoval),
			makeBackgroundRemovalHealthFunc(deps.BackgroundRemoval),
		)
		registry.Register(capability.RuntimeTypeBackgroundRemoval, bgAdapter)
	}

	return nil
}

func makeBackgroundRemovalCallFunc(reg backgroundremoval.Registry) capability.BackgroundRemovalCallFunc {
	return func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
		var req struct {
			Image    []byte `json:"image"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			Mode     string `json:"mode"`
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("invalid background removal input: %w", err)
		}

		bgPolicy := backgroundremoval.BackgroundPolicyConfig{
			ProviderName: req.Provider,
			Mode:         backgroundremoval.BackgroundMode(req.Mode),
		}
		inputDesc := backgroundremoval.ImageDescriptor{
			Width:  req.Width,
			Height: req.Height,
			MIME:   "image/png",
			Pixels: int64(req.Width) * int64(req.Height),
		}

		resolved, err := reg.Resolve(bgPolicy, inputDesc)
		if err != nil {
			return nil, fmt.Errorf("background removal resolve failed: %w", err)
		}
		if resolved.Provider == nil {
			return nil, fmt.Errorf("no background removal provider available")
		}

		v2, ok := resolved.Provider.(backgroundremoval.BackgroundRemovalProviderV2)
		if !ok {
			return nil, fmt.Errorf("background removal provider does not support V2 API")
		}

		bgReq := backgroundremoval.BackgroundRemovalRequest{
			RequestID: uuid.New().String(),
			Image:     decodeNRGBA(req.Image, req.Width, req.Height),
			Mode:      backgroundremoval.BackgroundMode(req.Mode),
		}
		result, err := v2.RemoveBackgroundV2(ctx, bgReq)
		if err != nil {
			return nil, fmt.Errorf("background removal failed: %w", err)
		}

		output := struct {
			Image        []byte  `json:"image"`
			Mask         []byte  `json:"mask"`
			Width        int     `json:"width"`
			Height       int     `json:"height"`
			Provider     string  `json:"provider"`
			Degraded     bool    `json:"degraded"`
			RemovedRatio float64 `json:"removedRatio"`
		}{
			Width:        result.Width,
			Height:       result.Height,
			Provider:     result.Provider,
			Degraded:     result.Degraded,
			RemovedRatio: result.Measurements.RemovedRatio,
		}
		output.Image = encodeNRGBA(result.Foreground)
		output.Mask = encodeGray(result.Mask)

		return json.Marshal(output)
	}
}

func makeBackgroundRemovalHealthFunc(reg backgroundremoval.Registry) func(ctx context.Context) capability.HealthStatus {
	return func(ctx context.Context) capability.HealthStatus {
		if reg == nil {
			return capability.HealthUnknown
		}
		providers := reg.List()
		if len(providers) == 0 {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}

func decodeNRGBA(data []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if len(data) > 0 {
		copy(img.Pix, data)
	}
	return img
}

func encodeNRGBA(img *image.NRGBA) []byte {
	if img == nil {
		return nil
	}
	return img.Pix
}

func encodeGray(img *image.Gray) []byte {
	if img == nil {
		return nil
	}
	return img.Pix
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
		host, err := factory.Get(instanceID)
		if err != nil || host == nil {
			return capability.HealthUnhealthy
		}
		return capability.HealthReady
	}
}

func makeWASMCallFunc(factory *wasm_runtime.WASMRuntimeFactory, mgr *wasm_runtime.ModuleManager) capability.WASMCallFunc {
	return func(ctx context.Context, moduleHash string, exportName string, input json.RawMessage) (json.RawMessage, error) {
		if factory == nil || mgr == nil {
			return nil, fmt.Errorf("WASM runtime not configured")
		}
		_, ok := mgr.Get(moduleHash)
		if !ok {
			return nil, fmt.Errorf("WASM module not found: %s", moduleHash)
		}
		result, err := factory.Invoke(ctx, moduleHash, input)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("WASM invocation returned nil result")
		}
		return result.Output, nil
	}
}

func makeWASMHealthFunc(mgr *wasm_runtime.ModuleManager) capability.WASMHealthFunc {
	return func(ctx context.Context, moduleHash string) capability.HealthStatus {
		if mgr == nil {
			return capability.HealthUnknown
		}
		_, ok := mgr.Get(moduleHash)
		if !ok {
			return capability.HealthUnhealthy
		}
		return capability.HealthReady
	}
}

func makeTrustedServiceCallFunc(supervisor runtime_supervisor.Supervisor) capability.TrustedServiceCallFunc {
	return func(ctx context.Context, serviceID string, handlerName string, input json.RawMessage) (json.RawMessage, error) {
		if supervisor == nil {
			return nil, fmt.Errorf("trusted service supervisor not configured")
		}
		request := runtime_supervisor.InvocationRequest{
			InstanceID: serviceID,
			Operation:  handlerName,
			Input:      input,
		}
		result := supervisor.Invoke(ctx, request)
		if result.Status != "success" {
			return nil, fmt.Errorf("trusted service call failed: %s", result.Status)
		}
		return result.Output, nil
	}
}

func makeTrustedServiceHealthFunc(supervisor runtime_supervisor.Supervisor) capability.TrustedServiceHealthFunc {
	return func(ctx context.Context, serviceID string) capability.HealthStatus {
		if supervisor == nil {
			return capability.HealthUnknown
		}
		snapshot := supervisor.Snapshot(ctx, runtime_supervisor.DefinitionID(serviceID))
		for _, inst := range snapshot.Instances {
			if inst.InstanceID == serviceID {
				return capability.HealthStatus(inst.Health)
			}
		}
		return capability.HealthUnhealthy
	}
}

func makeTaskEnqueueFunc(svc *task_runtime.TaskRuntimeService) capability.TaskEnqueueFunc {
	return func(ctx context.Context, request capability.TaskAdapterEnqueueRequest) (string, error) {
		if svc == nil {
			return "", fmt.Errorf("task runtime service not configured")
		}
		return "", fmt.Errorf("task adapter enqueue not implemented")
	}
}

func makeTaskStatusFunc(svc *task_runtime.TaskRuntimeService) capability.TaskStatusFunc {
	return func(ctx context.Context, taskRunID string) (capability.TaskRunStatus, error) {
		if svc == nil {
			return capability.TaskRunStatus{}, fmt.Errorf("task runtime service not configured")
		}
		return capability.TaskRunStatus{}, fmt.Errorf("task adapter status not implemented")
	}
}
