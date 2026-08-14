package runtime

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TopologyStore struct {
	mu            sync.RWMutex
	topologies    map[domain.RuntimeInstanceID]*RuntimeTopology
	graphs        map[domain.RuntimeInstanceID]*DependencyGraph
	definitionIDs map[domain.RuntimeInstanceID]map[domain.ServiceID]string
	moduleIDs     map[domain.RuntimeInstanceID]map[domain.ServiceID]string
}

func NewTopologyStore() *TopologyStore {
	return &TopologyStore{
		topologies:    make(map[domain.RuntimeInstanceID]*RuntimeTopology),
		graphs:        make(map[domain.RuntimeInstanceID]*DependencyGraph),
		definitionIDs: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]string),
		moduleIDs:     make(map[domain.RuntimeInstanceID]map[domain.ServiceID]string),
	}
}

func (s *TopologyStore) GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (RuntimeTopologySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topology, ok := s.topologies[runtimeID]
	if !ok {
		return RuntimeTopologySnapshot{}, &TopologyError{
			Code:      ErrNotFound,
			Message:   "topology not found for runtime: " + string(runtimeID),
			RuntimeID: string(runtimeID),
		}
	}
	return topology.Snapshot(), nil
}

func (s *TopologyStore) GetDependencyGraphSnapshot(runtimeID domain.RuntimeInstanceID) (DependencyGraphSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	graph, ok := s.graphs[runtimeID]
	if !ok {
		return DependencyGraphSnapshot{}, &TopologyError{
			Code:      ErrNotFound,
			Message:   "dependency graph not found for runtime: " + string(runtimeID),
			RuntimeID: string(runtimeID),
		}
	}
	return graph.Snapshot(), nil
}

func (s *TopologyStore) GetTopology(runtimeID domain.RuntimeInstanceID) (TopologyAccessor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topology, ok := s.topologies[runtimeID]
	if !ok {
		return nil, &TopologyError{
			Code:      ErrNotFound,
			Message:   "topology not found for runtime: " + string(runtimeID),
			RuntimeID: string(runtimeID),
		}
	}
	return topology, nil
}

func (s *TopologyStore) PutRuntimeGraph(
	runtime *domain.RuntimeInstance,
	descriptor domain.PluginDescriptor,
	definitionIDs map[domain.ServiceID]string,
) error {
	if runtime == nil {
		return &TopologyError{Code: ErrInvalidArgument, Message: "runtime instance must not be nil"}
	}

	builder := NewTopologyBuilder()
	topology, err := builder.Build(runtime, descriptor, runtime.CreatedAt)
	if err != nil {
		return err
	}

	snapshot := topology.Snapshot()
	depBuilder := NewDependencyGraphBuilder()
	graph, err := depBuilder.Build(&snapshot)
	if err != nil {
		return err
	}

	defs := make(map[domain.ServiceID]string, len(definitionIDs))
	for k, v := range definitionIDs {
		defs[k] = v
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.topologies[runtime.ID] = topology
	s.graphs[runtime.ID] = graph
	s.definitionIDs[runtime.ID] = defs

	return nil
}

func (s *TopologyStore) RemoveRuntime(runtimeID domain.RuntimeInstanceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.topologies[runtimeID]; !ok {
		return &TopologyError{
			Code:      ErrNotFound,
			Message:   "runtime not found: " + string(runtimeID),
			RuntimeID: string(runtimeID),
		}
	}

	delete(s.topologies, runtimeID)
	delete(s.graphs, runtimeID)
	delete(s.definitionIDs, runtimeID)
	delete(s.moduleIDs, runtimeID)
	return nil
}

func (s *TopologyStore) BindModuleID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, moduleID string) error {
	if moduleID == "" {
		return &TopologyError{Code: ErrInvalidArgument, Message: "module id must not be empty", RuntimeID: string(runtimeID), ServiceID: string(serviceID)}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.topologies[runtimeID]; !ok {
		return &TopologyError{Code: ErrNotFound, Message: "runtime not found: " + string(runtimeID), RuntimeID: string(runtimeID)}
	}
	if s.moduleIDs[runtimeID] == nil {
		s.moduleIDs[runtimeID] = make(map[domain.ServiceID]string)
	}
	s.moduleIDs[runtimeID][serviceID] = moduleID
	return nil
}

func (s *TopologyStore) ResolveModuleID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	moduleIDs, ok := s.moduleIDs[runtimeID]
	if !ok {
		return "", &TopologyError{Code: ErrNotFound, Message: "runtime module bindings not found", RuntimeID: string(runtimeID)}
	}
	moduleID, ok := moduleIDs[serviceID]
	if !ok {
		return "", &TopologyError{Code: ErrNotFound, Message: "module not found for service: " + string(serviceID), RuntimeID: string(runtimeID), ServiceID: string(serviceID)}
	}
	return moduleID, nil
}

func (s *TopologyStore) ResolveDefinitionID(
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	defs, ok := s.definitionIDs[runtimeID]
	if !ok {
		return "", &TopologyError{
			Code:      ErrNotFound,
			Message:   "runtime not found: " + string(runtimeID),
			RuntimeID: string(runtimeID),
		}
	}

	defID, ok := defs[serviceID]
	if !ok {
		return "", &TopologyError{
			Code:      ErrNotFound,
			Message:   "definition not found for service: " + string(serviceID),
			RuntimeID: string(runtimeID),
			ServiceID: string(serviceID),
		}
	}

	return defID, nil
}

var _ RuntimeTopologyStore = (*TopologyStore)(nil)
var _ ServiceDefinitionBindingResolver = (*TopologyStore)(nil)
