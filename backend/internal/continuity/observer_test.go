package continuity

import (
	"context"
	"testing"
	"time"
)

func TestObserverUpdatesThreadAndIsIdempotent(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "消息运行时重构", Goal: "完成统一流式消息链", Status: ThreadStatusActive}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	observer := NewObserver(repo)
	payload := ObservePayload{
		ThreadID:       thread.ID,
		SpaceID:        "space-1",
		ConversationID: "conv-1",
		RequestID:      "req-1",
		ExecutionID:    "exec-1",
		UserMessage:    "继续修改 Electron",
		AssistantReply: "Electron reducer 已接入新的流式事件。下一步：修复 tool-call reconnect merge。",
	}
	if err := observer.Observe(context.Background(), payload); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if err := observer.Observe(context.Background(), payload); err != nil {
		t.Fatalf("duplicate observe: %v", err)
	}

	updated, err := repo.GetThread(thread.ID, "space-1")
	if err != nil || updated == nil {
		t.Fatalf("get updated thread: %v", err)
	}
	if updated.NextAction == "" {
		t.Fatalf("expected next action to be extracted")
	}
	events, err := repo.ListRecentEvents(thread.ID, 20)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	observed := 0
	for _, event := range events {
		if event.EventType == "turn.observed" {
			observed++
		}
	}
	if observed != 1 {
		t.Fatalf("expected one idempotent turn.observed event, got %d", observed)
	}
}

func TestObserverRollbackAllowsRetryAfterPartialFailure(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "原子更新", Goal: "验证失败回滚", Status: ThreadStatusActive}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`CREATE TRIGGER block_continuity_binding BEFORE INSERT ON continuity_thread_bindings BEGIN SELECT RAISE(ABORT, 'binding blocked'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(repo)
	payload := ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-rollback",
		UserMessage: "继续处理", AssistantReply: "已经完成数据库迁移。下一步：验证接口。",
	}
	if err := observer.Observe(context.Background(), payload); err == nil {
		t.Fatal("expected binding failure")
	}
	events, err := repo.ListRecentEvents(thread.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventType == "turn.observed" {
			t.Fatal("turn.observed must roll back with the failed transaction")
		}
	}
	if err := repo.db.Exec(`DROP TRIGGER block_continuity_binding`).Error; err != nil {
		t.Fatal(err)
	}
	if err := observer.Observe(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated == nil || updated.NextAction == "" {
		t.Fatal("expected retry to apply the observation")
	}
	events, _ = repo.ListRecentEvents(thread.ID, 20)
	observed := 0
	for _, event := range events {
		if event.EventType == "turn.observed" {
			observed++
		}
	}
	if observed != 1 {
		t.Fatalf("turn.observed count=%d, want 1", observed)
	}
}

func TestObserverDoesNotMutateTerminalThread(t *testing.T) {
	repo := testRepository(t)
	completedAt := time.Now().UTC()
	thread := &Thread{SpaceID: "space-1", Title: "已完成事项", Goal: "保持终态", CurrentState: "已完成", Status: ThreadStatusCompleted, CompletedAt: &completedAt}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(repo)
	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-terminal",
		UserMessage: "继续处理", AssistantReply: "延迟到达的旧回复。下一步：不应该恢复事项。",
	}); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated.Status != ThreadStatusCompleted || updated.CurrentState != "已完成" || updated.NextAction != "" {
		t.Fatalf("terminal thread mutated: %#v", updated)
	}
	events, _ := repo.ListRecentEvents(thread.ID, 20)
	for _, event := range events {
		if event.EventType == "turn.observed" {
			t.Fatal("terminal thread must not receive delayed observation events")
		}
	}
}

func TestObserverWaitsForUserAndResumesOnReply(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "服务器迁移", Goal: "迁移服务器", Status: ThreadStatusActive}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	observer := NewObserver(repo)

	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID:       thread.ID,
		SpaceID:        "space-1",
		ConversationID: "conv-1",
		RequestID:      "req-wait",
		AssistantReply: "需要你确认新服务器 IP，请提供 IP 后我再继续。",
	}); err != nil {
		t.Fatalf("observe wait: %v", err)
	}
	waits, err := repo.ListOpenWaits(thread.ID, 10)
	if err != nil || len(waits) != 1 {
		t.Fatalf("expected one user wait, got %d err=%v", len(waits), err)
	}
	waitingThread, _ := repo.GetThread(thread.ID, "space-1")
	if waitingThread.Status != ThreadStatusWaiting {
		t.Fatalf("status = %s, want waiting", waitingThread.Status)
	}

	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID:       thread.ID,
		SpaceID:        "space-1",
		ConversationID: "conv-1",
		RequestID:      "req-resume",
		UserMessage:    "新服务器 IP 是 10.0.0.8，继续",
		AssistantReply: "已收到 IP，开始继续部署。",
	}); err != nil {
		t.Fatalf("observe resume: %v", err)
	}
	waits, err = repo.ListOpenWaits(thread.ID, 10)
	if err != nil {
		t.Fatalf("list waits: %v", err)
	}
	if len(waits) != 0 {
		t.Fatalf("expected user wait to be resolved, got %d", len(waits))
	}
	resumed, _ := repo.GetThread(thread.ID, "space-1")
	if resumed.Status != ThreadStatusActive {
		t.Fatalf("status = %s, want active", resumed.Status)
	}
}

func TestObserverDoesNotResolveMultipleUserWaitsWithOneReply(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "多个用户等待", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	first := &Wait{ThreadID: thread.ID, WaitType: WaitTypeUser, Status: WaitStatusWaiting, Description: "等待确认地址", ConditionJSON: `{"kind":"user_reply"}`}
	second := &Wait{ThreadID: thread.ID, WaitType: WaitTypeUser, Status: WaitStatusWaiting, Description: "等待确认时间", ConditionJSON: `{"kind":"user_reply"}`}
	if err := repo.CreateWait(first); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateWait(second); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(repo)
	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-multi-user",
		UserMessage: "继续处理", AssistantReply: "继续处理。",
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		wait, _ := repo.GetWait(id)
		if wait == nil || wait.Status != WaitStatusWaiting {
			t.Fatalf("user wait %s must remain open", id)
		}
	}
}

func TestObserverDoesNotResolveAmbiguousWaitType(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "等待多个外部条件", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	first := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 CI", ConditionJSON: `{"buildId":"b1"}`}
	second := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 DNS", ConditionJSON: `{"recordId":"d1"}`}
	if err := repo.CreateWait(first); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateWait(second); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(repo)
	observer.SetIntelligence(NewIntelligence(observerJSONGenerator{response: `{"stateChanged":true,"status":"active","waits":[{"action":"resolve","type":"external","resolution":{"any":"value"}}]}`}))
	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-ambiguous",
		UserMessage: "外部条件好了", AssistantReply: "继续。",
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		wait, _ := repo.GetWait(id)
		if wait == nil || wait.Status != WaitStatusWaiting {
			t.Fatalf("ambiguous resolution changed wait %s", id)
		}
	}
	observer.SetIntelligence(NewIntelligence(observerJSONGenerator{response: `{"stateChanged":true,"status":"active","waits":[{"action":"resolve","waitId":"` + second.ID + `","type":"external","resolution":{"recordId":"d1"}}]}`}))
	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-exact",
		UserMessage: "DNS 已完成", AssistantReply: "继续。",
	}); err != nil {
		t.Fatal(err)
	}
	updatedFirst, _ := repo.GetWait(first.ID)
	updatedSecond, _ := repo.GetWait(second.ID)
	if updatedFirst.Status != WaitStatusWaiting {
		t.Fatalf("first wait must remain open: first=%s/%s second=%s/%s", updatedFirst.ID, updatedFirst.Status, updatedSecond.ID, updatedSecond.Status)
	}
	if updatedSecond.Status != WaitStatusResolved {
		t.Fatal("second wait must be resolved by explicit id")
	}
}

func TestObserverExplicitContinuationResumesPausedThread(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", Title: "暂停事项", Goal: "继续完成", Status: ThreadStatusPaused}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(repo)
	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-resume-paused",
		UserMessage: "继续这个", AssistantReply: "继续处理，下一步完成剩余检查。",
	}); err != nil {
		t.Fatal(err)
	}
	updated, _ := repo.GetThread(thread.ID, "space-1")
	if updated.Status != ThreadStatusActive {
		t.Fatalf("status=%s, want active", updated.Status)
	}
}

type observerJSONGenerator struct {
	response string
}

func (g observerJSONGenerator) GenerateWorkshopJSON(context.Context, string, string) (string, string, string, error) {
	return g.response, "", "", nil
}

func TestObserverStructuredResolutionIsInlineAndDoesNotWakeAgain(t *testing.T) {
	repo := testRepository(t)
	thread := &Thread{SpaceID: "space-1", CharacterID: "char-1", Title: "等待 CI", Status: ThreadStatusWaiting}
	if err := repo.CreateThread(thread); err != nil {
		t.Fatal(err)
	}
	wait := &Wait{ThreadID: thread.ID, WaitType: WaitTypeExternal, Status: WaitStatusWaiting, Description: "等待 CI", ConditionJSON: `{"buildId":"b1"}`, AutoResume: true}
	if err := repo.CreateWait(wait); err != nil {
		t.Fatal(err)
	}
	dispatcher := &recordingWakeDispatcher{}
	coordinator := NewWaitCoordinator(repo, dispatcher, DefaultWaitCoordinatorConfig())
	observer := NewObserver(repo)
	observer.SetWaitCoordinator(coordinator)
	observer.SetIntelligence(NewIntelligence(observerJSONGenerator{response: `{"stateChanged":true,"status":"active","currentState":"CI 已完成","nextAction":"继续部署","confidence":0.98,"eventType":"ci.completed","eventSummary":"CI 已完成","waits":[{"action":"resolve","waitId":"` + wait.ID + `","type":"external","resolution":{"buildId":"b1"}}]}`}))

	if err := observer.Observe(context.Background(), ObservePayload{
		ThreadID: thread.ID, SpaceID: "space-1", ConversationID: "conv-1", RequestID: "req-ci-chat",
		UserMessage: "CI 已经跑完了，继续", AssistantReply: "收到，CI 已完成，继续部署。",
	}); err != nil {
		t.Fatal(err)
	}
	resolved, err := repo.GetWait(wait.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != WaitStatusResolved || resolved.WakeState != WakeStateSkipped {
		t.Fatalf("wait status=%s wake=%s", resolved.Status, resolved.WakeState)
	}
	if err := coordinator.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("conversation-consumed condition emitted %d duplicate wake(s)", dispatcher.count())
	}
}
