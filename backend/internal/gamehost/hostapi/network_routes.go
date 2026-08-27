package hostapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	kernelhostapi "github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

type networkRouteTopology interface {
	ResolveServiceIDByModule(runtimeID ghdomain.RuntimeInstanceID, moduleID string) (ghdomain.ServiceID, error)
	ResolveModuleID(runtimeID ghdomain.RuntimeInstanceID, serviceID ghdomain.ServiceID) (string, error)
	ResolveDefinitionID(runtimeID ghdomain.RuntimeInstanceID, serviceID ghdomain.ServiceID) (string, error)
}

type networkDefinitionReader interface {
	GetDefinition(serviceID string) (*trusted_service.ServiceRuntimeDefinition, error)
}

type NetworkRouteDeps struct {
	Gateway    kernelhostapi.Gateway
	Topology   networkRouteTopology
	Supervisor networkDefinitionReader
}

type networkRouteHost struct {
	topology    networkRouteTopology
	supervisor  networkDefinitionReader
	connections *networkConnectionManager
}

// NetworkLifecycle owns the host-side socket handles created by GameHost
// network routes. It is intentionally separate from plugin process networking:
// the plugin remains sandboxed while these handles live exclusively in the host.
// Lifecycle hooks are wired into service stop, process crash and host shutdown so
// a dead generation can never keep a game-side connection alive.
type NetworkLifecycle struct {
	topology    networkRouteTopology
	connections *networkConnectionManager
}

// OnServiceStopped closes only handles owned by the stopped service. The
// topology lookup converts stable GameHost service IDs back to kernel module IDs
// because RuntimeIdentity is intentionally module-scoped.
func (l *NetworkLifecycle) OnServiceStopped(runtimeID, serviceID string) {
	if l == nil || l.connections == nil || l.topology == nil || runtimeID == "" || serviceID == "" {
		return
	}
	moduleID, err := l.topology.ResolveModuleID(ghdomain.RuntimeInstanceID(runtimeID), ghdomain.ServiceID(serviceID))
	if err != nil || moduleID == "" {
		// Fail safe: if topology disappeared during teardown, close all handles for
		// the runtime rather than retaining an unowned host socket.
		l.connections.closeRuntimeModuleGeneration(runtimeID, "", 0)
		return
	}
	l.connections.closeRuntimeModuleGeneration(runtimeID, moduleID, 0)
}

// CloseRuntimeGenerationNetwork is used by the unexpected-process-exit bridge.
// moduleID may be empty when older supervisor metadata cannot identify the exact
// service; in that case every handle from the crashed runtime generation is
// closed before recovery begins.
func (l *NetworkLifecycle) CloseRuntimeGenerationNetwork(runtimeID, moduleID string, generation int64) int {
	if l == nil || l.connections == nil || runtimeID == "" {
		return 0
	}
	return l.connections.closeRuntimeModuleGeneration(runtimeID, moduleID, generation)
}

// CountRuntimeNetworkHandles is used by safety verification/tests without
// exposing raw host socket objects.
func (l *NetworkLifecycle) CountRuntimeNetworkHandles(runtimeID string) int {
	if l == nil || l.connections == nil || runtimeID == "" {
		return 0
	}
	l.connections.mu.Lock()
	defer l.connections.mu.Unlock()
	count := 0
	for _, handle := range l.connections.handles {
		if handle.owner.InstanceID == runtimeID {
			count++
		}
	}
	return count
}

func (l *NetworkLifecycle) Shutdown() {
	if l == nil || l.connections == nil {
		return
	}
	l.connections.shutdown()
}

type networkRouteSpec struct {
	method  kernelhostapi.Method
	timeout time.Duration
	rate    kernelhostapi.RateLimit
	handler kernelhostapi.Handler
}

// RegisterNetworkRoute installs the complete host-mediated restricted network
// boundary. It is retained for unit/integration callers that do not own a
// GameHost lifecycle. Production composition uses RegisterNetworkRouteWithLifecycle.
func RegisterNetworkRoute(deps NetworkRouteDeps) error {
	_, err := RegisterNetworkRouteWithLifecycle(deps)
	return err
}

// RegisterNetworkRouteWithLifecycle installs the complete host-mediated
// restricted network boundary and returns the host resource lifecycle that must
// be wired into service/process/host teardown.
func RegisterNetworkRouteWithLifecycle(deps NetworkRouteDeps) (*NetworkLifecycle, error) {
	if deps.Gateway == nil || deps.Topology == nil || deps.Supervisor == nil {
		return nil, fmt.Errorf("gamehost hostapi: network route requires gateway, topology, and trusted service supervisor")
	}
	h := &networkRouteHost{
		topology:    deps.Topology,
		supervisor:  deps.Supervisor,
		connections: newNetworkConnectionManager(),
	}
	lifecycle := &NetworkLifecycle{topology: deps.Topology, connections: h.connections}
	defaultRate := kernelhostapi.RateLimit{MaxPerSecond: 30, MaxPerMinute: 900, Burst: 15}
	openRate := kernelhostapi.RateLimit{MaxPerSecond: 8, MaxPerMinute: 120, Burst: 4}
	routes := []networkRouteSpec{
		{kernelhostapi.MethodNetworkRequest, 125 * time.Second, kernelhostapi.RateLimit{MaxPerSecond: 10, MaxPerMinute: 120, Burst: 5}, h.networkRequest},
		{kernelhostapi.MethodNetworkTCPOpen, 125 * time.Second, openRate, h.tcpOpen},
		{kernelhostapi.MethodNetworkTCPRead, 125 * time.Second, defaultRate, h.tcpRead},
		{kernelhostapi.MethodNetworkTCPWrite, 125 * time.Second, defaultRate, h.tcpWrite},
		{kernelhostapi.MethodNetworkTCPClose, 10 * time.Second, defaultRate, h.tcpClose},
		{kernelhostapi.MethodNetworkUDPOpen, 125 * time.Second, openRate, h.udpOpen},
		{kernelhostapi.MethodNetworkUDPReceive, 125 * time.Second, defaultRate, h.udpReceive},
		{kernelhostapi.MethodNetworkUDPSend, 125 * time.Second, defaultRate, h.udpSend},
		{kernelhostapi.MethodNetworkUDPClose, 10 * time.Second, defaultRate, h.udpClose},
		{kernelhostapi.MethodNetworkWebSocketOpen, 125 * time.Second, openRate, h.webSocketOpen},
		{kernelhostapi.MethodNetworkWebSocketReceive, 125 * time.Second, defaultRate, h.webSocketReceive},
		{kernelhostapi.MethodNetworkWebSocketSend, 125 * time.Second, defaultRate, h.webSocketSend},
		{kernelhostapi.MethodNetworkWebSocketClose, 10 * time.Second, defaultRate, h.webSocketClose},
	}

	// Preflight every route before mutating the Gateway so a collision cannot
	// leave a partially registered transport surface.
	for _, spec := range routes {
		if _, exists := deps.Gateway.QueryCapability(context.Background(), spec.method); exists {
			lifecycle.Shutdown()
			return nil, fmt.Errorf("gamehost hostapi: refusing to replace existing route %s", spec.method)
		}
	}
	for _, spec := range routes {
		route := kernelhostapi.Route{
			Method:          spec.method,
			Version:         1,
			Permission:      kernelhostapi.RoutePermissionForMethod(spec.method),
			ScopePolicy:     kernelhostapi.RouteScopeForMethod(spec.method),
			RiskLevel:       kernelhostapi.RouteRiskForMethod(spec.method),
			SideEffectLevel: kernelhostapi.RouteSideEffectForMethod(spec.method),
			RateLimit:       spec.rate,
			Timeout:         spec.timeout,
			Handler:         spec.handler,
		}
		if err := deps.Gateway.RegisterRoute(route); err != nil {
			lifecycle.Shutdown()
			return nil, fmt.Errorf("gamehost hostapi: register %s: %w", route.Method, err)
		}
	}
	return lifecycle, nil
}

func (h *networkRouteHost) resolveDefinition(identity runtime_supervisor.RuntimeIdentity) (*trusted_service.ServiceRuntimeDefinition, error) {
	if identity.InstanceID == "" || identity.ModuleID == "" || identity.ExtensionID == "" {
		return nil, fmt.Errorf("%w: incomplete GameHost runtime identity", kernelhostapi.ErrIdentityInvalid)
	}
	runtimeID := ghdomain.RuntimeInstanceID(identity.InstanceID)
	serviceID, err := h.topology.ResolveServiceIDByModule(runtimeID, string(identity.ModuleID))
	if err != nil {
		return nil, fmt.Errorf("%w: runtime is not a GameHost service: %v", kernelhostapi.ErrHostUnavailable, err)
	}
	definitionID, err := h.topology.ResolveDefinitionID(runtimeID, serviceID)
	if err != nil || definitionID == "" {
		return nil, fmt.Errorf("%w: resolve network policy: %v", kernelhostapi.ErrHostUnavailable, err)
	}
	definition, err := h.supervisor.GetDefinition(definitionID)
	if err != nil || definition == nil {
		return nil, fmt.Errorf("%w: resolve network definition: %v", kernelhostapi.ErrHostUnavailable, err)
	}
	return definition, nil
}

func (h *networkRouteHost) restrictedClient(identity runtime_supervisor.RuntimeIdentity) (*restrictedHTTPClient, error) {
	definition, err := h.resolveDefinition(identity)
	if err != nil {
		return nil, err
	}
	client, err := newRestrictedHTTPClient(definition.Network)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	return client, nil
}

func marshalNetworkOutput(out any) (kernelhostapi.CallResult, error) {
	encoded, err := json.Marshal(out)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: encode network response: %v", kernelhostapi.ErrOutputInvalid, err)
	}
	return kernelhostapi.CallResult{Status: kernelhostapi.StatusSuccess, Output: encoded}, nil
}

func networkOperationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %v", kernelhostapi.ErrCancelled, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", kernelhostapi.ErrTimeout, err)
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return fmt.Errorf("%w: %v", kernelhostapi.ErrTimeout, err)
	}
	return fmt.Errorf("%w: %v", kernelhostapi.ErrHostUnavailable, err)
}

func networkOpenError(err error) error {
	if errors.Is(err, errNetworkPolicyDenied) {
		return fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	return networkOperationError(err)
}

func (h *networkRouteHost) networkRequest(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	client, err := h.restrictedClient(req.RuntimeIdentity)
	if err != nil {
		return kernelhostapi.CallResult{}, err
	}
	var in NetworkRequestInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	out, err := client.Do(ctx, in)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) tcpOpen(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	client, err := h.restrictedClient(req.RuntimeIdentity)
	if err != nil {
		return kernelhostapi.CallResult{}, err
	}
	var in NetworkSocketOpenInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	conn, err := openApprovedSocket(ctx, client, "tcp", in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOpenError(err)
	}
	handleID, err := h.connections.add(req.RuntimeIdentity, networkConnectionTCP, conn, nil, client.maxConnections())
	if err != nil {
		_ = conn.Close()
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	return marshalNetworkOutput(NetworkSocketOpenOutput{
		HandleID: handleID, Transport: "tcp", LocalAddress: conn.LocalAddr().String(), RemoteAddress: conn.RemoteAddr().String(),
	})
}

func (h *networkRouteHost) tcpRead(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkSocketReadInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionTCP)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := readApprovedSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) tcpWrite(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkSocketWriteInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionTCP)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := writeApprovedSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) tcpClose(_ context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	return h.closeSocket(req, networkConnectionTCP)
}

func (h *networkRouteHost) udpOpen(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	client, err := h.restrictedClient(req.RuntimeIdentity)
	if err != nil {
		return kernelhostapi.CallResult{}, err
	}
	var in NetworkSocketOpenInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	conn, err := openApprovedSocket(ctx, client, "udp", in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOpenError(err)
	}
	handleID, err := h.connections.add(req.RuntimeIdentity, networkConnectionUDP, conn, nil, client.maxConnections())
	if err != nil {
		_ = conn.Close()
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	return marshalNetworkOutput(NetworkSocketOpenOutput{
		HandleID: handleID, Transport: "udp", LocalAddress: conn.LocalAddr().String(), RemoteAddress: conn.RemoteAddr().String(),
	})
}

func (h *networkRouteHost) udpReceive(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkSocketReadInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionUDP)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := readApprovedSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) udpSend(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkSocketWriteInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionUDP)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := writeApprovedSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) udpClose(_ context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	return h.closeSocket(req, networkConnectionUDP)
}

func (h *networkRouteHost) webSocketOpen(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	client, err := h.restrictedClient(req.RuntimeIdentity)
	if err != nil {
		return kernelhostapi.CallResult{}, err
	}
	var in NetworkWebSocketOpenInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	conn, subprotocol, err := openApprovedWebSocket(ctx, client, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOpenError(err)
	}
	handleID, err := h.connections.add(req.RuntimeIdentity, networkConnectionWebSocket, nil, conn, client.maxConnections())
	if err != nil {
		_ = conn.Close()
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	remote := ""
	if underlying := conn.UnderlyingConn(); underlying != nil && underlying.RemoteAddr() != nil {
		remote = underlying.RemoteAddr().String()
	}
	return marshalNetworkOutput(NetworkWebSocketOpenOutput{HandleID: handleID, Subprotocol: subprotocol, RemoteAddress: remote})
}

func (h *networkRouteHost) webSocketReceive(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkWebSocketReceiveInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionWebSocket)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := receiveApprovedWebSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) webSocketSend(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	var in NetworkWebSocketSendInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.get(req.RuntimeIdentity, in.HandleID, networkConnectionWebSocket)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	out, err := sendApprovedWebSocket(ctx, handle, in)
	if err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(out)
}

func (h *networkRouteHost) webSocketClose(_ context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	return h.closeSocket(req, networkConnectionWebSocket)
}

func (h *networkRouteHost) closeSocket(req kernelhostapi.CallRequest, kind networkConnectionKind) (kernelhostapi.CallResult, error) {
	var in NetworkSocketCloseInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	handle, err := h.connections.remove(req.RuntimeIdentity, in.HandleID, kind)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	if err := handle.close(); err != nil {
		return kernelhostapi.CallResult{}, networkOperationError(err)
	}
	return marshalNetworkOutput(NetworkSocketCloseOutput{Closed: true})
}

func decodeStrictJSON(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", kernelhostapi.ErrInputInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: multiple JSON values are not allowed", kernelhostapi.ErrInputInvalid)
		}
		return fmt.Errorf("%w: trailing JSON is invalid: %v", kernelhostapi.ErrInputInvalid, err)
	}
	return nil
}
