package resource

import "context"

type StorageBackend interface {
	ResourceStore
	ReferenceStore
	TransferStore
	ReleasePlanStore
	CleanupJobStore
	OrphanReportStore
}

type ResourceStore interface {
	SaveResource(ctx context.Context, rec ResourceRecord) error
	UpdateResource(ctx context.Context, resourceID string, resource ResourceRecord) error
	GetResource(ctx context.Context, resourceID string) (*ResourceRecord, error)
	ListResourcesByOwner(ctx context.Context, owner ResourceOwner) ([]ResourceRecord, error)
	ListResourcesByType(ctx context.Context, resourceType ResourceType) ([]ResourceRecord, error)
	UpdateResourceState(ctx context.Context, resourceID string, state ResourceState) error
	DeleteResource(ctx context.Context, resourceID string) error
}

type ReferenceStore interface {
	SaveReference(ctx context.Context, ref ResourceReference) error
	GetReference(ctx context.Context, referenceID string) (*ResourceReference, error)
	DeleteReference(ctx context.Context, referenceID string) error
	ListReferencesBySource(ctx context.Context, sourceResourceID string) ([]ResourceReference, error)
	ListReferencesByTarget(ctx context.Context, targetResourceID string) ([]ResourceReference, error)
	ListAllReferences(ctx context.Context, resourceID string) ([]ResourceReference, error)
}

type TransferStore interface {
	SaveTransfer(ctx context.Context, record OwnershipTransferRecord) error
	GetTransfer(ctx context.Context, transferID string) (*OwnershipTransferRecord, error)
	ListTransfersByResource(ctx context.Context, resourceID string) ([]OwnershipTransferRecord, error)
}

type ReleasePlanStore interface {
	SaveReleasePlan(ctx context.Context, plan ResourceReleasePlan) error
	GetReleasePlan(ctx context.Context, planID string) (*ResourceReleasePlan, error)
	DeleteReleasePlan(ctx context.Context, planID string) error
	ListReleasePlansByResource(ctx context.Context, resourceID string) ([]ResourceReleasePlan, error)
}

type CleanupJobStore interface {
	SaveCleanupJob(ctx context.Context, job CleanupJob) error
	GetCleanupJob(ctx context.Context, jobID string) (*CleanupJob, error)
	ListPendingCleanupJobs(ctx context.Context) ([]CleanupJob, error)
	UpdateCleanupJobStatus(ctx context.Context, jobID string, status string) error
	DeleteCleanupJob(ctx context.Context, jobID string) error
}

type OrphanReportStore interface {
	SaveOrphanReport(ctx context.Context, report OrphanReport) error
	GetOrphanReport(ctx context.Context, reportID string) (*OrphanReport, error)
}
