package schedule

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
)

type ResolverDependencyChecker struct {
	Resolver dependency.Resolver
}

func NewResolverDependencyChecker(resolver dependency.Resolver) *ResolverDependencyChecker {
	return &ResolverDependencyChecker{Resolver: resolver}
}

func (c *ResolverDependencyChecker) CheckDependencies(ctx context.Context, requirements []DependencyRequirement) (bool, string, error) {
	if c == nil {
		return false, "dependency checker not configured", nil
	}
	if len(requirements) == 0 {
		return true, "", nil
	}
	if c.Resolver == nil {
		return true, "", nil
	}

	reqs := make([]dependency.Request, 0, len(requirements))
	for _, r := range requirements {
		reqs = append(reqs, dependency.Request{
			Type:     dependency.TargetType(r.Type),
			Target:   r.ID,
			Required: !r.Optional,
		})
	}

	result := c.Resolver.Resolve(ctx, dependency.ResolveRequest{
		Phase:    dependency.PhaseExecute,
		Requests: reqs,
	})

	missingCount := 0
	conflictCount := 0
	for _, res := range result.Resolutions {
		if res.Status == dependency.StatusMissing {
			missingCount++
		}
		if res.Status == dependency.StatusConflict {
			conflictCount++
		}
	}

	if missingCount > 0 || conflictCount > 0 {
		return false, fmt.Sprintf("dependencies not satisfied: missing=%d conflict=%d", missingCount, conflictCount), nil
	}
	return true, "", nil
}

func (c *ResolverDependencyChecker) CreateSnapshot(ctx context.Context, requirements []DependencyRequirement) (string, error) {
	snapshotID := "dep-snap-" + uuid.NewString()
	return snapshotID, nil
}

var _ DependencyChecker = (*ResolverDependencyChecker)(nil)
