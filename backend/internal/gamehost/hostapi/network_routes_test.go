package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	kernelhostapi "github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

type networkTestGateway struct {
	routes map[kernelhostapi.Method]kernelhostapi.Route
}

func (g *networkTestGateway) RegisterRoute(route kernelhostapi.Route) error {
	if g.routes == nil {
		g.routes = make(map[kernelhostapi.Method]kernelhostapi.Route)
	}
	if _, exists := g.routes[route.Method]; exists {
		return kernelhostapi.ErrRouteExists
	}
	g.routes[route.Method] = route
	return nil
}
func (g *networkTestGateway) OpenSession(context.Context, runtime_supervisor.RuntimeIdentity, map[kernelhostapi.Method]int) (kernelhostapi.Session, error) {
	return kernelhostapi.Session{}, nil
}
func (g *networkTestGateway) CloseSession(context.Context, string) error { return nil }
func (g *networkTestGateway) Call(context.Context, kernelhostapi.CallRequest) kernelhostapi.CallResult {
	return kernelhostapi.CallResult{}
}
func (g *networkTestGateway) QueryCapability(_ context.Context, method kernelhostapi.Method) (kernelhostapi.Route, bool) {
	route, ok := g.routes[method]
	return route, ok
}
func (g *networkTestGateway) ListMethods(context.Context) []kernelhostapi.Method { return nil }

type networkTestTopology struct {
	serviceID    ghdomain.ServiceID
	moduleID     string
	definitionID string
	err          error
}

func (t networkTestTopology) ResolveServiceIDByModule(ghdomain.RuntimeInstanceID, string) (ghdomain.ServiceID, error) {
	if t.err != nil {
		return "", t.err
	}
	return t.serviceID, nil
}
func (t networkTestTopology) ResolveDefinitionID(ghdomain.RuntimeInstanceID, ghdomain.ServiceID) (string, error) {
	if t.err != nil {
		return "", t.err
	}
	return t.definitionID, nil
}
func (t networkTestTopology) ResolveModuleID(ghdomain.RuntimeInstanceID, ghdomain.ServiceID) (string, error) {
	if t.err != nil {
		return "", t.err
	}
	if t.moduleID != "" {
		return t.moduleID, nil
	}
	return "runtime-module", nil
}

type networkTestDefinitions struct {
	definition *trusted_service.ServiceRuntimeDefinition
	err        error
}

func (d networkTestDefinitions) GetDefinition(string) (*trusted_service.ServiceRuntimeDefinition, error) {
	return d.definition, d.err
}

func restrictedTestDefinition() *trusted_service.ServiceRuntimeDefinition {
	return &trusted_service.ServiceRuntimeDefinition{Network: trusted_service.ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedDomains: []string{"example.com"}, AllowedPorts: []int{443},
	}}
}

func gameRuntimeIdentity() runtime_supervisor.RuntimeIdentity {
	return runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: kerneldomain.ExtensionID("example/plugin"),
		ModuleID:    kerneldomain.ModuleID("runtime-module"),
		Generation:  1,
	}
}

func TestRegisterNetworkRouteGovernance(t *testing.T) {
	gateway := &networkTestGateway{}
	lifecycle, err := RegisterNetworkRouteWithLifecycle(NetworkRouteDeps{
		Gateway:    gateway,
		Topology:   networkTestTopology{serviceID: "runtime-service", definitionID: "definition-1"},
		Supervisor: networkTestDefinitions{definition: restrictedTestDefinition()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Shutdown()
	methods := []kernelhostapi.Method{
		kernelhostapi.MethodNetworkRequest,
		kernelhostapi.MethodNetworkTCPOpen, kernelhostapi.MethodNetworkTCPRead, kernelhostapi.MethodNetworkTCPWrite, kernelhostapi.MethodNetworkTCPClose,
		kernelhostapi.MethodNetworkUDPOpen, kernelhostapi.MethodNetworkUDPReceive, kernelhostapi.MethodNetworkUDPSend, kernelhostapi.MethodNetworkUDPClose,
		kernelhostapi.MethodNetworkWebSocketOpen, kernelhostapi.MethodNetworkWebSocketReceive, kernelhostapi.MethodNetworkWebSocketSend, kernelhostapi.MethodNetworkWebSocketClose,
	}
	for _, method := range methods {
		route, ok := gateway.QueryCapability(context.Background(), method)
		if !ok {
			t.Fatalf("%s was not registered", method)
		}
		if len(route.Permission) != 1 || route.Permission[0].Name != "service.network.request" {
			t.Fatalf("%s unexpected permission mapping: %+v", method, route.Permission)
		}
		if route.RiskLevel != kernelhostapi.RiskMedium || route.SideEffectLevel != kernelhostapi.SideEffectExternal {
			t.Fatalf("%s unexpected route governance: risk=%s sideEffect=%s", method, route.RiskLevel, route.SideEffectLevel)
		}
	}
}

func TestNetworkRouteRejectsNonGameIdentity(t *testing.T) {
	h := &networkRouteHost{
		topology:   networkTestTopology{serviceID: "runtime-service", definitionID: "definition-1"},
		supervisor: networkTestDefinitions{definition: restrictedTestDefinition()},
	}
	_, err := h.networkRequest(context.Background(), kernelhostapi.CallRequest{Input: json.RawMessage(`{"url":"https://example.com/"}`)})
	if !errors.Is(err, kernelhostapi.ErrIdentityInvalid) {
		t.Fatalf("error = %v, want ErrIdentityInvalid", err)
	}
}

func TestNetworkRouteRejectsDestinationOutsideAllowlistBeforeDial(t *testing.T) {
	h := &networkRouteHost{
		topology:   networkTestTopology{serviceID: "runtime-service", definitionID: "definition-1"},
		supervisor: networkTestDefinitions{definition: restrictedTestDefinition()},
	}
	_, err := h.networkRequest(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: gameRuntimeIdentity(),
		Input:           json.RawMessage(`{"method":"GET","url":"https://not-example.invalid/"}`),
	})
	if !errors.Is(err, kernelhostapi.ErrPermissionDenied) {
		t.Fatalf("error = %v, want ErrPermissionDenied", err)
	}
}

func TestNetworkRouteRejectsNonRestrictedDefinition(t *testing.T) {
	h := &networkRouteHost{
		topology: networkTestTopology{serviceID: "runtime-service", definitionID: "definition-1"},
		supervisor: networkTestDefinitions{definition: &trusted_service.ServiceRuntimeDefinition{Network: trusted_service.ServiceNetworkPolicy{
			Mode: "unrestricted", Enforce: true, AllowOutbound: true,
		}}},
	}
	_, err := h.networkRequest(context.Background(), kernelhostapi.CallRequest{
		RuntimeIdentity: gameRuntimeIdentity(),
		Input:           json.RawMessage(`{"method":"GET","url":"https://example.com/"}`),
	})
	if !errors.Is(err, kernelhostapi.ErrPermissionDenied) {
		t.Fatalf("error = %v, want ErrPermissionDenied", err)
	}
}

func TestDecodeStrictJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	var in NetworkRequestInput
	if err := decodeStrictJSON(json.RawMessage(`{"url":"https://example.com/","unknown":true}`), &in); !errors.Is(err, kernelhostapi.ErrInputInvalid) {
		t.Fatalf("unknown field error = %v, want ErrInputInvalid", err)
	}
	if err := decodeStrictJSON(json.RawMessage(`{"url":"https://example.com/"} {}`), &in); !errors.Is(err, kernelhostapi.ErrInputInvalid) {
		t.Fatalf("trailing JSON error = %v, want ErrInputInvalid", err)
	}
}
