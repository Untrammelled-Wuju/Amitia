package resource

import "context"

type ResourceOwnershipService interface {
	Register(ctx context.Context, resource *ResourceRecord) error
	AddReference(ctx context.Context, ref ResourceReference) error
	RemoveReference(ctx context.Context, referenceID string) error
	TransferOwnership(ctx context.Context, request OwnershipTransferRequest) error
	PlanRelease(ctx context.Context, request ResourceReleaseRequest) (*ResourceReleasePlan, error)
	ExecuteRelease(ctx context.Context, plan *ResourceReleasePlan) (*ResourceReleaseResult, error)
	ListOwned(ctx context.Context, owner ResourceOwner) ([]ResourceRecord, error)
	ListReferences(ctx context.Context, resourceID string) ([]ResourceReference, error)
	GetResource(ctx context.Context, resourceID string) (*ResourceRecord, error)
	UpdateState(ctx context.Context, resourceID string, state ResourceState) error
	ScanOrphans(ctx context.Context) (*OrphanReport, error)
}
