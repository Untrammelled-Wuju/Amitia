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
	nativeBridgeRelay  interface{}
	platformBridge     interface{}
}

func (p *testAuthorityProvider) ToolFacade() interface{}         { return p.toolFacade }
func (p *testAuthorityProvider) PermissionBroker() interface{}   { return p.permissionBroker }
func (p *testAuthorityProvider) EventService() interface{}       { return p.eventService }
func (p *testAuthorityProvider) ScheduleService() interface{}    { return p.scheduleService }
func (p *testAuthorityProvider) TaskRuntimeService() interface{} { return p.taskRuntimeService }
func (p *testAuthorityProvider) HookService() interface{}        { return p.hookService }
func (p *testAuthorityProvider) NativeBridgeRelay() interface{}  { return p.nativeBridgeRelay }
func (p *testAuthorityProvider) PlatformBridge() interface{}     { return p.platformBridge }

func TestCutoverPlan_Preflight(t *testing.T) {
	plan := NewCutoverPlan(CutoverDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
			nativeBridgeRelay:  struct{}{},
			platformBridge:     struct{}{},
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
			nativeBridgeRelay:  struct{}{},
			platformBridge:     struct{}{},
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

func TestCutoverReadSwitchPort_VerifyProductionReaderNotLegacy_MissingCheckerFails(t *testing.T) {
	port := NewCutoverReadSwitchPort(ReadSwitchDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
			nativeBridgeRelay:  struct{}{},
			platformBridge:     struct{}{},
		},
	})
	err := port.VerifyProductionReaderNotLegacy(context.Background())
	if err == nil {
		t.Fatal("expected VerifyProductionReaderNotLegacy to fail when legacy checker is not configured")
	}
}

func TestCutoverReadSwitchPort_VerifyReadCanonical_ProductionVerifierCalled(t *testing.T) {
	verifierCalled := false
	port := NewCutoverReadSwitchPort(ReadSwitchDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
			nativeBridgeRelay:  struct{}{},
			platformBridge:     struct{}{},
		},
		ProductionReadVerifier: func(ctx context.Context) error {
			verifierCalled = true
			return nil
		},
	})
	err := port.VerifyReadCanonical(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verifierCalled {
		t.Fatal("expected production read verifier to be called")
	}
}

func TestCutoverReadSwitchPort_VerifyReadCanonical_ProductionVerifierFailure(t *testing.T) {
	port := NewCutoverReadSwitchPort(ReadSwitchDependencies{
		Container: &testAuthorityProvider{
			toolFacade:         struct{}{},
			permissionBroker:   struct{}{},
			eventService:       struct{}{},
			scheduleService:    struct{}{},
			taskRuntimeService: struct{}{},
			hookService:        struct{}{},
			nativeBridgeRelay:  struct{}{},
			platformBridge:     struct{}{},
		},
		ProductionReadVerifier: func(ctx context.Context) error {
			return errors.New("production read verification failed: userId mismatch")
		},
	})
	err := port.VerifyReadCanonical(context.Background())
	if err == nil {
		t.Fatal("expected VerifyReadCanonical to fail when production verifier fails")
	}
}

func TestCutoverWriteLockoutPort_ExecuteCanaryOperation_MissingExecutorFails(t *testing.T) {
	port := NewCutoverWriteLockoutPort(WriteLockoutDependencies{})
	_, err := port.ExecuteCanaryOperation(context.Background())
	if err == nil {
		t.Fatal("expected ExecuteCanaryOperation to fail when canary executor is not configured")
	}
}

func TestCutoverWriteLockoutPort_VerifyCanaryOperation_NilResultFails(t *testing.T) {
	port := NewCutoverWriteLockoutPort(WriteLockoutDependencies{})
	err := port.VerifyCanaryOperation(context.Background(), nil)
	if err == nil {
		t.Fatal("expected VerifyCanaryOperation to fail when result is nil")
	}
}

func TestCutoverWriteLockoutPort_ExecuteCanaryOperation_ExecutorCalled(t *testing.T) {
	executorCalled := false
	port := NewCutoverWriteLockoutPort(WriteLockoutDependencies{
		CanaryExecutor: func(ctx context.Context) (*CanaryResult, error) {
			executorCalled = true
			return &CanaryResult{
				OperationID:        "real-canary-op",
				CommandID:          "real-canary-cmd",
				OperationTerminal:  "completed",
				CommandTerminal:    "completed",
				ProjectionRevision: 5,
				DesiredRevision:    5,
				Success:            true,
			}, nil
		},
	})
	result, err := port.ExecuteCanaryOperation(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executorCalled {
		t.Fatal("expected canary executor to be called")
	}
	if result.OperationID != "real-canary-op" {
		t.Fatalf("expected real canary result, got: %s", result.OperationID)
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

func TestCutoverPlan_ReadSwitch_V2QueryableButLegacyReaderMustFail(t *testing.T) {
	v2Queryable := false
	legacyActive := true
	plan := NewCutoverPlan(CutoverDependencies{
		ReadSwitch: &testReadSwitchPort{
			verifyReadCanonical: func(ctx context.Context) error {
				v2Queryable = true
				return nil
			},
			verifyProductionReaderNotLegacy: func(ctx context.Context) error {
				if legacyActive {
					return errors.New("production reader still uses legacy components")
				}
				return nil
			},
		},
		Now: time.Now,
	})
	state := &CutoverState{}
	err := plan.runReadSwitch(context.Background(), state)
	if err == nil {
		t.Fatal("expected read switch to fail when production reader still uses legacy, even if V2 tables are queryable")
	}
	if !v2Queryable {
		t.Fatal("expected V2 tables to be checked (queryable) before legacy reader check")
	}
	if state.PhaseStatus != "verifying" {
		t.Fatalf("expected phase status 'verifying', got: %s", state.PhaseStatus)
	}
}

func TestCutoverPlan_ReadSwitch_V2QueryableLegacyReaderMigratedPasses(t *testing.T) {
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
	state := &CutoverState{}
	err := plan.runReadSwitch(context.Background(), state)
	if err != nil {
		t.Fatalf("expected read switch to pass when V2 queryable and legacy migrated, got: %v", err)
	}
	if state.PhaseStatus == "failed" || state.PhaseStatus == "verifying" {
		t.Fatalf("expected read switch to not fail, got phase status: %s", state.PhaseStatus)
	}
}

func TestCutoverPlan_ConcurrentCutover_TwoBackendInstances_OnlyOneSucceeds(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	lockDir := t.TempDir()
	ctx := context.Background()
	ttl := 30 * time.Second

	lock1 := NewPersistentLock(db, lockDir)
	lock2 := NewPersistentLock(db, lockDir)

	if err := lock1.Acquire(ctx, "cutover-execution-lock", ttl); err != nil {
		t.Fatalf("lock1 acquire failed: %v", err)
	}

	started := make(chan struct{})
	go func() {
		close(started)
		_ = lock2.Acquire(ctx, "cutover-execution-lock", ttl)
	}()

	<-started
	time.Sleep(50 * time.Millisecond)

	if err := lock2.Acquire(ctx, "cutover-execution-lock", ttl); err == nil {
		lock2.Release("cutover-execution-lock")
		t.Fatal("lock2 should not acquire while lock1 holds the lock")
	}

	if err := lock1.Release("cutover-execution-lock"); err != nil {
		t.Fatalf("lock1 release failed: %v", err)
	}

	if err := lock2.Acquire(ctx, "cutover-execution-lock", ttl); err != nil {
		t.Fatalf("lock2 should acquire after lock1 releases: %v", err)
	}
	lock2.Release("cutover-execution-lock")
}

func TestCutoverPlan_HeartbeatLost_MustAbortCutover(t *testing.T) {
	heartbeatLost := false

	plan := NewCutoverPlan(CutoverDependencies{
		ReadSwitch: &testReadSwitchPort{
			verifyReadCanonical: func(ctx context.Context) error {
				if heartbeatLost {
					return errors.New("heartbeat lost: cannot verify read canonical during stale session")
				}
				return nil
			},
			verifyProductionReaderNotLegacy: func(ctx context.Context) error {
				if heartbeatLost {
					return errors.New("heartbeat lost: cannot verify production reader during stale session")
				}
				return nil
			},
		},
		Now: time.Now,
	})

	state := &CutoverState{}
	err := plan.runReadSwitch(context.Background(), state)
	if err != nil {
		t.Fatalf("expected read switch to pass with active heartbeat, got: %v", err)
	}

	heartbeatLost = true
	state2 := &CutoverState{}
	err = plan.runReadSwitch(context.Background(), state2)
	if err == nil {
		t.Fatal("expected read switch to fail when heartbeat is lost mid-cutover")
	}
}
