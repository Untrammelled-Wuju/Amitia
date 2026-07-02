package memory

import (
	"context"
	"testing"
)

type testConversationStateStore struct {
	state ConversationWorkingMemoryState
	ok    bool
}

func (s *testConversationStateStore) GetWorkingMemoryState(convID string) (ConversationWorkingMemoryState, bool) {
	if !s.ok || s.state.ConversationID != convID {
		return ConversationWorkingMemoryState{}, false
	}
	return s.state, true
}

func (s *testConversationStateStore) UpsertWorkingMemoryState(convID string, state ConversationWorkingMemoryState) {
	state.ConversationID = convID
	s.state = state
	s.ok = true
}

func TestWorkingMemoryProcessWritesConversationState(t *testing.T) {
	store := &testConversationStateStore{}
	svc := NewWorkingMemoryService(store)
	messages := []map[string]string{
		{"role": "user", "content": "明天提醒我检查项目进度"},
		{"role": "assistant", "content": "好，我记住这个事项"},
	}

	err := svc.Process(context.Background(), "conv1", messages, "本轮重点是项目进度提醒")
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !store.ok {
		t.Fatal("state not written")
	}
	if store.state.LastInteractionSummary != "本轮重点是项目进度提醒" {
		t.Fatalf("bad summary: %s", store.state.LastInteractionSummary)
	}
	if store.state.MessageCount != len(messages) {
		t.Fatalf("bad message count: %d", store.state.MessageCount)
	}
	if store.state.LastMessageAt.IsZero() {
		t.Fatal("last message time not set")
	}
	if len(store.state.ActiveThreads) == 0 {
		t.Fatal("active threads not set")
	}
}

func TestWorkingMemoryProcessPreservesExistingConversationState(t *testing.T) {
	store := &testConversationStateStore{
		ok: true,
		state: ConversationWorkingMemoryState{
			ConversationID: "conv2",
			ActiveThreads:  []string{"已有线程"},
		},
	}
	svc := NewWorkingMemoryService(store)

	err := svc.Process(context.Background(), "conv2", []map[string]string{{"role": "user", "content": "继续聊发布计划"}}, "发布计划进入收尾")
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if len(store.state.ActiveThreads) < 2 {
		t.Fatalf("threads not merged: %v", store.state.ActiveThreads)
	}
}

func TestWorkingMemoryProcessWithoutStoreSkips(t *testing.T) {
	svc := NewWorkingMemoryService()
	err := svc.Process(context.Background(), "conv3", nil, "摘要")
	if err != ErrSkip {
		t.Fatalf("expected skip, got %v", err)
	}
}
