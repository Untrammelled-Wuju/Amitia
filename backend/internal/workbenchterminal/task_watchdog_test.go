package workbenchterminal

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestTaskWatchdogRevokesWithoutClientRequests(t *testing.T) {
	m, process, id := sessionFixture(t)
	s := m.sessions[id]
	s.taskID, s.workspaceURI, s.conversationID = "task-42", "amitia://workspace/@tree/", "conversation-3"
	var calls atomic.Int32
	h := &Handler{manager: m, taskVerifier: func(_ context.Context, uri, task, conversation string) error {
		calls.Add(1)
		if uri != s.workspaceURI || task != s.taskID || conversation != s.conversationID {
			t.Fatal("wrong task scope")
		}
		return ErrTaskBinding
	}}
	checked, revoked := h.checkTaskBindings(context.Background())
	if checked != 1 || revoked != 1 || calls.Load() != 1 {
		t.Fatalf("checked=%d revoked=%d", checked, revoked)
	}
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked task retained")
	}
	process.mu.Lock()
	closed := process.closed
	process.mu.Unlock()
	if !closed {
		t.Fatal("revoked task retained a live process")
	}
}

func TestTaskWatchdogDoesNotKillOnTemporaryVerifierFailure(t *testing.T) {
	m, _, id := sessionFixture(t)
	s := m.sessions[id]
	s.taskID, s.workspaceURI, s.conversationID = "task-42", "amitia://workspace/@tree/", "conversation-3"
	h := &Handler{manager: m, taskVerifier: func(context.Context, string, string, string) error {
		return errors.New("temporary repository unavailable")
	}}
	_, revoked := h.checkTaskBindings(context.Background())
	if revoked != 0 {
		t.Fatal("transient failures must not discard user session")
	}
	if _, err := m.Read("first", id, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTaskWatchdogLeavesNormalSessionsUntouched(t *testing.T) {
	m, _, id := sessionFixture(t)
	h := &Handler{manager: m, taskVerifier: func(context.Context, string, string, string) error {
		t.Fatal("called verifier for ordinary terminal")
		return nil
	}}
	checked, revoked := h.checkTaskBindings(context.Background())
	if checked != 0 || revoked != 0 {
		t.Fatalf("%d/%d", checked, revoked)
	}
	if _, err := m.Read("first", id, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTaskWatchdogClosesMalformedBinding(t *testing.T) {
	m, _, id := sessionFixture(t)
	s := m.sessions[id]
	s.taskID = "task-42"
	h := &Handler{manager: m, taskVerifier: func(context.Context, string, string, string) error { return nil }}
	_, revoked := h.checkTaskBindings(context.Background())
	if revoked != 1 {
		t.Fatal("missing conversation must be revoked")
	}
}

func TestTaskWatchdogHonorsCancel(t *testing.T) {
	m, _, id := sessionFixture(t)
	s := m.sessions[id]
	s.taskID, s.conversationID = "task-42", "conversation-3"
	h := &Handler{manager: m, taskVerifier: func(context.Context, string, string, string) error { t.Fatal("called on canceled context"); return nil }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checked, revoked := h.checkTaskBindings(ctx)
	if checked != 0 || revoked != 0 {
		t.Fatalf("%d/%d", checked, revoked)
	}
}

func TestOpenTaskRejectsMissingConversationBeforeLaunchingProcess(t *testing.T) {
	manager, _, _ := sessionFixture(t)
	for _, conversation := range []string{"", "bad/conv"} {
		if _, err := manager.OpenTask("owner", "/tmp", "amitia://workspace/@tree/", "task-42", 30, 90, conversation); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected to reject unsafe conversation %q: %v", conversation, err)
		}
	}
}
