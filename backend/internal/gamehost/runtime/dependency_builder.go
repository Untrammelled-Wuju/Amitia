package runtime

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DependencyGraphBuilder struct{}

func NewDependencyGraphBuilder() *DependencyGraphBuilder {
	return &DependencyGraphBuilder{}
}

func (b *DependencyGraphBuilder) Build(topology *RuntimeTopologySnapshot) (*DependencyGraph, error) {
	if topology == nil {
		return nil, NewTopologyError(ErrInvalidArgument, "topology snapshot must not be nil")
	}

	graph := newDependencyGraph(topology.RuntimeID)

	for _, svc := range topology.Services {
		seenDeps := make(map[domain.ServiceID]struct{})
		normalizedDeps := make([]domain.ServiceID, 0, len(svc.Dependencies))

		for _, depID := range svc.Dependencies {
			if depID == svc.ServiceID {
				return nil, NewTopologyErrorWithCause(ErrSelfDependency,
					"service cannot depend on itself",
					NewTopologyError(ErrSelfDependency, string(svc.ServiceID)))
			}

			if _, exists := seenDeps[depID]; exists {
				return nil, NewTopologyErrorWithCause(ErrDuplicateDependency,
					"duplicate dependency detected",
					NewTopologyError(ErrDuplicateDependency, string(svc.ServiceID)+" -> "+string(depID)))
			}
			seenDeps[depID] = struct{}{}

			depFound := false
			for _, otherSvc := range topology.Services {
				if otherSvc.ServiceID == depID {
					depFound = true
					break
				}
			}
			if !depFound {
				return nil, NewTopologyErrorWithCause(ErrDependencyNotFound,
					"service depends on unknown service",
					NewTopologyError(ErrDependencyNotFound, string(svc.ServiceID)+" -> "+string(depID)))
			}

			normalizedDeps = append(normalizedDeps, depID)
		}

		node := &DependencyNode{
			ServiceID:    svc.ServiceID,
			Dependencies: normalizedDeps,
			Dependents:   make([]domain.ServiceID, 0),
		}
		graph.nodes[svc.ServiceID] = node
	}

	for _, svc := range topology.Services {
		node := graph.nodes[svc.ServiceID]
		for _, depID := range node.Dependencies {
			if depNode, exists := graph.nodes[depID]; exists {
				depNode.Dependents = append(depNode.Dependents, svc.ServiceID)
			}
		}
	}

	for _, node := range graph.nodes {
		sortServiceIDs(node.Dependencies)
		sortServiceIDs(node.Dependents)
	}

	return graph, nil
}

func (b *DependencyGraphBuilder) BuildFromSnapshots(services []ServiceInstanceSnapshot, runtimeID domain.RuntimeInstanceID) (*DependencyGraph, error) {
	snapshot := &RuntimeTopologySnapshot{
		RuntimeID: runtimeID,
		Services:  services,
	}
	return b.Build(snapshot)
}

func sortServiceIDs(ids []domain.ServiceID) {
	for i := 0; i < len(ids)-1; i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[i] > ids[j] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
}
