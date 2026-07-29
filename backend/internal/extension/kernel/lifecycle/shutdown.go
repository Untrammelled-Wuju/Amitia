package lifecycle

import (
	"context"
	"sync"
	"time"
)

type DrainResult struct {
	Drained         []string
	Cancelled       []string
	ForcedRemaining []string
	TimedOut        bool
	CompletedAt     time.Time
}

type DrainController struct {
	mu          sync.Mutex
	reason      ShutdownReason
	rejectNew   bool
	active      map[string]ActiveOperation
	cancelFuncs map[string]context.CancelFunc
}

func NewDrainController() *DrainController {
	return &DrainController{
		active:      make(map[string]ActiveOperation),
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

func (d *DrainController) Begin(reason ShutdownReason) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reason = reason
	d.rejectNew = true
}

func (d *DrainController) RejectNew() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rejectNew
}

func (d *DrainController) Register(op ActiveOperation, cancel context.CancelFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active[op.ID] = op
	if cancel != nil {
		d.cancelFuncs[op.ID] = cancel
	}
}

func (d *DrainController) Complete(opID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.active, opID)
	delete(d.cancelFuncs, opID)
}

func (d *DrainController) ActiveOperations() []ActiveOperation {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]ActiveOperation, 0, len(d.active))
	for _, op := range d.active {
		out = append(out, op)
	}
	return out
}

func (d *DrainController) Wait(ctx context.Context, timeout time.Duration) DrainResult {
	result := DrainResult{}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		d.mu.Lock()
		activeCount := len(d.active)
		d.mu.Unlock()
		if activeCount == 0 {
			result.CompletedAt = now()
			return result
		}
		select {
		case <-ctx.Done():
			result.TimedOut = true
			result.CompletedAt = now()
			return result
		case <-time.After(time.Until(deadline)):
			result.TimedOut = true
			result.CompletedAt = now()
			return result
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			result.TimedOut = true
			result.CompletedAt = now()
			return result
		}
	}
}

func (d *DrainController) CancelRemaining(ctx context.Context) DrainResult {
	d.mu.Lock()
	ids := make([]string, 0, len(d.active))
	for id := range d.active {
		ids = append(ids, id)
	}
	d.mu.Unlock()
	result := DrainResult{CompletedAt: now()}
	for _, id := range ids {
		d.mu.Lock()
		cancel, ok := d.cancelFuncs[id]
		op, opOK := d.active[id]
		d.mu.Unlock()
		if ok && cancel != nil {
			cancel()
		}
		if opOK {
			result.Cancelled = append(result.Cancelled, op.ID)
		}
	}
	return result
}

type ShutdownCoordinator struct {
	registry   *ComponentRegistry
	journal    *Journal
	drain      *DrainController
	audit      LifecycleAuditWriter
	store      JournalStore
	mu         sync.Mutex
	inProgress bool
	shutdownID string
	reason     ShutdownReason
}

func NewShutdownCoordinator(registry *ComponentRegistry, journal *Journal, drain *DrainController, audit LifecycleAuditWriter, store JournalStore) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		registry: registry,
		journal:  journal,
		drain:    drain,
		audit:    audit,
		store:    store,
	}
}

func (s *ShutdownCoordinator) Shutdown(ctx context.Context, reason ShutdownReason, timeout time.Duration) error {
	s.mu.Lock()
	if s.inProgress {
		s.mu.Unlock()
		return ErrShutdownInProgress
	}
	s.inProgress = true
	s.reason = reason
	s.shutdownID = newID("shutdown")
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.inProgress = false
		s.mu.Unlock()
	}()

	s.drain.Begin(reason)
	if s.audit != nil {
		s.audit.RecordShutdownEvent(ctx, ShutdownAuditEvent{
			ShutdownID: s.shutdownID,
			Phase:      ShutdownPhaseRequested,
			Reason:     string(reason),
			Timestamp:  now(),
		})
	}

	drainResult := s.drain.Wait(ctx, timeout/2)
	if drainResult.TimedOut {
		s.drain.CancelRemaining(ctx)
	}

	metas := s.registry.AllMetadata()
	ordered := orderForShutdown(metas)
	for _, meta := range ordered {
		component, ok := s.registry.Get(meta.ID)
		if !ok {
			continue
		}
		entry := ShutdownJournalEntry{
			ShutdownID:  s.shutdownID,
			ComponentID: meta.ID,
			Phase:       ShutdownPhaseStopRuntimes,
			Status:      ShutdownStatusStopping,
			StartedAt:   now(),
		}
		stopCtx, cancel := context.WithTimeout(ctx, effectiveStopTimeout(meta.StopTimeout, timeout))
		err := component.Stop(stopCtx, reason)
		cancel()
		finishedAt := now()
		entry.FinishedAt = &finishedAt
		if err != nil {
			entry.Status = ShutdownStatusFailed
			entry.ErrorCode = "stop_failed"
			entry.Error = err.Error()
		} else {
			entry.Status = ShutdownStatusStopped
		}
		s.journal.RecordShutdown(entry)
		if s.store != nil {
			_ = s.store.PersistShutdown(ctx, entry)
		}
		if s.audit != nil {
			s.audit.RecordShutdownEvent(ctx, ShutdownAuditEvent{
				ShutdownID:  s.shutdownID,
				ComponentID: meta.ID,
				Phase:       ShutdownPhaseStopRuntimes,
				Status:      string(entry.Status),
				Reason:      string(reason),
				Error:       entry.Error,
				Timestamp:   finishedAt,
			})
		}
	}

	clean := !drainResult.TimedOut
	s.journal.MarkCleanShutdown(s.shutdownID, clean)
	if s.store != nil {
		if memStore, ok := s.store.(*InMemoryJournalStore); ok {
			memStore.SetCleanShutdown(clean, s.shutdownID)
		}
	}
	if s.audit != nil {
		s.audit.RecordShutdownEvent(ctx, ShutdownAuditEvent{
			ShutdownID: s.shutdownID,
			Phase:      ShutdownPhaseExit,
			Status:     "complete",
			Reason:     string(reason),
			Clean:      clean,
			Timestamp:  now(),
		})
	}
	return nil
}

func orderForShutdown(metas []BootstrapComponent) []BootstrapComponent {
	out := append([]BootstrapComponent{}, metas...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if phaseOrder(out[j].Phase) > phaseOrder(out[i].Phase) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func effectiveStopTimeout(configured, global time.Duration) time.Duration {
	if configured > 0 && configured < global {
		return configured
	}
	if global <= 0 {
		return 30 * time.Second
	}
	return global
}
