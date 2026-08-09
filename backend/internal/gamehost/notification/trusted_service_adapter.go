package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TrustedServiceNotificationAdapter struct {
	bridge  NotificationBridge
	routeByService map[string]RouteContext
}

func NewTrustedServiceNotificationAdapter(bridge NotificationBridge) *TrustedServiceNotificationAdapter {
	return &TrustedServiceNotificationAdapter{
		bridge:         bridge,
		routeByService: make(map[string]RouteContext),
	}
}

func (a *TrustedServiceNotificationAdapter) RegisterRoute(serviceID domain.ServiceID, route RouteContext) {
	a.routeByService[string(serviceID)] = route
}

func (a *TrustedServiceNotificationAdapter) UnregisterRoute(serviceID domain.ServiceID) {
	delete(a.routeByService, string(serviceID))
}

func (a *TrustedServiceNotificationAdapter) Handle(
	ctx context.Context,
	serviceID domain.ServiceID,
	method string,
	params json.RawMessage,
) error {
	route, ok := a.routeByService[string(serviceID)]
	if !ok {
		return fmt.Errorf("trusted_service_adapter: unknown service %s", serviceID)
	}
	return a.bridge.Handle(ctx, route, method, params, nil)
}

type DirectNotificationSource struct {
	Route RouteContext
}

func (s DirectNotificationSource) AsRoute() RouteContext {
	return s.Route
}

func BuildRoute(plugin domain.PluginID, runtime domain.RuntimeInstanceID, service domain.ServiceID) RouteContext {
	return RouteContext{
		PluginID:  plugin,
		RuntimeID: runtime,
		ServiceID: service,
	}
}
