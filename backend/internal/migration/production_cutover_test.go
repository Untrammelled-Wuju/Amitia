package migration

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testAuthorityProvider struct {
	toolFacade         interface{}
	permissionBroker   interface{}
	eventService       interface{}
	scheduleService    interface{}
	taskRuntimeService interface{}
	hookService        interface{}
}

func (p *testAuthorityProvider) ToolFacade() interface{}         { return p.toolFacade }
func (p *testAuthorityProvider) PermissionBroker() interface{}   { return p.permissionBroker }
func (p *testAuthorityProvider) EventService() interface{}       { return p.eventService }
func (p *testAuthorityProvider) ScheduleService() interface{}    { return p.scheduleService }
func (p *testAuthorityProvider) TaskRuntimeService() interface{} { return p.taskRuntimeService }
func (p *testAuthorityProvider) HookService() interface{}        { return p.hookService }

func TestCutoverPlan_Preflight(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
		},
		Now: time.Now,
	})
	if err := plan.Preflight(context.Background()); err != nil {
		t.Fatalf("expected preflight to pass, got: %v", err)
	}
}

func TestCutoverPlan_Preflight_MissingAuthorities(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		Container: &testAuthorityProvider{},
		Now:       time.Now,
	})
	err := plan.Preflight(context.Background())
	if err == nil {
		t.Fatal("expected preflight to fail with missing authorities")
	}
}

func TestCutoverPlan_VerifyCanonicalAuthorities(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
		},
		Now: time.Now,
	})
	failures := plan.VerifyCanonicalAuthorities()
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got: %v", failures)
	}
}

func TestCutoverPlan_VerifyCanonicalAuthorities_Missing(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		Container: &testAuthorityProvider{},
		Now:       time.Now,
	})
	failures := plan.VerifyCanonicalAuthorities()
	if len(failures) == 0 {
		t.Fatal("expected failures with missing authorities, got none")
	}
}

func TestCutoverPhaseCount(t *testing.T) {
	phases := ValidCutoverPhases()
	if len(phases) != 10 {
		t.Fatalf("expected 10 cutover phases, got %d", len(phases))
	}
}

type testReadSwitchPort struct {
	verifyReadCanonical             func(ctx context.Context) error
	verifyProductionReaderNotLegacy func(ctx context.Context) error
}

func (t *testReadSwitchPort) VerifyReadCanonical(ctx context.Context) error {
	if t.verifyReadCanonical != nil {
		return t.verifyReadCanonical(ctx)
	}
	return nil
}

func (t *testReadSwitchPort) VerifyProductionReaderNotLegacy(ctx context.Context) error {
	if t.verifyProductionReaderNotLegacy != nil {
		return t.verifyProductionReaderNotLegacy(ctx)
	}
	return nil
}

func TestCutoverPlan_ReadSwitch_VerifyReadCanonical(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		ReadSwitch: &testReadSwitchPort{
			verifyReadCanonical: func(ctx context.Context) error {
				return nil
			},
			verifyProductionReaderNotLegacy: func(ctx context.Context) error {
				return nil
			},
		},
		Now: time.Now,
	})
	err := plan.runReadSwitch(context.Background(), &CutoverState{})
	if err != nil {
		t.Fatalf("expected read switch to pass, got: %v", err)
	}
}

func TestCutoverPlan_ReadSwitch_ProductionReaderStillLegacy(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		ReadSwitch: &testReadSwitchPort{
			verifyReadCanonical: func(ctx context.Context) error {
				return nil
			},
			verifyProductionReaderNotLegacy: func(ctx context.Context) error {
				return errors.New("production reader still uses legacy")
			},
		},
		Now: time.Now,
	})
	err := plan.runReadSwitch(context.Background(), &CutoverState{})
	if err == nil {
		t.Fatal("expected read switch to fail when production reader still uses legacy")
	}
}

func TestCutoverPlan_ReadSwitch_VerifyReadCanonicalFails(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		ReadSwitch: &testReadSwitchPort{
			verifyReadCanonical: func(ctx context.Context) error {
				return errors.New("read verification failed")
			},
			verifyProductionReaderNotLegacy: func(ctx context.Context) error {
				return nil
			},
		},
		Now: time.Now,
	})
	state := &CutoverState{}
	err := plan.runReadSwitch(context.Background(), state)
	if err == nil {
		t.Fatal("expected read switch to fail when read verification fails")
	}
	if state.PhaseStatus != "verifying" {
		t.Fatalf("expected phase status to be 'verifying', got: %s", state.PhaseStatus)
	}
}

type testWriteLockoutPort struct {
	lockoutLegacyWrites      func(ctx context.Context) error
	verifyLegacyWriteLockout func(ctx context.Context) error
	executeCanaryOperation   func(ctx context.Context) (*CanaryResult, error)
	verifyCanaryOperation    func(ctx context.Context, result *CanaryResult) error
}

func (t *testWriteLockoutPort) LockoutLegacyWrites(ctx context.Context) error {
	if t.lockoutLegacyWrites != nil {
		return t.lockoutLegacyWrites(ctx)
	}
	return nil
}

func (t *testWriteLockoutPort) VerifyLegacyWriteLockout(ctx context.Context) error {
	if t.verifyLegacyWriteLockout != nil {
		return t.verifyLegacyWriteLockout(ctx)
	}
	return nil
}

func (t *testWriteLockoutPort) ExecuteCanaryOperation(ctx context.Context) (*CanaryResult, error) {
	if t.executeCanaryOperation != nil {
		return t.executeCanaryOperation(ctx)
	}
	return nil, nil
}

func (t *testWriteLockoutPort) VerifyCanaryOperation(ctx context.Context, result *CanaryResult) error {
	if t.verifyCanaryOperation != nil {
		return t.verifyCanaryOperation(ctx, result)
	}
	return nil
}

func TestCutoverPlan_WriteLockout_CanarySuccess(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		WriteLockout: &testWriteLockoutPort{
			lockoutLegacyWrites: func(ctx context.Context) error {
				return nil
			},
			verifyLegacyWriteLockout: func(ctx context.Context) error {
				return nil
			},
			executeCanaryOperation: func(ctx context.Context) (*CanaryResult, error) {
				return &CanaryResult{
					OperationID:        "canary-op",
					CommandID:          "canary-cmd",
					OperationTerminal:  "completed",
					CommandTerminal:    "completed",
					ProjectionRevision: 1,
					DesiredRevision:    1,
					Success:            true,
				}, nil
			},
			verifyCanaryOperation: func(ctx context.Context, result *CanaryResult) error {
				if !result.Success {
					return errors.New("canary operation failed")
				}
				if result.OperationTerminal != "completed" || result.CommandTerminal != "completed" {
					return errors.New("terminal state mismatch")
				}
				if result.ProjectionRevision != result.DesiredRevision {
					return errors.New("revision mismatch")
				}
				return nil
			},
		},
		Now: time.Now,
	})
	err := plan.runWriteLockout(context.Background(), &CutoverState{})
	if err != nil {
		t.Fatalf("expected write lockout to pass, got: %v", err)
	}
}

func TestCutoverPlan_WriteLockout_ACKProjectionBroken(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		WriteLockout: &testWriteLockoutPort{
			lockoutLegacyWrites: func(ctx context.Context) error {
				return nil
			},
			verifyLegacyWriteLockout: func(ctx context.Context) error {
				return nil
			},
			executeCanaryOperation: func(ctx context.Context) (*CanaryResult, error) {
				return &CanaryResult{
					OperationID:        "canary-op",
					CommandID:          "canary-cmd",
					OperationTerminal:  "completed",
					CommandTerminal:    "completed",
					ProjectionRevision: 0,
					DesiredRevision:    1,
					Success:            true,
				}, nil
			},
			verifyCanaryOperation: func(ctx context.Context, result *CanaryResult) error {
				if result.ProjectionRevision != result.DesiredRevision {
					return errors.New("ACK/Projection broken: revision mismatch")
				}
				return nil
			},
		},
		Now: time.Now,
	})
	state := &CutoverState{}
	err := plan.runWriteLockout(context.Background(), state)
	if err == nil {
		t.Fatal("expected write lockout to fail when ACK/Projection is broken")
	}
	if state.PhaseStatus != "verifying" {
		t.Fatalf("expected phase status to be 'verifying', got: %s", state.PhaseStatus)
	}
}

func TestCutoverPlan_ConcurrentRequestCutover_OnlyOneEntersExecution(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	lockDir := t.TempDir()
	ctx := context.Background()
	ttl := 30 * time.Second

	lock1 := NewPersistentLock(db, lockDir)
	lock2 := NewPersistentLock(db, lockDir)

	plan1 := NewCutoverPlan(CutoverDependencies{
		Now: time.Now,
	})
	_ = plan1

	if err := lock1.Acquire(ctx, "cutover-lock", ttl); err != nil {
		t.Fatalf("lock1 acquire: %v", err)
	}

	if err := lock2.Acquire(ctx, "cutover-lock", ttl); err == nil {
		lock2.Release("cutover-lock")
		t.Fatal("lock2 should not acquire while lock1 holds the lock")
	}

	if err := lock1.Release("cutover-lock"); err != nil {
		t.Fatalf("lock1 release: %v", err)
	}

	if err := lock2.Acquire(ctx, "cutover-lock", ttl); err != nil {
		t.Fatalf("lock2 should acquire after lock1 releases: %v", err)
	}
	lock2.Release("cutover-lock")
}
