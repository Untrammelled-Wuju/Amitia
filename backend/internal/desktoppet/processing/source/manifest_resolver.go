package source

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/processing/contracts"
)

type ManifestSourceResolver interface {
	Resolve(ctx context.Context, manifestID string, userID string) (*ProcessingSourceDescriptor, error)
}

type ManifestSourceResolverImpl struct {
	store     ManifestStore
	validator UnifiedSourceValidator
}

func NewManifestSourceResolver(store ManifestStore, validator UnifiedSourceValidator) *ManifestSourceResolverImpl {
	return &ManifestSourceResolverImpl{
		store:     store,
		validator: validator,
	}
}

func (r *ManifestSourceResolverImpl) Resolve(ctx context.Context, manifestID string, userID string) (*ProcessingSourceDescriptor, error) {
	manifest, err := r.store.GetByID(ctx, manifestID)
	if err != nil {
		return nil, contracts.NewProcessingError(
			contracts.ErrCodeSourceManifestMissing,
			fmt.Sprintf("get manifest %s", manifestID),
			contracts.WithCause(err),
		)
	}
	if manifest == nil {
		return nil, contracts.NewProcessingError(
			contracts.ErrCodeSourceManifestMissing,
			fmt.Sprintf("manifest %s not found", manifestID),
		)
	}

	if !manifest.VerifyHash() {
		return nil, contracts.NewProcessingError(
			contracts.ErrCodeSourceManifestHashMismatch,
			fmt.Sprintf("manifest hash mismatch for %s", manifestID),
		)
	}

	if err := r.validator.Validate(ctx, manifest, userID); err != nil {
		return nil, err
	}

	descriptor, err := manifest.ToDescriptor()
	if err != nil {
		return nil, contracts.NewProcessingError(
			ErrCodeProcessingSourceInvalid,
			fmt.Sprintf("manifest %s to descriptor", manifestID),
			contracts.WithCause(err),
		)
	}

	return descriptor, nil
}
