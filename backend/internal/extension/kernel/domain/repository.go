package domain

import (
	"context"
	"sort"
	"sync"
	"time"
)

type InMemoryDefinitionRepository struct {
	mu      sync.RWMutex
	entries map[string]ExtensionDefinition
}

func NewInMemoryDefinitionRepository() *InMemoryDefinitionRepository {
	return &InMemoryDefinitionRepository{entries: make(map[string]ExtensionDefinition)}
}

func extensionKey(id ExtensionID, version SemanticVersion) string {
	return string(id) + "@" + version.String()
}

func (r *InMemoryDefinitionRepository) Put(_ context.Context, def ExtensionDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[extensionKey(def.ID, def.Version)] = def
	return nil
}

func (r *InMemoryDefinitionRepository) PutExtension(ctx context.Context, def ExtensionDefinition) error {
	return r.Put(ctx, def)
}

func (r *InMemoryDefinitionRepository) GetExtension(_ context.Context, id ExtensionID, version SemanticVersion) (ExtensionDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.entries[extensionKey(id, version)]
	if !ok {
		return ExtensionDefinition{}, ErrInvalidExtensionID
	}
	return def, nil
}

func (r *InMemoryDefinitionRepository) ListExtensions(_ context.Context) ([]ExtensionDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionDefinition, 0, len(r.entries))
	for _, def := range r.entries {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i].ID) < string(out[j].ID)
	})
	return out, nil
}

func (r *InMemoryDefinitionRepository) DeleteExtension(_ context.Context, id ExtensionID, version SemanticVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, extensionKey(id, version))
	return nil
}

var _ DefinitionRepository = (*InMemoryDefinitionRepository)(nil)

type InMemoryInstallationRepository struct {
	mu      sync.RWMutex
	entries map[string]ExtensionInstallation
}

func NewInMemoryInstallationRepository() *InMemoryInstallationRepository {
	return &InMemoryInstallationRepository{entries: make(map[string]ExtensionInstallation)}
}

func (r *InMemoryInstallationRepository) PutInstallation(_ context.Context, inst ExtensionInstallation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst.UpdatedAt.IsZero() {
		inst.UpdatedAt = time.Now().UTC()
	}
	r.entries[string(inst.ExtensionID)] = inst
	return nil
}

func (r *InMemoryInstallationRepository) GetInstallation(_ context.Context, id ExtensionID) (ExtensionInstallation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.entries[string(id)]
	if !ok {
		return ExtensionInstallation{}, ErrInvalidExtensionID
	}
	return inst, nil
}

func (r *InMemoryInstallationRepository) ListInstallations(_ context.Context) ([]ExtensionInstallation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionInstallation, 0, len(r.entries))
	for _, inst := range r.entries {
		out = append(out, inst)
	}
	return out, nil
}

func (r *InMemoryInstallationRepository) DeleteInstallation(_ context.Context, id ExtensionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, string(id))
	return nil
}

var _ InstallationRepository = (*InMemoryInstallationRepository)(nil)

type InMemoryPackageRepository struct {
	mu      sync.RWMutex
	entries map[string]ExtensionPackage
}

func NewInMemoryPackageRepository() *InMemoryPackageRepository {
	return &InMemoryPackageRepository{entries: make(map[string]ExtensionPackage)}
}

func (r *InMemoryPackageRepository) PutPackage(_ context.Context, pkg ExtensionPackage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[pkg.PackageID] = pkg
	return nil
}

func (r *InMemoryPackageRepository) GetPackage(_ context.Context, packageID string) (ExtensionPackage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pkg, ok := r.entries[packageID]
	if !ok {
		return ExtensionPackage{}, ErrInvalidExtensionID
	}
	return pkg, nil
}

func (r *InMemoryPackageRepository) ListPackages(_ context.Context, extensionID ExtensionID) ([]ExtensionPackage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ExtensionPackage
	for _, pkg := range r.entries {
		if pkg.ExtensionID == extensionID {
			out = append(out, pkg)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version.Compare(out[j].Version) > 0
	})
	return out, nil
}

var _ PackageRepository = (*InMemoryPackageRepository)(nil)

type InMemoryRuntimeRepository struct {
	mu      sync.RWMutex
	entries map[string]RuntimeInstance
}

func NewInMemoryRuntimeRepository() *InMemoryRuntimeRepository {
	return &InMemoryRuntimeRepository{entries: make(map[string]RuntimeInstance)}
}

func (r *InMemoryRuntimeRepository) PutInstance(_ context.Context, instance RuntimeInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[instance.InstanceID] = instance
	return nil
}

func (r *InMemoryRuntimeRepository) GetInstance(_ context.Context, instanceID string) (RuntimeInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.entries[instanceID]
	if !ok {
		return RuntimeInstance{}, ErrInvalidExtensionID
	}
	return inst, nil
}

func (r *InMemoryRuntimeRepository) ListInstances(_ context.Context, extensionID ExtensionID) ([]RuntimeInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []RuntimeInstance
	for _, inst := range r.entries {
		if inst.ExtensionID == extensionID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (r *InMemoryRuntimeRepository) DeleteInstance(_ context.Context, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, instanceID)
	return nil
}

var _ RuntimeRepository = (*InMemoryRuntimeRepository)(nil)
