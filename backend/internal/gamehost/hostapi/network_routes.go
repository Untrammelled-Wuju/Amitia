package hostapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	kernelhostapi "github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

type networkRouteTopology interface {
	ResolveServiceIDByModule(runtimeID ghdomain.RuntimeInstanceID, moduleID string) (ghdomain.ServiceID, error)
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
	topology   networkRouteTopology
	supervisor networkDefinitionReader
}

// RegisterNetworkRoute installs the host-mediated outbound HTTP route used by
// game services whose manifest requests network.mode=restricted. The child
// process itself remains in the platform's no-network sandbox; only this host
// route can reach an allowlisted public endpoint.
func RegisterNetworkRoute(deps NetworkRouteDeps) error {
	if deps.Gateway == nil || deps.Topology == nil || deps.Supervisor == nil {
		return fmt.Errorf("gamehost hostapi: network route requires gateway, topology, and trusted service supervisor")
	}
	if _, exists := deps.Gateway.QueryCapability(context.Background(), kernelhostapi.MethodNetworkRequest); exists {
		return fmt.Errorf("gamehost hostapi: refusing to replace existing route %s", kernelhostapi.MethodNetworkRequest)
	}
	h := &networkRouteHost{topology: deps.Topology, supervisor: deps.Supervisor}
	route := kernelhostapi.Route{
		Method:          kernelhostapi.MethodNetworkRequest,
		Version:         1,
		Permission:      kernelhostapi.RoutePermissionForMethod(kernelhostapi.MethodNetworkRequest),
		ScopePolicy:     kernelhostapi.RouteScopeForMethod(kernelhostapi.MethodNetworkRequest),
		RiskLevel:       kernelhostapi.RouteRiskForMethod(kernelhostapi.MethodNetworkRequest),
		SideEffectLevel: kernelhostapi.RouteSideEffectForMethod(kernelhostapi.MethodNetworkRequest),
		RateLimit:       kernelhostapi.RateLimit{MaxPerSecond: 10, MaxPerMinute: 120, Burst: 5},
		Timeout:         125 * time.Second,
		Handler:         h.networkRequest,
	}
	if err := deps.Gateway.RegisterRoute(route); err != nil {
		return fmt.Errorf("gamehost hostapi: register %s: %w", route.Method, err)
	}
	return nil
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

func (h *networkRouteHost) networkRequest(ctx context.Context, req kernelhostapi.CallRequest) (kernelhostapi.CallResult, error) {
	definition, err := h.resolveDefinition(req.RuntimeIdentity)
	if err != nil {
		return kernelhostapi.CallResult{}, err
	}
	client, err := newRestrictedHTTPClient(definition.Network)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	var in NetworkRequestInput
	if err := decodeStrictJSON(req.Input, &in); err != nil {
		return kernelhostapi.CallResult{}, err
	}
	out, err := client.Do(ctx, in)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: %v", kernelhostapi.ErrPermissionDenied, err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return kernelhostapi.CallResult{}, fmt.Errorf("%w: encode network response: %v", kernelhostapi.ErrOutputInvalid, err)
	}
	return kernelhostapi.CallResult{Status: kernelhostapi.StatusSuccess, Output: encoded}, nil
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
