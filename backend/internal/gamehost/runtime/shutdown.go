package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ShutdownCoordinator struct {
	executor    RuntimeExecutor
	maxParallel int
	timeout     time.Duration
}

func NewShutdownCoordinator(executor RuntimeExecutor, maxParallel int, timeout time.Duration) (*ShutdownCoordinator, error) {
	if executor == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "executor must not be nil"}
	}
	if maxParallel <= 0 {
		maxParallel = 4
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &ShutdownCoordinator{
		executor:    executor,
		maxParallel: maxParallel,
		timeout:     timeout,
	}, nil
}

func (s *ShutdownCoordinator) ShutdownAll(ctx context.Context, runtimeIDs []domain.RuntimeInstanceID) error {
	if len(runtimeIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout*2)
	defer cancel()

	sem := make(chan struct{}, s.maxParallel)
	var wg sync.WaitGroup
	errCh := make(chan error, len(runtimeIDs))

	for _, rtID := range runtimeIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(id domain.RuntimeInstanceID) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.executor.StopRuntime(ctx, id); err != nil {
				errCh <- err
			}
		}(rtID)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return &RuntimeStopError{
			RuntimeID:  "multi-runtime",
			StopErrors: errs,
		}
	}

	return nil
}

func (s *ShutdownCoordinator) ForceCleanupAll(ctx context.Context, runtimeIDs []domain.RuntimeInstanceID) error {
	if len(runtimeIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(runtimeIDs))

	for _, rtID := range runtimeIDs {
		wg.Add(1)
		go func(id domain.RuntimeInstanceID) {
			defer wg.Done()
			if err := s.executor.CleanupRuntime(ctx, id); err != nil {
				errCh <- err
			}
		}(rtID)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return &RuntimeStopError{
			RuntimeID:     "multi-runtime",
			CleanupErrors: errs,
		}
	}

	return nil
}
