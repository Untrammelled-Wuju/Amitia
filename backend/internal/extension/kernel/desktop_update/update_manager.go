package desktop_update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type UpdateOperation struct {
	OperationID        string
	ExtensionID        string
	FromVersion        string
	ToVersion          string
	SourceID           string
	Status             UpdateState
	PackageHash        string
	DownloadPath       string
	StagingPath        string
	OldGeneration      int64
	NewGeneration      int64
	PermissionDiffJSON string
	ScopeDiffJSON      string
	RuntimeDiffJSON    string
	MigrationPlanJSON  string
	RollbackPlanJSON   string
	StartedAt          time.Time
	FinishedAt         *time.Time
	ErrorCode          string
	ErrorMessage       string
	Plan               *ExtensionUpdatePlan
	Metadata           ExtensionUpdateMetadata
	UserConfirmed      bool
}

type UpdateManager struct {
	mu              sync.RWMutex
	sources         *UpdateSourceRegistry
	downloads       *DownloadManager
	journal         *UpdateJournal
	preflight       *PreflightChecker
	health          *HealthChecker
	recovery        *RecoveryService
	stateMachine    *StateMachine
	operations      map[string]*UpdateOperation
	operationsByExt map[string][]string
	dataDir         string
	hostVersion     string
	activeByExt     map[string]string
	genCounter      int64
	currentVersions map[string]string
}

func NewUpdateManager(dataDir, hostVersion string) *UpdateManager {
	sources := NewUpdateSourceRegistry()
	downloads := NewDownloadManager(dataDir)
	journal := NewUpdateJournal()
	sm := NewStateMachine()
	preflight := NewPreflightChecker(hostVersion)
	health := NewHealthChecker()
	recovery := NewRecoveryService(journal, sm)

	mgr := &UpdateManager{
		sources:         sources,
		downloads:       downloads,
		journal:         journal,
		preflight:       preflight,
		health:          health,
		recovery:        recovery,
		stateMachine:    sm,
		operations:      make(map[string]*UpdateOperation),
		operationsByExt: make(map[string][]string),
		dataDir:         dataDir,
		hostVersion:     hostVersion,
		activeByExt:     make(map[string]string),
		genCounter:      0,
		currentVersions: make(map[string]string),
	}

	SetDownloadStateProvider(func(operationID string) (*DownloadState, bool) {
		return mgr.downloads.GetDownloadState(operationID)
	})

	return mgr
}

func (m *UpdateManager) Sources() *UpdateSourceRegistry  { return m.sources }
func (m *UpdateManager) Downloads() *DownloadManager      { return m.downloads }
func (m *UpdateManager) Journal() *UpdateJournal          { return m.journal }
func (m *UpdateManager) Preflight() *PreflightChecker     { return m.preflight }
func (m *UpdateManager) Health() *HealthChecker           { return m.health }
func (m *UpdateManager) StateMachine() *StateMachine      { return m.stateMachine }
func (m *UpdateManager) Recovery() *RecoveryService       { return m.recovery }

func (m *UpdateManager) SetCurrentVersion(extensionID, version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentVersions[extensionID] = version
}

func (m *UpdateManager) GetCurrentVersion(extensionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentVersions[extensionID]
}

func (m *UpdateManager) transitionState(op *UpdateOperation, target UpdateState) error {
	if err := m.stateMachine.Transition(op.Status, target); err != nil {
		return err
	}
	op.Status = target
	return nil
}

func (m *UpdateManager) failOperation(op *UpdateOperation, errCode, errMsg string) {
	now := time.Now().UTC()
	op.ErrorCode = errCode
	op.ErrorMessage = errMsg
	op.FinishedAt = &now

	m.stateMachine.Transition(op.Status, StateFailed)
	op.Status = StateFailed

	m.journal.FailStep(op.OperationID, errCode, errMsg, "operation failed")

	m.mu.Lock()
	delete(m.activeByExt, op.ExtensionID)
	m.mu.Unlock()
}

func (m *UpdateManager) compensate(op *UpdateOperation, reason string) {
	if !op.Plan.RollbackPlan.CanRollback {
		return
	}

	m.stateMachine.Transition(op.Status, StateRollbackPending)
	op.Status = StateRollbackPending

	m.journal.Record(JournalEntry{
		OperationID:  op.OperationID,
		Step:         "compensate_rollback",
		Status:       JournalStatusInProgress,
		StartedAt:    time.Now().UTC(),
		Compensation: reason,
	})

	m.stateMachine.Transition(op.Status, StateRollingBack)
	op.Status = StateRollingBack

	if op.OldGeneration > 0 {
		op.NewGeneration = op.OldGeneration
	}

	m.stateMachine.Transition(op.Status, StateRolledBack)
	op.Status = StateRolledBack
	now := time.Now().UTC()
	op.FinishedAt = &now

	m.journal.Record(JournalEntry{
		OperationID: op.OperationID,
		Step:        "compensate_rollback",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
	})

	m.mu.Lock()
	delete(m.activeByExt, op.ExtensionID)
	m.mu.Unlock()
}

func (m *UpdateManager) CheckForUpdates(ctx context.Context, extensionID string) ([]ExtensionUpdateMetadata, error) {
	m.mu.RLock()
	if activeOpID, ok := m.activeByExt[extensionID]; ok {
		if op, exists := m.operations[activeOpID]; exists && !m.stateMachine.IsTerminal(op.Status) {
			m.mu.RUnlock()
			return nil, fmt.Errorf("%w: extension %s has active operation %s", ErrUpdateAlreadyRunning, extensionID, activeOpID)
		}
	}
	m.mu.RUnlock()

	var results []ExtensionUpdateMetadata

	for _, source := range m.sources.ListEnabled() {
		if !source.IsTrusted() {
			continue
		}

		switch source.SourceType {
		case SourceTypeLocalFile:
			continue
		case SourceTypeOfficialRegistry, SourceTypePublisherRegistry, SourceTypeCustomRegistry:
			meta := m.queryRegistry(ctx, source, extensionID)
			if meta != nil {
				currentVersion := m.GetCurrentVersion(extensionID)
				if currentVersion != "" {
					updateType, err := CompareVersions(currentVersion, meta.Version)
					if err != nil {
						continue
					}
					if updateType == UpdateTypeDowngrade || updateType == UpdateTypeSame {
						continue
					}
				}
				results = append(results, *meta)
			}
		}
	}

	return results, nil
}

func (m *UpdateManager) queryRegistry(ctx context.Context, source ExtensionUpdateSource, extensionID string) *ExtensionUpdateMetadata {
	return nil
}

func (m *UpdateManager) CreateUpdateOperation(ctx context.Context, extensionID string, metadata ExtensionUpdateMetadata) (*UpdateOperation, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if activeOpID, ok := m.activeByExt[extensionID]; ok {
		if op, exists := m.operations[activeOpID]; exists && !m.stateMachine.IsTerminal(op.Status) {
			m.mu.Unlock()
			return nil, fmt.Errorf("%w: extension %s has active operation %s", ErrUpdateAlreadyRunning, extensionID, activeOpID)
		}
	}

	opID := fmt.Sprintf("upd-%s-%d", extensionID, time.Now().UnixNano())
	currentVersion := m.currentVersions[extensionID]

	plan := BuildUpdatePlan(opID, extensionID, currentVersion, metadata)

	permDiffJSON, _ := json.Marshal(plan.PermissionDiff)
	scopeDiffJSON, _ := json.Marshal(plan.ScopeDiff)
	rtDiffJSON, _ := json.Marshal(plan.RuntimeDiff)
	migPlanJSON, _ := json.Marshal(plan.MigrationPlan)
	rbPlanJSON, _ := json.Marshal(plan.RollbackPlan)

	op := &UpdateOperation{
		OperationID:        opID,
		ExtensionID:        extensionID,
		FromVersion:        currentVersion,
		ToVersion:          metadata.Version,
		Status:             StateCreated,
		StartedAt:          time.Now().UTC(),
		Plan:               plan,
		Metadata:           metadata,
		PermissionDiffJSON: string(permDiffJSON),
		ScopeDiffJSON:      string(scopeDiffJSON),
		RuntimeDiffJSON:    string(rtDiffJSON),
		MigrationPlanJSON:  string(migPlanJSON),
		RollbackPlanJSON:   string(rbPlanJSON),
	}

	m.operations[opID] = op
	m.operationsByExt[extensionID] = append(m.operationsByExt[extensionID], opID)
	m.activeByExt[extensionID] = opID
	m.mu.Unlock()

	m.journal.Record(JournalEntry{
		OperationID: opID,
		Step:        "create_operation",
		Status:      JournalStatusCompleted,
		StartedAt:   op.StartedAt,
		FinishedAt:  &op.StartedAt,
	})

	return op, nil
}

func (m *UpdateManager) DownloadUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if err := m.transitionState(op, StateDownloading); err != nil {
		return err
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "download",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	source, _ := m.sources.Get("official-registry")
	allowInternal := false
	if source != nil && source.SourceType == SourceTypeCustomRegistry {
		allowInternal = true
	}

	if err := m.downloads.StartDownload(ctx, operationID, op.Metadata.PackageURL, op.Metadata.PackageSHA256, op.Metadata.PackageSize, allowInternal); err != nil {
		m.journal.FailStep(operationID, "download", err.Error(), "cancel download")
		m.downloads.CancelDownload(operationID)
		m.failOperation(op, "download_failed", err.Error())
		return err
	}

	if err := m.waitForDownload(ctx, operationID); err != nil {
		m.journal.FailStep(operationID, "download", err.Error(), "cancel download")
		m.downloads.CancelDownload(operationID)
		m.failOperation(op, "download_failed", err.Error())
		return err
	}

	dlState, _ := m.downloads.GetDownloadState(operationID)
	if dlState == nil || dlState.Status != DownloadStatusCompleted {
		err := fmt.Errorf("%w: download did not complete", ErrDownloadFailed)
		m.journal.FailStep(operationID, "download", err.Error(), "")
		m.failOperation(op, "download_failed", err.Error())
		return err
	}

	op.DownloadPath = dlState.TempPath
	op.PackageHash = dlState.ExpectedHash

	if err := m.transitionState(op, StateDownloaded); err != nil {
		return err
	}

	now := time.Now().UTC()
	m.journal.CompleteStep(operationID, "download", op.PackageHash)
	_ = now

	return nil
}

func (m *UpdateManager) waitForDownload(ctx context.Context, operationID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			state, ok := m.downloads.GetDownloadState(operationID)
			if !ok {
				return fmt.Errorf("%w: download state not found", ErrDownloadFailed)
			}
			switch state.Status {
			case DownloadStatusCompleted:
				return nil
			case DownloadStatusFailed:
				return fmt.Errorf("%w: %s", ErrDownloadFailed, state.Error)
			case DownloadStatusCancelled:
				return fmt.Errorf("%w: download was cancelled", ErrDownloadCancelled)
			case DownloadStatusDownloading, DownloadStatusPending, DownloadStatusPaused:
				continue
			}
		}
	}
}

func (m *UpdateManager) VerifyUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if err := m.transitionState(op, StateVerifying); err != nil {
		return err
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "verify",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	if op.DownloadPath == "" {
		err := fmt.Errorf("%w: no download path", ErrDownloadFailed)
		m.journal.FailStep(operationID, "verify", err.Error(), "")
		m.failOperation(op, "verify_failed", err.Error())
		return err
	}

	if _, err := os.Stat(op.DownloadPath); err != nil {
		err := fmt.Errorf("%w: download file not found: %v", ErrDownloadFailed, err)
		m.journal.FailStep(operationID, "verify", err.Error(), "")
		m.failOperation(op, "verify_failed", err.Error())
		return err
	}

	actualHash, err := computeFileSHA256(op.DownloadPath)
	if err != nil {
		m.journal.FailStep(operationID, "verify", err.Error(), "")
		m.failOperation(op, "verify_failed", err.Error())
		return err
	}

	if op.Metadata.PackageSHA256 != "" {
		if len(op.Metadata.PackageSHA256) == 64 && actualHash != op.Metadata.PackageSHA256 {
			err := fmt.Errorf("%w: expected %s got %s", ErrHashMismatch, op.Metadata.PackageSHA256, actualHash)
			m.journal.FailStep(operationID, "verify", err.Error(), "rollback")
			m.compensate(op, "hash mismatch during verification")
			return err
		}
	}

	op.PackageHash = actualHash

	if err := m.transitionState(op, StateStaging); err != nil {
		return err
	}

	m.journal.CompleteStep(operationID, "verify", actualHash)

	return nil
}

func (m *UpdateManager) StageUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "stage",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	stagingDir := filepath.Join(m.dataDir, "extensions", "staging", operationID)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		m.journal.FailStep(operationID, "stage", err.Error(), "")
		m.failOperation(op, "stage_failed", err.Error())
		return err
	}

	op.StagingPath = stagingDir
	m.mu.Lock()
	m.genCounter++
	op.NewGeneration = m.genCounter
	m.mu.Unlock()

	if err := m.transitionState(op, StatePreflight); err != nil {
		return err
	}

	m.journal.CompleteStep(operationID, "stage", fmt.Sprintf("gen:%d", op.NewGeneration))

	return nil
}

func (m *UpdateManager) RunPreflight(ctx context.Context, operationID string) (*PreflightResult, error) {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "preflight",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	result, err := m.preflight.Check(ctx, op.Plan, op.FromVersion)
	if err != nil {
		m.journal.FailStep(operationID, "preflight", err.Error(), "")
		m.failOperation(op, "preflight_failed", err.Error())
		return result, err
	}

	if !result.Passed {
		errMsg := fmt.Sprintf("preflight failed with %d errors", len(result.Errors))
		m.journal.FailStep(operationID, "preflight", errMsg, "rollback")
		m.failOperation(op, "preflight_failed", errMsg)
		return result, fmt.Errorf("%w: %s", ErrPreflightFailed, errMsg)
	}

	if op.Plan.RequiresUserConfirmation && !op.UserConfirmed {
		if err := m.transitionState(op, StateWaitingConfirmation); err != nil {
			return result, err
		}
		m.journal.CompleteStep(operationID, "preflight", "waiting_confirmation")
		return result, nil
	}

	m.journal.CompleteStep(operationID, "preflight", "passed")

	return result, nil
}

func (m *UpdateManager) ConfirmUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateWaitingConfirmation {
		return fmt.Errorf("%w: operation not in waiting_confirmation state", ErrUpdateConflict)
	}

	op.UserConfirmed = true

	if err := m.transitionState(op, StateDraining); err != nil {
		return err
	}

	now := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "user_confirmation",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
	})

	return nil
}

func (m *UpdateManager) DrainRuntime(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateDraining {
		if err := m.transitionState(op, StateDraining); err != nil {
			return err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "drain",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	drainPlan := op.Plan.RuntimeDrainPlan
	if drainPlan.CancelTimeoutSec <= 0 {
		drainPlan.CancelTimeoutSec = 30
	}

	timeout := time.Duration(drainPlan.CancelTimeoutSec) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	drained := false
	for !drained {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("%w: drain cancelled by context", ErrDrainTimeout)
			m.journal.FailStep(operationID, "drain", err.Error(), "rollback")
			m.compensate(op, "drain timeout")
			return err
		case <-timer.C:
			err := fmt.Errorf("%w: drain exceeded %v", ErrDrainTimeout, timeout)
			m.journal.FailStep(operationID, "drain", err.Error(), "rollback")
			m.compensate(op, "drain timeout")
			return err
		default:
			drained = true
		}
	}

	if err := m.transitionState(op, StateMigrating); err != nil {
		return err
	}

	m.journal.CompleteStep(operationID, "drain", "drained")

	return nil
}

func (m *UpdateManager) MigrateData(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateMigrating {
		if err := m.transitionState(op, StateMigrating); err != nil {
			return err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "migrate",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	if op.Plan.MigrationPlan != nil && op.Plan.MigrationPlan.HasMigration {
		if !op.Plan.MigrationPlan.IsReversible && !op.UserConfirmed {
			err := fmt.Errorf("%w: irreversible migration requires user confirmation", ErrMigrationFailed)
			m.journal.FailStep(operationID, "migrate", err.Error(), "rollback")
			m.compensate(op, "irreversible migration without confirmation")
			return err
		}

		migrationDir := filepath.Join(op.StagingPath, "migration")
		if err := os.MkdirAll(migrationDir, 0o755); err != nil {
			err := fmt.Errorf("%w: cannot create migration dir: %v", ErrMigrationFailed, err)
			m.journal.FailStep(operationID, "migrate", err.Error(), "rollback")
			m.compensate(op, "migration directory creation failed")
			return err
		}

		if !op.Plan.MigrationPlan.IsReversible {
			snapshotDir := filepath.Join(op.StagingPath, "data-snapshot")
			if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
				err := fmt.Errorf("%w: cannot create snapshot dir: %v", ErrMigrationFailed, err)
				m.journal.FailStep(operationID, "migrate", err.Error(), "rollback")
				m.compensate(op, "snapshot creation failed")
				return err
			}
		}
	}

	if err := m.transitionState(op, StateActivating); err != nil {
		return err
	}

	m.journal.CompleteStep(operationID, "migrate", "migration_complete")

	return nil
}

func (m *UpdateManager) ActivateGeneration(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateActivating {
		if err := m.transitionState(op, StateActivating); err != nil {
			return err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "activate",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	if op.NewGeneration <= 0 {
		m.mu.Lock()
		m.genCounter++
		op.NewGeneration = m.genCounter
		m.mu.Unlock()
	}

	if err := m.transitionState(op, StateVerifyingHealth); err != nil {
		return err
	}

	m.journal.CompleteStep(operationID, "activate", fmt.Sprintf("gen:%d", op.NewGeneration))

	return nil
}

func (m *UpdateManager) VerifyHealth(ctx context.Context, operationID string) (*HealthCheckResult, error) {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateVerifyingHealth {
		if err := m.transitionState(op, StateVerifyingHealth); err != nil {
			return nil, err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "verify_health",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	result, err := m.health.Check(ctx, op.ExtensionID, op.NewGeneration)
	if err != nil {
		m.journal.FailStep(operationID, "verify_health", err.Error(), "rollback")
		m.compensate(op, "health check error")
		return result, err
	}

	if !result.Passed {
		var failedChecks []string
		for _, c := range result.Checks {
			if !c.Passed {
				failedChecks = append(failedChecks, c.Name)
			}
		}
		errMsg := fmt.Sprintf("health check failed: %v", failedChecks)
		m.journal.FailStep(operationID, "verify_health", errMsg, "rollback")
		m.compensate(op, "health check failed")
		return result, fmt.Errorf("%w: %s", ErrHealthCheckFailed, errMsg)
	}

	if err := m.transitionState(op, StateCommitting); err != nil {
		return result, err
	}

	m.journal.CompleteStep(operationID, "verify_health", "passed")

	return result, nil
}

func (m *UpdateManager) CommitUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateCommitting {
		if err := m.transitionState(op, StateCommitting); err != nil {
			return err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "commit",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	m.mu.Lock()
	m.currentVersions[op.ExtensionID] = op.ToVersion
	op.OldGeneration = op.NewGeneration
	m.mu.Unlock()

	if err := m.transitionState(op, StateCompleted); err != nil {
		m.journal.FailStep(operationID, "commit", err.Error(), "rollback")
		m.compensate(op, "commit failed")
		return err
	}

	now := time.Now().UTC()
	op.FinishedAt = &now

	m.mu.Lock()
	delete(m.activeByExt, op.ExtensionID)
	m.mu.Unlock()

	m.journal.CompleteStep(operationID, "commit", fmt.Sprintf("version:%s", op.ToVersion))

	m.downloads.CleanupDownload(operationID)

	return nil
}

func (m *UpdateManager) RollbackUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if !op.Plan.RollbackPlan.CanRollback {
		err := fmt.Errorf("%w: rollback not supported for this operation", ErrRollbackFailed)
		m.failOperation(op, "rollback_not_supported", err.Error())
		return err
	}

	if err := m.transitionState(op, StateRollbackPending); err != nil {
		if op.Status != StateRollbackPending {
			return err
		}
	}

	stepStart := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "rollback",
		Status:      JournalStatusInProgress,
		StartedAt:   stepStart,
	})

	if err := m.transitionState(op, StateRollingBack); err != nil {
		m.journal.FailStep(operationID, "rollback", err.Error(), "")
		m.failOperation(op, "rollback_failed", err.Error())
		return err
	}

	if op.OldGeneration > 0 {
		op.NewGeneration = op.OldGeneration
	}

	if err := m.transitionState(op, StateRolledBack); err != nil {
		m.journal.FailStep(operationID, "rollback", err.Error(), "")
		m.failOperation(op, "rollback_failed", err.Error())
		return err
	}

	now := time.Now().UTC()
	op.FinishedAt = &now

	m.mu.Lock()
	delete(m.activeByExt, op.ExtensionID)
	m.mu.Unlock()

	m.journal.CompleteStep(operationID, "rollback", "rolled_back")

	if op.DownloadPath != "" {
		os.Remove(op.DownloadPath)
	}
	if op.StagingPath != "" {
		os.RemoveAll(op.StagingPath)
	}
	m.downloads.CleanupDownload(operationID)

	return nil
}

func (m *UpdateManager) CancelUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if m.stateMachine.IsTerminal(op.Status) {
		return fmt.Errorf("%w: cannot cancel operation in terminal state %s", ErrUpdateConflict, op.Status)
	}

	if op.Status == StateDownloading {
		m.downloads.CancelDownload(operationID)
	}

	if err := m.transitionState(op, StateCancelled); err != nil {
		op.Status = StateCancelled
	}

	now := time.Now().UTC()
	op.FinishedAt = &now

	m.mu.Lock()
	delete(m.activeByExt, op.ExtensionID)
	m.mu.Unlock()

	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "cancel",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
		Compensation: "operation cancelled by user",
	})

	if op.DownloadPath != "" {
		os.Remove(op.DownloadPath)
	}
	if op.StagingPath != "" {
		os.RemoveAll(op.StagingPath)
	}
	m.downloads.CleanupDownload(operationID)

	return nil
}

func (m *UpdateManager) RetryUpdate(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	if op.Status != StateFailed && op.Status != StateRecoveryRequired {
		return fmt.Errorf("%w: can only retry failed or recovery_required operations", ErrUpdateConflict)
	}

	if err := m.transitionState(op, StateCreated); err != nil {
		return err
	}

	op.ErrorCode = ""
	op.ErrorMessage = ""
	op.FinishedAt = nil

	now := time.Now().UTC()
	op.StartedAt = now

	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "retry",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
	})

	m.mu.Lock()
	m.activeByExt[op.ExtensionID] = operationID
	m.mu.Unlock()

	return nil
}

func (m *UpdateManager) GetOperation(operationID string) (*UpdateOperation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	op, ok := m.operations[operationID]
	if !ok {
		return nil, false
	}
	return op, true
}

func (m *UpdateManager) ListOperationsByExtension(extensionID string) []UpdateOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	opIDs := m.operationsByExt[extensionID]
	out := make([]UpdateOperation, 0, len(opIDs))
	for _, id := range opIDs {
		if op, ok := m.operations[id]; ok {
			out = append(out, *op)
		}
	}
	return out
}

func (m *UpdateManager) ListAllOperations() []UpdateOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]UpdateOperation, 0, len(m.operations))
	for _, op := range m.operations {
		out = append(out, *op)
	}
	return out
}

func (m *UpdateManager) ResumeOperation(ctx context.Context, operationID string) error {
	op, ok := m.GetOperation(operationID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUpdateOperationNotFound, operationID)
	}

	switch op.Status {
	case StateCreated:
		return m.CheckAndDownload(ctx, operationID)
	case StateDownloading:
		return m.DownloadUpdate(ctx, operationID)
	case StateDownloaded, StateVerifying:
		return m.VerifyUpdate(ctx, operationID)
	case StateStaging:
		return m.StageUpdate(ctx, operationID)
	case StatePreflight:
		if _, err := m.RunPreflight(ctx, operationID); err != nil {
			return err
		}
		if op.Status == StateWaitingConfirmation {
			return nil
		}
		return m.DrainRuntime(ctx, operationID)
	case StateWaitingConfirmation:
		return nil
	case StateDraining:
		return m.DrainRuntime(ctx, operationID)
	case StateMigrating:
		return m.MigrateData(ctx, operationID)
	case StateActivating:
		return m.ActivateGeneration(ctx, operationID)
	case StateVerifyingHealth:
		_, err := m.VerifyHealth(ctx, operationID)
		return err
	case StateCommitting:
		return m.CommitUpdate(ctx, operationID)
	case StateRollbackPending, StateRollingBack:
		return m.RollbackUpdate(ctx, operationID)
	case StateRecoveryRequired:
		return m.RetryUpdate(ctx, operationID)
	default:
		return fmt.Errorf("%w: cannot resume from state %s", ErrUpdateConflict, op.Status)
	}
}

func (m *UpdateManager) CheckAndDownload(ctx context.Context, operationID string) error {
	if err := m.transitionState(m.operations[operationID], StateChecking); err != nil {
		return err
	}
	now := time.Now().UTC()
	m.journal.Record(JournalEntry{
		OperationID: operationID,
		Step:        "check",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
		FinishedAt:  &now,
	})

	if err := m.transitionState(m.operations[operationID], StateAvailable); err != nil {
		return err
	}

	return m.DownloadUpdate(ctx, operationID)
}

func (m *UpdateManager) RunFullUpdate(ctx context.Context, operationID string) error {
	steps := []func(context.Context) error{
		func(ctx context.Context) error { return m.DownloadUpdate(ctx, operationID) },
		func(ctx context.Context) error { return m.VerifyUpdate(ctx, operationID) },
		func(ctx context.Context) error { return m.StageUpdate(ctx, operationID) },
	}

	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}

	if _, err := m.RunPreflight(ctx, operationID); err != nil {
		return err
	}

	op, _ := m.GetOperation(operationID)
	if op.Status == StateWaitingConfirmation {
		return nil
	}

	continueSteps := []func(context.Context) error{
		func(ctx context.Context) error { return m.DrainRuntime(ctx, operationID) },
		func(ctx context.Context) error { return m.MigrateData(ctx, operationID) },
		func(ctx context.Context) error { return m.ActivateGeneration(ctx, operationID) },
	}

	for _, step := range continueSteps {
		if err := step(ctx); err != nil {
			return err
		}
	}

	if _, err := m.VerifyHealth(ctx, operationID); err != nil {
		return err
	}

	return m.CommitUpdate(ctx, operationID)
}
