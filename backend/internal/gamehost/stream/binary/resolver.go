package binary

import (
	"context"
	"errors"
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

	if request.Lifetime == "" {
		request.Lifetime = BinaryLifetimeMessage
	}
	if err := request.Lifetime.Validate(); err != nil {
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
		Size:      request.ExpectedSize,
		Lifetime:  request.Lifetime,
		MediaType: request.MediaType,
		Metadata:  request.Metadata,
	}

	if err := r.registry.InsertWriting(ctx, record); err != nil {
		if handle.Abort != nil {
			_ = handle.Abort()
		} else {
			_ = provider.Release(ctx, owner, handle.ObjectID)
		}
		return WritingHandle{}, err
	}

	originalSeal := handle.Seal
	handle.Seal = func(actualSize int64, checksum *Checksum) (BinaryReference, error) {
		ref, err := originalSeal(actualSize, checksum)
		if err != nil {
			return BinaryReference{}, err
		}
		if err := r.registry.SealObject(ctx, ref.ID, ref.Size, ref.Checksum); err != nil {
			// The provider has already sealed the object. If registry admission rejects
			// the final size, release both sides so a failed seal cannot leave an
			// unreachable provider object or a permanently-writing registry record.
			_ = provider.Release(ctx, owner, ref.ID)
			_ = r.registry.Release(ctx, ref.ID)
			return BinaryReference{}, err
		}
		return ref, nil
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
		record.Owner.ServiceID != consumer.ServiceID ||
		record.Owner.ChannelID != consumer.ChannelID {
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
	// Never pass caller-controlled metadata/checksum/media type to a provider.
	// The object registry is the canonical post-seal authority; callers may only
	// present the opaque id plus immutable routing fields validated above.
	canonical := BinaryReference{
		ID:        record.ID,
		Kind:      record.Kind,
		Size:      record.Size,
		MediaType: record.MediaType,
		Checksum:  record.Checksum,
		Lifetime:  record.Lifetime,
		Metadata:  record.Metadata,
	}

	return provider.Resolve(ctx, owner, canonical)
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

	provider, err := r.providers.Resolve(record.Kind)
	if err != nil {
		return err
	}
	if err := provider.Release(ctx, record.Owner, id); err != nil {
		return err
	}
	return r.registry.Release(ctx, id)
}

func (r *Resolver) ReleaseByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	records, err := r.registry.ListByRuntime(runtimeID)
	if err != nil {
		return err
	}
	var errs []error
	for _, record := range records {
		if provider, resolveErr := r.providers.Resolve(record.Kind); resolveErr == nil {
			if releaseErr := provider.Release(ctx, record.Owner, record.ID); releaseErr != nil {
				errs = append(errs, releaseErr)
			}
		} else {
			errs = append(errs, resolveErr)
		}
	}
	if _, removeErr := r.registry.RemoveByRuntime(ctx, runtimeID); removeErr != nil {
		errs = append(errs, removeErr)
	}
	return errors.Join(errs...)
}

func (r *Resolver) ReleaseByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	records, err := r.registry.ListByService(runtimeID, serviceID)
	if err != nil {
		return err
	}
	var errs []error
	for _, record := range records {
		if provider, resolveErr := r.providers.Resolve(record.Kind); resolveErr == nil {
			if releaseErr := provider.Release(ctx, record.Owner, record.ID); releaseErr != nil {
				errs = append(errs, releaseErr)
			}
		} else {
			errs = append(errs, resolveErr)
		}
	}
	if _, removeErr := r.registry.RemoveByService(ctx, runtimeID, serviceID); removeErr != nil {
		errs = append(errs, removeErr)
	}
	return errors.Join(errs...)
}

func (r *Resolver) Shutdown(ctx context.Context) error {
	return r.providers.Shutdown(ctx)
}
