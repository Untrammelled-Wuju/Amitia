package agentbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/readiness"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const defaultToolTimeout = 30 * time.Second

type RuntimeAdapter struct {
	plugins   *registry.Registry
	runtimes  *ghruntime.Manager
	topology  *ghruntime.TopologyStore
	control   ipc.ControlPlane
	readiness readiness.Reader
	sessions  *SessionRegistry
}

func NewRuntimeAdapter(plugins *registry.Registry, runtimes *ghruntime.Manager, topology *ghruntime.TopologyStore, control ipc.ControlPlane, runtimeReadiness readiness.Reader) (*RuntimeAdapter, error) {
	if plugins == nil || runtimes == nil || topology == nil || control == nil || runtimeReadiness == nil {
		return nil, fmt.Errorf("game agent bridge: plugin registry, runtime manager, topology store, control plane and runtime readiness resolver are required")
	}
	return &RuntimeAdapter{plugins: plugins, runtimes: runtimes, topology: topology, control: control, readiness: runtimeReadiness, sessions: NewSessionRegistry()}, nil
}

func (a *RuntimeAdapter) Supports(binding capability.RuntimeBinding) bool {
	return binding.RuntimeType == capability.RuntimeTypeGameHost
}

func (a *RuntimeAdapter) Execute(ctx context.Context, binding capability.RuntimeBinding, invocation capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	toolID := binding.HandlerName
	if !a.Supports(binding) {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeRuntimeUnavailable, "game_host runtime adapter does not support this binding", nil)
	}
	method := strings.TrimSpace(binding.HandlerName)
	if err := protocol.ValidatePluginMethod(method); err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidInput, "invalid plugin RPC method", err)
	}
	peer, err := a.resolvePeer(ctx, binding)
	if err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeRuntimeUnavailable, "plugin runtime is unavailable", err)
	}
	if err := a.ensurePeerReady(ctx, peer); err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeRuntimeUnavailable, "plugin runtime is not ready", err)
	}
	requestID := invocation.InvocationID
	if strings.TrimSpace(requestID) == "" {
		requestID = "plugin-tool-" + uuid.NewString()
	}
	var payload any
	if len(input) > 0 {
		payload = json.RawMessage(input)
	}
	envelope, err := protocol.NewRequestWithRoute(requestID, method, payload, string(peer.RuntimeID), string(peer.PluginID), string(peer.ServiceID))
	if err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidInput, "build plugin request", err)
	}
	if peer.Generation > 0 {
		envelope.Generation = uint64(peer.Generation)
	}

	// Establish host interaction context before the request is sent. Plugins are
	// allowed to publish events while handling the very first RPC; binding only
	// after the response creates a race where those events are dropped.
	a.bindInvocationContext(peer, invocation)

	timeout := invocation.DeadlineDuration
	if timeout <= 0 {
		timeout = defaultToolTimeout
	}
	started := time.Now()
	response, err := a.control.SendRequest(ctx, peer, envelope, timeout)
	if err != nil {
		if ctx.Err() != nil {
			result := capability.ResultFromContextError(invocation.InvocationID, ctx.Err())
			result.ToolID = toolID
			return result
		}
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeConnectionLost, "plugin RPC failed", err)
	}
	if response == nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidResult, "plugin returned an empty response", nil)
	}
	if response.Error != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeExecutionFailed, response.Error.Message, fmt.Errorf("%s", response.Error.Code))
	}

	result := capability.NewToolSuccessResult(invocation.InvocationID, toolID)
	result.RuntimeID = string(peer.RuntimeID)
	result.Generation = peer.Generation
	result.Structured = append(json.RawMessage(nil), response.Payload...)
	result.DurationMS = time.Since(started).Milliseconds()
	result.Metadata = map[string]any{
		"pluginId":  string(peer.PluginID),
		"serviceId": string(peer.ServiceID),
		"rpcMethod": method,
	}
	return result
}

func (a *RuntimeAdapter) SessionRegistry() *SessionRegistry {
	if a == nil {
		return nil
	}
	return a.sessions
}

// BindAgentContext explicitly binds a validated GameHost runtime route to an
// Agent interaction context without requiring a tool call. This is the host
// entry point for UI/session activation flows and keeps plugin-originated event
// delivery independent from the first Agent -> plugin invocation.
func (a *RuntimeAdapter) BindAgentContext(ctx context.Context, binding capability.RuntimeBinding, invocation capability.ToolInvocationContext) error {
	if a == nil {
		return fmt.Errorf("game agent bridge: adapter is nil")
	}
	if !a.Supports(binding) {
		return fmt.Errorf("game agent bridge: unsupported runtime binding %q", binding.RuntimeType)
	}
	peer, err := a.resolvePeer(ctx, binding)
	if err != nil {
		return err
	}
	a.bindInvocationContext(peer, invocation)
	return nil
}

func (a *RuntimeAdapter) bindInvocationContext(peer ipc.Peer, invocation capability.ToolInvocationContext) {
	if a == nil || a.sessions == nil {
		return
	}
	a.sessions.Bind(SessionScope{
		PluginID: peer.PluginID, RuntimeID: peer.RuntimeID, ServiceID: peer.ServiceID, Generation: peer.Generation,
		UserID: invocation.UserID, CharacterID: invocation.CharacterID,
		ConversationID: invocation.ConversationID, Channel: invocation.Channel,
		HostSessionID: invocation.SessionID,
	})
}

func (a *RuntimeAdapter) ensurePeerReady(ctx context.Context, peer ipc.Peer) error {
	if a == nil || a.readiness == nil {
		return fmt.Errorf("game agent bridge: runtime readiness resolver is unavailable")
	}
	snapshot, err := a.readiness.Resolve(ctx, peer.RuntimeID)
	if err != nil {
		return fmt.Errorf("game agent bridge: resolve runtime readiness: %w", err)
	}
	service, found := snapshot.Service(peer.ServiceID)
	if !found {
		return fmt.Errorf("game agent bridge: service %s is not present in runtime %s topology", peer.ServiceID, peer.RuntimeID)
	}
	if !snapshot.Ready {
		return fmt.Errorf("game agent bridge: runtime %s is not ready (%s)", peer.RuntimeID, snapshot.Reason)
	}
	if !service.Ready {
		return fmt.Errorf("game agent bridge: service %s is not ready", peer.ServiceID)
	}
	return nil
}

func (a *RuntimeAdapter) Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus {
	if a == nil || !a.Supports(binding) || a.readiness == nil {
		return capability.HealthUnhealthy
	}
	peer, err := a.resolvePeer(ctx, binding)
	if err != nil {
		return capability.HealthUnhealthy
	}
	snapshot, err := a.readiness.Resolve(ctx, peer.RuntimeID)
	if err != nil {
		return capability.HealthUnhealthy
	}
	if snapshot.State == ghdomain.RuntimeStateStopped || snapshot.State == ghdomain.RuntimeStateFailed {
		return capability.HealthShutdown
	}
	service, found := snapshot.Service(peer.ServiceID)
	if !found {
		return capability.HealthUnhealthy
	}
	// Runtime readiness is topology-wide, while a capability binding targets a
	// concrete service. Both must be ready before the capability is advertised
	// as healthy. RuntimeStateDegraded can represent an unrelated optional
	// service failure, so it must not make a ready binding non-executable.
	if snapshot.Ready && service.Ready {
		return capability.HealthReady
	}
	if snapshot.Operational || snapshot.State == ghdomain.RuntimeStateStarting || snapshot.State == ghdomain.RuntimeStateRestarting {
		return capability.HealthDegraded
	}
	return capability.HealthUnknown
}

func (a *RuntimeAdapter) resolvePeer(ctx context.Context, binding capability.RuntimeBinding) (ipc.Peer, error) {
	plugin, err := a.resolvePlugin(ctx, binding)
	if err != nil {
		return ipc.Peer{}, err
	}
	runtimeID, err := a.resolveRuntime(ctx, binding, plugin.ID)
	if err != nil {
		return ipc.Peer{}, err
	}
	snapshot, err := a.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return ipc.Peer{}, err
	}
	serviceID := metadataString(binding.Metadata, "serviceId")
	processServices := make([]ghdomain.ServiceID, 0, len(snapshot.Services))
	for _, svc := range snapshot.Services {
		if svc.ServiceKind == ghdomain.ServiceKindProcess {
			processServices = append(processServices, svc.ServiceID)
		}
	}
	if serviceID == "" {
		switch len(processServices) {
		case 0:
			return ipc.Peer{}, fmt.Errorf("no executable plugin service is available")
		case 1:
			serviceID = string(processServices[0])
		default:
			return ipc.Peer{}, fmt.Errorf("runtime binding must declare metadata.serviceId when plugin has multiple executable services")
		}
	} else {
		found := false
		for _, candidate := range processServices {
			if string(candidate) == serviceID {
				found = true
				break
			}
		}
		if !found {
			return ipc.Peer{}, fmt.Errorf("runtime binding service %q is not an executable service of runtime %s", serviceID, runtimeID)
		}
	}
	generation, err := a.runtimes.GetCurrentGeneration(runtimeID)
	if err != nil {
		return ipc.Peer{}, err
	}
	if generation <= 0 {
		return ipc.Peer{}, fmt.Errorf("runtime %s has no active generation", runtimeID)
	}
	return ipc.Peer{PluginID: plugin.ID, RuntimeID: runtimeID, ServiceID: ghdomain.ServiceID(serviceID), Generation: generation}, nil
}

func (a *RuntimeAdapter) resolvePlugin(ctx context.Context, binding capability.RuntimeBinding) (ghdomain.PluginDescriptor, error) {
	pluginID := metadataString(binding.Metadata, "pluginId")
	if pluginID != "" {
		return a.plugins.Get(ctx, ghdomain.PluginID(pluginID))
	}
	extensionID := metadataString(binding.Metadata, "extensionId")
	if extensionID == "" {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("game_host binding requires metadata.extensionId or metadata.pluginId")
	}
	plugins, err := a.plugins.ListByExtension(ctx, extensionID)
	if err != nil {
		return ghdomain.PluginDescriptor{}, err
	}
	if len(plugins) == 0 {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("no enabled game-host plugin for extension %s", extensionID)
	}
	if len(plugins) > 1 {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("extension %s exposes multiple game-host plugins; metadata.pluginId is required", extensionID)
	}
	return plugins[0], nil
}

func (a *RuntimeAdapter) resolveRuntime(ctx context.Context, binding capability.RuntimeBinding, pluginID ghdomain.PluginID) (ghdomain.RuntimeInstanceID, error) {
	if strings.TrimSpace(binding.RuntimeID) != "" {
		runtimeID := ghdomain.RuntimeInstanceID(binding.RuntimeID)
		rt, err := a.runtimes.Get(ctx, runtimeID)
		if err != nil {
			return "", err
		}
		if rt.PluginID != pluginID {
			return "", fmt.Errorf("runtime %s belongs to plugin %s, expected %s", runtimeID, rt.PluginID, pluginID)
		}
		return runtimeID, nil
	}
	runtimes, err := a.runtimes.List(ctx)
	if err != nil {
		return "", err
	}
	var candidate *ghdomain.RuntimeInstance
	for i := range runtimes {
		if runtimes[i].PluginID != pluginID {
			continue
		}
		if a.readiness != nil {
			if ready, readyErr := a.readiness.IsReady(ctx, runtimes[i].ID); readyErr == nil && ready {
				return runtimes[i].ID, nil
			}
		}
		if candidate == nil || (readiness.IsOperationalRuntimeState(runtimes[i].State) && !readiness.IsOperationalRuntimeState(candidate.State)) {
			copy := runtimes[i]
			candidate = &copy
		}
	}
	if candidate != nil {
		return candidate.ID, nil
	}
	return "", fmt.Errorf("no runtime found for plugin %s", pluginID)
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func failure(invocationID, toolID, code, message string, cause error) capability.UnifiedToolResult {
	return capability.NewToolFailureResult(invocationID, toolID, &capability.ToolError{
		Code:      code,
		Category:  capability.ErrorCategoryForCode(code),
		Message:   message,
		Cause:     cause,
		Retryable: code == capability.ErrorCodeConnectionLost || code == capability.ErrorCodeRuntimeUnavailable,
	})
}
