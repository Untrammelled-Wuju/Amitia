package continuity

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

type recordingWakeDispatcher struct {
	mu       sync.Mutex
	requests []WakeRequest
	failures int
}

type concurrentWakeDispatcher struct {
	started chan string
	release chan struct{}
}

func (d *concurrentWakeDispatcher) DispatchContinuityWake(_ context.Context, request WakeRequest) error {
	d.started <- request.WaitID
	<-d.release
	return nil
}

func (d *recordingWakeDispatcher) DispatchContinuityWake(_ context.Context, request WakeRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, request)
	if d.failures > 0 {
		d.failures--
		return errors.New("temporary wake failure")
	}
	return nil
}

func (d *recordingWakeDispatcher) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.requests)
}

func TestWaitCoordinatorTimeWaitPersistsAndDeliversWake(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", CharacterID: "char-1", Title: "服务器迁移", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "conversation", "conv-1", "context", "test", 1); err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(-time.Minute)
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeTime, Status: WaitStatusWaiting, Description: "等待 DNS TTL", DueAt: &due, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetWait(wait.ID)
	if err != nil || updated == nil {
		t.Fatalf("get wait: %v", err)
	}
	if updated.Status != WaitStatusResolved || updated.WakeState != WakeStateDelivered {
		t.Fatalf("wait status=%s wake=%s", updated.Status, updated.WakeState)
	}
	if dispatcher.count() != 1 {
		t.Fatalf("wake count=%d, want 1", dispatcher.count())
	}
	active, _ := repo.GetThread(thread.ID, "space-1")
	if active.Status != ThreadStatusActive {
		t.Fatalf("thread status=%s, want active", active.Status)
	}
}

func TestWaitCoordinatorDispatchesPendingWakesConcurrently(t *testing.T) {
	repo := testRepository(t)
	waits := make([]*Wait, 0, 2)
	for _, title := range []string{"条件一", "条件二"} {
		thread := &Thread{SpaceID: "space-1", Title: title, Status: ThreadStatusWaiting}
		if err := repo.CreateThread(thread); err != nil {
			t.Fatal(err)
		}
		wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, ConditionJSON: `{"id":"` + thread.ID + `"}`, AutoResume: true}
		if err := repo.CreateWait(wait); err != nil {
			t.Fatal(err)
		}
		if _, err := NewWaitCoordinator(repo, nil, DefaultWaitCoordinatorConfig()).ResolveWait(context.Background(), wait.ID, "test", nil, true); err != nil {
			t.Fatal(err)
		}
		waits = append(waits, wait)
	}
	dispatcher := &concurrentWakeDispatcher{started: make(chan string, 2), release: make(chan struct{})}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	done := make(chan error, 1)
	go func() { done <- coordinator.Tick(context.Background()) }()
	started := map[string]bool{}
	for len(started) < 2 {
		select {
		case id := <-dispatcher.started:
			started[id] = true
		case <-time.After(time.Second):
			t.Fatal("pending wakes were not dispatched concurrently")
		}
	}
	close(dispatcher.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, wait := range waits {
		if !started[wait.ID] {
			t.Fatalf("wait %s was not dispatched", wait.ID)
		}
	}
}

func TestWaitCoordinatorInlineUserResolutionNeverEmitsSecondWake(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "确认部署", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeUser, Status: WaitStatusWaiting, Description: "等待用户确认", AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	if _, err := coordinator.ResolveInline(context.Background(), wait.ID, "user_message", map[string]any{"requestId": "req-1"}); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetWait(wait.ID)
	if updated.WakeState != WakeStateSkipped {
		t.Fatalf("wake state=%s, want skipped", updated.WakeState)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("inline user resolution emitted %d wake(s)", dispatcher.count())
	}
}

func TestWaitCoordinatorRetriesFailedWakeWithStableRequestID(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", CharacterID: "char-1", Title: "等待外部构建", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 CI", ConditionJSON: `{"buildId":"build-1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{failures: 1}
	cfg := DefaultWaitCoordinatorConfig()
	cfg.MaxBackoff = time.Second
	coordinator := NewWaitCoordinator(repo, dispatcher, cfg)
	resolved, err := coordinator.Signal(context.Background(), Signal{WaitType: WaitTypeExternal, SpaceID: "space-1", Source: "ci", SourceID: "build-1", Attributes: map[string]any{"buildId": "build-1"}})
	if err != nil || len(resolved) != 1 {
		t.Fatalf("signal resolved=%d err=%v", len(resolved), err)
	}
	first, _ := repo.GetWait(wait.ID)
	if first.WakeState != WakeStatePending {
		// Signal only persists the wake; delivery happens in Tick.
		t.Fatalf("wake state=%s, want pending", first.WakeState)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	failed, _ := repo.GetWait(wait.ID)
	if failed.WakeState != WakeStateFailed || failed.WakeAttempts != 1 {
		t.Fatalf("after failure state=%s attempts=%d", failed.WakeState, failed.WakeAttempts)
	}
	stableID := failed.WakeRequestID
	if stableID == "" {
		t.Fatal("missing stable wake request id")
	}
	past := time.Now().UTC().Add(-time.Second)
	if err := repo.UpdateWait(wait.ID, map[string]interface{}{"next_wake_at": past}); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	delivered, _ := repo.GetWait(wait.ID)
	if delivered.WakeState != WakeStateDelivered || delivered.WakeAttempts != 2 {
		t.Fatalf("after retry state=%s attempts=%d", delivered.WakeState, delivered.WakeAttempts)
	}
	if delivered.WakeRequestID != stableID {
		t.Fatalf("request id changed: %q -> %q", stableID, delivered.WakeRequestID)
	}
}

func TestExternalSignalCannotResolveUnscopedWait(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "外部事件", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "bad external wait", ConditionJSON: `{"kind":"webhook"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	coordinator := NewWaitCoordinator(repo, &recordingWakeDispatcher{}, DefaultWaitCoordinatorConfig())
	resolved, err := coordinator.Signal(context.Background(), Signal{WaitType: WaitTypeExternal, SpaceID: "space-1", Source: "webhook", Attributes: map[string]any{"anything": "value"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 0 {
		t.Fatalf("unscoped external wait resolved unexpectedly: %d", len(resolved))
	}
}

func TestExternalSignalScansPastBatchOfUnrelatedWaits(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "批量信号", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, ConditionJSON: `{"jobId":"unrelated-` + strconv.Itoa(i) + `"}`}
		if err := repo.CreateWait(wait); err != nil {
			t.Fatal(err)
		}
	}
	matching := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, ConditionJSON: `{"jobId":"match-101"}`}
	if err := repo.CreateWait(matching); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	resolved, err := coordinator.Signal(context.Background(), Signal{WaitType: WaitTypeExternal, SpaceID: "space-1", Source: "test", Attributes: map[string]any{"jobId": "match-101"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved[0].ID != matching.ID {
		t.Fatalf("resolved=%#v, want matching wait", resolved)
	}
}

func TestWaitCoordinatorUsesConversationChannelForWake(t *testing.T) {
	repo := testRepository(t)
	if err := repo.db.Exec(`CREATE TABLE conversations (id TEXT PRIMARY KEY, channel TEXT, peer_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`INSERT INTO conversations(id, channel, peer_id) VALUES ('conv-qq', 'qq', 'peer-1')`).Error; err != nil {
		t.Fatal(err)
	}
	thread := &Thread{SpaceID: "space-1", Title: "跨渠道恢复", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.Bind(thread.ID, "conversation", "conv-qq", "context", "test", 1); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, ConditionJSON: `{"jobId":"job-1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	if _, err := coordinator.ResolveWait(context.Background(), wait.ID, "test", nil, true); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()
	if len(dispatcher.requests) != 1 {
		t.Fatalf("requests=%d, want 1", len(dispatcher.requests))
	}
	if dispatcher.requests[0].Channel != "qq" || dispatcher.requests[0].PeerID != "peer-1" {
		t.Fatalf("wake route=%s/%s, want qq/peer-1", dispatcher.requests[0].Channel, dispatcher.requests[0].PeerID)
	}
}

func TestTerminalThreadSkipsPendingWake(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "结束事项", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 CI", ConditionJSON: `{"buildId":"b1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	if _, err := coordinator.ResolveWait(context.Background(), wait.ID, "ci", map[string]any{"buildId": "b1"}, true); err != nil {
		t.Fatal(err)
	}
	latest, _ := repo.GetThread(thread.ID, "space-1")
	if _, err := repo.UpdateThreadCAS(latest.ID, latest.Revision, map[string]interface{}{"status": ThreadStatusCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetWait(wait.ID)
	if updated.WakeState != WakeStateSkipped {
		t.Fatalf("wake state=%s, want skipped", updated.WakeState)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("terminal thread emitted wake")
	}
}

func TestWaitCoordinatorDoesNotWakeUntilAllWaitsResolved(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", CharacterID: "char-1", Title: "双条件事项", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	first := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 CI", ConditionJSON: `{"buildId":"b1"}`, AutoResume: true}
	second := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待审核", ConditionJSON: `{"reviewId":"r1"}`, AutoResume: true}
	if err := repo.CreateWait(first); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateWait(second); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())

	resolvedFirst, err := coordinator.ResolveWait(context.Background(), first.ID, "ci", map[string]any{"buildId": "b1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedFirst.WakeState != WakeStateSkipped {
		t.Fatalf("first wait wake=%s, want skipped while another wait remains", resolvedFirst.WakeState)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("premature wake count=%d", dispatcher.count())
	}
	stillWaiting, _ := repo.GetThread(thread.ID, "space-1")
	if stillWaiting.Status != ThreadStatusWaiting {
		t.Fatalf("thread status=%s, want waiting", stillWaiting.Status)
	}

	resolvedSecond, err := coordinator.ResolveWait(context.Background(), second.ID, "review", map[string]any{"reviewId": "r1"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedSecond.WakeState != WakeStatePending {
		t.Fatalf("final wait wake=%s, want pending", resolvedSecond.WakeState)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dispatcher.count() != 1 {
		t.Fatalf("wake count=%d, want 1", dispatcher.count())
	}
}

func TestCancelLastWaitReactivatesThreadWithoutProactiveWake(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", CharacterID: "char-1", Title: "手动取消等待", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待外部条件", ConditionJSON: `{"jobId":"job-1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	cancelled, err := coordinator.CancelWait(context.Background(), wait.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled == nil || cancelled.Status != WaitStatusCancelled || cancelled.WakeState != WakeStateSkipped {
		t.Fatalf("cancelled wait=%+v", cancelled)
	}
	active, err := repo.GetThread(thread.ID, "space-1")
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != ThreadStatusActive {
		t.Fatalf("thread status=%s, want active", active.Status)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("manual cancellation emitted %d wake(s)", dispatcher.count())
	}
}
