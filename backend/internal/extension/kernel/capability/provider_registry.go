package capability

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type baseRegistry[T any] struct {
	mu      sync.RWMutex
	entries map[string]T
}

func newBaseRegistry[T any]() *baseRegistry[T] {
	return &baseRegistry[T]{
		entries: make(map[string]T),
	}
}

func baseGet[T any](r *baseRegistry[T], key string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.entries[key]
	return v, ok
}

func baseSet[T any](r *baseRegistry[T], key string, value T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = value
}

func baseDel[T any](r *baseRegistry[T], key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, key)
}

func baseSize[T any](r *baseRegistry[T]) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func baseClone[T any](r *baseRegistry[T]) map[string]T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]T, len(r.entries))
	for k, v := range r.entries {
		out[k] = v
	}
	return out
}

func filterInstances(instances []*CapabilityProviderInstance, filters ...func(*CapabilityProviderInstance) bool) []*CapabilityProviderInstance {
	if len(filters) == 0 {
		return instances
	}
	result := make([]*CapabilityProviderInstance, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		keep := true
		for _, fn := range filters {
			if fn == nil {
				continue
			}
			if !fn(inst) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, inst)
		}
	}
	return result
}

func matchPlatform(platforms []runtimeidentity.Platform, target runtimeidentity.Platform) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, p := range platforms {
		if p == target {
			return true
		}
	}
	return false
}

func sortDefs(defs []*CapabilityProviderDefinition) {
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Placement != defs[j].Placement {
			return defs[i].Placement < defs[j].Placement
		}
		if defs[i].Priority != defs[j].Priority {
			return defs[i].Priority > defs[j].Priority
		}
		if defs[i].Kind != defs[j].Kind {
			return defs[i].Kind < defs[j].Kind
		}
		return defs[i].ID < defs[j].ID
	})
}

type ProviderRegistry struct {
	mu           sync.RWMutex
	definitions  map[string]*CapabilityProviderDefinition
	instances    map[string]*CapabilityProviderInstance
	byProvider   map[string][]string
	byCapability map[string][]string
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		definitions:  make(map[string]*CapabilityProviderDefinition),
		instances:    make(map[string]*CapabilityProviderInstance),
		byProvider:   make(map[string][]string),
		byCapability: make(map[string][]string),
	}
}

func (r *ProviderRegistry) getDefinition(key string) (*CapabilityProviderDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.definitions[key]
	if !ok {
		return nil, false
	}
	return cloneProviderDefinition(v), true
}

func (r *ProviderRegistry) setDefinition(key string, def *CapabilityProviderDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[key] = def
}

func (r *ProviderRegistry) delDefinition(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.definitions, key)
}

func (r *ProviderRegistry) getInstance(key string) (*CapabilityProviderInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.instances[key]
	if !ok {
		return nil, false
	}
	return cloneProviderInstance(v), true
}

func (r *ProviderRegistry) setInstance(key string, inst *CapabilityProviderInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances[key] = inst
}

func (r *ProviderRegistry) delInstance(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.instances, key)
}

func (r *ProviderRegistry) cloneDefinitions() map[string]*CapabilityProviderDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*CapabilityProviderDefinition, len(r.definitions))
	for k, v := range r.definitions {
		out[k] = v
	}
	return out
}

func (r *ProviderRegistry) cloneInstances() map[string]*CapabilityProviderInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*CapabilityProviderInstance, len(r.instances))
	for k, v := range r.instances {
		out[k] = v
	}
	return out
}

func (r *ProviderRegistry) cloneByProvider() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]string, len(r.byProvider))
	for k, v := range r.byProvider {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func (r *ProviderRegistry) cloneByCapability() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]string, len(r.byCapability))
	for k, v := range r.byCapability {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

type providerRegistrySnapshot struct {
	definitions  map[string]*CapabilityProviderDefinition
	instances    map[string]*CapabilityProviderInstance
	byProvider   map[string][]string
	byCapability map[string][]string
}

func (r *ProviderRegistry) snapshot() *providerRegistrySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap := &providerRegistrySnapshot{
		definitions:  make(map[string]*CapabilityProviderDefinition, len(r.definitions)),
		instances:    make(map[string]*CapabilityProviderInstance, len(r.instances)),
		byProvider:   make(map[string][]string, len(r.byProvider)),
		byCapability: make(map[string][]string, len(r.byCapability)),
	}
	for k, v := range r.definitions {
		snap.definitions[k] = v
	}
	for k, v := range r.instances {
		snap.instances[k] = v
	}
	for k, v := range r.byProvider {
		cp := make([]string, len(v))
		copy(cp, v)
		snap.byProvider[k] = cp
	}
	for k, v := range r.byCapability {
		cp := make([]string, len(v))
		copy(cp, v)
		snap.byCapability[k] = cp
	}
	return snap
}

func (r *ProviderRegistry) restoreSnapshot(snap *providerRegistrySnapshot) {
	if snap == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions = snap.definitions
	r.instances = snap.instances
	r.byProvider = snap.byProvider
	r.byCapability = snap.byCapability
}

func (r *ProviderRegistry) RegisterDefinition(def CapabilityProviderDefinition) error {
	def = def.Normalize()
	if err := def.Validate(); err != nil {
		return err
	}
	normalizedID := string(def.ID)

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, found := r.definitions[normalizedID]
	if found && existing.Placement != def.Placement {
		return ErrProviderPlacementMismatch
	}

	if found && existing.CapabilityID != def.CapabilityID {
		r.byCapability = removeFromSliceMap(r.byCapability, string(existing.CapabilityID), normalizedID)
	}

	if found {
		def.Revision = existing.Revision + 1
	} else {
		def.Revision = 1
	}
	r.definitions[normalizedID] = &def
	r.byCapability = appendToSliceMap(r.byCapability, string(def.CapabilityID), normalizedID)
	return nil
}

func (r *ProviderRegistry) DeregisterDefinition(id ProviderID) (bool, error) {
	normalized := ParseProviderID(string(id))
	if normalized == "" {
		return false, ErrProviderInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	def, existed := r.definitions[string(normalized)]
	if !existed {
		return false, nil
	}
	delete(r.definitions, string(normalized))
	r.byCapability = removeFromSliceMap(r.byCapability, string(def.CapabilityID), string(normalized))
	return true, nil
}

func (r *ProviderRegistry) DeregisterDefinitionCascade(id ProviderID) (bool, error) {
	normalized := ParseProviderID(string(id))
	if normalized == "" {
		return false, ErrProviderInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	def, existed := r.definitions[string(normalized)]
	if !existed {
		return false, nil
	}
	capID := string(def.CapabilityID)
	delete(r.definitions, string(normalized))
	r.byCapability = removeFromSliceMap(r.byCapability, capID, string(normalized))

	if instanceIDs, ok := r.byProvider[string(normalized)]; ok {
		for _, instID := range instanceIDs {
			delete(r.instances, instID)
		}
		delete(r.byProvider, string(normalized))
	}

	return true, nil
}

func (r *ProviderRegistry) RegisterInstance(inst CapabilityProviderInstance) error {
	inst = inst.Normalize()
	if err := inst.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	def, found := r.definitions[string(inst.ProviderID)]
	if !found {
		return ErrProviderInstanceInvalid
	}
	if def.CapabilityID != inst.CapabilityID {
		return ErrProviderCapabilityMismatch
	}
	if def.Placement != inst.Placement {
		return ErrProviderPlacementMismatch
	}
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now().UTC()
	}
	if inst.UpdatedAt.IsZero() {
		inst.UpdatedAt = inst.RegisteredAt
	}

	if existing, ok := r.instances[string(inst.ID)]; ok {
		inst.Revision = existing.Revision + 1
	} else {
		inst.Revision = 1
	}

	r.instances[string(inst.ID)] = &inst
	r.byProvider[string(inst.ProviderID)] = appendIDLocked(r.byProvider[string(inst.ProviderID)], string(inst.ID))
	return nil
}

func (r *ProviderRegistry) UpdateInstanceHealth(instanceID ProviderInstanceID, health HealthStatus) error {
	normalized := ParseProviderInstanceID(string(instanceID))
	if normalized == "" {
		return ErrProviderInstanceInvalid
	}
	if !health.IsValid() {
		return ErrProviderInstanceInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, found := r.instances[string(normalized)]
	if !found {
		return ErrProviderInstanceNotFound
	}
	if current.Health == health {
		return nil
	}

	updated := cloneProviderInstance(current)
	updated.Health = health
	updated.UpdatedAt = time.Now().UTC()
	updated.Revision = current.Revision + 1

	r.instances[string(updated.ID)] = updated
	return nil
}

func (r *ProviderRegistry) UpdateInstanceAvailability(instanceID ProviderInstanceID, availability ProviderAvailabilityState) error {
	normalized := ParseProviderInstanceID(string(instanceID))
	if normalized == "" {
		return ErrProviderInstanceInvalid
	}
	if !availability.IsValid() {
		return ErrProviderInstanceInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, found := r.instances[string(normalized)]
	if !found {
		return ErrProviderInstanceNotFound
	}
	if current.Availability == availability {
		return nil
	}

	updated := cloneProviderInstance(current)
	updated.Availability = availability
	updated.UpdatedAt = time.Now().UTC()
	updated.Revision = current.Revision + 1

	r.instances[string(updated.ID)] = updated
	return nil
}

func (r *ProviderRegistry) DeregisterInstance(instanceID ProviderInstanceID) (bool, error) {
	normalized := ParseProviderInstanceID(string(instanceID))
	if normalized == "" {
		return false, ErrProviderInstanceInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, found := r.instances[string(normalized)]
	if !found {
		return false, nil
	}
	providerID := current.ProviderID

	delete(r.instances, string(normalized))
	r.byProvider[string(providerID)] = removeIDLocked(r.byProvider[string(providerID)], string(normalized))
	if len(r.byProvider[string(providerID)]) == 0 {
		delete(r.byProvider, string(providerID))
	}
	return true, nil
}

func (r *ProviderRegistry) FlushInstancesByProvider(providerID ProviderID) (int, error) {
	normalized := ParseProviderID(string(providerID))
	if normalized == "" {
		return 0, ErrProviderInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.byProvider[string(normalized)]
	if len(ids) == 0 {
		return 0, nil
	}

	count := 0
	for _, id := range ids {
		if _, found := r.instances[id]; found {
			delete(r.instances, id)
			count++
		}
	}
	delete(r.byProvider, string(normalized))
	return count, nil
}

func (r *ProviderRegistry) FlushInstancesByCapability(capabilityID CapabilityID) (int, error) {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return 0, ErrProviderInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for id, inst := range r.instances {
		if inst == nil || inst.CapabilityID != normalized {
			continue
		}
		delete(r.instances, id)
		r.byProvider[string(inst.ProviderID)] = removeIDLocked(r.byProvider[string(inst.ProviderID)], id)
		if len(r.byProvider[string(inst.ProviderID)]) == 0 {
			delete(r.byProvider, string(inst.ProviderID))
		}
		count++
	}
	return count, nil
}

func (r *ProviderRegistry) FlushInstancesByPlacement(placement ProviderPlacement) (int, error) {
	if !placement.IsValid() {
		return 0, ErrProviderInstanceInvalid
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for id, inst := range r.instances {
		if inst == nil || inst.Placement != placement {
			continue
		}
		delete(r.instances, id)
		r.byProvider[string(inst.ProviderID)] = removeIDLocked(r.byProvider[string(inst.ProviderID)], id)
		if len(r.byProvider[string(inst.ProviderID)]) == 0 {
			delete(r.byProvider, string(inst.ProviderID))
		}
		count++
	}
	return count, nil
}

func (r *ProviderRegistry) FlushInstancesByType(kind ProviderKind) (int, error) {
	if !kind.IsValid() {
		return 0, ErrProviderInvalid
	}

	defs := r.cloneDefinitions()
	providerIDs := make([]ProviderID, 0)
	for _, def := range defs {
		if def == nil {
			continue
		}
		if def.Kind == kind {
			providerIDs = append(providerIDs, def.ID)
		}
	}

	total := 0
	for _, id := range providerIDs {
		count, err := r.FlushInstancesByProvider(id)
		if err != nil {
			return total, err
		}
		total += count
	}
	return total, nil
}

func (r *ProviderRegistry) DrainByProvider(providerID ProviderID) (int, error) {
	normalized := ParseProviderID(string(providerID))
	if normalized == "" {
		return 0, ErrProviderInvalid
	}

	r.mu.Lock()
	ids := r.byProvider[string(normalized)]
	r.mu.Unlock()

	count := 0
	for _, id := range ids {
		inst, found := r.getInstance(id)
		if !found {
			continue
		}
		if inst.Availability == ProviderAvailabilityDraining {
			continue
		}
		if err := r.UpdateInstanceAvailability(ProviderInstanceID(id), ProviderAvailabilityDraining); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (r *ProviderRegistry) DrainByCapability(capabilityID CapabilityID) (int, error) {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return 0, ErrProviderInvalid
	}

	defs := r.cloneDefinitions()
	providerIDs := make([]ProviderID, 0)
	for _, def := range defs {
		if def == nil {
			continue
		}
		if def.CapabilityID == normalized {
			providerIDs = append(providerIDs, def.ID)
		}
	}

	total := 0
	for _, id := range providerIDs {
		count, err := r.DrainByProvider(id)
		if err != nil {
			return total, err
		}
		total += count
	}
	return total, nil
}

func (r *ProviderRegistry) DrainByPlacement(placement ProviderPlacement) (int, error) {
	if !placement.IsValid() {
		return 0, ErrProviderInstanceInvalid
	}

	allInsts := r.cloneInstances()

	result := filterInstances(mapValues(allInsts), placementFilter(placement))
	count := 0
	for _, inst := range result {
		if inst == nil || inst.Availability == ProviderAvailabilityDraining {
			continue
		}
		if err := r.UpdateInstanceAvailability(inst.ID, ProviderAvailabilityDraining); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (r *ProviderRegistry) DrainByType(kind ProviderKind) (int, error) {
	if !kind.IsValid() {
		return 0, ErrProviderInvalid
	}

	defs := r.cloneDefinitions()
	providerIDs := make([]ProviderID, 0)
	for _, def := range defs {
		if def == nil {
			continue
		}
		if def.Kind == kind {
			providerIDs = append(providerIDs, def.ID)
		}
	}

	total := 0
	for _, id := range providerIDs {
		count, err := r.DrainByProvider(id)
		if err != nil {
			return total, err
		}
		total += count
	}
	return total, nil
}

func (r *ProviderRegistry) GetByID(providerID ProviderID) (*CapabilityProviderDefinition, bool) {
	normalized := ParseProviderID(string(providerID))
	if normalized == "" {
		return nil, false
	}
	return r.getDefinition(string(normalized))
}

func (r *ProviderRegistry) HasByID(providerID ProviderID) bool {
	normalized := ParseProviderID(string(providerID))
	if normalized == "" {
		return false
	}
	_, found := r.getDefinition(string(normalized))
	return found
}

func (r *ProviderRegistry) ListAllProviders() []*CapabilityProviderDefinition {
	defs := r.cloneDefinitions()
	result := make([]*CapabilityProviderDefinition, 0, len(defs))
	for _, def := range defs {
		if def != nil {
			result = append(result, def)
		}
	}
	sortDefs(result)
	return result
}

func (r *ProviderRegistry) ListByCapability(capabilityID CapabilityID) []*CapabilityProviderDefinition {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return nil
	}
	r.mu.RLock()
	ids := r.byCapability[string(normalized)]
	r.mu.RUnlock()
	if len(ids) == 0 {
		return nil
	}
	result := make([]*CapabilityProviderDefinition, 0, len(ids))
	for _, id := range ids {
		if def, ok := r.getDefinition(id); ok {
			result = append(result, def)
		}
	}
	sortDefs(result)
	return result
}

func (r *ProviderRegistry) ListByKind(kind ProviderKind) []*CapabilityProviderDefinition {
	if !kind.IsValid() {
		return nil
	}
	defs := r.cloneDefinitions()
	result := make([]*CapabilityProviderDefinition, 0)
	for _, def := range defs {
		if def != nil && def.Kind == kind {
			result = append(result, def)
		}
	}
	sortDefs(result)
	return result
}

func (r *ProviderRegistry) ListByExtension(extensionID string) []*CapabilityProviderDefinition {
	id := strings.TrimSpace(extensionID)
	if id == "" {
		return nil
	}
	defs := r.cloneDefinitions()
	result := make([]*CapabilityProviderDefinition, 0)
	for _, def := range defs {
		if def != nil && def.ExtensionID == id {
			result = append(result, def)
		}
	}
	sortDefs(result)
	return result
}

func (r *ProviderRegistry) ListByPlacement(placement ProviderPlacement) []*CapabilityProviderDefinition {
	if !placement.IsValid() {
		return nil
	}
	defs := r.cloneDefinitions()
	result := make([]*CapabilityProviderDefinition, 0)
	for _, def := range defs {
		if def != nil && def.Placement == placement {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (r *ProviderRegistry) ListByPlatform(platform runtimeidentity.Platform) []*CapabilityProviderDefinition {
	if !platform.IsKnown() {
		return nil
	}
	defs := r.cloneDefinitions()
	result := make([]*CapabilityProviderDefinition, 0)
	for _, def := range defs {
		if def != nil && matchPlatform(def.Platforms, platform) {
			result = append(result, def)
		}
	}
	sortDefs(result)
	return result
}

func (r *ProviderRegistry) LookupByCapability(capabilityID CapabilityID, placement FilterPlacement, platform runtimeidentity.Platform) []*CapabilityProviderDefinition {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return nil
	}

	candidates := r.ListByCapability(normalized)
	if len(candidates) == 0 {
		return nil
	}

	placementFiltered := placementFilterFn(placement)
	platformFiltered := platformFilterFn(platform)

	result := make([]*CapabilityProviderDefinition, 0, len(candidates))
	for _, def := range candidates {
		if def == nil {
			continue
		}
		if !placementFiltered(def) {
			continue
		}
		if !platformFiltered(def) {
			continue
		}
		result = append(result, def)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (r *ProviderRegistry) GetRuntimeBinding(def CapabilityProviderDefinition) (RuntimeBinding, error) {
	if err := def.Validate(); err != nil {
		return RuntimeBinding{}, ErrProviderInvalid
	}
	return def.Runtime, nil
}

func (r *ProviderRegistry) GetInstanceByID(instanceID ProviderInstanceID) (*CapabilityProviderInstance, bool) {
	normalized := ParseProviderInstanceID(string(instanceID))
	if normalized == "" {
		return nil, false
	}
	return r.getInstance(string(normalized))
}

func (r *ProviderRegistry) ListInstancesByProvider(providerID ProviderID) []*CapabilityProviderInstance {
	normalized := ParseProviderID(string(providerID))
	if normalized == "" {
		return nil
	}
	r.mu.RLock()
	ids := r.byProvider[string(normalized)]
	r.mu.RUnlock()
	if len(ids) == 0 {
		return nil
	}
	result := make([]*CapabilityProviderInstance, 0, len(ids))
	for _, id := range ids {
		if inst, ok := r.getInstance(id); ok {
			result = append(result, inst)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (r *ProviderRegistry) ListInstancesByCapability(capabilityID CapabilityID) []*CapabilityProviderInstance {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return nil
	}
	all := r.cloneInstances()
	return filterInstances(mapValues(all), capabilityFilter(normalized))
}

func (r *ProviderRegistry) ListInstancesByPlacement(placement ProviderPlacement) []*CapabilityProviderInstance {
	if !placement.IsValid() {
		return nil
	}
	all := r.cloneInstances()
	return filterInstances(mapValues(all), placementFilter(placement))
}

func (r *ProviderRegistry) ListInstancesByProviderCapability(providerID ProviderID, capabilityID CapabilityID) []*CapabilityProviderInstance {
	normalizedProvider := ParseProviderID(string(providerID))
	normalizedCap := ParseCapabilityID(string(capabilityID))
	if normalizedProvider == "" || normalizedCap == "" {
		return nil
	}
	all := r.cloneInstances()
	return filterInstances(mapValues(all), providerCapabilityFilter(normalizedProvider, normalizedCap))
}

func (r *ProviderRegistry) ListInstancesByOwner(owner Owner) []*CapabilityProviderInstance {
	if !owner.IsValid() {
		return nil
	}
	all := r.cloneInstances()
	return filterInstances(mapValues(all), ownerFilter(owner))
}

func (r *ProviderRegistry) ListExecutableInstances(capabilityID CapabilityID) []*CapabilityProviderInstance {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return nil
	}
	all := r.cloneInstances()
	return filterInstances(mapValues(all), executableFilter(), capabilityFilter(normalized))
}

func (r *ProviderRegistry) ResolveAvailableInstances(capabilityID CapabilityID, placement FilterPlacement, owner Owner, identity runtimeidentity.Identity) []*CapabilityProviderInstance {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return nil
	}

	candidates := r.ListExecutableInstances(normalized)
	if len(candidates) == 0 {
		return nil
	}

	placementFiltered := instancePlacementFilter(placement)
	ownerFiltered := ownerFilterFn(owner)
	identityFiltered := instanceIdentityFilter(identity)

	result := make([]*CapabilityProviderInstance, 0, len(candidates))
	for _, inst := range candidates {
		if inst == nil {
			continue
		}
		if !placementFiltered(inst) {
			continue
		}
		if !ownerFiltered(inst) {
			continue
		}
		if !identityFiltered(inst) {
			continue
		}
		result = append(result, inst)
	}

	return result
}

func (r *ProviderRegistry) CountProviders() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.definitions)
}

func (r *ProviderRegistry) CountInstances() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

func (r *ProviderRegistry) CountInstancesByProvider(providerID ProviderID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byProvider[string(ParseProviderID(string(providerID)))]
	return len(list)
}

func (r *ProviderRegistry) CountInstancesByCapability(capabilityID CapabilityID) int {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return 0
	}
	return len(r.ListInstancesByCapability(normalized))
}

func (r *ProviderRegistry) CountExecutableInstances(capabilityID CapabilityID) int {
	normalized := ParseCapabilityID(string(capabilityID))
	if normalized == "" {
		return 0
	}
	return len(r.ListExecutableInstances(normalized))
}

func (r *ProviderRegistry) AvailabilitySummary() map[ProviderAvailabilityState]int {
	all := r.cloneInstances()
	summary := make(map[ProviderAvailabilityState]int)
	for _, inst := range all {
		if inst == nil {
			continue
		}
		summary[inst.Availability]++
	}
	return summary
}

func (r *ProviderRegistry) AvailabilitySummaryByProvider(providerID ProviderID) map[ProviderAvailabilityState]int {
	instances := r.ListInstancesByProvider(providerID)
	summary := make(map[ProviderAvailabilityState]int)
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		summary[inst.Availability]++
	}
	return summary
}

func (r *ProviderRegistry) AvailabilitySummaryByCapability(capabilityID CapabilityID) map[ProviderAvailabilityState]int {
	instances := r.ListInstancesByCapability(capabilityID)
	summary := make(map[ProviderAvailabilityState]int)
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		summary[inst.Availability]++
	}
	return summary
}

func (r *ProviderRegistry) InstanceIDSet() map[ProviderInstanceID]struct{} {
	all := r.cloneInstances()
	result := make(map[ProviderInstanceID]struct{}, len(all))
	for _, inst := range all {
		if inst == nil {
			continue
		}
		result[inst.ID] = struct{}{}
	}
	return result
}

func (r *ProviderRegistry) InstanceIDSetByProvider(providerID ProviderID) map[ProviderInstanceID]struct{} {
	instances := r.ListInstancesByProvider(providerID)
	result := make(map[ProviderInstanceID]struct{}, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		result[inst.ID] = struct{}{}
	}
	return result
}

func (r *ProviderRegistry) InstanceIDSetByCapability(capabilityID CapabilityID) map[ProviderInstanceID]struct{} {
	instances := r.ListInstancesByCapability(capabilityID)
	result := make(map[ProviderInstanceID]struct{}, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		result[inst.ID] = struct{}{}
	}
	return result
}

func (r *ProviderRegistry) SnapshotDefinitions() []*CapabilityProviderDefinition {
	return r.ListAllProviders()
}

func (r *ProviderRegistry) SnapshotInstances() []*CapabilityProviderInstance {
	all := r.cloneInstances()
	return mapValues(all)
}

func (r *ProviderRegistry) SnapshotBindings() map[ProviderID]RuntimeBinding {
	defs := r.cloneDefinitions()
	out := make(map[ProviderID]RuntimeBinding, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		out[def.ID] = cloneRuntimeBinding(def.Runtime)
	}
	return out
}

func mapValues[T any](m map[string]T) []T {
	if len(m) == 0 {
		return nil
	}
	result := make([]T, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func executableFilter() func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		return inst.IsExecutable()
	}
}

func capabilityFilter(id CapabilityID) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		return inst.CapabilityID == id
	}
}

func placementFilter(placement ProviderPlacement) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		return inst.Placement == placement
	}
}

func providerCapabilityFilter(provider ProviderID, cap CapabilityID) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		return inst.ProviderID == provider && inst.CapabilityID == cap
	}
}

func ownerFilter(owner Owner) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		return inst.UserID == owner.UserID && inst.DeviceID == owner.DeviceID && inst.RuntimeID == owner.RuntimeID
	}
}

func ownerFilterFn(owner Owner) func(*CapabilityProviderInstance) bool {
	if !owner.IsValid() {
		return noopInstanceFilter()
	}
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		if owner.UserID != "" && inst.UserID != owner.UserID {
			return false
		}
		if owner.DeviceID != "" && inst.DeviceID != owner.DeviceID {
			return false
		}
		if owner.RuntimeID != "" && inst.RuntimeID != owner.RuntimeID {
			return false
		}
		return true
	}
}

func noopInstanceFilter() func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		return inst != nil
	}
}

func instanceIdentityFilter(identity runtimeidentity.Identity) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		if identity.UserID != "" && inst.UserID != identity.UserID {
			return false
		}
		if identity.DeviceID != "" && inst.DeviceID != identity.DeviceID {
			return false
		}
		if identity.RuntimeID != "" && inst.RuntimeID != identity.RuntimeID {
			return false
		}
		return true
	}
}

func placementFilterFn(placement FilterPlacement) func(*CapabilityProviderDefinition) bool {
	return func(def *CapabilityProviderDefinition) bool {
		if def == nil {
			return false
		}
		if placement == "" {
			return true
		}
		return string(def.Placement) == string(placement)
	}
}

func instancePlacementFilter(placement FilterPlacement) func(*CapabilityProviderInstance) bool {
	return func(inst *CapabilityProviderInstance) bool {
		if inst == nil {
			return false
		}
		if placement == "" {
			return true
		}
		return string(inst.Placement) == string(placement)
	}
}

func platformFilterFn(platform runtimeidentity.Platform) func(*CapabilityProviderDefinition) bool {
	return func(def *CapabilityProviderDefinition) bool {
		if def == nil {
			return false
		}
		return matchPlatform(def.Platforms, platform)
	}
}

func appendIDLocked(list []string, id string) []string {
	for _, existing := range list {
		if existing == id {
			return list
		}
	}
	return append(list, id)
}

func removeIDLocked(list []string, id string) []string {
	result := make([]string, 0, len(list))
	for _, existing := range list {
		if existing == id {
			continue
		}
		result = append(result, existing)
	}
	return result
}

func appendToSliceMap(m map[string][]string, key, id string) map[string][]string {
	for _, existing := range m[key] {
		if existing == id {
			return m
		}
	}
	m[key] = append(m[key], id)
	return m
}

func removeFromSliceMap(m map[string][]string, key, id string) map[string][]string {
	result := make([]string, 0, len(m[key]))
	for _, existing := range m[key] {
		if existing == id {
			continue
		}
		result = append(result, existing)
	}
	if len(result) == 0 {
		delete(m, key)
	} else {
		m[key] = result
	}
	return m
}
