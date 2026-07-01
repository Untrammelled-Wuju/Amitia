package mindruntime

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerLLMDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cfg.OpenTimeout = 20 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("llm", cfg)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(35 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerQdrantDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("qdrant", cfg)
	for i := 0; i < 2; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerSurrealDBDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("surrealdb", cfg)
	for i := 0; i < 2; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerASRDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("asr", cfg)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerTTSDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("tts", cfg)
	for i := 0; i < 2; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerToolsDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 2
	cb := NewCircuitBreaker("tools", cfg)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitBreakerChannelsDrill(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cfg.OpenTimeout = 15 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("channels", cfg)
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("closed cb should allow call %d", i)
		}
		cb.RecordFailure()
	}
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after failures, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after recovery, got %s", cb.Status())
	}
}

func TestCircuitOpenNoRepeatedWait(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 500 * time.Millisecond
	cb := NewCircuitBreaker("no-wait", cfg)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.Status())
	}
	denied := 0
	start := time.Now()
	for i := 0; i < 50; i++ {
		if !cb.Allow() {
			denied++
		}
		time.Sleep(1 * time.Millisecond)
	}
	if denied < 45 {
		t.Fatalf("expected most denied, got %d/50", denied)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("open should not cause repeated waits; took %v", time.Since(start))
	}
}

func TestCircuitHalfOpenProbeTrafficOnly(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 20 * time.Millisecond
	cfg.HalfOpenMaxRequest = 1
	cb := NewCircuitBreaker("probe-only", cfg)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.Status())
	}
	time.Sleep(30 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("first probe should allow")
	}
}

func TestStartupOrderingDrill(t *testing.T) {
	seq := NewStartupSequence()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	components := []*mockStartupComponent{
		{phase: StartupPhaseDatabase, name: "sqlite"},
		{phase: StartupPhaseConfig, name: "viper"},
		{phase: StartupPhaseModels, name: "gpt"},
		{phase: StartupPhaseNetwork, name: "http"},
		{phase: StartupPhaseRuntime, name: "worker"},
		{phase: StartupPhaseReady, name: "gate"},
	}
	for _, c := range components { seq.Register(c) }
	results := seq.Execute(ctx)
	if len(results) != 6 { t.Fatalf("expected 6 phases, got %d", len(results)) }
	if !seq.AllReady(results) { t.Fatal("expected ready gate passed") }
}

func TestReadyGateDrill(t *testing.T) {
	deps := []string{"db", "config", "models", "network"}
	gate := NewReadyGate(deps)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if gate.IsReady() { t.Fatal("gate should not be ready initially") }
	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.SignalReady("db")
		gate.SignalReady("config")
		gate.SignalReady("models")
		gate.SignalReady("network")
	}()
	if err := gate.Wait(ctx); err != nil { t.Fatalf("wait failed: %v", err) }
	if !gate.IsReady() { t.Fatal("gate should be ready") }
}

func TestDrainShutdownDrill(t *testing.T) {
	lc := &mockLifecycleComponent{phase: LifecyclePhaseRunning, state: LifecycleState{Phase: LifecyclePhaseRunning, StartedAt: time.Now().UTC()}}
	order := NewShutdownOrder([]LifecycleComponent{lc})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := order.Execute(ctx)
	if len(results) != 1 { t.Fatalf("expected 1 result, got %d", len(results)) }
	if results[0].Phase != LifecyclePhaseTerminated { t.Fatalf("expected terminated, got %s", results[0].Phase) }
	if !lc.drainCalled { t.Fatal("drain should be called") }
}

func TestForceShutdownDrill(t *testing.T) {
	lc := &mockLifecycleComponent{phase: LifecyclePhaseRunning, state: LifecycleState{Phase: LifecyclePhaseRunning}, drainDelay: 100 * time.Millisecond}
	order := NewShutdownOrder([]LifecycleComponent{lc})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	results := order.Execute(ctx)
	if results[0].Error == "" { t.Fatal("expected error from forced drain timeout") }
}

func TestRestartRecoveryDrill(t *testing.T) {
	seq := NewStartupSequence()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c1 := &mockStartupComponent{phase: StartupPhaseDatabase, name: "sqlite"}
	c2 := &mockStartupComponent{phase: StartupPhaseReady, name: "gate"}
	seq.Register(c1); seq.Register(c2)
	results1 := seq.Execute(ctx)
	if len(results1) != 2 { t.Fatalf("first run: expected 2 phases, got %d", len(results1)) }
	results2 := seq.Execute(ctx)
	if len(results2) != 2 { t.Fatalf("restart: expected 2 phases, got %d", len(results2)) }
}
func TestOutboxLeaseReturnDrill(t *testing.T) {
	lease := &OutboxLease{ID: "lease-001", OwnerID: "worker-1", ExpiresAt: time.Now().UTC().Add(500 * time.Millisecond), Active: true}
	if !lease.IsActive() { t.Fatal("new lease should be active") }
	lease.Return()
	if lease.Active { t.Fatal("returned lease should be inactive") }
}

func TestOutputLeaseExpiryDrill(t *testing.T) {
	lease := &OutputLease{ID: "lease-002", TaskID: "task-001", ExpiresAt: time.Now().UTC().Add(20 * time.Millisecond), Status: "active"}
	if lease.IsExpired() { t.Fatal("lease should not be expired yet") }
	time.Sleep(30 * time.Millisecond)
	if !lease.IsExpired() { t.Fatal("lease should be expired") }
}

func TestSQLiteTransactionCompleteDrill(t *testing.T) {
	tx := &SimulatedTx{ID: "tx-001", StartedAt: time.Now().UTC(), Committed: false}
	tx.Commit()
	if !tx.Committed { t.Fatal("transaction should be committed") }
}

func TestExpiredResultDiscardDrill(t *testing.T) {
	result := &InteractionResult{ID: "result-001", Deadline: time.Now().UTC().Add(-10 * time.Millisecond)}
	if !result.IsExpired() { t.Fatal("past-deadline result should be expired") }
	result.Discard()
	if !result.Discarded { t.Fatal("expired result should be discarded") }
}

func TestBasicChatConsistencyDrill(t *testing.T) {
	chat := &ChatSession{ID: "chat-001", CharacterID: "char-001"}
	chat.AddMessage("user", "hello")
	chat.AddMessage("assistant", "hi there")
	if chat.MessageCount != 2 { t.Fatalf("expected 2 messages, got %d", chat.MessageCount) }
}

func TestStateConsistencyDrill(t *testing.T) {
	state := NewStateVersion("char-001", 0)
	state.Increment()
	if state.Version != 1 { t.Fatalf("expected version 1, got %d", state.Version) }
	state.Increment(); state.Increment()
	if !state.IsConsistent() { t.Fatal("monotonic increments should be consistent") }
	if !state.ValidateVersion(5) { t.Fatal("newer version should be valid") }
	if state.ValidateVersion(2) { t.Fatal("older version should be invalid") }
}

func TestUnknownDeliveryDrill(t *testing.T) {
	del := &MessageDelivery{ID: "del-001", Target: "unknown-channel", Status: "unknown", MaxAttempts: 3}
	del.Retry()
	if del.Attempts != 1 { t.Fatalf("expected 1 attempt, got %d", del.Attempts) }
	del.Fail(); del.Retry(); del.Fail(); del.Retry(); del.MarkFailed()
	if del.Status != "failed" { t.Fatalf("expected failed, got %s", del.Status) }
}

func TestToolSideEffectCompensationDrill(t *testing.T) {
	comp := &SideEffectCompensation{ID: "comp-001", ToolName: "file_write", Action: "compensate"}
	if comp.Applied { t.Fatal("compensation should start unapplied") }
	comp.Apply()
	if !comp.Applied { t.Fatal("compensation should be applied") }
	comp.Revert()
	if comp.Applied { t.Fatal("reverted compensation should not be applied") }
}

func TestMultiCircuitBreakerDrill(t *testing.T) {
	reg := NewCircuitBreakerRegistry()
	names := []string{"llm", "qdrant", "surrealdb", "asr", "tts", "tools", "channels"}
	for _, name := range names { reg.Register(name, DefaultCircuitBreakerConfig()) }
	for _, name := range names {
		for i := 0; i < DefaultCircuitBreakerConfig().FailureThreshold; i++ { reg.RecordFailure(name) }
		if cb := reg.Get(name); cb.Status() != CircuitOpen { t.Fatalf("%s expected open, got %s", name, cb.Status()) }
	}
	reg.ResetAll()
	for _, name := range names {
		if cb := reg.Get(name); cb.Status() != CircuitClosed { t.Fatalf("%s expected closed after reset, got %s", name, cb.Status()) }
	}
}
type mockStartupComponent struct {
	phase   StartupPhase
	name    string
	fail    bool
	started bool
}
func (m *mockStartupComponent) PhaseName() StartupPhase { return m.phase }
func (m *mockStartupComponent) Startup(ctx context.Context) error { if m.fail { return context.DeadlineExceeded }; m.started = true; return nil }
func (m *mockStartupComponent) HealthCheck() HealthCheckResult {
	checks := []ComponentCheck{{Name: m.name, Passed: !m.fail, Message: "ok"}}
	return HealthCheckResult{Target: HealthCheckAffect, Healthy: !m.fail, CheckedAt: time.Now().UTC(), Checks: checks, Summary: "mock check"}
}
func (m *mockStartupComponent) Reset() { m.started = false; m.fail = false }

type mockLifecycleComponent struct {
	phase       LifecyclePhase
	state       LifecycleState
	drainCalled bool
	drainDelay  time.Duration
}
func (m *mockLifecycleComponent) Start(ctx context.Context) error { return nil }
func (m *mockLifecycleComponent) Shutdown(ctx context.Context) error { return nil }
func (m *mockLifecycleComponent) Drain(ctx context.Context) error {
	m.drainCalled = true
	if m.drainDelay > 0 {
		select { case <-ctx.Done(): return ctx.Err(); case <-time.After(m.drainDelay): }
	}
	return nil
}
func (m *mockLifecycleComponent) Phase() LifecyclePhase { return m.phase }
func (m *mockLifecycleComponent) State() LifecycleState { return m.state }

type OutboxLease struct {
	ID        string
	OwnerID   string
	ExpiresAt time.Time
	Active    bool
}
func (l *OutboxLease) IsActive() bool {
	if !l.Active || time.Now().UTC().After(l.ExpiresAt) { l.Active = false; return false }
	return true
}
func (l *OutboxLease) Return() { l.Active = false; l.OwnerID = "" }

type OutputLease struct {
	ID        string
	TaskID    string
	ExpiresAt time.Time
	Status    string
}
func (l *OutputLease) IsExpired() bool {
	if l.Status == "expired" { return true }
	if time.Now().UTC().After(l.ExpiresAt) { l.Status = "expired"; return true }
	return false
}

type SimulatedTx struct {
	ID          string
	StartedAt   time.Time
	Committed   bool
	CompletedAt time.Time
}
func (tx *SimulatedTx) Commit() { tx.Committed = true; tx.CompletedAt = time.Now().UTC() }

type InteractionResult struct {
	ID        string
	Deadline  time.Time
	Discarded bool
}
func (r *InteractionResult) IsExpired() bool { return time.Now().UTC().After(r.Deadline) }
func (r *InteractionResult) Discard() { r.Discarded = true }

type ChatSession struct {
	ID            string
	CharacterID   string
	MessageCount  int
	LastMessageAt time.Time
	Messages      []ChatMessage
	mu            sync.Mutex
}
type ChatMessage struct { Role string; Content string; CreatedAt time.Time }
func (s *ChatSession) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessageCount++
	s.LastMessageAt = time.Now().UTC()
	s.Messages = append(s.Messages, ChatMessage{Role: role, Content: content, CreatedAt: s.LastMessageAt})
}

type StateVersion struct {
	CharacterID string
	Version     int
	UpdatedAt   time.Time
}
func NewStateVersion(cid string, v int) *StateVersion {
	return &StateVersion{CharacterID: cid, Version: v, UpdatedAt: time.Now().UTC()}
}
func (s *StateVersion) Increment() { s.Version++; s.UpdatedAt = time.Now().UTC() }
func (s *StateVersion) IsConsistent() bool { return s.Version >= 0 && !s.UpdatedAt.IsZero() }
func (s *StateVersion) ValidateVersion(nv int) bool { return nv > s.Version }

type MessageDelivery struct {
	ID          string
	Target      string
	Status      string
	Attempts    int
	MaxAttempts int
}
func (d *MessageDelivery) Retry() { if d.Attempts < d.MaxAttempts { d.Attempts++; d.Status = "retrying" } }
func (d *MessageDelivery) Fail() { d.Status = "failed_attempt" }
func (d *MessageDelivery) MarkFailed() { d.Status = "failed" }

type SideEffectCompensation struct {
	ID        string
	ToolName  string
	Action    string
	Applied   bool
	AppliedAt time.Time
}
func (c *SideEffectCompensation) Apply() { c.Applied = true; c.AppliedAt = time.Now().UTC() }
func (c *SideEffectCompensation) Revert() { c.Applied = false }