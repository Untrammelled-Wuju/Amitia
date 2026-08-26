package rpc

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Route struct {
	RuntimeID    domain.RuntimeInstanceID
	PluginID     domain.PluginID
	ServiceID    domain.ServiceID
	Namespace    Namespace
	ConnectionID string
}

func (r Route) OwnerKey() RouteKey {
	return RouteKey{
		RuntimeID: r.RuntimeID,
		Namespace: r.Namespace,
	}
}

func (r Route) ServiceKey() ServiceKey {
	return ServiceKey{
		RuntimeID: r.RuntimeID,
		ServiceID: r.ServiceID,
	}
}

type RouteKey struct {
	RuntimeID domain.RuntimeInstanceID
	Namespace Namespace
}

type ServiceKey struct {
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
}

func (r Route) Validate() error {
	if r.RuntimeID == "" {
		return fmt.Errorf("route runtime id must not be empty")
	}
	if r.PluginID == "" {
		return fmt.Errorf("route plugin id must not be empty")
	}
	if r.ServiceID == "" {
		return fmt.Errorf("route service id must not be empty")
	}
	if r.Namespace == "" {
		return fmt.Errorf("route namespace must not be empty")
	}
	return nil
}
