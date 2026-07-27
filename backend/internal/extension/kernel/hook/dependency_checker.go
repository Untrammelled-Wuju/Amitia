package hook

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
)

type DependencyResolverChecker struct {
	resolver dependency.Resolver
}

func NewDependencyResolverChecker(resolver dependency.Resolver) *DependencyResolverChecker {
	return &DependencyResolverChecker{resolver: resolver}
}

func (c *DependencyResolverChecker) Check(ctx context.Context, requirements []DependencyRequirement) (bool, string) {
	if len(requirements) == 0 {
		return true, ""
	}
	requests := make([]dependency.Request, 0, len(requirements))
	for _, req := range requirements {
		requests = append(requests, dependency.Request{
			Type:         dependency.TargetType(req.Type),
			Target:       req.ID,
			VersionRange: req.MinVersion,
			Required:     !req.Optional,
		})
	}
	result := c.resolver.Resolve(ctx, dependency.ResolveRequest{
		SourceID: "hook-dependency-check",
		Phase:    dependency.PhaseExecute,
		Requests: requests,
	})
	for _, res := range result.Resolutions {
		switch res.Status {
		case dependency.StatusResolved, dependency.StatusDowngraded:
			continue
		case dependency.StatusMissing, dependency.StatusConflict, dependency.StatusPending:
			if res.Request.Required {
				detail := fmt.Sprintf("dependency unavailable: %s", res.Request.Target)
				if len(res.Conflicts) > 0 {
					detail = res.Conflicts[0].Detail
				}
				return false, detail
			}
		}
	}
	return true, ""
}

var _ DependencyChecker = (*DependencyResolverChecker)(nil)
