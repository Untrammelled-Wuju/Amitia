package canary

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type GenerationRouter struct {
	cohortResolver *CohortResolver
	routes         map[string]GenerationRoute
	repo           *CanaryRepository
	mu             sync.RWMutex
}

func NewGenerationRouter(repo *CanaryRepository) *GenerationRouter {
	return &GenerationRouter{
		cohortResolver: NewCohortResolver(),
		routes:         make(map[string]GenerationRoute),
		repo:           repo,
	}
}

func (r *GenerationRouter) routeKey(extensionID, cohortType, cohortID string) string {
	return extensionID + ":" + cohortType + ":" + cohortID
}

func (r *GenerationRouter) Route(ctx context.Context, extensionID, contributionID string, invCtx InvocationContext, policy *CanaryPolicy, oldGen, newGen int64) (int64, RoutingReason, error) {
	cohortType := policy.CohortKey
	var cohortID string
	switch cohortType {
	case CohortKeyCharacter:
		cohortID = invCtx.CharacterID
	case CohortKeyConversation:
		cohortID = invCtx.ConversationID
	case CohortKeyInvocation:
		cohortID = invCtx.InvocationID
	case CohortKeyManualSet:
		cohortID = invCtx.CharacterID
	}

	if cohortID != "" {
		r.mu.RLock()
		persisted, ok := r.routes[r.routeKey(extensionID, string(cohortType), cohortID)]
		r.mu.RUnlock()
		if ok {
			return persisted.Generation, persisted.Reason, nil
		}
	}

	gen, reason, err := r.cohortResolver.ResolveCohort(ctx, policy, invCtx, oldGen, newGen)
	if err != nil {
		return oldGen, RoutingReasonFallback, err
	}

	route := GenerationRoute{
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		CohortType:     cohortType,
		CohortID:       cohortID,
		Generation:     gen,
		Reason:         reason,
		AssignedAt:     time.Now().UTC(),
	}
	if err := r.PersistRoute(ctx, route); err != nil {
		return gen, reason, err
	}

	return gen, reason, nil
}

func (r *GenerationRouter) RouteBackground(ctx context.Context, extensionID string, bgType BackgroundType, resourceID string, policy *CanaryPolicy, oldGen, newGen int64) (int64, RoutingReason, error) {
	if policy.Mode == CanaryModeFull {
		return newGen, RoutingReasonBackground, nil
	}
	return oldGen, RoutingReasonBackground, nil
}

func (r *GenerationRouter) PersistRoute(ctx context.Context, route GenerationRoute) error {
	r.mu.Lock()
	r.routes[r.routeKey(route.ExtensionID, string(route.CohortType), route.CohortID)] = route
	r.mu.Unlock()

	if r.repo != nil {
		if err := r.repo.SaveRoute(ctx, route); err != nil {
			return fmt.Errorf("canary: persist route: %w", err)
		}
	}
	return nil
}

func (r *GenerationRouter) GetPersistedRoute(ctx context.Context, extensionID, cohortType, cohortID string) (*GenerationRoute, error) {
	r.mu.RLock()
	route, ok := r.routes[r.routeKey(extensionID, cohortType, cohortID)]
	r.mu.RUnlock()
	if ok {
		return &route, nil
	}

	if r.repo != nil {
		persisted, err := r.repo.GetRoute(ctx, extensionID, cohortType, cohortID)
		if err != nil || persisted == nil {
			return nil, nil
		}
		r.mu.Lock()
		r.routes[r.routeKey(extensionID, cohortType, cohortID)] = *persisted
		r.mu.Unlock()
		return persisted, nil
	}

	return nil, nil
}
