package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimePeerResolver struct {
	topology RuntimeTopologyReader
}

func NewRuntimePeerResolver(topology RuntimeTopologyReader) *RuntimePeerResolver {
	return &RuntimePeerResolver{topology: topology}
}

func (r *RuntimePeerResolver) ResolveService(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
) (domain.PluginID, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.topology == nil {
		return "", fmt.Errorf("runtime topology is unavailable")
	}
	snapshot, err := r.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return "", err
	}
	for _, service := range snapshot.Services {
		if string(service.ServiceID) == string(serviceID) && string(service.RuntimeID) == string(runtimeID) {
			return service.PluginID, nil
		}
	}
	return "", fmt.Errorf("service %q not found in runtime %q", serviceID, runtimeID)
}
