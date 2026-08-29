package generation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ProviderQueryResult struct {
	ProviderStatus string
	IsCompleted    bool
	IsFailed       bool
	RetryAfterHint int
	RawMetadata    string
	ImageURLs      []string
}

type ProviderQueryFunc func(ctx context.Context, attempt *ActionGenerationAttempt) (*ProviderQueryResult, error)

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
	db            *gorm.DB
	attemptRepo   AttemptRepository
	artifactRepo  ArtifactRepository
	receiptRepo   ReceiptRepository
	config        RecoveryWorkerConfig
	providerQuery ProviderQueryFunc
	fileVerifier  FileVerifyFunc
	stopCh        chan struct{}
	wg            sync.WaitGroup
	lifecycleMu   sync.Mutex
	running       bool
	alive         atomic.Bool
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
			_ = w.scanOnce(ctx)
		}
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
		return w.handleUnknownSubmission(ctx, attempt)
	case AttemptStatusReconcilingSubmission:
		return w.handleReconcilingSubmission(ctx, attempt)
	case AttemptStatusSubmitted, AttemptStatusPolling:
		return w.handlePolling(ctx, attempt)
	case AttemptStatusResultReceived, AttemptStatusPersisting:
		return w.handleRepersist(ctx, attempt)
	case AttemptStatusPublishFailed:
		return w.handlePublishFailed(ctx, attempt)
	default:
		return NewGenerationError(ErrCodeRecoveryStatusUnknown, fmt.Sprintf("unsupported recovery status: %s", attempt.Status), nil)
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

	switch receipt.ProviderStatus {
	case "failed", "failed_confirmed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusFailedConfirmed),
			"error_code":    "provider_failed",
			"error_message": fmt.Sprintf("provider status: %s", receipt.ProviderStatus),
			"updated_at":    nowRFC3339(),
		})
	case "succeeded", "completed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	default:
		if w.providerQuery != nil && attempt.ProviderOperationID != "" {
			return w.queryProviderAndUpdate(ctx, attempt)
		}
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusSubmitted),
			"updated_at": nowRFC3339(),
		})
	}
}

func (w *RecoveryWorker) handleReconcilingSubmission(ctx context.Context, attempt *ActionGenerationAttempt) error {
	receipt, err := w.receiptRepo.GetByAttemptID(attempt.ID)
	if err != nil {
		if errors.Is(err, ErrProviderReceiptNotFound) {
			if w.providerQuery != nil && attempt.ProviderOperationID != "" {
				return w.queryProviderAndUpdate(ctx, attempt)
			}
			return nil
		}
		return err
	}

	switch receipt.ProviderStatus {
	case "submitted", "polling", "running", "pending":
		if w.providerQuery != nil && attempt.ProviderOperationID != "" {
			return w.queryProviderAndUpdate(ctx, attempt)
		}
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusSubmitted),
			"updated_at": nowRFC3339(),
		})
	case "failed", "failed_confirmed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusFailedConfirmed),
			"error_code":    "provider_failed",
			"error_message": fmt.Sprintf("provider status: %s", receipt.ProviderStatus),
			"updated_at":    nowRFC3339(),
		})
	case "succeeded", "completed":
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	default:
		if w.providerQuery != nil && attempt.ProviderOperationID != "" {
			return w.queryProviderAndUpdate(ctx, attempt)
		}
		return nil
	}
}

func (w *RecoveryWorker) queryProviderAndUpdate(ctx context.Context, attempt *ActionGenerationAttempt) error {
	result, err := w.providerQuery(ctx, attempt)
	if err != nil {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusSubmitted),
			"error_message": fmt.Sprintf("provider query failed: %v", err),
			"updated_at":    nowRFC3339(),
		})
	}

	if result.IsFailed {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusFailedConfirmed),
			"error_code":    "provider_query_failed",
			"error_message": fmt.Sprintf("provider query returned failed: %s", result.ProviderStatus),
			"updated_at":    nowRFC3339(),
		})
	}

	if result.IsCompleted {
		if result.RetryAfterHint > 0 {
			_ = w.receiptRepo.UpdatePolled(attempt.ID, nowRFC3339(), result.ProviderStatus)
		}
		if result.RawMetadata != "" {
			_ = w.receiptRepo.UpdateCompleted(attempt.ID, nowRFC3339(), "", result.ProviderStatus, result.RawMetadata)
		}
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusResultReceived),
			"updated_at": nowRFC3339(),
		})
	}

	updates := map[string]interface{}{
		"status":       string(AttemptStatusPolling),
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

	allPersisted := true
	for i := range artifacts {
		a := &artifacts[i]
		if a.Status == string(ArtifactStatusStaging) || a.Status == string(ArtifactStatusSaved) {
			if w.fileVerifier != nil {
				verifyResult, verifyErr := w.fileVerifier(a)
				if verifyErr != nil || !verifyResult.Exists {
					allPersisted = false
					continue
				}
			}
			if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
				"status":     string(ArtifactStatusPersisted),
				"updated_at": nowRFC3339(),
			}); err != nil {
				allPersisted = false
				continue
			}
		}
	}

	if !allPersisted {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":        string(AttemptStatusPublishFailed),
			"error_message": "some artifacts failed verification during repersist",
			"updated_at":    nowRFC3339(),
		})
	}

	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":     string(AttemptStatusSucceeded),
		"updated_at": nowRFC3339(),
	})
}

func (w *RecoveryWorker) handlePublishFailed(ctx context.Context, attempt *ActionGenerationAttempt) error {
	artifacts, err := w.artifactRepo.ListArtifactsByAttemptID(attempt.ID)
	if err != nil {
		return err
	}

	hasValidArtifact := false
	for i := range artifacts {
		a := &artifacts[i]
		if a.Status == string(ArtifactStatusPublishFailed) || a.Status == string(ArtifactStatusStaging) {
			if w.fileVerifier != nil {
				verifyResult, verifyErr := w.fileVerifier(a)
				if verifyErr == nil && verifyResult.Exists {
					if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
						"status":     string(ArtifactStatusPersisted),
						"updated_at": nowRFC3339(),
					}); err == nil {
						hasValidArtifact = true
					}
				}
			} else {
				if err := w.artifactRepo.UpdateArtifact(a.ID, map[string]interface{}{
					"status":     string(ArtifactStatusPersisted),
					"updated_at": nowRFC3339(),
				}); err == nil {
					hasValidArtifact = true
				}
			}
		}
	}

	if hasValidArtifact {
		return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
			"status":     string(AttemptStatusSucceeded),
			"updated_at": nowRFC3339(),
		})
	}

	return w.attemptRepo.UpdateAttemptStatus(attempt.ID, map[string]interface{}{
		"status":        string(AttemptStatusFailed),
		"error_code":    "publish_failed_no_valid_artifact",
		"error_message": "no valid artifact found after publish failure",
		"updated_at":    nowRFC3339(),
	})
}

func (w *RecoveryWorker) defaultFileVerifier(artifact *GenerationArtifact) (*FileVerifyResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact is nil")
	}

	var filePath string
	if artifact.RelativePath != "" {
		filePath = filepath.Join(w.config.DataDir, artifact.RelativePath)
	} else if artifact.StorageKey != "" {
		filePath = filepath.Join(w.config.DataDir, artifact.StorageKey)
	} else {
		return &FileVerifyResult{Exists: false}, nil
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileVerifyResult{Exists: false, FilePath: filePath}, nil
		}
		return nil, err
	}

	if info.IsDir() {
		return &FileVerifyResult{Exists: false, FilePath: filePath}, nil
	}

	return &FileVerifyResult{
		Exists:   true,
		FilePath: filePath,
		Hash:     artifact.Hash,
	}, nil
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
