package registry

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type RegistrationState string

const (
	RegistrationStateRegistered  RegistrationState = "registered"
	RegistrationStateActivating  RegistrationState = "activating"
	RegistrationStateActive      RegistrationState = "active"
	RegistrationStateDeactivating RegistrationState = "deactivating"
	RegistrationStateInactive    RegistrationState = "inactive"
	RegistrationStateUnregistered RegistrationState = "unregistered"
	RegistrationStateFailed      RegistrationState = "failed"
	RegistrationStateQuarantined RegistrationState = "quarantined"
)

type RegisteredContribution struct {
	Definition      domain.ContributionDefinition
	Generation      int64
	RegisteredAt    time.Time
	UpdatedAt       time.Time
	RegistrationState RegistrationState
	ActivationState string
	RuntimeBinding  *domain.RuntimeBinding
	Source          string
	Metadata        map[string]any
}

type ContributionRegistrationBatch struct {
	ExtensionID  domain.ExtensionID
	ModuleID     domain.ModuleID
	Generation   int64
	Contributions []domain.ContributionDefinition
	ReplaceExisting bool
	Source       string
}

type ContributionReplacementRequest struct {
	ExtensionID    domain.ExtensionID
	OldGeneration  int64
	NewGeneration  int64
	Contributions  []domain.ContributionDefinition
	Source         string
}

type ContributionUnregisterRequest struct {
	ExtensionID   domain.ExtensionID
	ModuleID      domain.ModuleID
	Contributions []domain.ContributionID
	Generation    int64
}

type ContributionRegistrationResult struct {
	Registered []RegisteredContribution
	Unregistered []RegisteredContribution
	Errors      []ContributionError
	Generation  int64
}

type ContributionError struct {
	ContributionID domain.ContributionID
	Code           string
	Message        string
}

type ContributionFilter struct {
	ExtensionID  domain.ExtensionID
	ModuleID     domain.ModuleID
	Kind         domain.ContributionKind
	State        RegistrationState
	ActiveOnly   bool
}

type ContributionRegistry interface {
	RegisterBatch(ctx context.Context, batch ContributionRegistrationBatch) ContributionRegistrationResult
	ReplaceGeneration(ctx context.Context, request ContributionReplacementRequest) ContributionRegistrationResult
	UnregisterBatch(ctx context.Context, request ContributionUnregisterRequest) ContributionRegistrationResult
	Get(ctx context.Context, id domain.ContributionID) (RegisteredContribution, error)
	List(ctx context.Context, filter ContributionFilter) ([]RegisteredContribution, error)
	Activate(ctx context.Context, id domain.ContributionID, binding domain.RuntimeBinding) error
	Deactivate(ctx context.Context, id domain.ContributionID) error
	Diff(ctx context.Context, oldGen, newGen int64) ContributionDiff
	Rebuild(ctx context.Context, contributions []RegisteredContribution) error
}

type ContributionDiff struct {
	Added    []RegisteredContribution
	Removed  []RegisteredContribution
	Modified []RegisteredContribution
}

type Adapter interface {
	Kind() domain.ContributionKind
	OnRegister(ctx context.Context, contribution RegisteredContribution) error
	OnActivate(ctx context.Context, contribution RegisteredContribution) error
	OnDeactivate(ctx context.Context, contribution RegisteredContribution) error
	OnUnregister(ctx context.Context, contribution RegisteredContribution) error
}

var (
	ErrContributionNotFound = errors.New("registry: contribution not found")
	ErrDuplicateContribution = errors.New("registry: duplicate contribution")
	ErrInvalidBatch         = errors.New("registry: invalid batch")
	ErrGenerationMismatch   = errors.New("registry: generation mismatch")
)

type DefaultRegistry struct {
	mu          sync.RWMutex
	entries     map[domain.ContributionID]RegisteredContribution
	byExt       map[domain.ExtensionID][]domain.ContributionID
	byModule    map[string][]domain.ContributionID
	byKind      map[domain.ContributionKind][]domain.ContributionID
	adapters    map[domain.ContributionKind]Adapter
	snapshots   map[int64]map[domain.ContributionID]RegisteredContribution
}

func NewDefaultRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		entries:   make(map[domain.ContributionID]RegisteredContribution),
		byExt:     make(map[domain.ExtensionID][]domain.ContributionID),
		byModule:  make(map[string][]domain.ContributionID),
		byKind:    make(map[domain.ContributionKind][]domain.ContributionID),
		adapters:  make(map[domain.ContributionKind]Adapter),
		snapshots: make(map[int64]map[domain.ContributionID]RegisteredContribution),
	}
}

func (r *DefaultRegistry) RegisterAdapter(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[a.Kind()] = a
}

func (r *DefaultRegistry) RegisterBatch(ctx context.Context, batch ContributionRegistrationBatch) ContributionRegistrationResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := ContributionRegistrationResult{Generation: batch.Generation}
	if batch.ExtensionID == "" {
		result.Errors = append(result.Errors, ContributionError{Code: "missing_extension_id", Message: "extension id required"})
		return result
	}
	for _, def := range batch.Contributions {
		if def.ID == "" {
			result.Errors = append(result.Errors, ContributionError{Code: "missing_id", Message: "contribution id required"})
			continue
		}
		if _, exists := r.entries[def.ID]; exists && !batch.ReplaceExisting {
			result.Errors = append(result.Errors, ContributionError{ContributionID: def.ID, Code: "duplicate", Message: "contribution already registered"})
			continue
		}
		now := time.Now().UTC()
		entry := RegisteredContribution{
			Definition:        def,
			Generation:        batch.Generation,
			RegisteredAt:      now,
			UpdatedAt:         now,
			RegistrationState: RegistrationStateRegistered,
			ActivationState:   "inactive",
			Source:            batch.Source,
			Metadata:          map[string]any{},
		}
		r.entries[def.ID] = entry
		r.recordSnapshot(batch.Generation, entry)
		r.indexAdd(def)
		adapter, ok := r.adapters[def.Kind]
		if ok {
			if err := adapter.OnRegister(ctx, entry); err != nil {
				result.Errors = append(result.Errors, ContributionError{ContributionID: def.ID, Code: "adapter_error", Message: err.Error()})
			}
		}
		result.Registered = append(result.Registered, entry)
	}
	return result
}

func (r *DefaultRegistry) ReplaceGeneration(ctx context.Context, request ContributionReplacementRequest) ContributionRegistrationResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := ContributionRegistrationResult{Generation: request.NewGeneration}
	var toRemove []domain.ContributionID
	for id, entry := range r.entries {
		if entry.Definition.ExtensionID == request.ExtensionID && entry.Generation == request.OldGeneration {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		old := r.entries[id]
		delete(r.entries, id)
		r.indexRemove(old.Definition)
		result.Unregistered = append(result.Unregistered, old)
		adapter, ok := r.adapters[old.Definition.Kind]
		if ok {
			_ = adapter.OnUnregister(ctx, old)
		}
	}
	for _, def := range request.Contributions {
		if def.ID == "" {
			continue
		}
		now := time.Now().UTC()
		entry := RegisteredContribution{
			Definition:        def,
			Generation:        request.NewGeneration,
			RegisteredAt:      now,
			UpdatedAt:         now,
			RegistrationState: RegistrationStateRegistered,
			ActivationState:   "inactive",
			Source:            request.Source,
			Metadata:          map[string]any{},
		}
		r.entries[def.ID] = entry
		r.recordSnapshot(request.NewGeneration, entry)
		r.indexAdd(def)
		adapter, ok := r.adapters[def.Kind]
		if ok {
			_ = adapter.OnRegister(ctx, entry)
		}
		result.Registered = append(result.Registered, entry)
	}
	return result
}

func (r *DefaultRegistry) UnregisterBatch(ctx context.Context, request ContributionUnregisterRequest) ContributionRegistrationResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := ContributionRegistrationResult{Generation: request.Generation}
	for _, id := range request.Contributions {
		entry, ok := r.entries[id]
		if !ok {
			result.Errors = append(result.Errors, ContributionError{ContributionID: id, Code: "not_found", Message: "contribution not found"})
			continue
		}
		adapter, ok := r.adapters[entry.Definition.Kind]
		if ok {
			_ = adapter.OnUnregister(ctx, entry)
		}
		delete(r.entries, id)
		r.indexRemove(entry.Definition)
		result.Unregistered = append(result.Unregistered, entry)
	}
	return result
}

func (r *DefaultRegistry) Get(_ context.Context, id domain.ContributionID) (RegisteredContribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[id]
	if !ok {
		return RegisteredContribution{}, ErrContributionNotFound
	}
	return entry, nil
}

func (r *DefaultRegistry) List(_ context.Context, filter ContributionFilter) ([]RegisteredContribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []RegisteredContribution
	for _, entry := range r.entries {
		if filter.ExtensionID != "" && entry.Definition.ExtensionID != filter.ExtensionID {
			continue
		}
		if filter.ModuleID != "" && entry.Definition.ModuleID != filter.ModuleID {
			continue
		}
		if filter.Kind != "" && entry.Definition.Kind != filter.Kind {
			continue
		}
		if filter.State != "" && entry.RegistrationState != filter.State {
			continue
		}
		if filter.ActiveOnly && entry.ActivationState != "active" {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i].Definition.ID) < string(out[j].Definition.ID)
	})
	return out, nil
}

func (r *DefaultRegistry) Activate(ctx context.Context, id domain.ContributionID, binding domain.RuntimeBinding) error {
	r.mu.Lock()
	entry, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return ErrContributionNotFound
	}
	entry.ActivationState = "active"
	entry.RuntimeBinding = &binding
	entry.RegistrationState = RegistrationStateActive
	entry.UpdatedAt = time.Now().UTC()
	r.entries[id] = entry
	adapter, hasAdapter := r.adapters[entry.Definition.Kind]
	r.mu.Unlock()
	if hasAdapter {
		return adapter.OnActivate(ctx, entry)
	}
	return nil
}

func (r *DefaultRegistry) Deactivate(ctx context.Context, id domain.ContributionID) error {
	r.mu.Lock()
	entry, ok := r.entries[id]
	if !ok {
		r.mu.Unlock()
		return ErrContributionNotFound
	}
	entry.ActivationState = "inactive"
	entry.RuntimeBinding = nil
	entry.RegistrationState = RegistrationStateInactive
	entry.UpdatedAt = time.Now().UTC()
	r.entries[id] = entry
	adapter, hasAdapter := r.adapters[entry.Definition.Kind]
	r.mu.Unlock()
	if hasAdapter {
		return adapter.OnDeactivate(ctx, entry)
	}
	return nil
}

func (r *DefaultRegistry) Diff(_ context.Context, oldGen, newGen int64) ContributionDiff {
	r.mu.RLock()
	defer r.mu.RUnlock()
	diff := ContributionDiff{}
	oldMap := r.snapshot(oldGen)
	newMap := r.snapshot(newGen)
	for id, entry := range newMap {
		old, ok := oldMap[id]
		if !ok {
			diff.Added = append(diff.Added, entry)
			continue
		}
		if !definitionEqual(old.Definition, entry.Definition) {
			diff.Modified = append(diff.Modified, entry)
		}
	}
	for id, entry := range oldMap {
		if _, ok := newMap[id]; !ok {
			diff.Removed = append(diff.Removed, entry)
		}
	}
	sort.Slice(diff.Added, func(i, j int) bool {
		return string(diff.Added[i].Definition.ID) < string(diff.Added[j].Definition.ID)
	})
	sort.Slice(diff.Removed, func(i, j int) bool {
		return string(diff.Removed[i].Definition.ID) < string(diff.Removed[j].Definition.ID)
	})
	sort.Slice(diff.Modified, func(i, j int) bool {
		return string(diff.Modified[i].Definition.ID) < string(diff.Modified[j].Definition.ID)
	})
	return diff
}

func (r *DefaultRegistry) snapshot(gen int64) map[domain.ContributionID]RegisteredContribution {
	snap, ok := r.snapshots[gen]
	if !ok {
		return map[domain.ContributionID]RegisteredContribution{}
	}
	out := make(map[domain.ContributionID]RegisteredContribution, len(snap))
	for k, v := range snap {
		out[k] = v
	}
	return out
}

func (r *DefaultRegistry) recordSnapshot(gen int64, entry RegisteredContribution) {
	snap, ok := r.snapshots[gen]
	if !ok {
		snap = make(map[domain.ContributionID]RegisteredContribution)
		r.snapshots[gen] = snap
	}
	snap[entry.Definition.ID] = entry
}

func definitionEqual(a, b domain.ContributionDefinition) bool {
	if a.ID != b.ID || a.Kind != b.Kind || a.ExtensionID != b.ExtensionID || a.ModuleID != b.ModuleID {
		return false
	}
	return true
}

func (r *DefaultRegistry) Rebuild(_ context.Context, contributions []RegisteredContribution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = make(map[domain.ContributionID]RegisteredContribution)
	r.byExt = make(map[domain.ExtensionID][]domain.ContributionID)
	r.byModule = make(map[string][]domain.ContributionID)
	r.byKind = make(map[domain.ContributionKind][]domain.ContributionID)
	r.snapshots = make(map[int64]map[domain.ContributionID]RegisteredContribution)
	for _, c := range contributions {
		r.entries[c.Definition.ID] = c
		r.recordSnapshot(c.Generation, c)
		r.indexAdd(c.Definition)
	}
	return nil
}

func (r *DefaultRegistry) indexAdd(def domain.ContributionDefinition) {
	r.byExt[def.ExtensionID] = appendIfMissing(r.byExt[def.ExtensionID], def.ID)
	key := moduleKey(def.ExtensionID, def.ModuleID)
	r.byModule[key] = appendIfMissing(r.byModule[key], def.ID)
	r.byKind[def.Kind] = appendIfMissing(r.byKind[def.Kind], def.ID)
}

func (r *DefaultRegistry) indexRemove(def domain.ContributionDefinition) {
	r.byExt[def.ExtensionID] = removeIfPresent(r.byExt[def.ExtensionID], def.ID)
	key := moduleKey(def.ExtensionID, def.ModuleID)
	r.byModule[key] = removeIfPresent(r.byModule[key], def.ID)
	r.byKind[def.Kind] = removeIfPresent(r.byKind[def.Kind], def.ID)
}

func moduleKey(extID domain.ExtensionID, modID domain.ModuleID) string {
	return fmt.Sprintf("%s/%s", extID, modID)
}

func appendIfMissing(slice []domain.ContributionID, id domain.ContributionID) []domain.ContributionID {
	for _, s := range slice {
		if s == id {
			return slice
		}
	}
	return append(slice, id)
}

func removeIfPresent(slice []domain.ContributionID, id domain.ContributionID) []domain.ContributionID {
	for i, s := range slice {
		if s == id {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

var _ ContributionRegistry = (*DefaultRegistry)(nil)
