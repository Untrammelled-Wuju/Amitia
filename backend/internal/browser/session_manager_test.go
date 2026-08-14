package browser

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeSessionManagerRuntime struct {
	mu     sync.Mutex
	state  BrowserRuntimeState
	health BrowserRuntimeHealth
	gen    uint64
}

func (r *fakeSessionManagerRuntime) Start(_ context.Context) (*BrowserRuntimeInfo, *BrowserError) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = BrowserRuntimeReady
	r.health = BrowserHealthHealthy
	r.gen++
	return &BrowserRuntimeInfo{State: BrowserRuntimeReady, Generation: r.gen}, nil
}

func (r *fakeSessionManagerRuntime) Stop(_ context.Context) *BrowserError {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = BrowserRuntimeStopped
	r.health = BrowserHealthUnavailable
	return nil
}

func (r *fakeSessionManagerRuntime) Status(_ context.Context) BrowserRuntimeInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return BrowserRuntimeInfo{State: r.state, Generation: r.gen}
}

func (r *fakeSessionManagerRuntime) Health(_ context.Context) BrowserRuntimeHealth {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.health
}

func (r *fakeSessionManagerRuntime) setFailed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = BrowserRuntimeFailed
	r.health = BrowserHealthUnhealthy
}

type fakeSessionManagerBackend struct {
	mu         sync.Mutex
	contexts   []BrowserContextID
	createErr  error
	disposeErr error
}

func (b *fakeSessionManagerBackend) CreateBrowserContext(_ context.Context) (BrowserContextID, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.createErr != nil {
		return "", b.createErr
	}
	id := BrowserContextID("ctx_" + string(rune(len(b.contexts)+'a')))
	b.contexts = append(b.contexts, id)
	return id, nil
}

func (b *fakeSessionManagerBackend) DisposeBrowserContext(_ context.Context, id BrowserContextID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disposeErr != nil {
		return b.disposeErr
	}
	for i, ctx := range b.contexts {
		if ctx == id {
			b.contexts = append(b.contexts[:i], b.contexts[i+1:]...)
			break
		}
	}
	return nil
}

func (b *fakeSessionManagerBackend) createdCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.contexts)
}

func TestProductionSessionManagerCreateSessionRuntimeNotReady(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeStopped}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	_, err := manager.CreateSession(ctx)
	if err == nil {
		t.Fatal("expected error when runtime not ready")
	}
	if !IsBrowserRuntimeNotReady(err) {
		t.Fatalf("expected runtime_not_ready error, got: %v", err)
	}
}

func TestProductionSessionManagerCreateSessionSuccess(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SessionID == "" {
		t.Fatal("session ID should not be empty")
	}
	if info.State != SessionStateReady {
		t.Fatalf("expected state ready, got: %s", info.State)
	}
	if info.CreatedAt.IsZero() {
		t.Fatal("createdAt should not be zero")
	}
}

func TestProductionSessionManagerCreateSessionQuotaReached(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 2)

	ctx := context.Background()
	_, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("first create should succeed: %v", err)
	}
	_, err = manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("second create should succeed: %v", err)
	}
	_, err = manager.CreateSession(ctx)
	if err == nil {
		t.Fatal("third create should fail due to quota")
	}
	if !IsSessionLimitReached(err) {
		t.Fatalf("expected session_limit_reached error, got: %v", err)
	}
}

func TestProductionSessionManagerGetSession(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	got, err := manager.GetSession(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}
	if got.SessionID != info.SessionID {
		t.Fatalf("expected session ID %s, got: %s", info.SessionID, got.SessionID)
	}
}

func TestProductionSessionManagerGetSessionNotFound(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	_, err := manager.GetSession(ctx, "bs_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !IsSessionNotFound(err) {
		t.Fatalf("expected session_not_found error, got: %v", err)
	}
}

func TestProductionSessionManagerListSessions(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	_, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session 1 failed: %v", err)
	}
	_, err = manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session 2 failed: %v", err)
	}

	sessions, err := manager.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list sessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got: %d", len(sessions))
	}
}

func TestProductionSessionManagerCloseSession(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	err = manager.CloseSession(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("close session failed: %v", err)
	}

	_, err = manager.GetSession(ctx, info.SessionID)
	if err == nil {
		t.Fatal("expected error after session closed")
	}
}

func TestProductionSessionManagerCloseSessionDoubleClose(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	err = manager.CloseSession(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("first close should succeed: %v", err)
	}

	err = manager.CloseSession(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("second close should be idempotent: %v", err)
	}
}

func TestProductionSessionManagerStaleGeneration(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	rt.gen = 2

	_, err = manager.GetSession(ctx, info.SessionID)
	if err == nil {
		t.Fatal("expected error for stale generation")
	}
	if !IsSessionStale(err) {
		t.Fatalf("expected session_stale error, got: %v", err)
	}
}

func TestProductionSessionManagerResolveSession(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	info, err := manager.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	resolver := manager.(SessionResolver)
	resolved, err := resolver.ResolveSession(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("resolve session failed: %v", err)
	}
	if resolved.SessionID != info.SessionID {
		t.Fatalf("expected session ID %s, got: %s", info.SessionID, resolved.SessionID)
	}
	if resolved.BrowserContextID == "" {
		t.Fatal("browser context ID should not be empty")
	}
	if resolved.RuntimeGeneration != 1 {
		t.Fatalf("expected runtime generation 1, got: %d", resolved.RuntimeGeneration)
	}
}

func TestProductionSessionManagerContextCancelled(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := manager.CreateSession(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestProductionSessionManagerBackendCreateFailure(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{createErr: &BrowserError{Code: ErrCodeSessionCreateFailed, Message: "mock failure"}}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	_, err := manager.CreateSession(ctx)
	if err == nil {
		t.Fatal("expected error when backend create fails")
	}
	if err.Code != ErrCodeSessionCreateFailed {
		t.Fatalf("expected session_create_failed error, got: %v", err)
	}
}

func TestProductionSessionManagerConcurrentCreate(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 5)

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, browserErr := manager.CreateSession(ctx)
			if browserErr == nil {
				results[idx] = nil
			} else {
				results[idx] = browserErr
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	failCount := 0
	for i, err := range results {
		if err == nil {
			successCount++
		} else {
			t.Logf("goroutine %d error: %v", i, err)
			failCount++
		}
	}

	if successCount != 5 {
		t.Fatalf("expected 5 successful creates, got: %d (failed: %d)", successCount, failCount)
	}
	if failCount != 5 {
		t.Fatalf("expected 5 failed creates, got: %d (success: %d)", failCount, successCount)
	}
}

func TestProductionSessionManagerMaxSessionsDefault(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 0)

	ctx := context.Background()
	for i := 0; i < DefaultMaxSessions; i++ {
		_, err := manager.CreateSession(ctx)
		if err != nil {
			t.Fatalf("create session %d should succeed: %v", i+1, err)
		}
	}

	_, err := manager.CreateSession(ctx)
	if err == nil {
		t.Fatal("should fail after reaching default max sessions")
	}
}

func TestProductionSessionManagerCreatedAtTimestamp(t *testing.T) {
	rt := &fakeSessionManagerRuntime{state: BrowserRuntimeReady, health: BrowserHealthHealthy, gen: 1}
	backend := &fakeSessionManagerBackend{}
	manager := NewProductionSessionManager(rt, backend, 8)

	ctx := context.Background()
	before := time.Now()
	info, err := manager.CreateSession(ctx)
	after := time.Now()

	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if info.CreatedAt.Before(before) || info.CreatedAt.After(after) {
		t.Fatalf("expected CreatedAt between %v and %v, got: %v", before, after, info.CreatedAt)
	}
}
