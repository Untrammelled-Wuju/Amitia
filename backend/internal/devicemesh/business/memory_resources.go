package business

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) prepareMemoryMutations(ctx context.Context, scope coordination.ExecutionScope, conversation string, derived []DerivedMemory, snapshot *coordination.DataSnapshot) ([]coordination.Mutation, error) {
	mutations, err := memoryMutations(scope, conversation, derived, *snapshot)
	if err != nil {
		return nil, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return mutations, nil
	}
	seen := make(map[string]bool, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		seen[resource.Kind+"/"+resource.ID] = true
	}
	for _, mutation := range mutations {
		key := mutation.Kind + "/" + mutation.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		resource, err := port.Resource(ctx, scope, mutation.Kind, mutation.ID)
		if err != nil {
			return nil, err
		}
		if resource == nil {
			continue
		}
		if resource.Deleted {
			return nil, coordination.ErrResourceVersion
		}
		if resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Kind != mutation.Kind || resource.ID != mutation.ID {
			return nil, coordination.ErrWrongOwner
		}
		snapshot.Resources = append(snapshot.Resources, *resource)
	}
	if err := coordination.ValidateSnapshot(scope, *snapshot); err != nil {
		return nil, err
	}
	return memoryMutations(scope, conversation, derived, *snapshot)
}
