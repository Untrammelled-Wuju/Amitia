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
	CandidateID         string
	ExtensionID         domain.ExtensionID
	InstanceIDs         []string
	GenerationID        string
	CandidateGeneration int64
	Contribs            []domain.ContributionDefinition
	ScheduleIDs         []string
	ArtifactPath        string
	DefinitionHash      string
	Status              CandidateStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
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

	mu               sync.RWMutex
	candidates       map[string]*CandidateRecord
	promotedRecords  map[string]*CandidateRecord
}

func NewCandidateContributionManager(
	installer *TypedContributionInstaller,
	generationMgr *update.GenerationManager,
	supervisor runtime_supervisor.Supervisor,
	repo *CandidateRepository,
) *CandidateContributionManager {
	return &CandidateContributionManager{
		installer:      installer,
		generationMgr:  generationMgr,
		supervisor:     supervisor,
		repo:           repo,
		candidates:     make(map[string]*CandidateRecord),
		promotedRecords: make(map[string]*CandidateRecord),
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

	if m.repo != nil {
		if err := m.repo.Save(ctx, record); err != nil {
			return fmt.Errorf("candidate-manager: persist candidate %s: %w", record.CandidateID, err)
		}
	}

	m.mu.Lock()
	m.candidates[record.CandidateID] = record
	m.mu.Unlock()

	log.Printf("[candidate-manager] registered candidate %s for extension %s (generation=%d, contribs=%d)",
		record.CandidateID, record.ExtensionID, record.CandidateGeneration, len(record.Contribs))
	return nil
}

func (m *CandidateContributionManager) HealthCandidate(ctx context.Context, candidateID string) error {
	if m == nil {
		return fmt.Errorf("candidate-manager: not initialized")
	}

	m.mu.RLock()
	record, ok := m.candidates[candidateID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("candidate-manager: candidate %s not found", candidateID)
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
	record.Status = CandidateStatusPromoting
	record.UpdatedAt = time.Now().UTC()
	m.mu.Unlock()

	if m.repo != nil {
		_ = m.repo.UpdateStatus(ctx, candidateID, CandidateStatusPromoting)
	}

	beforeScheduleIDs, _ := m.installer.ListScheduleIDs(ctx, record.ExtensionID)

	if err := m.installer.InstallContributions(ctx, record.Contribs, record.CandidateGeneration); err != nil {
		m.markFailed(ctx, candidateID, record)
		return fmt.Errorf("candidate-manager: install contributions for promote: %w", err)
	}

	afterScheduleIDs, _ := m.installer.ListScheduleIDs(ctx, record.ExtensionID)
	beforeSet := make(map[string]bool, len(beforeScheduleIDs))
	for _, id := range beforeScheduleIDs {
		beforeSet[id] = true
	}
	for _, id := range afterScheduleIDs {
		if !beforeSet[id] {
			record.ScheduleIDs = append(record.ScheduleIDs, id)
		}
	}

	if m.repo != nil && len(record.ScheduleIDs) > 0 {
		_ = m.repo.Save(ctx, record)
	}

	if err := m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateActive); err != nil {
		_ = m.installer.DiscardCandidateContributions(ctx, record.ExtensionID, record.CandidateGeneration, record.Contribs, record.ScheduleIDs)
		m.markFailed(ctx, candidateID, record)
		return fmt.Errorf("candidate-manager: promote generation: %w", err)
	}

	m.mu.Lock()
	record.Status = CandidateStatusPromoted
	record.UpdatedAt = time.Now().UTC()
	m.promotedRecords[candidateID] = record
	delete(m.candidates, candidateID)
	m.mu.Unlock()

	if m.repo != nil {
		_ = m.repo.Delete(ctx, candidateID)
	}

	log.Printf("[candidate-manager] promoted candidate %s for extension %s (generation=%d)",
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

	log.Printf("[candidate-manager] discarded candidate %s for extension %s (status=%s)",
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
		switch record.Status {
		case CandidateStatusRegistered:
			for _, instID := range record.InstanceIDs {
				_ = m.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
			}
			_ = m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateFailed)
			_ = m.generationMgr.RemoveGeneration(ctx, string(record.ExtensionID), record.GenerationID)
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=registered, not promoted)", record.CandidateID)

		case CandidateStatusPromoting:
			_ = m.installer.DiscardCandidateContributions(ctx, record.ExtensionID, record.CandidateGeneration, record.Contribs, record.ScheduleIDs)
			if m.installer != nil && m.installer.container != nil {
				inst, err := m.installer.container.InstallationRepository.GetInstallation(ctx, record.ExtensionID)
				if err == nil {
					stableContribs, listErr := m.installer.container.ContributionRepository.ListContributions(ctx, record.ExtensionID)
					if listErr == nil && len(stableContribs) > 0 {
						_ = m.installer.InstallContributions(ctx, stableContribs, inst.Generation)
						if inst.EnablementState == domain.EnablementEnabled {
							_ = m.installer.ActivateContributions(ctx, record.ExtensionID)
						}
					}
				}
			}
			for _, instID := range record.InstanceIDs {
				_ = m.supervisor.Stop(ctx, instID, runtime_supervisor.StopReasonRollback)
			}
			_ = m.generationMgr.Transition(ctx, string(record.ExtensionID), record.GenerationID, update.GenerationStateFailed)
			_ = m.generationMgr.RemoveGeneration(ctx, string(record.ExtensionID), record.GenerationID)
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=promoting, repaired production)", record.CandidateID)

		case CandidateStatusPromoted:
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=promoted, already live)", record.CandidateID)

		case CandidateStatusFailed:
			_ = m.repo.Delete(ctx, record.CandidateID)
			cleaned = append(cleaned, record.CandidateID)
			log.Printf("[candidate-manager] recovered orphan candidate %s (status=failed)", record.CandidateID)
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
