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
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const defaultToolTimeout = 30 * time.Second

type RequiredCompanionPreparer interface {
	PrepareRequired(ctx context.Context, extensionID, gameRoot, gameVersion string) error
}

type RuntimeAdapter struct {
	plugins    *registry.Registry
	runtimes   *ghruntime.Manager
	topology   *ghruntime.TopologyStore
	control    ipc.ControlPlane
	sessions   *SessionRegistry
	companions RequiredCompanionPreparer
}

func NewRuntimeAdapter(plugins *registry.Registry, runtimes *ghruntime.Manager, topology *ghruntime.TopologyStore, control ipc.ControlPlane) (*RuntimeAdapter, error) {
	if plugins == nil || runtimes == nil || topology == nil || control == nil {
		return nil, fmt.Errorf("game agent bridge: plugin registry, runtime manager, topology store and control plane are required")
	}
	return &RuntimeAdapter{plugins: plugins, runtimes: runtimes, topology: topology, control: control, sessions: NewSessionRegistry()}, nil
}

func (a *RuntimeAdapter) SetCompanionPreparer(preparer RequiredCompanionPreparer) {
	if a == nil {
		return
	}
	a.companions = preparer
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
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidInput, "invalid game plugin RPC method", err)
	}
	peer, err := a.resolvePeer(ctx, binding)
	if err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeRuntimeUnavailable, "game runtime is unavailable", err)
	}
	if err := a.prepareSessionCompanions(ctx, method, peer, input); err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeExecutionFailed, "prepare required game companion artifacts", err)
	}

	requestID := invocation.InvocationID
	if strings.TrimSpace(requestID) == "" {
		requestID = "game-tool-" + uuid.NewString()
	}
	var payload any
	if len(input) > 0 {
		payload = json.RawMessage(input)
	}
	envelope, err := protocol.NewRequestWithRoute(requestID, method, payload, string(peer.RuntimeID), string(peer.PluginID), string(peer.ServiceID))
	if err != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidInput, "build game plugin request", err)
	}
	if peer.Generation > 0 {
		envelope.Generation = uint64(peer.Generation)
	}

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
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeConnectionLost, "game plugin RPC failed", err)
	}
	if response == nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeInvalidResult, "game plugin returned an empty response", nil)
	}
	if response.Error != nil {
		return failure(invocation.InvocationID, toolID, capability.ErrorCodeExecutionFailed, response.Error.Message, fmt.Errorf("%s", response.Error.Code))
	}

	a.trackGameSession(method, peer, invocation, input, response.Payload)

	result := capability.NewToolSuccessResult(invocation.InvocationID, toolID)
	result.RuntimeID = string(peer.RuntimeID)
	result.Generation = peer.Generation
	result.Structured = append(json.RawMessage(nil), response.Payload...)
	result.DurationMS = time.Since(started).Milliseconds()
	result.Metadata = map[string]any{
		"gamePluginId": string(peer.PluginID),
		"serviceId":    string(peer.ServiceID),
		"rpcMethod":    method,
	}
	return result
}

func (a *RuntimeAdapter) prepareSessionCompanions(ctx context.Context, method string, peer ipc.Peer, input json.RawMessage) error {
	if a == nil || a.companions == nil || method != protocol.MethodGameSessionOpen {
		return nil
	}
	var req protocol.GameSessionOpenRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return fmt.Errorf("decode game session open request: %w", err)
		}
	}
	if !req.ShouldAutoInstallCompanions() {
		return nil
	}
	plugin, err := a.plugins.Get(ctx, peer.PluginID)
	if err != nil {
		return fmt.Errorf("resolve game plugin descriptor: %w", err)
	}
	return a.companions.PrepareRequired(ctx, plugin.ExtensionID, strings.TrimSpace(req.GameRoot), strings.TrimSpace(req.GameVersion))
}

func (a *RuntimeAdapter) SessionRegistry() *SessionRegistry {
	if a == nil {
		return nil
	}
	return a.sessions
}

func (a *RuntimeAdapter) trackGameSession(method string, peer ipc.Peer, invocation capability.ToolInvocationContext, input, output json.RawMessage) {
	if a == nil || a.sessions == nil {
		return
	}
	switch method {
	case protocol.MethodGameSessionOpen:
		var session protocol.GameSession
		if err := json.Unmarshal(output, &session); err != nil || strings.TrimSpace(session.ID) == "" {
			return
		}
		a.sessions.Bind(SessionScope{
			GameSessionID:  session.ID,
			PluginID:       peer.PluginID,
			RuntimeID:      peer.RuntimeID,
			ServiceID:      peer.ServiceID,
			UserID:         invocation.UserID,
			CharacterID:    firstNonEmpty(session.CharacterID, invocation.CharacterID),
			ConversationID: invocation.ConversationID,
			Channel:        invocation.Channel,
			HostSessionID:  invocation.SessionID,
		})
	case protocol.MethodGameSessionClose:
		var payload struct {
			SessionID string `json:"sessionId"`
			ID        string `json:"id"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return
		}
		sessionID := firstNonEmpty(payload.SessionID, payload.ID)
		if sessionID != "" {
			a.sessions.Remove(peer.RuntimeID, sessionID)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (a *RuntimeAdapter) Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus {
	if !a.Supports(binding) {
		return capability.HealthUnhealthy
	}
	peer, err := a.resolvePeer(ctx, binding)
	if err != nil {
		return capability.HealthUnhealthy
	}
	runtimeState, err := a.runtimes.GetRuntimeState(peer.RuntimeID)
	if err != nil {
		return capability.HealthUnhealthy
	}
	switch runtimeState {
	case ghdomain.RuntimeStateRunning:
		return capability.HealthReady
	case ghdomain.RuntimeStateDegraded, ghdomain.RuntimeStateStarting, ghdomain.RuntimeStateRestarting:
		return capability.HealthDegraded
	case ghdomain.RuntimeStateStopped, ghdomain.RuntimeStateFailed:
		return capability.HealthShutdown
	default:
		return capability.HealthUnknown
	}
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
	if serviceID == "" {
		for _, svc := range snapshot.Services {
			if svc.ServiceKind == ghdomain.ServiceKindProcess {
				serviceID = string(svc.ServiceID)
				break
			}
		}
	}
	if serviceID == "" {
		return ipc.Peer{}, fmt.Errorf("no executable game service is available")
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
	pluginID := metadataString(binding.Metadata, "gamePluginId")
	if pluginID != "" {
		return a.plugins.Get(ctx, ghdomain.PluginID(pluginID))
	}
	extensionID := metadataString(binding.Metadata, "extensionId")
	if extensionID == "" {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("game_host binding requires metadata.extensionId or metadata.gamePluginId")
	}
	plugins, err := a.plugins.ListByExtension(ctx, extensionID)
	if err != nil {
		return ghdomain.PluginDescriptor{}, err
	}
	if len(plugins) == 0 {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("no enabled game plugin for extension %s", extensionID)
	}
	if len(plugins) > 1 {
		return ghdomain.PluginDescriptor{}, fmt.Errorf("extension %s exposes multiple game plugins; metadata.gamePluginId is required", extensionID)
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
		if runtimes[i].State == ghdomain.RuntimeStateRunning || runtimes[i].State == ghdomain.RuntimeStateDegraded {
			return runtimes[i].ID, nil
		}
		if candidate == nil {
			copy := runtimes[i]
			candidate = &copy
		}
	}
	if candidate != nil {
		return candidate.ID, nil
	}
	return "", fmt.Errorf("no runtime found for game plugin %s", pluginID)
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
