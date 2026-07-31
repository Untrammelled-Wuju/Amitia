package editing

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	repo          Repository
	pollInterval  time.Duration
	leaseDuration time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewRecoveryWorker(repo Repository) *RecoveryWorker {
	return &RecoveryWorker{
		repo:          repo,
		pollInterval:  30 * time.Second,
		leaseDuration: 5 * time.Minute,
		stopCh:        make(chan struct{}),
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *RecoveryWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *RecoveryWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	w.recoverStuckJobs(ctx)
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recoverStuckJobs(ctx)
		}
	}
}

func (w *RecoveryWorker) recoverStuckJobs(ctx context.Context) {
	jobs, err := w.repo.ListJobsForRecovery(w.leaseDuration)
	if err != nil {
		log.Logger.Errorf("recovery worker list stuck jobs failed: %v", err)
		return
	}
	for i := range jobs {
		if ctx.Err() != nil {
			return
		}
		job := jobs[i]
		journals, jErr := w.repo.ListJournalsByJob(job.ID)
		if jErr != nil {
			log.Logger.Errorf("recovery worker list journals for job %s failed: %v", job.ID, jErr)
			continue
		}

		var lastState string
		if len(journals) > 0 {
			lastState = journals[len(journals)-1].State
		}

		if lastState == JournalStateFailed {
			if uErr := w.repo.UpdateJobFields(job.ID, map[string]any{
				"status": JobStatusFailedTerminal,
			}); uErr != nil {
				log.Logger.Errorf("recovery worker mark job %s terminal failed: %v", job.ID, uErr)
				continue
			}
			log.Logger.Infof("recovery worker marked job %s as failed_terminal", job.ID)
		} else {
			if uErr := w.repo.UpdateJobFields(job.ID, map[string]any{
				"status":            JobStatusQueued,
				"lease_owner":       "",
				"lease_expires_at":  "",
			}); uErr != nil {
				log.Logger.Errorf("recovery worker requeue job %s failed: %v", job.ID, uErr)
				continue
			}
			log.Logger.Infof("recovery worker requeued stuck job %s", job.ID)
		}
	}
}

type CleanupWorker struct {
	repo         Repository
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

func NewCleanupWorker(repo Repository) *CleanupWorker {
	return &CleanupWorker{
		repo:         repo,
		pollInterval: 1 * time.Hour,
		stopCh:       make(chan struct{}),
	}
}

func (w *CleanupWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *CleanupWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *CleanupWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	w.cleanupExpiredCandidates(ctx)
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanupExpiredCandidates(ctx)
		}
	}
}

func (w *CleanupWorker) cleanupExpiredCandidates(ctx context.Context) {
	candidates, err := w.repo.ListExpiredCandidates(CandidateRetentionDays)
	if err != nil {
		log.Logger.Errorf("cleanup worker list expired candidates failed: %v", err)
		return
	}
	for i := range candidates {
		if ctx.Err() != nil {
			return
		}
		candidate := candidates[i]
		if uErr := w.repo.UpdateCandidateFields(candidate.ID, map[string]any{
			"status": CandidateStatusArchived,
		}); uErr != nil {
			log.Logger.Errorf("cleanup worker archive candidate %s failed: %v", candidate.ID, uErr)
			continue
		}
		log.Logger.Infof("cleanup worker archived candidate %s", candidate.ID)
	}
}
