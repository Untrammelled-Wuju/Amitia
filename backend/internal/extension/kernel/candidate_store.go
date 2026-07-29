package kernel

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/update"
)

type CandidateKey struct {
	ExtensionID         string
	ModuleID            string
	CandidateGeneration int64
	ContributionID      string
	DefinitionHash      string
}

func (k CandidateKey) String() string {
	return fmt.Sprintf("%s:%s:%d:%s:%s", k.ExtensionID, k.ModuleID, k.CandidateGeneration, k.ContributionID, k.DefinitionHash)
}

type CandidateStatus string

const (
	CandidateStatusRegistered CandidateStatus = "registered"
	CandidateStatusPromoting  CandidateStatus = "promoting"
	CandidateStatusPromoted   CandidateStatus = "promoted"
	CandidateStatusFailed     CandidateStatus = "failed"
)

type CandidateRecord struct {
	CandidateID             string
	ExtensionID             domain.ExtensionID
	InstanceIDs             []string
	GenerationID            string
	CandidateGeneration     int64
	ExpectedStableGeneration int64
	Contribs                []domain.ContributionDefinition
	ScheduleIDs             []string
	ArtifactPath            string
	DefinitionHash           string
	Status                  CandidateStatus
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type StableSnapshot struct {
	Contributions   []domain.ContributionDefinition
	GenerationID    string
	Generation      int64
	DefinitionHash  string
	EnablementState domain.EnablementState
	CapturedAt      time.Time
}

func (r *CandidateRecord) CandidateKeys() []CandidateKey {
	keys := make([]CandidateKey, 0, len(r.Contribs))
	for _, contrib := range r.Contribs {
		keys = append(keys, CandidateKey{
			ExtensionID:         string(contrib.ExtensionID),
			ModuleID:            string(contrib.ModuleID),
			CandidateGeneration: r.CandidateGeneration,
			ContributionID:      string(contrib.ID),
			DefinitionHash:      r.DefinitionHash,
		})
	}
	return keys
}

type CandidateContributionManager struct {
	installer     *TypedContributionInstaller
	generationMgr *update.GenerationManager
	supervisor    runtime_supervisor.Supervisor
	repo          *CandidateRepository
	namespace     *CandidateNamespace

	mu              sync.RWMutex
	candidates      map[string]*CandidateRecord
	promotedRecords map[string]*CandidateRecord
	snapshots       map[string]*StableSnapshot
}

func NewCandidateContributionManager(
	installer *TypedContributionInstaller,
	generationMgr *update.GenerationManager,
	supervisor runtime_supervisor.Supervisor,
	repo *CandidateRepository,
	namespace *CandidateNamespace,
) *CandidateContributionManager {
	if namespace == nil {
		namespace = NewCandidateNamespace()
	}
	return &CandidateContributionManager{
		installer:       installer,
		generationMgr:   generationMgr,
		supervisor:      supervisor,
		repo:            repo,
		namespace:       namespace,
		candidates:      make(map[string]*CandidateRecord),
		promotedRecords: make(map[string]*CandidateRecord),
		snapshots:       make(map[string]*StableSnapshot),
	}
}

func (m *CandidateContributionManager) RegisterCandidate(ctx context.Context, record *CandidateRecord) error {
	if m == nil {
		return fmt.Errorf("candidate-manager: not initialized")
	}
	if record.CandidateID == "" {
		return fmt.Errorf("candidate-manager: candidateID is required")
	}
	now := time.Now().UTC()
	record.Status = CandidateStatusRegistered
	record.CreatedAt = now
	record.UpdatedAt = now

	if err := m.installer.RegisterCandidateContributions(
		ctx,
		record.CandidateID,
		record.ExtensionID,
		record.InstanceIDs,
		record.GenerationID,
		record.CandidateGeneration,
		record.Contribs,
		record.DefinitionHash,
		record.ArtifactPath,
	); err != nil {
		return fmt.Errorf("candidate-manager: register candidate contributions in namespace: %w", err)
	}

	if m.repo != nil {
		if err := m.repo.Save(ctx, record); err != nil {
			m.namespace.Remove(record.CandidateID)
			return fmt.Errorf("candidate-manager: persist candidate %s: %w", record.CandidateID, err)
		}
	}

	m.mu.Lock()
	m.candidates[record.CandidateID] = record
	m.mu.Unlock()

	log.Printf("[candidate-manager] registered candidate %s for extension %s (generation=%d, contribs=%d) in isolated namespace",
		record.CandidateID, record.ExtensionID, record.CandidateGeneration, len(record.Contribs))
	return nil
}

func (m *CandidateContributionManager) HealthCandidate(ctx context.Context, candidateID string) error {
	if m == nil {
		return fmt.Errorf("candidate-manager: not initialized")
	}

	if err := m.installer.ValidateCandidateContributions(ctx, candidateID); err != nil {
		return fmt.Errorf("candidate-manager: validate candidate contributions in namespace: %w", err)
	}

	m.mu.RLock()
	record, ok := m.candidates[candidateID]
	m.mu.RUnlock()
	if !ok {
		if promoted, pok := m.promotedRecords[candidateID]; pok {
			record = promoted
		} else {
			return fmt.Errorf("candidate-manager: candidate %s not found", candidateID)
		}
	}

	for _, instID := range record.InstanceIDs {
		snap, err := m.supervisor.GetInstance(ctx, instID)
		if err != nil {
			return fmt.Errorf("candidate-manager: get instance %s: %w", instID, err)
		}
		if snap.Health != runtime_supervisor.HealthHealthy && snap.Health != runtime_supervisor.HealthDegraded {
			return fmt.Errorf("candidate-manager: instance %s unhealthy (status=%s)", instID, snap.Health)
		}
		if snap.Actual != runtime_supervisor.ActualReady && snap.Actual != runtime_supervisor.ActualDegraded {
			return fmt.Errorf("candidate-manager: instance %s not ready (state=%s)", instID, snap.Actual)
		}
	}

	return nil
}

func (m *CandidateContributionManager) PromoteCandidate(ctx context.Context, candidateID string) error {
	if m == nil {
		return fmt.Errorf("candidate-manager: not initialized")
	}

	m.mu.Lock()
	record, ok := m.candidates[candidateID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("candidate-manager: candidate %s not found", candidateID)
	}
	if record.Status == CandidateStatusPromoted {
		m.mu.Unlock()
		return nil
	}
	if record.Status == CandidateStatusPromoting {
		m.mu.Unlock()
		return fmt.Errorf("candidate-manager: candidate %s is already being promoted", candidateID)
	}

	if record.ExpectedStableGeneration > 0 {
		activeGen := m.generationMgr.Active(ctx, string(record.ExtensionID))
		if activeGen == nil || int64(activeGen.Generation) != record.ExpectedStableGeneration {
			m.mu.Unlock()
			return fmt.Errorf("candidate-manager: CAS check failed for candidate %s: expected stable generation %d, got %v",
				candidateID, record.ExpectedStableGeneration, activeGen)
		}
	}

	record.Status = CandidateStatusPromoting
	record.UpdatedAt = time.Now().UTC()
	m.mu.Unlock()

	if m.repo != nil {
		_ = m.repo.UpdateStatus(ctx, candidateID, CandidateStatusPromoting)
	}

	stableSnap := m.captureStableSnapshot(ctx, record.ExtensionID)
	m.mu.Lock()
	m.snapshots[candidateID] = stableSnap
	m.mu.Unlock()

	newScheduleIDs, err := m.installer.PromoteCandidateContributions(ctx, candidateID)
	if err != nil {
		if restoreErr := m.restoreStableFromSnapshot(ctx, record.ExtensionID, stableSnap); restoreErr != nil {
			m.markRequiresRecovery(ctx, record.ExtensionID)
			log.Printf("[candidate-manager] stable restore failed for %s: %v (marked requires_recovery)", candidateID, restoreErr)
		}
		m.markFailed(ctx, candidateID, record)
		m.mu.Lock()
		delete(m.snapshots, candidateID)
		m.mu.Unlock()
		return fmt.Errorf("candidate-manager: promote candidate contributions: %w", err)
	}

	record.ScheduleIDs = append(record.ScheduleIDs, newScheduleIDs...)

	if m.repo != nil && len(record.ScheduleIDs) > 0 {
		_ = m.repo.Save(ctx, record)
	}

	if err := m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateActive); err != nil {
		_ = m.installer.DiscardCandidateContributions(ctx, record.ExtensionID, record.CandidateGeneration, record.Contribs, record.ScheduleIDs)
		_ = m.installer.DiscardCandidateNamespace(ctx, candidateID)
		if restoreErr := m.restoreStableFromSnapshot(ctx, record.ExtensionID, stableSnap); restoreErr != nil {
			m.markRequiresRecovery(ctx, record.ExtensionID)
			log.Printf("[candidate-manager] stable restore failed for %s: %v (marked requires_recovery)", candidateID, restoreErr)
		}
		m.markFailed(ctx, candidateID, record)
		m.mu.Lock()
		delete(m.snapshots, candidateID)
		m.mu.Unlock()
		return fmt.Errorf("candidate-manager: promote generation: %w", err)
	}

	m.mu.Lock()
	record.Status = CandidateStatusPromoted
	record.UpdatedAt = time.Now().UTC()
	m.promotedRecords[candidateID] = record
	delete(m.candidates, candidateID)
	delete(m.snapshots, candidateID)
	m.mu.Unlock()

	if m.repo != nil {
		_ = m.repo.Delete(ctx, candidateID)
	}

	log.Printf("[candidate-manager] promoted candidate %s for extension %s (generation=%d) - atomic namespace switch complete",
		candidateID, record.ExtensionID, record.CandidateGeneration)
	return nil
}

func (m *CandidateContributionManager) DiscardCandidate(ctx context.Context, candidateID string) error {
	if m == nil {
		return fmt.Errorf("candidate-manager: not initialized")
	}

	m.mu.Lock()
	record, ok := m.candidates[candidateID]
	m.mu.Unlock()
	if !ok {
		_ = m.installer.DiscardCandidateNamespace(ctx, candidateID)
		return nil
	}

	if record.Status == CandidateStatusPromoted {
		return nil
	}

	if record.Status == CandidateStatusPromoting || record.Status == CandidateStatusFailed {
		gen, err := m.generationMgr.Get(ctx, string(record.ExtensionID), record.GenerationID)
		if err == nil && gen.State != update.GenerationStateStopped && gen.State != update.GenerationStateFailed {
			_ = m.installer.DiscardCandidateContributions(ctx, record.ExtensionID, record.CandidateGeneration, record.Contribs, record.ScheduleIDs)
		}
	}

	_ = m.installer.DiscardCandidateNamespace(ctx, candidateID)

	for _, instID := range record.InstanceIDs {
		_ = m.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
	}

	_ = m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateFailed)
	_ = m.generationMgr.RemoveGeneration(ctx, string(record.ExtensionID), record.GenerationID)

	if m.repo != nil {
		_ = m.repo.Delete(ctx, candidateID)
	}
	m.mu.Lock()
	delete(m.candidates, candidateID)
	m.mu.Unlock()

	log.Printf("[candidate-manager] discarded candidate %s for extension %s (status=%s) - namespace and production cleaned",
		candidateID, record.ExtensionID, record.Status)
	return nil
}

func (m *CandidateContributionManager) RecoverOrphanCandidates(ctx context.Context) ([]string, error) {
	if m == nil {
		return nil, nil
	}

	var records []*CandidateRecord
	if m.repo != nil {
		var err error
		records, err = m.repo.ListAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("candidate-manager: list orphan candidates: %w", err)
		}
	}

	var cleaned []string
	for _, record := range records {
		_ = m.installer.DiscardCandidateNamespace(ctx, record.CandidateID)
		switch record.Status {
		case CandidateStatusRegistered:
			for _, instID := range record.InstanceIDs {
				_ = m.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
			}
			_ = m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateFailed)
			_ = m.generationMgr.RemoveGeneration(ctx, string(record.ExtensionID), record.GenerationID)
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=registered, not promoted, namespace cleaned)", record.CandidateID)

		case CandidateStatusPromoting:
			_ = m.installer.DiscardCandidateContributions(ctx, record.ExtensionID, record.CandidateGeneration, record.Contribs, record.ScheduleIDs)
			restoreSnap := m.captureStableSnapshot(ctx, record.ExtensionID)
			if restoreErr := m.restoreStableFromSnapshot(ctx, record.ExtensionID, restoreSnap); restoreErr != nil {
				m.markRequiresRecovery(ctx, record.ExtensionID)
				log.Printf("[candidate-manager] stable restore failed for orphan %s: %v (marked requires_recovery, skipping generation cleanup)", record.CandidateID, restoreErr)
				_ = m.repo.Delete(ctx, record.CandidateID)
				cleaned = append(cleaned, record.CandidateID)
				continue
			}
			for _, instID := range record.InstanceIDs {
				_ = m.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
			}
			_ = m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateFailed)
			_ = m.generationMgr.RemoveGeneration(ctx, string(record.ExtensionID), record.GenerationID)
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=promoting, repaired production, namespace cleaned)", record.CandidateID)

		case CandidateStatusPromoted:
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=promoted, already live)", record.CandidateID)

		case CandidateStatusFailed:
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=failed, namespace cleaned)", record.CandidateID)
		}
	}

	return cleaned, nil
}

func (m *CandidateContributionManager) GetCandidate(candidateID string) (*CandidateRecord, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if record, ok := m.candidates[candidateID]; ok {
		return record, true
	}
	if record, ok := m.promotedRecords[candidateID]; ok {
		return record, true
	}
	return nil, false
}

func (m *CandidateContributionManager) RemovePromotedRecord(candidateID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.promotedRecords, candidateID)
	m.mu.Unlock()
}

func (m *CandidateContributionManager) LoadFromStore(ctx context.Context) error {
	if m == nil || m.repo == nil {
		return nil
	}
	records, err := m.repo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("candidate-manager: load from store: %w", err)
	}
	m.mu.Lock()
	for _, record := range records {
		m.candidates[record.CandidateID] = record
		if record.Status == CandidateStatusRegistered {
			_ = m.installer.RegisterCandidateContributions(
				ctx,
				record.CandidateID,
				record.ExtensionID,
				record.InstanceIDs,
				record.GenerationID,
				record.CandidateGeneration,
				record.Contribs,
				record.DefinitionHash,
				record.ArtifactPath,
			)
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *CandidateContributionManager) markFailed(ctx context.Context, candidateID string, record *CandidateRecord) {
	m.mu.Lock()
	record.Status = CandidateStatusFailed
	record.UpdatedAt = time.Now().UTC()
	m.mu.Unlock()

	if m.repo != nil {
		_ = m.repo.UpdateStatus(ctx, candidateID, CandidateStatusFailed)
	}
	log.Printf("[candidate-manager] candidate %s marked as failed", candidateID)
}

func (m *CandidateContributionManager) captureStableSnapshot(ctx context.Context, extID domain.ExtensionID) *StableSnapshot {
	if m == nil || m.installer == nil || m.installer.container == nil {
		return nil
	}
	container := m.installer.container
	snap := &StableSnapshot{
		CapturedAt: time.Now().UTC(),
	}
	if activeGen := m.generationMgr.Active(ctx, string(extID)); activeGen != nil {
		snap.GenerationID = activeGen.GenerationID
		snap.Generation = int64(activeGen.Generation)
		snap.DefinitionHash = activeGen.DefinitionHash
	}
	if container.InstallationRepository != nil {
		if inst, err := container.InstallationRepository.GetInstallation(ctx, extID); err == nil {
			snap.EnablementState = inst.EnablementState
			if snap.Generation == 0 {
				snap.Generation = inst.Generation
			}
		}
	}
	if container.ContributionRepository != nil {
		if contribs, err := container.ContributionRepository.ListContributions(ctx, extID); err == nil {
			snap.Contributions = contribs
		}
	}
	return snap
}

func (m *CandidateContributionManager) restoreStableFromSnapshot(ctx context.Context, extID domain.ExtensionID, snap *StableSnapshot) error {
	if m == nil || snap == nil || len(snap.Contributions) == 0 {
		return nil
	}
	if err := m.installer.InstallContributions(ctx, snap.Contributions, snap.Generation); err != nil {
		return fmt.Errorf("candidate-manager: restore stable contributions: %w", err)
	}
	if snap.EnablementState == domain.EnablementEnabled {
		if err := m.installer.ActivateContributions(ctx, extID); err != nil {
			return fmt.Errorf("candidate-manager: activate restored stable contributions: %w", err)
		}
	}
	log.Printf("[candidate-manager] restored stable contributions for %s (generation=%d, contribs=%d)",
		extID, snap.Generation, len(snap.Contributions))
	return nil
}

func (m *CandidateContributionManager) markRequiresRecovery(ctx context.Context, extID domain.ExtensionID) {
	if m == nil || m.installer == nil || m.installer.container == nil {
		return
	}
	container := m.installer.container
	if container.InstallationRepository == nil {
		return
	}
	inst, err := container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return
	}
	inst.EnablementState = domain.EnablementRequiresRecovery
	inst.UpdatedAt = time.Now().UTC()
	_ = container.InstallationRepository.PutInstallation(ctx, inst)
	log.Printf("[candidate-manager] extension %s marked as requires_recovery", extID)
}
