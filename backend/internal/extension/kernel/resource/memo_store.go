package resource

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

var _ StorageBackend = (*MemoryStore)(nil)

type MemoryStore struct {
	mu            sync.RWMutex
	resources     map[string]ResourceRecord
	references    map[string]ResourceReference
	transfers     map[string]OwnershipTransferRecord
	releasePlans  map[string]ResourceReleasePlan
	cleanupJobs   map[string]CleanupJob
	orphanReports map[string]OrphanReport
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		resources:     make(map[string]ResourceRecord),
		references:    make(map[string]ResourceReference),
		transfers:     make(map[string]OwnershipTransferRecord),
		releasePlans:  make(map[string]ResourceReleasePlan),
		cleanupJobs:   make(map[string]CleanupJob),
		orphanReports: make(map[string]OrphanReport),
	}
}

func (s *MemoryStore) SaveResource(ctx context.Context, rec ResourceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.resources[rec.ResourceID]; ok {
		return ErrResourceAlreadyExists
	}
	s.resources[rec.ResourceID] = rec
	return nil
}

func (s *MemoryStore) UpdateResource(ctx context.Context, resourceID string, resource ResourceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.resources[resourceID]; !ok {
		return ErrResourceNotFound
	}
	s.resources[resourceID] = resource
	return nil
}

func (s *MemoryStore) GetResource(ctx context.Context, resourceID string) (*ResourceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.resources[resourceID]
	if !ok {
		return nil, ErrResourceNotFound
	}
	return &rec, nil
}

func (s *MemoryStore) ListResourcesByOwner(ctx context.Context, owner ResourceOwner) ([]ResourceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceRecord
	for _, rec := range s.resources {
		if rec.Owner.Equals(owner) {
			results = append(results, rec)
		}
	}
	return results, nil
}

func (s *MemoryStore) ListResourcesByType(ctx context.Context, resourceType ResourceType) ([]ResourceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceRecord
	for _, rec := range s.resources {
		if resourceType == "" || rec.ResourceType == resourceType {
			results = append(results, rec)
		}
	}
	return results, nil
}

func (s *MemoryStore) UpdateResourceState(ctx context.Context, resourceID string, state ResourceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.resources[resourceID]
	if !ok {
		return ErrResourceNotFound
	}

	if !IsValidStateTransition(rec.State, state) {
		return ErrInvalidStateTransition
	}

	rec.State = state
	rec.UpdatedAt = time.Now()
	s.resources[resourceID] = rec
	return nil
}

func (s *MemoryStore) DeleteResource(ctx context.Context, resourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.resources[resourceID]; !ok {
		return ErrResourceNotFound
	}
	delete(s.resources, resourceID)
	return nil
}

func (s *MemoryStore) SaveReference(ctx context.Context, ref ResourceReference) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.references[ref.ReferenceID]; ok {
		return ErrDuplicateReference
	}

	if ref.SourceResourceID == ref.TargetResourceID {
		return ErrCircularReference
	}

	s.references[ref.ReferenceID] = ref
	return nil
}

func (s *MemoryStore) GetReference(ctx context.Context, referenceID string) (*ResourceReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ref, ok := s.references[referenceID]
	if !ok {
		return nil, ErrReferenceNotFound
	}
	return &ref, nil
}

func (s *MemoryStore) DeleteReference(ctx context.Context, referenceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.references[referenceID]; !ok {
		return ErrReferenceNotFound
	}
	delete(s.references, referenceID)
	return nil
}

func (s *MemoryStore) ListReferencesBySource(ctx context.Context, sourceResourceID string) ([]ResourceReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceReference
	for _, ref := range s.references {
		if ref.SourceResourceID == sourceResourceID {
			results = append(results, ref)
		}
	}
	return results, nil
}

func (s *MemoryStore) ListReferencesByTarget(ctx context.Context, targetResourceID string) ([]ResourceReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceReference
	for _, ref := range s.references {
		if ref.TargetResourceID == targetResourceID {
			results = append(results, ref)
		}
	}
	return results, nil
}

func (s *MemoryStore) ListAllReferences(ctx context.Context, resourceID string) ([]ResourceReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceReference
	for _, ref := range s.references {
		if ref.SourceResourceID == resourceID || ref.TargetResourceID == resourceID {
			results = append(results, ref)
		}
	}
	return results, nil
}

func (s *MemoryStore) SaveTransfer(ctx context.Context, record OwnershipTransferRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transfers[record.TransferID] = record
	return nil
}

func (s *MemoryStore) GetTransfer(ctx context.Context, transferID string) (*OwnershipTransferRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.transfers[transferID]
	if !ok {
		return nil, ErrResourceNotFound
	}
	return &rec, nil
}

func (s *MemoryStore) ListTransfersByResource(ctx context.Context, resourceID string) ([]OwnershipTransferRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []OwnershipTransferRecord
	for _, rec := range s.transfers {
		if rec.ResourceID == resourceID {
			results = append(results, rec)
		}
	}
	return results, nil
}

func (s *MemoryStore) SaveReleasePlan(ctx context.Context, plan ResourceReleasePlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.releasePlans[plan.PlanID] = plan
	return nil
}

func (s *MemoryStore) GetReleasePlan(ctx context.Context, planID string) (*ResourceReleasePlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plan, ok := s.releasePlans[planID]
	if !ok {
		return nil, ErrReleasePlanNotFound
	}
	return &plan, nil
}

func (s *MemoryStore) DeleteReleasePlan(ctx context.Context, planID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.releasePlans[planID]; !ok {
		return ErrReleasePlanNotFound
	}
	delete(s.releasePlans, planID)
	return nil
}

func (s *MemoryStore) ListReleasePlansByResource(ctx context.Context, resourceID string) ([]ResourceReleasePlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []ResourceReleasePlan
	for _, plan := range s.releasePlans {
		if plan.RootResourceID == resourceID {
			results = append(results, plan)
		}
	}
	return results, nil
}

func (s *MemoryStore) SaveCleanupJob(ctx context.Context, job CleanupJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupJobs[job.JobID] = job
	return nil
}

func (s *MemoryStore) GetCleanupJob(ctx context.Context, jobID string) (*CleanupJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return nil, ErrCleanupJobNotFound
	}
	return &job, nil
}

func (s *MemoryStore) ListPendingCleanupJobs(ctx context.Context) ([]CleanupJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []CleanupJob
	for _, job := range s.cleanupJobs {
		if job.Status == CleanupJobStatusPending || job.Status == CleanupJobStatusFailed {
			if job.MaxRetries > 0 && job.Retries >= job.MaxRetries {
				continue
			}
			results = append(results, job)
		}
	}
	return results, nil
}

func (s *MemoryStore) UpdateCleanupJobStatus(ctx context.Context, jobID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return ErrCleanupJobNotFound
	}

	job.Status = status
	if status == CleanupJobStatusRunning && job.StartedAt == nil {
		t := time.Now()
		job.StartedAt = &t
	}
	if status == CleanupJobStatusCompleted || status == CleanupJobStatusFailed {
		t := time.Now()
		job.FinishedAt = &t
		if status == CleanupJobStatusFailed {
			job.Retries++
		}
	}
	s.cleanupJobs[jobID] = job
	return nil
}

func (s *MemoryStore) DeleteCleanupJob(ctx context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.cleanupJobs[jobID]; !ok {
		return ErrCleanupJobNotFound
	}
	delete(s.cleanupJobs, jobID)
	return nil
}

func (s *MemoryStore) SaveOrphanReport(ctx context.Context, report OrphanReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.orphanReports[report.ReportID] = report
	return nil
}

func (s *MemoryStore) GetOrphanReport(ctx context.Context, reportID string) (*OrphanReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report, ok := s.orphanReports[reportID]
	if !ok {
		return nil, ErrOrphanReportNotFound
	}
	return &report, nil
}

func NewResourceID() string {
	return "res_" + uuid.NewString()
}

func NewReferenceID() string {
	return "ref_" + uuid.NewString()
}

func NewTransferID() string {
	return "trf_" + uuid.NewString()
}

func NewPlanID() string {
	return "plan_" + uuid.NewString()
}

func NewJobID() string {
	return "job_" + uuid.NewString()
}

func NewReportID() string {
	return "rpt_" + uuid.NewString()
}
