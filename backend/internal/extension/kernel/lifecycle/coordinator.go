package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Coordinator struct {
	mu             sync.Mutex
	registry       *ComponentRegistry
	planner        *Planner
	journal        *Journal
	store          JournalStore
	recovery       *RecoveryScanner
	reconciler     StateReconciler
	readiness      *ReadinessService
	shutdownCoord  *ShutdownCoordinator
	drain          *DrainController
	audit          LifecycleAuditWriter

	startupID    string
	startupInProg bool
	plan         *BootstrapPlan
	bootstrapped bool
	startedAt    time.Time
}

func NewCoordinator(
	registry *ComponentRegistry,
	journal *Journal,
	store JournalStore,
	reconciler StateReconciler,
	readiness *ReadinessService,
	shutdownCoord *ShutdownCoordinator,
	drain *DrainController,
	audit LifecycleAuditWriter,
	recovery *RecoveryScanner,
) *Coordinator {
	return &Coordinator{
		registry:      registry,
		planner:        NewPlanner(registry),
		journal:        journal,
		store:          store,
		reconciler:     reconciler,
		readiness:      readiness,
		shutdownCoord:  shutdownCoord,
		drain:          drain,
		audit:          audit,
		recovery:       recovery,
	}
}

func (c *Coordinator) Bootstrap(ctx context.Context) (*BootstrapPlan, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.bootstrapped {
		c.mu.Unlock()
		return c.plan, nil
	}
	c.startupID = newID("startup")
	plan, err := c.planner.BuildPlan(c.startupID)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.plan = plan
	c.bootstrapped = true
	c.startedAt = now()
	c.mu.Unlock()

	if c.audit != nil {
		c.audit.RecordStartupEvent(ctx, StartupAuditEvent{
			StartupID: c.startupID,
			Phase:     StartupPhaseCore,
			Status:    string(StartupStatusStarted),
			Timestamp: now(),
			Metadata: map[string]any{
				"plan_hash": plan.PlanHash,
				"components": len(plan.Components),
			},
		})
	}
	return plan, nil
}

func (c *Coordinator) Startup(ctx context.Context, globalTimeout time.Duration) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	if c.startupInProg {
		c.mu.Unlock()
		return ErrStartupInProgress
	}
	c.startupInProg = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.startupInProg = false
		c.mu.Unlock()
	}()

	if !c.bootstrapped {
		if _, err := c.Bootstrap(ctx); err != nil {
			return err
		}
	}

	startupCtx, cancel := context.WithTimeout(ctx, effectiveStopTimeout(globalTimeout, 5*time.Minute))
	defer cancel()

	recoveryReport, err := c.recovery.Scan(startupCtx, c.startupID)
	if err != nil {
		return fmt.Errorf("recovery scan failed: %w", err)
	}
	if c.audit != nil {
		c.audit.RecordStartupEvent(startupCtx, StartupAuditEvent{
			StartupID: c.startupID,
			Phase:     StartupPhaseSecurityRecovery,
			Status:    string(StartupStatusStarted),
			Timestamp: now(),
			Metadata: map[string]any{
				"clean_shutdown":          recoveryReport.CleanShutdown,
				"interrupted_components":  recoveryReport.InterruptedComponents,
				"high_risk_items":         len(recoveryReport.HighRiskItems),
			},
		})
	}

	reconReport := c.reconciler.Inspect(startupCtx)
	reconPlan := c.reconciler.Plan(startupCtx, reconReport)
	c.reconciler.Apply(startupCtx, reconPlan)

	phaseGroups := c.planner.OrderByPhase(c.plan)
	for _, group := range phaseGroups {
		if err := c.startGroup(startupCtx, group); err != nil {
			return err
		}
	}

	c.readiness.SetReady(c.startupID, true, "all_components_started")
	if c.audit != nil {
		c.audit.RecordStartupEvent(startupCtx, StartupAuditEvent{
			StartupID: c.startupID,
			Phase:     StartupPhaseReady,
			Status:    string(StartupStatusReady),
			Timestamp: now(),
		})
	}
	return nil
}

func (c *Coordinator) startGroup(ctx context.Context, group []BootstrapComponent) error {
	for _, meta := range group {
		component, ok := c.registry.Get(meta.ID)
		if !ok {
			if meta.Required {
				return wrapLifecycleError(meta.ID, string(meta.Phase), "component_missing", ErrComponentNotFound)
			}
			continue
		}
		if err := c.startComponent(ctx, meta, component); err != nil {
			return err
		}
	}
	return nil
}

func (c *Coordinator) startComponent(ctx context.Context, meta BootstrapComponent, component LifecycleComponent) error {
	timeout := meta.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	startCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	enteredStatus := StartupStatusStarting
	attempt := 0
	maxAttempts := 1
	if meta.RetryPolicy.MaxAttempts > 1 {
		maxAttempts = meta.RetryPolicy.MaxAttempts
	}

	var lastErr error
	skipComponent := false
	for attempt = 1; attempt <= maxAttempts; attempt++ {
		entry := StartupJournalEntry{
			StartupID:   c.startupID,
			ComponentID: meta.ID,
			Phase:       meta.Phase,
			Status:      enteredStatus,
			Attempt:     attempt,
			StartedAt:   now(),
		}
		c.journal.RecordStartup(entry)
		if c.store != nil {
			_ = c.store.PersistStartup(ctx, entry)
		}
		err := component.Start(startCtx)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		finishedAt := now()
		entry.FinishedAt = &finishedAt
		entry.Status = StartupStatusFailed
		entry.ErrorCode = "start_failed"
		entry.Error = err.Error()
		c.journal.RecordStartup(entry)
		if c.store != nil {
			_ = c.store.PersistStartup(ctx, entry)
		}
		if c.audit != nil {
			c.audit.RecordStartupEvent(ctx, StartupAuditEvent{
				StartupID:   c.startupID,
				ComponentID: meta.ID,
				Phase:       meta.Phase,
				Attempt:     attempt,
				Status:      string(StartupStatusFailed),
				ErrorCode:   "start_failed",
				Error:       err.Error(),
				Timestamp:   finishedAt,
			})
		}
		switch meta.FailureMode {
		case FailureModeFailFast, "":
			if meta.Required {
				return wrapLifecycleError(meta.ID, string(meta.Phase), "start_failed", err)
			}
			skipComponent = true
		case FailureModeDegrade:
			skipComponent = true
		case FailureModeSkip:
			skipComponent = true
		case FailureModeRetry:
			if !isRetryable(err) || attempt == maxAttempts {
				if meta.Required {
					return wrapLifecycleError(meta.ID, string(meta.Phase), "start_failed", err)
				}
				skipComponent = true
				break
			}
			delay := meta.RetryPolicy.InitialDelay
			if delay <= 0 {
				delay = 100 * time.Millisecond
			}
			select {
			case <-startCtx.Done():
				return startCtx.Err()
			case <-time.After(delay):
			}
			continue
		case FailureModeQuarantine:
			skipComponent = true
		case FailureModeManualRecovery:
			if meta.Required {
				return wrapLifecycleError(meta.ID, string(meta.Phase), "manual_recovery_required", err)
			}
			skipComponent = true
		}
		if skipComponent {
			break
		}
	}
	if skipComponent {
		return nil
	}
	if lastErr != nil {
		return wrapLifecycleError(meta.ID, string(meta.Phase), "start_failed", lastErr)
	}

	if err := c.checkReady(ctx, meta, component); err != nil {
		return err
	}

	finishedAt := now()
	entry := StartupJournalEntry{
		StartupID:   c.startupID,
		ComponentID: meta.ID,
		Phase:       meta.Phase,
		Status:      StartupStatusReady,
		Attempt:     attempt,
		StartedAt:   finishedAt,
		FinishedAt:  &finishedAt,
	}
	c.journal.RecordStartup(entry)
	if c.store != nil {
		_ = c.store.PersistStartup(ctx, entry)
	}
	if c.audit != nil {
		c.audit.RecordStartupEvent(ctx, StartupAuditEvent{
			StartupID:   c.startupID,
			ComponentID: meta.ID,
			Phase:       meta.Phase,
			Attempt:     attempt,
			Status:      string(StartupStatusReady),
			Timestamp:   finishedAt,
		})
	}
	return nil
}

func (c *Coordinator) checkReady(ctx context.Context, meta BootstrapComponent, component LifecycleComponent) error {
	timeout := meta.ReadyTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := component.Ready(readyCtx); err != nil {
		if meta.Required {
			return wrapLifecycleError(meta.ID, string(meta.Phase), "ready_failed", err)
		}
	}
	return nil
}

func (c *Coordinator) Shutdown(ctx context.Context, reason ShutdownReason, timeout time.Duration) error {
	return c.shutdownCoord.Shutdown(ctx, reason, timeout)
}

func (c *Coordinator) Plan() *BootstrapPlan {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.plan
}

func (c *Coordinator) StartupID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.startupID
}

func (c *Coordinator) IsReady() bool {
	return c.readiness.IsReady()
}

func (c *Coordinator) CheckReadiness(ctx context.Context) ReadinessReport {
	return c.readiness.Check(ctx)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var le *LifecycleError
	if errors.As(err, &le) {
		switch le.Code {
		case "schema_incompatible", "permission_denied", "path_safety", "definition_corrupt", "circular_dependency":
			return false
		}
	}
	switch err {
	case ErrCircularDependency, ErrMissingDependency, ErrCoreComponentFailed:
		return false
	}
	return true
}
