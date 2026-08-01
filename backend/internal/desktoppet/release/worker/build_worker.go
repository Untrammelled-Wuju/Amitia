package worker

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/log"
)

type ReleaseBuildWorker struct {
	repo           release.ReleaseRepository
	sequenceAlloc  release.SequenceAllocator
	leaseManager   LeaseManagerPort
	buildHandler   BuildHandler
	checkInterval  time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	running        bool
}

type LeaseManagerPort interface {
	IsLeaseExpired(op *release.ReleaseBuildOperation) bool
}

type BuildHandler interface {
	HandleBuild(ctx context.Context, op *release.ReleaseBuildOperation) error
}

func NewReleaseBuildWorker(
	repo release.ReleaseRepository,
	sequenceAlloc release.SequenceAllocator,
	leaseManager LeaseManagerPort,
	buildHandler BuildHandler,
) *ReleaseBuildWorker {
	return &ReleaseBuildWorker{
		repo:          repo,
		sequenceAlloc: sequenceAlloc,
		leaseManager:  leaseManager,
		buildHandler:  buildHandler,
		checkInterval: 10 * time.Second,
		stopCh:        make(chan struct{}),
	}
}

func (w *ReleaseBuildWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	w.wg.Add(1)
	go w.run(ctx)
	log.Logger.Info("Release build worker started")
}

func (w *ReleaseBuildWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()
	w.wg.Wait()
	log.Logger.Info("Release build worker stopped")
}

func (w *ReleaseBuildWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processPendingOperations(ctx); err != nil {
				log.Logger.Warnf("Release build worker scan failed: %v", err)
			}
		}
	}
}

func (w *ReleaseBuildWorker) processPendingOperations(ctx context.Context) error {
	ops, err := w.repo.ListPendingBuildOperations()
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.State != release.BuildOpStateCreated {
			continue
		}
		if op.LeaseOwner != "" && !w.leaseManager.IsLeaseExpired(op) {
			continue
		}
		if err := w.acquireAndProcess(ctx, op); err != nil {
			log.Logger.Warnf("Failed to process operation %s: %v", op.ID, err)
		}
	}
	return nil
}

func (w *ReleaseBuildWorker) acquireAndProcess(ctx context.Context, op *release.ReleaseBuildOperation) error {
	op.State = release.BuildOpStateBuilding
	op.UpdatedAt = formatTimestamp(time.Now())
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return err
	}

	if err := w.buildHandler.HandleBuild(ctx, op); err != nil {
		op.State = release.BuildOpStateFailedRetryable
		op.ErrorCode = "BUILD_HANDLER_FAILED"
		op.ErrorMessage = err.Error()
		op.UpdatedAt = formatTimestamp(time.Now())
		w.repo.UpdateBuildOperation(op)
		return err
	}

	return nil
}

func formatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
