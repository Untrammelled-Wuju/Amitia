package generation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ProviderQueryResult struct {
	ProviderStatus   string
	IsCompleted      bool
	IsFailed         bool
	RetryAfterHint   int
	RawMetadata      string
	ImageURLs        []string
	GenerationResult *imageprovider.GenerationResult
}

type ProviderQueryFunc func(ctx context.Context, attempt *ActionGenerationAttempt) (*ProviderQueryResult, error)
type RecoveryTerminalFunc func(ctx context.Context, attempt *ActionGenerationAttempt) error

type FileVerifyResult struct {
	Exists   bool
	FilePath string
	Hash     string
}

type FileVerifyFunc func(artifact *GenerationArtifact) (*FileVerifyResult, error)

type RecoveryWorkerConfig struct {
	ScanInterval     time.Duration
	PollTimeout      time.Duration
	PollBaseInterval time.Duration
	PollMaxInterval  time.Duration
	MaxPollCount     int
	LeaseDuration    time.Duration
	DataDir          string
}

func DefaultRecoveryWorkerConfig() RecoveryWorkerConfig {
	return RecoveryWorkerConfig{
		ScanInterval:     30 * time.Second,
		PollTimeout:      10 * time.Minute,
		PollBaseInterval: 3 * time.Second,
		PollMaxInterval:  60 * time.Second,
		MaxPollCount:     200,
		LeaseDuration:    5 * time.Minute,
		DataDir:          config.AppCfg.Storage.DataDir,
	}
}

type RecoveryWorker struct {
	db                *gorm.DB
	attemptRepo       AttemptRepository
	artifactRepo      ArtifactRepository
	receiptRepo       ReceiptRepository
	config            RecoveryWorkerConfig
	providerQuery     ProviderQueryFunc
	fileVerifier      FileVerifyFunc
	artifactPersister *ArtifactPersister
	finalizer         *GenerationFinalizer
	onTerminal        RecoveryTerminalFunc
	stopCh            chan struct{}
	wg                sync.WaitGroup
	lifecycleMu       sync.Mutex
	running           bool
	alive             atomic.Bool
}

func NewRecoveryWorker(db *gorm.DB, attemptRepo AttemptRepository, artifactRepo ArtifactRepository, receiptRepo ReceiptRepository, config RecoveryWorkerConfig) *RecoveryWorker {
	w := &RecoveryWorker{
		db:           db,
		attemptRepo:  attemptRepo,
		artifactRepo: artifactRepo,
		receiptRepo:  receiptRepo,
		config:       config,
		stopCh:       make(chan struct{}),
	}
	w.fileVerifier = w.defaultFileVerifier
	return w
}

func (w *RecoveryWorker) WithProviderQuery(fn ProviderQueryFunc) *RecoveryWorker {
	w.providerQuery = fn
	return w
}

func (w *RecoveryWorker) WithFileVerifier(fn FileVerifyFunc) *RecoveryWorker {
	w.fileVerifier = fn
	return w
}

func (w *RecoveryWorker) WithArtifactPersister(persister *ArtifactPersister) *RecoveryWorker {
	w.artifactPersister = persister
	return w
}

func (w *RecoveryWorker) WithFinalizer(finalizer *GenerationFinalizer) *RecoveryWorker {
	w.finalizer = finalizer
	return w
}

func (w *RecoveryWorker) WithTerminalCallback(fn RecoveryTerminalFunc) *RecoveryWorker {
	w.onTerminal = fn
	return w
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if w.running {
		return
	}
	w.stopCh = make(chan struct{})
	w.running = true
	w.alive.Store(true)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *RecoveryWorker) Stop() {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.wg.Wait()
	w.running = false
	w.alive.Store(false)
}

func (w *RecoveryWorker) IsRunning() bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	return w.running && w.alive.Load()
}

func (w *RecoveryWorker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Errorf("generation recovery worker panic: %v", r)
		}
	}()
	ticker := time.NewTicker(w.config.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.scanOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Logger.Errorf("generation recovery scan failed: %v", err)
			}
		}
	}
}

func (w *RecoveryWorker) scanOnce(ctx context.Context) error {
	recoverableStatuses := []string{
		string(AttemptStatusPending),
		string(AttemptStatusPreparingReference),
		string(AttemptStatusBuildingPrompt),
		string(AttemptStatusWaitingRateLimit),
		string(AttemptStatusSubmitting),
		string(AttemptStatusUnknownSubmission),
		string(AttemptStatusReconcilingSubmission),
		string(AttemptStatusSubmitted),
		string(AttemptStatusPolling),
		string(AttemptStatusResultReceived),
		string(AttemptStatusPersisting),
		string(AttemptStatusPublishFailed),
	}
	terminalStatuses := []string{
		string(AttemptStatusSucceeded),
		string(AttemptStatusFailed),
		string(AttemptStatusFailedConfirmed),
		string(AttemptStatusManualReview),
	}

	// Never race a live GenerationExecutor. Recovery may only take over an
	// attempt whose parent task is no longer actively leased by the same
	// execution. Terminal attempts are revisited while the parent task is
	// still stuck in processing so action/task convergence is retryable.
	taskNow := time.Now().Format("2006-01-02 15:04:05")
	var attempts []ActionGenerationAttempt
	err := w.db.Model(&ActionGenerationAttempt{}).
		Where(`(
			status IN ? AND NOT EXISTS (
				SELECT 1 FROM desktop_pet_generation_tasks t
				WHERE t.id = desktop_pet_action_generation_attempts.task_id
				  AND t.status = 'processing'
				  AND t.execution_id = desktop_pet_action_generation_attempts.execution_id
				  AND t.lease_expires_at != ''
				  AND t.lease_expires_at > ?
			)
		) OR (
			status IN ? AND EXISTS (
				SELECT 1 FROM desktop_pet_generation_tasks t
				WHERE t.id = desktop_pet_action_generation_attempts.task_id
				  AND t.status = 'processing'
				  AND t.execution_id = desktop_pet_action_generation_attempts.execution_id
				  AND (t.lease_expires_at = '' OR t.lease_expires_at <= ?)
			)
		)`, recoverableStatuses, taskNow, terminalStatuses, taskNow).
		Order("updated_at ASC").
		Limit(100).
		Find(&attempts).Error
	if err != nil {
		return err
	}
	for i := range attempts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		attempt := &attempts[i]
		if attempt.ID == "" {
			continue
		}
		acquired, err := w.AcquireLease(attempt.ID, "recovery-worker")
		if err != nil {
			log.Logger.Errorf("generation recovery acquire attempt %s lease failed: %v", attempt.ID, err)
			continue
		}
		if !acquired {
			continue
		}
		if err := w.recoverAttempt(ctx, attempt); err != nil {
			log.Logger.Errorf("generation recovery attempt %s failed: %v", attempt.ID, err)
		}
		if err := w.ReleaseLease(attempt.ID, "recovery-worker"); err != nil {
			log.Logger.Errorf("generation recovery release attempt %s lease failed: %v", attempt.ID, err)
		}
	}
	return nil
}

func (w *RecoveryWorker) recoverAttempt(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt == nil {
		return nil
	}
	latest, err := w.attemptRepo.GetAttemptByID(attempt.ID)
	if err != nil {
		return err
	}
	attempt = latest
	if attempt.IsTerminal() {
		return w.convergeTerminalAttempt(ctx, attempt)
	}

	status := AttemptStatus(attempt.Status)
	switch status {
	case AttemptStatusPending, AttemptStatusPreparingReference, AttemptStatusBuildingPrompt, AttemptStatusWaitingRateLimit:
		err = w.handlePreSubmitInterrupted(ctx, attempt)
	case AttemptStatusSubmitting:
		err = w.handleSubmittingInterrupted(ctx, attempt)
	case AttemptStatusUnknownSubmission:
		err = w.handleUnknownSubmission(ctx, attempt)
	case AttemptStatusReconcilingSubmission:
		err = w.handleReconcilingSubmission(ctx, attempt)
	case AttemptStatusSubmitted, AttemptStatusPolling:
		err = w.handlePolling(ctx, attempt)
	case AttemptStatusResultReceived, AttemptStatusPersisting:
		err = w.handleRepersist(ctx, attempt)
	case AttemptStatusPublishFailed:
		err = w.handlePublishFailed(ctx, attempt)
	default:
		return NewGenerationError(ErrCodeRecoveryStatusUnknown, fmt.Sprintf("unsupported recovery status: %s", attempt.Status), nil)
	}
	if err != nil {
		return err
	}

	latest, err = w.attemptRepo.GetAttemptByID(attempt.ID)
	if err != nil {
		return err
	}
	if latest.IsTerminal() {
		return w.convergeTerminalAttempt(ctx, latest)
	}
	return nil
}

func (w *RecoveryWorker) handlePreSubmitInterrupted(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt == nil {
		return nil
	}
	var action struct {
		Status         string `gorm:"column:status"`
		CurrentAttempt int    `gorm:"column:current_attempt"`
	}
	if err := w.db.Table("desktop_pet_generation_task_actions").
		Select("status", "current_attempt").
		Where("id = ?", attempt.TaskActionID).
		First(&action).Error; err != nil {
		return err
	}
	if action.CurrentAttempt > 0 && attempt.AttemptNumber > 0 && action.CurrentAttempt != attempt.AttemptNumber {
		return nil
	}

	tx := w.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()
	now := nowRFC3339()
	if err := tx.Model(&ActionGenerationAttempt{}).Where("id = ?", attempt.ID).Updates(map[string]interface{}{
		"status":        string(AttemptStatusFailed),
		"error_code":    "recovery_pre_submit_interrupted",
		"error_message": "attempt was interrupted before provider submission and is safe to retry",
		"completed_at":  now,
		"updated_at":    now,
	}).Error; err != nil {
		return err
	}
	if action.Status == "running" {
		result := tx.Table("desktop_pet_generation_task_actions").Where("id = ?", attempt.TaskActionID).Updates(map[string]interface{}{
			"status":        "pending",
			"progress":      0,
			"error_code":    "",
			"error_message": "",
			"started_at":    "",
			"completed_at":  "",
			"updated_at":    now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("recovery task action not found: %s", attempt.TaskActionID)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	committed = true
	attempt.Status = string(AttemptStatusFailed)
	return w.notifyTerminal(ctx, attempt)
}

func (w *RecoveryWorker) handleSubmittingInterrupted(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt == nil {
		return nil
	}
	// If durable provider evidence exists, reconcile from that evidence. A
	// submitting attempt with no receipt/operation id is ambiguous and must not
	// be re-submitted automatically because that can duplicate a paid request.
	if strings.TrimSpace(attempt.ProviderOperationID) != "" && w.providerQuery != nil {
		return w.queryProviderAndUpdate(ctx, attempt)
	}
	receipt, err := w.receiptRepo.GetByAttemptID(attempt.ID)
	if err == nil && receipt != nil {
		return w.handleReceiptEvidence(ctx, attempt, receipt)
	}
	if err != nil && !errors.Is(err, ErrProviderReceiptNotFound) {
		return err
	}
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":        string(AttemptStatusManualReview),
		"error_code":    "recovery_submission_ambiguous",
		"error_message": "attempt was interrupted while submitting and has no durable provider evidence; automatic re-submit is blocked",
		"completed_at":  nowRFC3339(),
		"updated_at":    nowRFC3339(),
	})
}

func (w *RecoveryWorker) handleReceiptEvidence(ctx context.Context, attempt *ActionGenerationAttempt, receipt *ProviderReceipt) error {
	if receipt == nil {
		return nil
	}
	switch receipt.ProviderStatus {
	case "failed", "failed_confirmed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusFailedConfirmed),
			"error_code":    "provider_failed",
			"error_message": fmt.Sprintf("provider status: %s", receipt.ProviderStatus),
			"completed_at":  nowRFC3339(),
			"updated_at":    nowRFC3339(),
		})
	case "succeeded", "completed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	case "submitted", "polling", "running", "pending", "processing", "accepted":
		if w.providerQuery != nil && strings.TrimSpace(attempt.ProviderOperationID) != "" {
			return w.queryProviderAndUpdate(ctx, attempt)
		}
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusSubmitted),
			"updated_at": nowRFC3339(),
		})
	default:
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusManualReview),
			"error_code":    "recovery_receipt_status_unknown",
			"error_message": fmt.Sprintf("unrecognized provider receipt status: %s", receipt.ProviderStatus),
			"completed_at":  nowRFC3339(),
			"updated_at":    nowRFC3339(),
		})
	}
}

func (w *RecoveryWorker) handleUnknownSubmission(ctx context.Context, attempt *ActionGenerationAttempt) error {
	err := w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":     string(AttemptStatusReconcilingSubmission),
		"updated_at": nowRFC3339(),
	})
	if err != nil {
		return err
	}

	receipt, err := w.receiptRepo.GetByAttemptID(attempt.ID)
	if err != nil {
		if errors.Is(err, ErrProviderReceiptNotFound) {
			if w.providerQuery != nil && attempt.ProviderOperationID != "" {
				return w.queryProviderAndUpdate(ctx, attempt)
			}
			return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
				"status":        string(AttemptStatusFailed),
				"error_code":    "no_receipt_no_query",
				"error_message": "no provider receipt found and no query capability",
				"updated_at":    nowRFC3339(),
			})
		}
		return err
	}

	return w.handleReceiptEvidence(ctx, attempt, receipt)
}

func (w *RecoveryWorker) handleReconcilingSubmission(ctx context.Context, attempt *ActionGenerationAttempt) error {
	receipt, err := w.receiptRepo.GetByAttemptID(attempt.ID)
	if err != nil {
		if errors.Is(err, ErrProviderReceiptNotFound) {
			if w.providerQuery != nil && attempt.ProviderOperationID != "" {
				return w.queryProviderAndUpdate(ctx, attempt)
			}
			return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
				"status":        string(AttemptStatusManualReview),
				"error_code":    "recovery_receipt_missing",
				"error_message": "submission reconciliation has no provider receipt or query capability",
				"completed_at":  nowRFC3339(),
				"updated_at":    nowRFC3339(),
			})
		}
		return err
	}

	return w.handleReceiptEvidence(ctx, attempt, receipt)
}

func (w *RecoveryWorker) queryProviderAndUpdate(ctx context.Context, attempt *ActionGenerationAttempt) error {
	result, err := w.providerQuery(ctx, attempt)
	if err != nil {
		nextPollCount := attempt.PollCount + 1
		if nextPollCount >= w.config.MaxPollCount {
			return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
				"status":        string(AttemptStatusManualReview),
				"poll_count":    nextPollCount,
				"error_code":    "provider_query_exhausted",
				"error_message": fmt.Sprintf("provider query failed repeatedly: %v", err),
				"completed_at":  nowRFC3339(),
				"updated_at":    nowRFC3339(),
			})
		}
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusSubmitted),
			"poll_count":    nextPollCount,
			"error_message": fmt.Sprintf("provider query failed: %v", err),
			"heartbeat_at":  nowRFC3339(),
			"updated_at":    nowRFC3339(),
		})
	}
	if result == nil {
		return fmt.Errorf("provider query returned nil result")
	}

	if result.IsFailed {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusFailedConfirmed),
			"error_code":    "provider_query_failed",
			"error_message": fmt.Sprintf("provider query returned failed: %s", result.ProviderStatus),
			"completed_at":  nowRFC3339(),
			"updated_at":    nowRFC3339(),
		})
	}

	if result.IsCompleted {
		if err := w.persistRecoveryReceipt(attempt, result); err != nil {
			return err
		}
		if result.GenerationResult != nil && result.GenerationResult.HasCandidates() {
			return w.persistRecoveredProviderResult(ctx, attempt, result.GenerationResult)
		}
		// A completed provider response without recoverable bytes must never be
		// promoted to success merely because the provider said "completed".
		// If artifacts already exist, the repersist path can verify them; otherwise
		// the next pass will converge the attempt to manual review.
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	}

	updates := map[string]interface{}{
		"status":       string(AttemptStatusPolling),
		"poll_count":   attempt.PollCount + 1,
		"updated_at":   nowRFC3339(),
		"heartbeat_at": nowRFC3339(),
	}
	if result.RetryAfterHint > 0 {
		updates["retry_after_hint"] = result.RetryAfterHint
	}
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, updates)
}

func (w *RecoveryWorker) handlePolling(ctx context.Context, attempt *ActionGenerationAttempt) error {
	ok, err := w.RenewLease(attempt.ID, "recovery-worker")
	if err != nil {
		return err
	}
	if !ok {
		return NewGenerationError(ErrCodeRecoveryLeaseConflict, "lease renew failed during polling", nil)
	}

	if w.providerQuery != nil && attempt.ProviderOperationID != "" {
		return w.queryProviderAndUpdate(ctx, attempt)
	}

	return w.pollAttempt(ctx, attempt)
}

func (w *RecoveryWorker) pollAttempt(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt.PollCount >= w.config.MaxPollCount {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusManualReview),
			"error_code":    "polling_exhausted",
			"error_message": fmt.Sprintf("max poll count %d reached", w.config.MaxPollCount),
			"updated_at":    nowRFC3339(),
		})
	}
	interval := w.computePollInterval(attempt)
	if interval > w.config.PollTimeout {
		interval = w.config.PollTimeout
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interval):
	}
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"poll_count":   attempt.PollCount + 1,
		"updated_at":   nowRFC3339(),
		"heartbeat_at": nowRFC3339(),
	})
}

func (w *RecoveryWorker) handleRepersist(ctx context.Context, attempt *ActionGenerationAttempt) error {
	artifacts, err := w.artifactRepo.ListArtifactsByAttemptID(attempt.ID)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusManualReview),
			"error_code":    "recovery_artifact_missing",
			"error_message": "provider result was received but no persisted artifacts are available for recovery",
			"completed_at":  nowRFC3339(),
			"updated_at":    nowRFC3339(),
		})
	}

	allPersisted := true
	hasPrimary := false
	for i := range artifacts {
		a := &artifacts[i]
		if a.IsPrimaryArtifact() {
			hasPrimary = true
		}
		switch ArtifactStatus(a.Status) {
		case ArtifactStatusPersisted, ArtifactStatusVerified:
			verifyResult, verifyErr := w.fileVerifier(a)
			if verifyErr != nil || verifyResult == nil || !verifyResult.Exists {
				allPersisted = false
			}
		case ArtifactStatusStaging, ArtifactStatusSaved:
			verifyResult, verifyErr := w.fileVerifier(a)
			if verifyErr != nil || verifyResult == nil || !verifyResult.Exists {
				allPersisted = false
				continue
			}
			if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
				"status":     string(ArtifactStatusPersisted),
				"updated_at": nowRFC3339(),
			}); err != nil {
				return err
			}
			a.Status = string(ArtifactStatusPersisted)
		default:
			allPersisted = false
		}
	}

	if !allPersisted || !hasPrimary {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusPublishFailed),
			"error_message": "artifact verification failed during recovery repersist",
			"updated_at":    nowRFC3339(),
		})
	}

	return w.finalizeRecoveredSuccess(ctx, attempt, artifacts)
}

func (w *RecoveryWorker) handlePublishFailed(ctx context.Context, attempt *ActionGenerationAttempt) error {
	artifacts, err := w.artifactRepo.ListArtifactsByAttemptID(attempt.ID)
	if err != nil {
		return err
	}

	hasValidPrimary := false
	for i := range artifacts {
		a := &artifacts[i]
		if !a.IsPrimaryArtifact() {
			continue
		}
		verifyResult, verifyErr := w.fileVerifier(a)
		if verifyErr != nil || verifyResult == nil || !verifyResult.Exists {
			continue
		}
		if a.Status != string(ArtifactStatusPersisted) && a.Status != string(ArtifactStatusVerified) {
			if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
				"status":     string(ArtifactStatusPersisted),
				"updated_at": nowRFC3339(),
			}); err != nil {
				return err
			}
			a.Status = string(ArtifactStatusPersisted)
		}
		hasValidPrimary = true
		break
	}

	if hasValidPrimary {
		return w.finalizeRecoveredSuccess(ctx, attempt, artifacts)
	}

	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":        string(AttemptStatusFailed),
		"error_code":    "publish_failed_no_valid_artifact",
		"error_message": "no valid primary artifact found after publish failure",
		"completed_at":  nowRFC3339(),
		"updated_at":    nowRFC3339(),
	})
}

func (w *RecoveryWorker) persistRecoveryReceipt(attempt *ActionGenerationAttempt, result *ProviderQueryResult) error {
	if w.receiptRepo == nil || attempt == nil || result == nil {
		return nil
	}
	completedAt := nowRFC3339()
	if err := w.receiptRepo.UpdateCompleted(attempt.ID, completedAt, "", result.ProviderStatus, result.RawMetadata); err == nil {
		return nil
	} else if !errors.Is(err, ErrProviderReceiptNotFound) {
		return fmt.Errorf("persist recovery provider receipt: %w", err)
	}
	return w.receiptRepo.Create(nil, &ProviderReceipt{
		AttemptID:         attempt.ID,
		ProviderID:        attempt.Provider,
		Model:             attempt.Model,
		ProviderRequestID: attempt.ProviderRequestID,
		ProviderTaskID:    attempt.ProviderOperationID,
		SubmittedAt:       attempt.SubmittedAt,
		CompletedAt:       completedAt,
		ProviderStatus:    result.ProviderStatus,
		RawMetadataJSON:   result.RawMetadata,
	})
}

func (w *RecoveryWorker) persistRecoveredProviderResult(ctx context.Context, attempt *ActionGenerationAttempt, result *imageprovider.GenerationResult) error {
	if w.artifactPersister == nil || w.finalizer == nil {
		return fmt.Errorf("generation recovery persistence/finalizer is not configured")
	}
	if attempt == nil || result == nil || !result.HasCandidates() {
		return fmt.Errorf("generation recovery provider result has no candidates")
	}
	var plan GenerationPlanSnapshot
	if strings.TrimSpace(attempt.PlanJSON) == "" {
		return fmt.Errorf("generation recovery attempt %s has no plan snapshot", attempt.ID)
	}
	if err := json.Unmarshal([]byte(attempt.PlanJSON), &plan); err != nil {
		return fmt.Errorf("decode generation recovery plan for attempt %s: %w", attempt.ID, err)
	}

	tx := w.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin recovered provider result transaction: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()

	persisted, err := w.artifactPersister.Persist(PersistInput{
		Tx:                  tx,
		TaskID:              attempt.TaskID,
		TaskActionID:        attempt.TaskActionID,
		AttemptID:           attempt.ID,
		Plan:                &plan,
		Result:              result,
		SegmentIndex:        0,
		ExecutionID:         attempt.ExecutionID,
		ProviderRequestID:   result.RequestID,
		ProviderOperationID: result.OperationID,
	})
	if err != nil {
		return fmt.Errorf("persist recovered provider artifacts: %w", err)
	}
	cleanupPersisted := func(cause error) error {
		cleanupErr := w.artifactPersister.RollbackPersistedFiles(w.config.DataDir, persisted)
		if cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback recovered provider artifacts: %w", cleanupErr))
		}
		return cause
	}
	if persisted == nil || persisted.PrimaryArtifact == nil {
		return cleanupPersisted(fmt.Errorf("persist recovered provider result produced no primary artifact"))
	}

	actualInputUnits, actualOutputUnits := 0, 0
	if result.Usage != nil {
		actualInputUnits = int(result.Usage.PromptTokens)
		actualOutputUnits = int(result.Usage.CompletionTokens)
	}
	if err := w.finalizer.FinalizeAttempt(FinalizeAttemptRequest{
		Tx:                tx,
		AttemptID:         attempt.ID,
		TaskActionID:      attempt.TaskActionID,
		TaskID:            attempt.TaskID,
		PrimaryArtifactID: persisted.PrimaryArtifact.ID,
		ArtifactHash:      persisted.PrimaryArtifact.Hash,
		ExecutionID:       attempt.ExecutionID,
		ActualCost:        attempt.ActualCost,
		ActualInputUnits:  actualInputUnits,
		ActualOutputUnits: actualOutputUnits,
		AutoPromote:       true,
	}); err != nil {
		return cleanupPersisted(fmt.Errorf("finalize recovered provider result: %w", err))
	}
	if err := tx.Commit().Error; err != nil {
		return cleanupPersisted(fmt.Errorf("commit recovered provider result: %w", err))
	}
	committed = true

	attempt.Status = string(AttemptStatusSucceeded)
	attempt.ActualInputUnits = actualInputUnits
	attempt.ActualOutputUnits = actualOutputUnits
	return w.notifyTerminal(ctx, attempt)
}

func (w *RecoveryWorker) finalizeRecoveredSuccess(ctx context.Context, attempt *ActionGenerationAttempt, artifacts []GenerationArtifact) error {
	if w.finalizer == nil {
		return fmt.Errorf("generation recovery finalizer is not configured")
	}
	var action struct {
		Status         string `gorm:"column:status"`
		CurrentAttempt int    `gorm:"column:current_attempt"`
	}
	if err := w.db.Table("desktop_pet_generation_task_actions").
		Select("status", "current_attempt").
		Where("id = ?", attempt.TaskActionID).
		First(&action).Error; err != nil {
		return err
	}
	if action.CurrentAttempt > 0 && attempt.AttemptNumber > 0 && action.CurrentAttempt != attempt.AttemptNumber {
		return nil
	}
	var primary *GenerationArtifact
	for i := range artifacts {
		if artifacts[i].IsPrimaryArtifact() {
			primary = &artifacts[i]
			break
		}
	}
	if primary == nil {
		return fmt.Errorf("generation recovery attempt %s has no primary artifact", attempt.ID)
	}
	verifyResult, err := w.fileVerifier(primary)
	if err != nil || verifyResult == nil || !verifyResult.Exists {
		if err == nil {
			err = fmt.Errorf("primary artifact %s is missing", primary.ID)
		}
		return fmt.Errorf("verify recovered primary artifact: %w", err)
	}

	tx := w.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()
	artifactHash := primary.Hash
	if artifactHash == "" {
		artifactHash = primary.ContentHash
	}
	if err := w.finalizer.FinalizeAttempt(FinalizeAttemptRequest{
		Tx:                tx,
		AttemptID:         attempt.ID,
		TaskActionID:      attempt.TaskActionID,
		TaskID:            attempt.TaskID,
		PrimaryArtifactID: primary.ID,
		ArtifactHash:      artifactHash,
		ExecutionID:       attempt.ExecutionID,
		ActualCost:        attempt.ActualCost,
		ActualInputUnits:  attempt.ActualInputUnits,
		ActualOutputUnits: attempt.ActualOutputUnits,
		AutoPromote:       true,
	}); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	committed = true
	attempt.Status = string(AttemptStatusSucceeded)
	return w.notifyTerminal(ctx, attempt)
}

func (w *RecoveryWorker) convergeTerminalAttempt(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt == nil {
		return nil
	}
	if AttemptStatus(attempt.Status) == AttemptStatusSucceeded {
		artifacts, err := w.artifactRepo.ListArtifactsByAttemptID(attempt.ID)
		if err != nil {
			return err
		}
		return w.finalizeRecoveredSuccess(ctx, attempt, artifacts)
	}

	var action struct {
		Status         string `gorm:"column:status"`
		CurrentAttempt int    `gorm:"column:current_attempt"`
	}
	if err := w.db.Table("desktop_pet_generation_task_actions").
		Select("status", "current_attempt").
		Where("id = ?", attempt.TaskActionID).
		First(&action).Error; err != nil {
		return fmt.Errorf("load recovery task action %s: %w", attempt.TaskActionID, err)
	}
	if action.CurrentAttempt > 0 && attempt.AttemptNumber > 0 && action.CurrentAttempt != attempt.AttemptNumber {
		return nil
	}
	if action.Status == "succeeded" {
		return nil
	}

	tx := w.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()
	now := nowRFC3339()
	code := attempt.ErrorCode
	if code == "" {
		code = "generation_recovery_failed"
	}
	message := attempt.ErrorMessage
	if message == "" {
		message = fmt.Sprintf("generation attempt ended in %s during recovery", attempt.Status)
	}
	result := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", attempt.TaskActionID).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_code":    code,
			"error_message": message,
			"completed_at":  now,
			"updated_at":    now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("recovery task action not found: %s", attempt.TaskActionID)
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	committed = true
	return w.notifyTerminal(ctx, attempt)
}

func (w *RecoveryWorker) notifyTerminal(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if w.onTerminal == nil {
		return nil
	}
	return w.onTerminal(ctx, attempt)
}

func (w *RecoveryWorker) defaultFileVerifier(artifact *GenerationArtifact) (*FileVerifyResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact is nil")
	}

	relative := strings.TrimSpace(artifact.RelativePath)
	if relative == "" {
		relative = strings.TrimSpace(artifact.StorageKey)
	}
	if relative == "" || filepath.IsAbs(relative) {
		return &FileVerifyResult{Exists: false}, nil
	}
	cleaned := filepath.Clean(relative)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("artifact path escapes data dir: %s", relative)
	}

	baseAbs, err := filepath.Abs(w.config.DataDir)
	if err != nil {
		return nil, err
	}
	candidateAbs, err := filepath.Abs(filepath.Join(baseAbs, cleaned))
	if err != nil {
		return nil, err
	}
	if !pathWithinBase(baseAbs, candidateAbs) {
		return nil, fmt.Errorf("artifact path escapes data dir: %s", relative)
	}
	resolved, err := filepath.EvalSymlinks(candidateAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileVerifyResult{Exists: false, FilePath: candidateAbs}, nil
		}
		return nil, err
	}
	if !pathWithinBase(baseAbs, resolved) {
		return nil, fmt.Errorf("artifact symlink escapes data dir: %s", relative)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileVerifyResult{Exists: false, FilePath: resolved}, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return &FileVerifyResult{Exists: false, FilePath: resolved}, nil
	}
	if artifact.Size > 0 && info.Size() != artifact.Size {
		return nil, fmt.Errorf("artifact size mismatch for %s: expected=%d actual=%d", artifact.ID, artifact.Size, info.Size())
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, file)
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	actualHash := hex.EncodeToString(h.Sum(nil))
	expectedHash := strings.TrimSpace(artifact.Hash)
	if expectedHash == "" {
		expectedHash = strings.TrimSpace(artifact.ContentHash)
	}
	if expectedHash != "" && !strings.EqualFold(expectedHash, actualHash) {
		return nil, fmt.Errorf("artifact hash mismatch for %s", artifact.ID)
	}

	return &FileVerifyResult{Exists: true, FilePath: resolved, Hash: actualHash}, nil
}

func pathWithinBase(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (w *RecoveryWorker) AcquireLease(attemptID, owner string) (bool, error) {
	expiresAt := time.Now().Add(w.config.LeaseDuration).UTC().Format(time.RFC3339)
	now := nowRFC3339()
	result := w.db.Model(&ActionGenerationAttempt{}).
		Where("id = ? AND (lease_owner = ? OR lease_owner = '' OR lease_expires_at = '' OR lease_expires_at < ?)", attemptID, owner, now).
		Updates(map[string]interface{}{
			"lease_owner":      owner,
			"lease_expires_at": expiresAt,
			"updated_at":       now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (w *RecoveryWorker) RenewLease(attemptID, owner string) (bool, error) {
	expiresAt := time.Now().Add(w.config.LeaseDuration).UTC().Format(time.RFC3339)
	result := w.db.Model(&ActionGenerationAttempt{}).
		Where("id = ? AND lease_owner = ?", attemptID, owner).
		Updates(map[string]interface{}{
			"lease_expires_at": expiresAt,
			"updated_at":       nowRFC3339(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (w *RecoveryWorker) ReleaseLease(attemptID, owner string) error {
	result := w.db.Model(&ActionGenerationAttempt{}).
		Where("id = ? AND lease_owner = ?", attemptID, owner).
		Updates(map[string]interface{}{
			"lease_owner":      "",
			"lease_expires_at": "",
			"updated_at":       nowRFC3339(),
		})
	return result.Error
}

func (w *RecoveryWorker) computePollInterval(attempt *ActionGenerationAttempt) time.Duration {
	if attempt.RetryAfterHint > 0 {
		retryAfter := time.Duration(attempt.RetryAfterHint) * time.Second
		if retryAfter > 0 {
			return retryAfter
		}
	}
	n := attempt.PollCount
	if n < 0 {
		n = 0
	}
	base := float64(w.config.PollBaseInterval)
	max := float64(w.config.PollMaxInterval)
	backoff := math.Min(base*math.Pow(2, float64(n)), max)
	jitterMax := int64(w.config.PollBaseInterval)
	if jitterMax <= 0 {
		jitterMax = 1
	}
	jitter := time.Duration(rand.Int63n(jitterMax))
	interval := time.Duration(backoff) + jitter
	if interval > w.config.PollMaxInterval {
		interval = w.config.PollMaxInterval
	}
	if interval < w.config.PollBaseInterval {
		interval = w.config.PollBaseInterval
	}
	return interval
}
