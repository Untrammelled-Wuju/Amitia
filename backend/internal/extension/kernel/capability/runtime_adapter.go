package capability

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimeType string

const (
	RuntimeTypeBuiltin           RuntimeType = "builtin"
	RuntimeTypePluginJS          RuntimeType = "plugin_js"
	RuntimeTypePluginService     RuntimeType = "plugin_service"
	RuntimeTypeMCP               RuntimeType = "mcp"
	RuntimeTypeWorkflow          RuntimeType = "workflow"
	RuntimeTypeInternal          RuntimeType = "internal"
	RuntimeTypeLegacy            RuntimeType = "legacy"
	RuntimeTypeJavaScript        RuntimeType = "javascript"
	RuntimeTypeWASM              RuntimeType = "wasm"
	RuntimeTypeTrustedService    RuntimeType = "trusted_service"
	RuntimeTypeTask              RuntimeType = "task"
	RuntimeTypeBrowser           RuntimeType = "browser"
	RuntimeTypeSearch            RuntimeType = "search"
	RuntimeTypeAndroid_Native    RuntimeType = "android_native"
	RuntimeTypeAndroidLinux      RuntimeType = "android_linux"
	RuntimeTypeIOS_Native        RuntimeType = "ios_native"
	RuntimeTypeDesktop_Extension RuntimeType = "desktop_extension"
	RuntimeTypeMedia             RuntimeType = "media"
	RuntimeTypeWorkspace         RuntimeType = "workspace"
)

type RuntimeBinding struct {
	RuntimeType RuntimeType    `json:"runtimeType"`
	RuntimeID   string         `json:"runtimeId"`
	HandlerName string         `json:"handlerName"`
	Endpoint    string         `json:"endpoint,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

var (
	ErrRuntimeAdapterAlreadyRegistered       = errors.New("runtime adapter already registered")
	ErrDeviceRuntimeAdapterAlreadyRegistered = errors.New("device runtime adapter already registered")
	ErrRuntimeAdapterNotFound                = errors.New("runtime adapter not found")
)

type RuntimeAdapter interface {
	Supports(binding RuntimeBinding) bool

	Execute(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) UnifiedToolResult

	Health(
		ctx context.Context,
		binding RuntimeBinding,
	) HealthStatus
}

type StreamingRuntimeAdapter interface {
	RuntimeAdapter

	ExecuteStream(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		input json.RawMessage,
		emitter ToolStreamEmitter,
	) UnifiedToolResult
}

type CancellableRuntimeAdapter interface {
	RuntimeAdapter

	Cancel(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		reason ToolCancellationReason,
	) error
}

type ErrRuntimeCancellationUnsupported struct{}

func (e ErrRuntimeCancellationUnsupported) Error() string {
	return "runtime does not support explicit cancellation"
}

type RuntimeExecutionRoute struct {
	Binding RuntimeBinding

	Placement ProviderPlacement

	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID

	ProviderRuntimeInstanceID string

	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	RuntimeSessionID runtimeidentity.RuntimeSessionID

	ConnectionGeneration int64

	RemoteDevice bool
}

type RuntimeExecutionResolver interface {
	ResolveRuntimeExecution(
		ctx context.Context,
		tool ToolDefinition,
		invocation ToolInvocationContext,
	) (RuntimeExecutionRoute, error)
}

type RoutedRuntimeAdapter interface {
	RuntimeAdapter

	ExecuteRoute(
		ctx context.Context,
		route RuntimeExecutionRoute,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) UnifiedToolResult

	HealthRoute(
		ctx context.Context,
		route RuntimeExecutionRoute,
	) HealthStatus
}

type RoutedStreamingRuntimeAdapter interface {
	RoutedRuntimeAdapter

	ExecuteStreamRoute(
		ctx context.Context,
		route RuntimeExecutionRoute,
		invocation ToolInvocationContext,
		input json.RawMessage,
		emitter ToolStreamEmitter,
	) UnifiedToolResult
}

type RoutedCancellableRuntimeAdapter interface {
	RoutedRuntimeAdapter

	CancelRoute(
		ctx context.Context,
		route RuntimeExecutionRoute,
		invocation ToolInvocationContext,
		reason ToolCancellationReason,
	) error
}

type RuntimeExecutionPlan struct {
	Route   RuntimeExecutionRoute
	Adapter RuntimeAdapter
}

type RuntimeAdapterRegistrySnapshot struct {
	RuntimeTypes            []RuntimeType
	DeviceAdapterRegistered bool
}

type RuntimeAdapterRegistry struct {
	mu            sync.RWMutex
	adapters      map[RuntimeType]RuntimeAdapter
	deviceAdapter RuntimeAdapter
}

func NewRuntimeAdapterRegistry() *RuntimeAdapterRegistry {
	return &RuntimeAdapterRegistry{
		adapters: make(map[RuntimeType]RuntimeAdapter),
	}
}

func (r *RuntimeAdapterRegistry) Register(rt RuntimeType, adapter RuntimeAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[rt] = adapter
}

func (r *RuntimeAdapterRegistry) RegisterAdapter(rt RuntimeType, adapter RuntimeAdapter) error {
	if rt == "" {
		return errors.New("runtime type cannot be empty")
	}
	if adapter == nil {
		return errors.New("adapter cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, exists := r.adapters[rt]; exists {
		if existing != adapter {
			return ErrRuntimeAdapterAlreadyRegistered
		}
	}
	r.adapters[rt] = adapter
	return nil
}

func (r *RuntimeAdapterRegistry) RegisterDeviceAdapter(adapter RuntimeAdapter) error {
	if adapter == nil {
		return errors.New("device adapter cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deviceAdapter != nil && r.deviceAdapter != adapter {
		return ErrDeviceRuntimeAdapterAlreadyRegistered
	}
	r.deviceAdapter = adapter
	return nil
}

func (r *RuntimeAdapterRegistry) Resolve(binding RuntimeBinding) (RuntimeAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[binding.RuntimeType]
	return adapter, ok
}

func (r *RuntimeAdapterRegistry) ResolveRoute(route RuntimeExecutionRoute) (RuntimeAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if route.RemoteDevice {
		return r.deviceAdapter, r.deviceAdapter != nil
	}
	adapter, ok := r.adapters[route.Binding.RuntimeType]
	return adapter, ok
}

func (r *RuntimeAdapterRegistry) DeviceAdapter() (RuntimeAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deviceAdapter, r.deviceAdapter != nil
}

func (r *RuntimeAdapterRegistry) Snapshot() RuntimeAdapterRegistrySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]RuntimeType, 0, len(r.adapters))
	for rt := range r.adapters {
		types = append(types, rt)
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})
	return RuntimeAdapterRegistrySnapshot{
		RuntimeTypes:            types,
		DeviceAdapterRegistered: r.deviceAdapter != nil,
	}
}

func BuildRuntimeExecutionPlan(
	ctx context.Context,
	resolver RuntimeExecutionResolver,
	registry *RuntimeAdapterRegistry,
	tool ToolDefinition,
	invocation ToolInvocationContext,
) (RuntimeExecutionPlan, error) {
	route, err := resolver.ResolveRuntimeExecution(ctx, tool, invocation)
	if err != nil {
		return RuntimeExecutionPlan{}, err
	}
	adapter, ok := registry.ResolveRoute(route)
	if !ok {
		return RuntimeExecutionPlan{}, ErrRuntimeAdapterNotFound
	}
	return RuntimeExecutionPlan{
		Route:   route,
		Adapter: adapter,
	}, nil
}
