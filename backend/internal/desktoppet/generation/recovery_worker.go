package generation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

type RecoveryWorkerConfig struct {
	ScanInterval     time.Duration
	PollTimeout      time.Duration
	PollBaseInterval time.Duration
	PollMaxInterval  time.Duration
	MaxPollCount     int
	LeaseDuration    time.Duration
}

func DefaultRecoveryWorkerConfig() RecoveryWorkerConfig {
	return RecoveryWorkerConfig{
		ScanInterval:     30 * time.Second,
		PollTimeout:      10 * time.Minute,
		PollBaseInterval: 3 * time.Second,
		PollMaxInterval:  60 * time.Second,
		MaxPollCount:     200,
		LeaseDuration:    5 * time.Minute,
	}
}

type RecoveryWorker struct {
	db           *gorm.DB
	attemptRepo  AttemptRepository
	artifactRepo ArtifactRepository
	receiptRepo  ReceiptRepository
	config       RecoveryWorkerConfig
	stopCh       chan struct{}
}

func NewRecoveryWorker(db *gorm.DB, attemptRepo AttemptRepository, artifactRepo ArtifactRepository, receiptRepo ReceiptRepository, config RecoveryWorkerConfig) *RecoveryWorker {
	return &RecoveryWorker{
		db:           db,
		attemptRepo:  attemptRepo,
		artifactRepo: artifactRepo,
		receiptRepo:  receiptRepo,
		config:       config,
		stopCh:       make(chan struct{}),
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.config.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			_ = w.scanOnce(ctx)
		}
	}
}

func (w *RecoveryWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}

func (w *RecoveryWorker) scanOnce(ctx context.Context) error {
	recoverableStatuses := []string{
		string(AttemptStatusUnknownSubmission),
		string(AttemptStatusReconcilingSubmission),
		string(AttemptStatusSubmitted),
		string(AttemptStatusPolling),
		string(AttemptStatusResultReceived),
		string(AttemptStatusPersisting),
		string(AttemptStatusPublishFailed),
	}
	var attempts []ActionGenerationAttempt
	err := w.db.Where("status IN ?", recoverableStatuses).Limit(100).Find(&attempts).Error
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
		if err != nil || !acquired {
			continue
		}
		_ = w.recoverAttempt(ctx, attempt)
		_ = w.ReleaseLease(attempt.ID, "recovery-worker")
	}
	return nil
}

func (w *RecoveryWorker) recoverAttempt(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt == nil {
		return nil
	}
	status := AttemptStatus(attempt.Status)
	switch status {
	case AttemptStatusUnknownSubmission:
		return w.handleUnknownSubmission(attempt)
	case AttemptStatusReconcilingSubmission:
		return w.handleReconcilingSubmission(attempt)
	case AttemptStatusSubmitted, AttemptStatusPolling:
		return w.handlePolling(ctx, attempt)
	case AttemptStatusResultReceived, AttemptStatusPersisting:
		return w.handleRepersist(attempt)
	case AttemptStatusPublishFailed:
		return w.handlePublishFailed(attempt)
	default:
		return NewGenerationError(ErrCodeRecoveryStatusUnknown, fmt.Sprintf("unsupported recovery status: %s", attempt.Status), nil)
	}
}

func (w *RecoveryWorker) handleUnknownSubmission(attempt *ActionGenerationAttempt) error {
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
			return nil
		}
		return err
	}
	if receipt.ProviderStatus == "failed" || receipt.ProviderStatus == "failed_confirmed" {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusFailedConfirmed),
			"updated_at": nowRFC3339(),
		})
	}
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":     string(AttemptStatusSubmitted),
		"updated_at": nowRFC3339(),
	})
}

func (w *RecoveryWorker) handleReconcilingSubmission(attempt *ActionGenerationAttempt) error {
	receipt, err := w.receiptRepo.GetByAttemptID(attempt.ID)
	if err != nil {
		if errors.Is(err, ErrProviderReceiptNotFound) {
			return nil
		}
		return err
	}
	switch receipt.ProviderStatus {
	case "submitted", "polling", "running":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusSubmitted),
			"updated_at": nowRFC3339(),
		})
	case "failed", "failed_confirmed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusFailedConfirmed),
			"updated_at": nowRFC3339(),
		})
	case "succeeded", "completed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	default:
		return nil
	}
}

func (w *RecoveryWorker) handlePolling(ctx context.Context, attempt *ActionGenerationAttempt) error {
	ok, err := w.RenewLease(attempt.ID, "recovery-worker")
	if err != nil {
		return err
	}
	if !ok {
		return NewGenerationError(ErrCodeRecoveryLeaseConflict, "lease renew failed during polling", nil)
	}
	return w.pollAttempt(ctx, attempt)
}

func (w *RecoveryWorker) pollAttempt(ctx context.Context, attempt *ActionGenerationAttempt) error {
	if attempt.PollCount >= w.config.MaxPollCount {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusManualReview),
			"updated_at": nowRFC3339(),
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
		"poll_count":  attempt.PollCount + 1,
		"updated_at":  nowRFC3339(),
		"heartbeat_at": nowRFC3339(),
	})
}

func (w *RecoveryWorker) handleRepersist(attempt *ActionGenerationAttempt) error {
	artifacts, err := w.artifactRepo.ListArtifactsByAttemptID(attempt.ID)
	if err != nil {
		return err
	}
	for i := range artifacts {
		a := &artifacts[i]
		if a.Status == string(ArtifactStatusStaging) || a.Status == string(ArtifactStatusSaved) {
			if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
				"status":     string(ArtifactStatusPersisted),
				"updated_at": nowRFC3339(),
			}); err != nil {
				return err
			}
		}
	}
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":     string(AttemptStatusSucceeded),
		"updated_at": nowRFC3339(),
	})
}

func (w *RecoveryWorker) handlePublishFailed(attempt *ActionGenerationAttempt) error {
	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":     string(AttemptStatusPersisting),
		"updated_at": nowRFC3339(),
	})
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
