package integration

import (
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ControlTopologyAdapter struct {
	topologyStore *runtime.TopologyStore
}

func NewControlTopologyAdapter(topologyStore *runtime.TopologyStore) *ControlTopologyAdapter {
	return &ControlTopologyAdapter{
		topologyStore: topologyStore,
	}
}

func (a *ControlTopologyAdapter) HasRuntime(runtimeID domain.RuntimeInstanceID) bool {
	topo, err := a.topologyStore.GetTopology(runtimeID)
	if err != nil {
		return false
	}
	return topo != nil
}

func (a *ControlTopologyAdapter) GetPluginID(runtimeID domain.RuntimeInstanceID) (domain.PluginID, bool) {
	topo, err := a.topologyStore.GetTopology(runtimeID)
	if err != nil {
		return "", false
	}
	snapshot := topo.Snapshot()
	return snapshot.PluginID, true
}

func (a *ControlTopologyAdapter) ServiceBelongsToRuntime(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) bool {
	topo, err := a.topologyStore.GetTopology(runtimeID)
	if err != nil {
		return false
	}
	_, err = topo.GetService(serviceID)
	return err == nil
}

var _ control.TopologyReader = (*ControlTopologyAdapter)(nil)
