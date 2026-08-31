// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	qualityLeaseDuration     = 5 * time.Minute
	qualityPollInterval      = 5 * time.Second
	qualityWorkerID          = "quality-worker-1"
	qualityHeartbeatInterval = 30 * time.Second
	qualityTimeFormat        = time.RFC3339
	maxConcurrentEvaluations = 2
	qualityCleanupTimeout    = 5 * time.Second
)

type qualityReliabilityProvider interface {
	RecoveryWorker() quality.QualityRecoveryWorker
	OutboxPublisher() quality.QualityOutboxPublisher
}

type Worker struct {
	db         *gorm.DB
	qualitySvc quality.QualityService
	stopCh     chan struct{}
	wg         sync.WaitGroup
	sem        chan struct{}
	alive      atomic.Bool

	lifecycleMu sync.Mutex
	running     bool
	stopping    bool
	runCancel   context.CancelFunc

	recoveryWorker  quality.QualityRecoveryWorker
	outboxPublisher quality.QualityOutboxPublisher
}

func NewWorker(db *gorm.DB, svc quality.QualityService, dataDir string) *Worker {
	_ = dataDir
	w := &Worker{
		db:         db,
		qualitySvc: svc,
		stopCh:     make(chan struct{}),
		sem:        make(chan struct{}, maxConcurrentEvaluations),
	}
	if provider, ok := svc.(qualityReliabilityProvider); ok {
		w.recoveryWorker = provider.RecoveryWorker()
		w.outboxPublisher = provider.OutboxPublisher()
	}
	return w
}

func (w *Worker) Start(ctx context.Context) {
	w.lifecycleMu.Lock()
	if w.running || w.stopping {
		w.lifecycleMu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.stopCh = make(chan struct{})
	w.runCancel = cancel
	w.running = true
	w.alive.Store(true)
	w.wg.Add(1)
	w.lifecycleMu.Unlock()

	go w.run(runCtx)
}

func (w *Worker) Stop() {
	w.lifecycleMu.Lock()
	if !w.running || w.stopping {
		w.lifecycleMu.Unlock()
		return
	}
	w.stopping = true
	stopCh := w.stopCh
	cancel := w.runCancel
	close(stopCh)
	if cancel != nil {
		cancel()
	}
	w.lifecycleMu.Unlock()

	// Every evaluation and heartbeat is attached to runCtx and participates in
	// this wait group, so Stop cannot hang on work that ignores the lifecycle.
	w.wg.Wait()

	w.lifecycleMu.Lock()
	w.running = false
	w.stopping = false
	w.runCancel = nil
	w.alive.Store(false)
	w.lifecycleMu.Unlock()
}

func (w *Worker) IsRunning() bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	return w.running && !w.stopping && w.alive.Load()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)

	if w.recoveryWorker != nil {
		if _, err := w.recoveryWorker.RecoverStuckEvaluations(ctx); err != nil && ctx.Err() == nil {
			log.Logger.Errorf("initial quality recovery failed: %v", err)
		}
		w.recoveryWorker.Start(ctx)
		defer w.recoveryWorker.Stop()
	} else {
		w.recoverStuckEvaluations(ctx)
	}
	w.flushOutbox(ctx)

	ticker := time.NewTicker(qualityPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.flushOutbox(ctx)
			w.pollAndProcess(ctx)
		}
	}
}

func (w *Worker) pollAndProcess(ctx context.Context) {
	evals, err := w.listPendingEvaluations(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Logger.Errorf("quality worker poll evaluations failed: %v", err)
		}
		return
	}
	if len(evals) == 0 {
		return
	}
	for i := range evals {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case w.sem <- struct{}{}:
		}
		eval := evals[i]
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.sem }()
			w.processEvaluation(ctx, eval)
		}()
	}
}

func (w *Worker) processEvaluation(ctx context.Context, eval *quality.QualityEvaluation) {
	if eval == nil || ctx.Err() != nil {
		return
	}
	executionID := "quality-" + uuid.New().String()

	acquired, err := w.acquireLease(ctx, eval.ID, executionID)
	if err != nil {
		if ctx.Err() == nil {
			log.Logger.Errorf("quality worker acquire lease for evaluation %s failed: %v", eval.ID, err)
		}
		return
	}
	if !acquired {
		log.Logger.Infof("quality evaluation %s already claimed by another worker", eval.ID)
		return
	}
	log.Logger.Infof("quality worker claimed evaluation %s (execution=%s)", eval.ID, executionID)

	// Reload after AcquireLease so the in-memory record reflects the execution
	// owner and any fields changed between poll and claim. Fallback assignments
	// preserve compatibility with alternate QualityService implementations.
	if current, getErr := w.getEvaluationRecord(ctx, eval.ID); getErr == nil && current != nil {
		eval = current
	} else if getErr != nil && ctx.Err() == nil {
		log.Logger.Warnf("quality worker reload evaluation %s after lease failed: %v", eval.ID, getErr)
	}
	eval.ExecutionID = executionID
	eval.WorkerID = qualityWorkerID
	eval.LeaseOwner = qualityWorkerID
	eval.ExecutionStatus = quality.EvalRunning

	evalCtx, evalCancel := context.WithCancel(ctx)
	defer evalCancel()

	heartbeatCtx, heartbeatCancel := context.WithCancel(evalCtx)
	w.startHeartbeat(heartbeatCtx, eval.ID, executionID, evalCancel)
	defer heartbeatCancel()

	profile := w.composeProfile(eval)
	req := quality.EvaluateRequest{
		ActionRevisionID:   eval.ActionRevisionID,
		ProcessingTaskID:   eval.ProcessingTaskID,
		ProcessingActionID: eval.ProcessingActionID,
		ActionKey:          eval.ActionKey,
		Profile:            profile,
		ExecutionID:        executionID,
		WorkerID:           qualityWorkerID,
		ActionContentHash:  eval.ActionContentHash,
	}

	_, execErr := w.executeEvaluation(evalCtx, eval, req)
	if execErr != nil && evalCtx.Err() == nil {
		log.Logger.Errorf("quality worker execute evaluation %s failed: %v", eval.ID, execErr)
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), qualityCleanupTimeout)
	defer cleanupCancel()
	released, releaseErr := w.releaseLease(cleanupCtx, eval.ID, executionID)
	if releaseErr != nil {
		log.Logger.Errorf("quality worker release lease for evaluation %s failed: %v", eval.ID, releaseErr)
	} else if !released {
		log.Logger.Infof("quality worker lease already changed before release for evaluation %s", eval.ID)
	}

	// Shutdown/lease cancellation must not strand a running row with an empty
	// lease. Once release succeeds, a CAS recovery with the now-empty owner turns
	// only that still-running row back to pending; a newly acquired owner wins the
	// race and makes this update a no-op.
	if evalCtx.Err() != nil && released {
		if _, recoverErr := w.recoverExpiredEvaluation(cleanupCtx, eval.ID, "", time.Now().UTC()); recoverErr != nil {
			log.Logger.Errorf("quality worker requeue cancelled evaluation %s failed: %v", eval.ID, recoverErr)
		}
	}

	// Successful commits use the transactional outbox as the sole terminal
	// event source. Flush after each evaluation for low-latency delivery; the
	// periodic flush remains the retry path for transient publisher failures.
	w.flushOutbox(cleanupCtx)
}

func (w *Worker) recoverStuckEvaluations(ctx context.Context) {
	evals, err := w.listStuckEvaluations(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Logger.Errorf("list stuck quality evaluations failed: %v", err)
		}
		return
	}
	now := time.Now().UTC()
	for _, eval := range evals {
		if ctx.Err() != nil {
			return
		}
		var leaseExpires time.Time
		if eval.LeaseExpiresAt != "" {
			if t, parseErr := time.Parse(qualityTimeFormat, eval.LeaseExpiresAt); parseErr == nil {
				leaseExpires = t
			}
		}
		if !leaseExpires.IsZero() && now.Before(leaseExpires) {
			continue
		}
		ok, recoverErr := w.recoverExpiredEvaluation(ctx, eval.ID, eval.ExecutionID, now)
		if recoverErr != nil {
			log.Logger.Errorf("recover stuck quality evaluation %s failed: %v", eval.ID, recoverErr)
			continue
		}
		if ok {
			log.Logger.Infof("recovered stuck quality evaluation: %s", eval.ID)
		}
	}
}

func (w *Worker) startHeartbeat(ctx context.Context, evaluationID, executionID string, leaseLostCancel context.CancelFunc) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(qualityHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				leaseLostCancel()
				return
			case <-ticker.C:
				ok, err := w.renewLease(ctx, evaluationID, executionID)
				if err != nil {
					if ctx.Err() == nil {
						log.Logger.Errorf("renew quality lease evaluation %s failed: %v", evaluationID, err)
					}
					continue
				}
				if !ok {
					log.Logger.Warnf("quality lease lost for evaluation %s, cancelling execution", evaluationID)
					leaseLostCancel()
					return
				}
			}
		}
	}()
}

func (w *Worker) flushOutbox(ctx context.Context) {
	if w.outboxPublisher == nil || ctx.Err() != nil {
		return
	}
	if err := w.outboxPublisher.Flush(ctx); err != nil && ctx.Err() == nil {
		log.Logger.Errorf("quality outbox flush failed: %v", err)
	}
}

func (w *Worker) listPendingEvaluations(ctx context.Context) ([]*quality.QualityEvaluation, error) {
	type lister interface {
		ListPendingEvaluations(ctx context.Context) ([]*quality.QualityEvaluation, error)
	}
	svc, ok := w.qualitySvc.(lister)
	if !ok {
		return w.fallbackListByStatus(ctx, string(quality.EvalPending))
	}
	return svc.ListPendingEvaluations(ctx)
}

func (w *Worker) listStuckEvaluations(ctx context.Context) ([]*quality.QualityEvaluation, error) {
	return w.fallbackListByStatus(ctx, string(quality.EvalRunning))
}

func (w *Worker) fallbackListByStatus(ctx context.Context, status string) ([]*quality.QualityEvaluation, error) {
	var evals []*quality.QualityEvaluation
	err := w.db.WithContext(ctx).
		Where("execution_status = ?", status).
		Order("created_at ASC").
		Find(&evals).Error
	if err != nil {
		return nil, err
	}
	return evals, nil
}

func (w *Worker) acquireLease(ctx context.Context, evaluationID, executionID string) (bool, error) {
	type leaser interface {
		AcquireLease(ctx context.Context, evaluationID, executionID, workerID string, leaseDuration string) (bool, error)
	}
	svc, ok := w.qualitySvc.(leaser)
	if !ok {
		return false, nil
	}
	return svc.AcquireLease(ctx, evaluationID, executionID, qualityWorkerID, qualityLeaseDuration.String())
}

func (w *Worker) renewLease(ctx context.Context, evaluationID, executionID string) (bool, error) {
	type renewer interface {
		RenewLease(ctx context.Context, evaluationID, executionID string, leaseDuration string) (bool, error)
	}
	svc, ok := w.qualitySvc.(renewer)
	if !ok {
		return true, nil
	}
	return svc.RenewLease(ctx, evaluationID, executionID, qualityLeaseDuration.String())
}

func (w *Worker) releaseLease(ctx context.Context, evaluationID, executionID string) (bool, error) {
	type releaser interface {
		ReleaseLease(ctx context.Context, evaluationID, executionID string) (bool, error)
	}
	svc, ok := w.qualitySvc.(releaser)
	if !ok {
		return true, nil
	}
	return svc.ReleaseLease(ctx, evaluationID, executionID)
}

func (w *Worker) recoverExpiredEvaluation(ctx context.Context, evaluationID, executionID string, now time.Time) (bool, error) {
	type recoverer interface {
		RecoverExpiredEvaluation(ctx context.Context, evaluationID, executionID string, now time.Time) (bool, error)
	}
	if svc, ok := w.qualitySvc.(recoverer); ok {
		return svc.RecoverExpiredEvaluation(ctx, evaluationID, executionID, now)
	}
	return quality.NewRepository(w.db).RecoverExpiredEvaluation(ctx, evaluationID, executionID, now)
}

func (w *Worker) getEvaluationRecord(ctx context.Context, evaluationID string) (*quality.QualityEvaluation, error) {
	type getter interface {
		GetEvaluationRecord(ctx context.Context, evaluationID string) (*quality.QualityEvaluation, error)
	}
	if svc, ok := w.qualitySvc.(getter); ok {
		return svc.GetEvaluationRecord(ctx, evaluationID)
	}
	return quality.NewRepository(w.db).GetEvaluation(ctx, evaluationID)
}

func (w *Worker) executeEvaluation(ctx context.Context, eval *quality.QualityEvaluation, req quality.EvaluateRequest) (*quality.EvaluateResult, error) {
	type executor interface {
		ExecuteEvaluation(ctx context.Context, eval *quality.QualityEvaluation, req quality.EvaluateRequest) (*quality.EvaluateResult, error)
	}
	svc, ok := w.qualitySvc.(executor)
	if !ok {
		return nil, nil
	}
	return svc.ExecuteEvaluation(ctx, eval, req)
}

func (w *Worker) composeProfile(eval *quality.QualityEvaluation) quality.QualityProfileSnapshot {
	return quality.QualityProfileSnapshot{
		ProfileID:   eval.ProfileID,
		QualityMode: eval.QualityMode,
	}
}
