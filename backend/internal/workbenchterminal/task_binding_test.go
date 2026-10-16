package workbenchterminal

import (
	"context"
	"errors"
	"testing"
)

func TestTaskTerminalRequiresActiveBinding(t *testing.T) {
	m, p, id := sessionFixture(t)
	s := m.sessions[id]
	s.taskID = "task-42"
	s.workspaceURI = "amitia://workspace/@worktree/"
	s.conversationID = "conversation-3"
	checked := 0
	h := &Handler{manager: m, taskVerifier: func(_ context.Context, workspaceURI, taskID, conversationID string) error {
		checked++
		if workspaceURI != s.workspaceURI || taskID != "task-42" || conversationID != "conversation-3" {
			return ErrTaskBinding
		}
		return nil
	}}
	if err := h.validateTask(context.Background(), "first", id, "conversation-3"); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatalf("called %d times", checked)
	}
	if err := h.validateTask(context.Background(), "first", id, "wrong"); !errors.Is(err, ErrTaskBinding) {
		t.Fatalf("wrong conversation: %v", err)
	}
	if _, err := m.Read("first", id, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked task session still active: %v", err)
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if !closed {
		t.Fatal("revoked terminal process still running")
	}
}

func TestTaskVerifierFailureDoesNotAffectUnboundSessions(t *testing.T) {
	m, _, id := sessionFixture(t)
	h := &Handler{manager: m, taskVerifier: func(context.Context, string, string, string) error { return ErrTaskBinding }}
	if err := h.validateTask(context.Background(), "first", id, "conversation-3"); err != nil {
		t.Fatal(err)
	}
	if err := h.validateTask(context.Background(), "other", id, "conversation-3"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTaskPatternBoundaries(t *testing.T) {
	for _, id := range []string{"", "-task", "../bad", "task/id", "a\n", "a b"} {
		if taskIDPattern.MatchString(id) {
			t.Fatalf("accepted task id %q", id)
		}
	}
	for _, id := range []string{"a", "task_1:a-3", "task.worker.3"} {
		if !taskIDPattern.MatchString(id) {
			t.Fatalf("rejected task id %q", id)
		}
	}
}

func TestExportIsOwnerScopedBoundedAndMarkedTruncated(t *testing.T) {
	m, _, id := sessionFixture(t)
	s := m.sessions[id]
	s.mu.Lock()
	s.taskID = "task-42"
	s.workspaceURI = "amitia://workspace/@worktree/"
	s.output.append(make([]byte, bufferCapacity+31))
	s.mu.Unlock()
	if _, err := m.Export("other", id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign export: %v", err)
	}
	result, err := m.Export("first", id)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != id || result.TaskID != "task-42" || !result.Truncated || result.BaseCursor != 31 || result.EndCursor != bufferCapacity+31 {
		t.Fatalf("bad transcript metadata: %+v", result)
	}
	if len(result.Data) > 4*bufferCapacity/3+8 {
		t.Fatal("export beyond cap")
	}
}

func TestTicketCarriesConversationAndIsSingleUse(t *testing.T) {
	var tickets ticketRegistry
	token, err := tickets.issue("first", "terminal", "https://localhost:4567", "conversation-3")
	if err != nil {
		t.Fatal(err)
	}
	grant, err := tickets.consume(token, "https://localhost:4567")
	if err != nil || grant.conversationID != "conversation-3" {
		t.Fatalf("ticket lost task scope %+v %v", grant, err)
	}
	if _, err := tickets.consume(token, "https://localhost:4567"); !errors.Is(err, ErrTicket) {
		t.Fatal("reused ticket")
	}
}

func TestTaskTransientVerifierErrorDoesNotKillProcess(t *testing.T) {
	manager, _, id := sessionFixture(t)
	s := manager.sessions[id]
	s.taskID, s.workspaceURI, s.conversationID = "task-42", "amitia://workspace/@worktree/", "conversation-3"
	temporarilyUnavailable := errors.New("repository unavailable")
	handler := &Handler{manager: manager, taskVerifier: func(context.Context, string, string, string) error { return temporarilyUnavailable }}
	if err := handler.validateTask(context.Background(), "first", id, "conversation-3"); !errors.Is(err, temporarilyUnavailable) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := manager.Read("first", id, 0); err != nil {
		t.Fatalf("temporary backend outage unexpectedly destroyed session: %v", err)
	}
}
