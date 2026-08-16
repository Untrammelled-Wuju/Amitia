package event

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type EventSubscriptionRegistry struct {
	mu                sync.RWMutex
	subscriptions     map[string]*ResolvedSubscription
	byType            map[EventTypeID][]string
	byExtension       map[string][]string
	schemaRegistry    EventTypeRegistry
	maxSubscribers    int
	repository        SubscriptionRepository
	effectiveResolver EffectiveResolver
}

func NewEventSubscriptionRegistry(schemaRegistry EventTypeRegistry, maxSubscribers int) *EventSubscriptionRegistry {
	if maxSubscribers <= 0 {
		maxSubscribers = 64
	}
	return &EventSubscriptionRegistry{
		subscriptions:  make(map[string]*ResolvedSubscription),
		byType:         make(map[EventTypeID][]string),
		byExtension:    make(map[string][]string),
		schemaRegistry: schemaRegistry,
		maxSubscribers: maxSubscribers,
	}
}

func (r *EventSubscriptionRegistry) SetRepository(repo SubscriptionRepository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repository = repo
}

func (r *EventSubscriptionRegistry) SetEffectiveResolver(resolver EffectiveResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.effectiveResolver = resolver
}

func (r *EventSubscriptionRegistry) LoadFromRepository(ctx context.Context) error {
	if r.repository == nil {
		return nil
	}
	defs, err := r.repository.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("event: load subscriptions: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, def := range defs {
		if _, exists := r.subscriptions[def.ContributionID]; exists {
			continue
		}
		resolved, err := r.resolveLocked(ctx, def)
		if err != nil {
			continue
		}
		r.subscriptions[def.ContributionID] = resolved
		r.byType[def.EventTypeID] = append(r.byType[def.EventTypeID], def.ContributionID)
		r.byExtension[def.ExtensionID] = append(r.byExtension[def.ExtensionID], def.ContributionID)
	}
	return nil
}

func (r *EventSubscriptionRegistry) RebuildEffectiveStates(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.effectiveResolver == nil {
		return
	}
	for id, sub := range r.subscriptions {
		sub.Effective = r.effectiveResolver.Resolve(ctx, sub.Definition)
		r.subscriptions[id] = sub
	}
}

func (r *EventSubscriptionRegistry) RegisterBatch(ctx context.Context, defs []EventSubscriptionDefinition) error {
	if len(defs) == 0 {
		return nil
	}
	for i := range defs {
		if err := defs[i].Validate(); err != nil {
			return fmt.Errorf("event: subscription %s invalid: %w", defs[i].ContributionID, err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, def := range defs {
		if _, exists := r.subscriptions[def.ContributionID]; exists {
			continue
		}
		if r.repository != nil {
			if err := r.repository.Upsert(ctx, def); err != nil {
				return fmt.Errorf("event: persist subscription %s: %w", def.ContributionID, err)
			}
		}
		resolved, err := r.resolveLocked(ctx, def)
		if err != nil {
			if r.repository != nil {
				_ = r.repository.Delete(ctx, def.ContributionID)
			}
			return err
		}
		r.subscriptions[def.ContributionID] = resolved
		r.byType[def.EventTypeID] = append(r.byType[def.EventTypeID], def.ContributionID)
		r.byExtension[def.ExtensionID] = append(r.byExtension[def.ExtensionID], def.ContributionID)
	}
	return nil
}

func (r *EventSubscriptionRegistry) Register(ctx context.Context, def EventSubscriptionDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.subscriptions[def.ContributionID]; exists {
		return fmt.Errorf("%w: %s", ErrSubscriptionConflict, def.ContributionID)
	}
	if r.repository != nil {
		if err := r.repository.Upsert(ctx, def); err != nil {
			return fmt.Errorf("event: persist subscription: %w", err)
		}
	}
	resolved, err := r.resolveLocked(ctx, def)
	if err != nil {
		if r.repository != nil {
			_ = r.repository.Delete(ctx, def.ContributionID)
		}
		return err
	}
	r.subscriptions[def.ContributionID] = resolved
	r.byType[def.EventTypeID] = append(r.byType[def.EventTypeID], def.ContributionID)
	r.byExtension[def.ExtensionID] = append(r.byExtension[def.ExtensionID], def.ContributionID)
	return nil
}

func (r *EventSubscriptionRegistry) resolveLocked(ctx context.Context, def EventSubscriptionDefinition) (*ResolvedSubscription, error) {
	typeDef, err := r.schemaRegistry.GetEventType(ctx, def.EventTypeID, 0)
	if err != nil {
		return nil, fmt.Errorf("event: type %s not registered: %w", def.EventTypeID, err)
	}
	if !def.MatchesVersion(typeDef.Version) {
		return nil, fmt.Errorf("%w: subscription expects %s, type is v%d", ErrUnsupportedVersion, def.EventVersionRange, typeDef.Version)
	}
	if typeDef.SubscriberPolicy.AllowThirdParty == false && def.ExtensionID != "" && def.ExtensionID != "host" {
		return nil, fmt.Errorf("%w: third party not allowed for %s", ErrPermissionDenied, def.EventTypeID)
	}
	allowedFields := typeDef.SubscriberPolicy.AllowedFilterFields
	if len(allowedFields) == 0 {
		allowedFields = defaultFilterFields(typeDef)
	}
	var compiledFilter *CompiledFilter
	if def.Filter.Root.Operator != "" {
		cf, err := CompileFilter(def.Filter, allowedFields)
		if err != nil {
			return nil, err
		}
		compiledFilter = cf
	}
	projector := NewPayloadProjector(typeDef)
	if err := ValidateProjectionRequest(def.Projection, typeDef); err != nil {
		return nil, err
	}
	var effective SubscriptionEffectiveState
	if r.effectiveResolver != nil {
		effective = r.effectiveResolver.Resolve(ctx, def)
	} else {
		effective = SubscriptionEffectiveState{
			Enabled:           def.Enabled,
			Generation:        def.Generation,
			PermissionGranted: false,
			ScopeValid:        false,
			DependenciesReady: false,
			RuntimeAvailable:  false,
			CircuitState:      CircuitClosed,
		}
	}
	return &ResolvedSubscription{
		Definition:     def,
		CompiledFilter: compiledFilter,
		Projector:      projector,
		Effective:      effective,
	}, nil
}

func (r *EventSubscriptionRegistry) Unregister(ctx context.Context, contributionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subscriptions[contributionID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSubscriptionNotFound, contributionID)
	}
	delete(r.subscriptions, contributionID)
	r.byType[sub.Definition.EventTypeID] = r.removeFromIndex(r.byType[sub.Definition.EventTypeID], contributionID)
	if len(r.byType[sub.Definition.EventTypeID]) == 0 {
		delete(r.byType, sub.Definition.EventTypeID)
	}
	r.byExtension[sub.Definition.ExtensionID] = r.removeFromIndex(r.byExtension[sub.Definition.ExtensionID], contributionID)
	if len(r.byExtension[sub.Definition.ExtensionID]) == 0 {
		delete(r.byExtension, sub.Definition.ExtensionID)
	}
	if r.repository != nil {
		_ = r.repository.Delete(ctx, contributionID)
	}
	return nil
}

func (r *EventSubscriptionRegistry) removeFromIndex(slice []string, id string) []string {
	for i, s := range slice {
		if s == id {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func (r *EventSubscriptionRegistry) Activate(ctx context.Context, contributionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subscriptions[contributionID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSubscriptionNotFound, contributionID)
	}
	sub.Effective.Enabled = true
	sub.Definition.Enabled = true
	if r.repository != nil {
		_ = r.repository.SetEnabled(ctx, contributionID, true)
	}
	return nil
}

func (r *EventSubscriptionRegistry) Deactivate(ctx context.Context, contributionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.subscriptions[contributionID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSubscriptionNotFound, contributionID)
	}
	sub.Effective.Enabled = false
	sub.Definition.Enabled = false
	if r.repository != nil {
		_ = r.repository.SetEnabled(ctx, contributionID, false)
	}
	return nil
}

func (r *EventSubscriptionRegistry) UpdateGeneration(ctx context.Context, extensionID string, newGeneration int64, defs []EventSubscriptionDefinition) error {
	for i := range defs {
		defs[i].Generation = newGeneration
		defs[i].Enabled = true
		if err := defs[i].Validate(); err != nil {
			return fmt.Errorf("event: validate %s: %w", defs[i].ContributionID, err)
		}
	}
	candidates := make([]*ResolvedSubscription, 0, len(defs))
	for i := range defs {
		resolved, err := r.resolveLocked(ctx, defs[i])
		if err != nil {
			return fmt.Errorf("event: resolve %s: %w", defs[i].ContributionID, err)
		}
		candidates = append(candidates, resolved)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.byExtension[extensionID]
	for _, id := range old {
		sub := r.subscriptions[id]
		delete(r.subscriptions, id)
		if sub != nil {
			r.byType[sub.Definition.EventTypeID] = r.removeFromIndex(r.byType[sub.Definition.EventTypeID], id)
			if len(r.byType[sub.Definition.EventTypeID]) == 0 {
				delete(r.byType, sub.Definition.EventTypeID)
			}
		}
	}
	delete(r.byExtension, extensionID)
	for _, resolved := range candidates {
		r.subscriptions[resolved.Definition.ContributionID] = resolved
		r.byType[resolved.Definition.EventTypeID] = append(r.byType[resolved.Definition.EventTypeID], resolved.Definition.ContributionID)
		r.byExtension[resolved.Definition.ExtensionID] = append(r.byExtension[resolved.Definition.ExtensionID], resolved.Definition.ContributionID)
	}
	if r.repository != nil {
		if err := r.repository.UpdateGeneration(ctx, extensionID, newGeneration, defs); err != nil {
			return fmt.Errorf("event: persist generation update: %w", err)
		}
	}
	return nil
}

func (r *EventSubscriptionRegistry) DeactivateByExtension(ctx context.Context, extensionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.byExtension[extensionID]
	for _, id := range ids {
		if sub, ok := r.subscriptions[id]; ok {
			sub.Effective.Enabled = false
			sub.Definition.Enabled = false
			if r.repository != nil {
				_ = r.repository.SetEnabled(ctx, id, false)
			}
		}
	}
	return nil
}

func (r *EventSubscriptionRegistry) RemoveByExtension(ctx context.Context, extensionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.byExtension[extensionID]
	for _, id := range ids {
		sub := r.subscriptions[id]
		delete(r.subscriptions, id)
		if sub != nil {
			r.byType[sub.Definition.EventTypeID] = r.removeFromIndex(r.byType[sub.Definition.EventTypeID], id)
			if len(r.byType[sub.Definition.EventTypeID]) == 0 {
				delete(r.byType, sub.Definition.EventTypeID)
			}
		}
	}
	delete(r.byExtension, extensionID)
	if r.repository != nil {
		_, _ = r.repository.DeleteByExtension(ctx, extensionID)
	}
	return nil
}

func (r *EventSubscriptionRegistry) Resolve(ctx context.Context, envelope EventEnvelope) ([]*ResolvedSubscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byType[envelope.EventTypeID]
	var result []*ResolvedSubscription
	for _, id := range ids {
		sub, ok := r.subscriptions[id]
		if !ok {
			continue
		}
		if !sub.Definition.MatchesVersion(envelope.EventVersion) {
			continue
		}
		if !sub.Effective.IsActive() {
			continue
		}
		if sub.CompiledFilter != nil {
			fields := ExtractFilterFields(envelope.Payload, sub.CompiledFilter.AllowedFieldsList())
			if !sub.CompiledFilter.Match(fields) {
				continue
			}
		}
		result = append(result, sub)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Definition.Generation != result[j].Definition.Generation {
			return result[i].Definition.Generation > result[j].Definition.Generation
		}
		return result[i].Definition.ContributionID < result[j].Definition.ContributionID
	})
	if len(result) > r.maxSubscribers {
		result = result[:r.maxSubscribers]
	}
	return result, nil
}

func (r *EventSubscriptionRegistry) ResolveForDelivery(ctx context.Context, envelope EventEnvelope) ([]*ResolvedSubscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byType[envelope.EventTypeID]
	var result []*ResolvedSubscription
	for _, id := range ids {
		sub, ok := r.subscriptions[id]
		if !ok {
			continue
		}
		if !sub.Definition.MatchesVersion(envelope.EventVersion) {
			continue
		}
		if !sub.Effective.IsActive() {
			continue
		}
		if r.effectiveResolver != nil {
			deliveryState := r.effectiveResolver.ResolveForDelivery(ctx, sub.Definition, envelope)
			if !deliveryState.IsActive() {
				continue
			}
		}
		if sub.CompiledFilter != nil {
			fields := ExtractFilterFields(envelope.Payload, sub.CompiledFilter.AllowedFieldsList())
			if !sub.CompiledFilter.Match(fields) {
				continue
			}
		}
		result = append(result, sub)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Definition.Generation != result[j].Definition.Generation {
			return result[i].Definition.Generation > result[j].Definition.Generation
		}
		return result[i].Definition.ContributionID < result[j].Definition.ContributionID
	})
	if len(result) > r.maxSubscribers {
		result = result[:r.maxSubscribers]
	}
	return result, nil
}

func (r *EventSubscriptionRegistry) Get(ctx context.Context, contributionID string) (*ResolvedSubscription, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sub, ok := r.subscriptions[contributionID]
	return sub, ok
}

func (r *EventSubscriptionRegistry) ListByExtension(ctx context.Context, extensionID string) []*ResolvedSubscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byExtension[extensionID]
	var result []*ResolvedSubscription
	for _, id := range ids {
		if sub, ok := r.subscriptions[id]; ok {
			result = append(result, sub)
		}
	}
	return result
}

func (r *EventSubscriptionRegistry) ListByType(ctx context.Context, typeID EventTypeID) []*ResolvedSubscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byType[typeID]
	var result []*ResolvedSubscription
	for _, id := range ids {
		if sub, ok := r.subscriptions[id]; ok {
			result = append(result, sub)
		}
	}
	return result
}

func (r *EventSubscriptionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subscriptions)
}

func (r *EventSubscriptionRegistry) CountByType(typeID EventTypeID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byType[typeID])
}

func (r *EventSubscriptionRegistry) MarkPermissionRevoked(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.PermissionGranted = false
	}
}

func (r *EventSubscriptionRegistry) MarkScopeInvalid(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.ScopeValid = false
	}
}

func (r *EventSubscriptionRegistry) RestorePermission(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.PermissionGranted = true
	}
}

func (r *EventSubscriptionRegistry) RestoreScope(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.ScopeValid = true
	}
}

func (r *EventSubscriptionRegistry) MarkRuntimeUnavailable(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.RuntimeAvailable = false
	}
}

func (r *EventSubscriptionRegistry) MarkRuntimeAvailable(contributionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.RuntimeAvailable = true
	}
}

func (r *EventSubscriptionRegistry) MarkCircuitState(contributionID string, state CircuitState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sub, ok := r.subscriptions[contributionID]; ok {
		sub.Effective.CircuitState = state
	}
}

func defaultFilterFields(typeDef EventTypeDefinition) []string {
	if len(typeDef.ProjectionRules) > 0 {
		fields := make([]string, 0, len(typeDef.ProjectionRules))
		for _, r := range typeDef.ProjectionRules {
			if r.TargetPath != "" {
				fields = append(fields, r.TargetPath)
			}
		}
		return fields
	}
	return []string{"eventTypeId", "aggregateId", "partitionKey", "depth"}
}

func (d EventSubscriptionDefinition) ProducerPolicyFor(extensionID string) bool {
	return d.ExtensionID == extensionID
}

var _ = errors.New
