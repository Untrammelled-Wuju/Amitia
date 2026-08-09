package binary

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type providerEntry struct {
	kind     BinaryStorageKind
	provider BinaryProvider
}

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[BinaryStorageKind]BinaryProvider
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[BinaryStorageKind]BinaryProvider),
	}
}

func (r *ProviderRegistry) Register(provider BinaryProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Kind()] = provider
}

func (r *ProviderRegistry) Resolve(kind BinaryStorageKind) (BinaryProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[kind]
	if !ok {
		return nil, fmt.Errorf("binary: no provider registered for kind %s", kind)
	}
	return provider, nil
}

func (r *ProviderRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	providers := make([]BinaryProvider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.Unlock()

	var lastErr error
	for _, p := range providers {
		if err := p.Shutdown(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type Resolver struct {
	registry  ObjectRegistry
	providers *ProviderRegistry
}

func NewResolver(registry ObjectRegistry, providers *ProviderRegistry) *Resolver {
	return &Resolver{
		registry:  registry,
		providers: providers,
	}
}

func (r *Resolver) Create(
	ctx context.Context,
	owner BinaryOwner,
	kind BinaryStorageKind,
	request CreateRequest,
) (WritingHandle, error) {
	if err := owner.Validate(); err != nil {
		return WritingHandle{}, err
	}
	if err := kind.Validate(); err != nil {
		return WritingHandle{}, err
	}

	provider, err := r.providers.Resolve(kind)
	if err != nil {
		return WritingHandle{}, err
	}

	handle, err := provider.Create(ctx, owner, request)
	if err != nil {
		return WritingHandle{}, err
	}

	record := BinaryObjectRecord{
		ID:        handle.ObjectID,
		Kind:      kind,
		Owner:     owner,
		Lifetime:  BinaryLifetimeMessage,
		MediaType: request.MediaType,
		Metadata:  request.Metadata,
	}

	if err := r.registry.InsertWriting(ctx, record); err != nil {
		provider.Release(ctx, owner, handle.ObjectID)
		return WritingHandle{}, err
	}

	return handle, nil
}

func (r *Resolver) Resolve(
	ctx context.Context,
	consumer BinaryOwner,
	ref BinaryReference,
) (ResolvedBinary, error) {
	if err := ref.Validate(); err != nil {
		return ResolvedBinary{}, err
	}

	record, err := r.registry.Get(ctx, ref.ID)
	if err != nil {
		return ResolvedBinary{}, err
	}

	if record.State != ObjectStateReady {
		return ResolvedBinary{}, ErrObjectNotReady
	}

	if record.Owner.PluginID != consumer.PluginID ||
		record.Owner.RuntimeID != consumer.RuntimeID ||
		record.Owner.ServiceID != consumer.ServiceID {
		return ResolvedBinary{}, ErrOwnerMismatch
	}

	if record.Kind != ref.Kind {
		return ResolvedBinary{}, ErrKindMismatch
	}

	if record.Size != ref.Size {
		return ResolvedBinary{}, ErrSizeMismatch
	}

	if record.Lifetime != ref.Lifetime {
		return ResolvedBinary{}, ErrLifetimeMismatch
	}

	provider, err := r.providers.Resolve(ref.Kind)
	if err != nil {
		return ResolvedBinary{}, err
	}

	owner := BinaryOwner{
		PluginID:  record.Owner.PluginID,
		RuntimeID: record.Owner.RuntimeID,
		ServiceID: record.Owner.ServiceID,
		ChannelID: record.Owner.ChannelID,
	}

	return provider.Resolve(ctx, owner, ref)
}

func (r *Resolver) Release(
	ctx context.Context,
	owner BinaryOwner,
	id BinaryObjectID,
) error {
	record, err := r.registry.Get(ctx, id)
	if err != nil {
		return err
	}

	if record.Owner.PluginID != owner.PluginID ||
		record.Owner.RuntimeID != owner.RuntimeID ||
		record.Owner.ServiceID != owner.ServiceID {
		return ErrOwnerMismatch
	}

	if err := r.registry.Release(ctx, id); err != nil {
		return err
	}

	provider, err := r.providers.Resolve(record.Kind)
	if err != nil {
		return err
	}

	return provider.Release(ctx, record.Owner, id)
}

func (r *Resolver) ReleaseByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	_, err := r.registry.RemoveByRuntime(ctx, runtimeID)
	return err
}

func (r *Resolver) ReleaseByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	_, err := r.registry.RemoveByService(ctx, runtimeID, serviceID)
	return err
}

func (r *Resolver) Shutdown(ctx context.Context) error {
	return r.providers.Shutdown(ctx)
}
