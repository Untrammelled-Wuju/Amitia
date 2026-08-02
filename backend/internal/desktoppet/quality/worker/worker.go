// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"sync"
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
)

type Worker struct {
	db         *gorm.DB
	qualitySvc quality.QualityService
	stopCh     chan struct{}
	wg         sync.WaitGroup
	sem        chan struct{}
}

func NewWorker(db *gorm.DB, svc quality.QualityService, dataDir string) *Worker {
	return &Worker{
		db:         db,
		qualitySvc: svc,
		stopCh:     make(chan struct{}),
		sem:        make(chan struct{}, maxConcurrentEvaluations),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.recoverStuckEvaluations(ctx)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(qualityPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAndProcess(ctx)
		}
	}
}

func (w *Worker) pollAndProcess(ctx context.Context) {
	evals, err := w.listPendingEvaluations(ctx)
	if err != nil {
		log.Logger.Errorf("quality worker poll evaluations failed: %v", err)
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
	executionID := "quality-" + uuid.New().String()

	acquired, err := w.acquireLease(ctx, eval.ID, executionID)
	if err != nil {
		log.Logger.Errorf("quality worker acquire lease for evaluation %s failed: %v", eval.ID, err)
		return
	}
	if !acquired {
		log.Logger.Infof("quality evaluation %s already claimed by another worker", eval.ID)
		return
	}
	log.Logger.Infof("quality worker claimed evaluation %s (execution=%s)", eval.ID, executionID)

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
	}

	_, err = w.executeEvaluation(ctx, eval, req)
	if err != nil {
		log.Logger.Errorf("quality worker execute evaluation %s failed: %v", eval.ID, err)
	}

	if releaseErr := w.releaseLease(ctx, eval.ID); releaseErr != nil {
		log.Logger.Errorf("quality worker release lease for evaluation %s failed: %v", eval.ID, releaseErr)
	}
}

func (w *Worker) recoverStuckEvaluations(ctx context.Context) {
	evals, err := w.listStuckEvaluations(ctx)
	if err != nil {
		log.Logger.Errorf("list stuck quality evaluations failed: %v", err)
		return
	}
	for _, eval := range evals {
		if ctx.Err() != nil {
			return
		}
		now := time.Now().UTC()
		var leaseExpires time.Time
		if eval.LeaseExpiresAt != "" {
			if t, parseErr := time.Parse(qualityTimeFormat, eval.LeaseExpiresAt); parseErr == nil {
				leaseExpires = t
			}
		}
		if !leaseExpires.IsZero() && now.Before(leaseExpires) {
			continue
		}

		eval.ExecutionStatus = quality.EvalPending
		eval.ExecutionID = ""
		eval.WorkerID = ""
		eval.LeaseExpiresAt = ""
		if updateErr := w.updateEvaluation(ctx, eval); updateErr != nil {
			log.Logger.Errorf("recover stuck quality evaluation %s failed: %v", eval.ID, updateErr)
			continue
		}
		log.Logger.Infof("recovered stuck quality evaluation: %s", eval.ID)
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
				return
			case <-ticker.C:
				ok, err := w.renewLease(ctx, evaluationID, executionID)
				if err != nil {
					log.Logger.Errorf("renew quality lease evaluation %s failed: %v", evaluationID, err)
					continue
				}
				if !ok {
					log.Logger.Warnf("quality lease lost for evaluation %s, stopping heartbeat", evaluationID)
					leaseLostCancel()
					return
				}
			}
		}
	}()
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

func (w *Worker) releaseLease(ctx context.Context, evaluationID string) error {
	type releaser interface {
		ReleaseLease(ctx context.Context, evaluationID string) error
	}
	svc, ok := w.qualitySvc.(releaser)
	if !ok {
		return nil
	}
	return svc.ReleaseLease(ctx, evaluationID)
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

func (w *Worker) updateEvaluation(ctx context.Context, eval *quality.QualityEvaluation) error {
	type updater interface {
		UpdateEvaluation(ctx context.Context, eval *quality.QualityEvaluation) error
	}
	svc, ok := w.qualitySvc.(updater)
	if !ok {
		return nil
	}
	return svc.UpdateEvaluation(ctx, eval)
}

func (w *Worker) composeProfile(eval *quality.QualityEvaluation) quality.QualityProfileSnapshot {
	return quality.QualityProfileSnapshot{
		QualityMode: eval.QualityMode,
	}
}
