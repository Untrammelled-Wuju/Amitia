package realtime

import (
	"testing"
)

func TestNewVoiceSession(t *testing.T) {
	s := NewVoiceSession("sess-1", "conv-1", "char-1")
	if s.SessionID != "sess-1" {
		t.Fatalf("expected sess-1, got %s", s.SessionID)
	}
	if s.ConversationID != "conv-1" {
		t.Fatalf("expected conv-1, got %s", s.ConversationID)
	}
	if s.StateVersion != 1 {
		t.Fatalf("expected version 1, got %d", s.StateVersion)
	}
}

func TestVoiceSessionBeginTurn(t *testing.T) {
	s := NewVoiceSession("sess-2", "conv-2", "char-2")
	turn := s.BeginTurn("turn-1", "hello")
	if turn.TurnID != "turn-1" {
		t.Fatalf("expected turn-1, got %s", turn.TurnID)
	}
	if turn.Status != TranscriptionInterim {
		t.Fatalf("expected interim status, got %s", turn.Status)
	}
	if s.CurrentTurn == nil {
		t.Fatal("current turn should not be nil")
	}
}

func TestVoiceSessionCommitTurn(t *testing.T) {
	s := NewVoiceSession("sess-3", "conv-3", "char-3")
	s.BeginTurn("turn-1", "hello world")
	committed := s.CommitTurn("turn-1")
	if committed == nil {
		t.Fatal("expected committed turn")
	}
	if committed.Status != TranscriptionFinal {
		t.Fatalf("expected final status, got %s", committed.Status)
	}
	if s.CurrentTurn != nil {
		t.Fatal("current turn should be nil after commit")
	}
	if s.LastCommittedEvent != "turn-1" {
		t.Fatalf("expected last committed turn-1, got %s", s.LastCommittedEvent)
	}
	if s.StateVersion != 2 {
		t.Fatalf("expected version 2, got %d", s.StateVersion)
	}
}

func TestVoiceSessionCancelTurn(t *testing.T) {
	s := NewVoiceSession("sess-4", "conv-4", "char-4")
	s.BeginTurn("turn-1", "interrupted text")
	s.CancelTurn("turn-1")
	if s.CurrentTurn != nil {
		t.Fatal("current turn should be nil after cancel")
	}
	if len(s.CompletedTurns) != 1 {
		t.Fatalf("expected 1 completed turn, got %d", len(s.CompletedTurns))
	}
	if s.CompletedTurns[0].Status != TranscriptionCancel {
		t.Fatalf("expected cancel status, got %s", s.CompletedTurns[0].Status)
	}
}

func TestVoiceSessionNewTurnCancelsCurrent(t *testing.T) {
	s := NewVoiceSession("sess-5", "conv-5", "char-5")
	s.BeginTurn("turn-1", "hello")
	s.BeginTurn("turn-2", "hello world")
	if len(s.CompletedTurns) != 1 {
		t.Fatalf("expected 1 completed turn (cancelled), got %d", len(s.CompletedTurns))
	}
	if s.CompletedTurns[0].Status != TranscriptionCancel {
		t.Fatalf("turn-1 should be cancelled, got %s", s.CompletedTurns[0].Status)
	}
	if s.CurrentTurn.TurnID != "turn-2" {
		t.Fatalf("current turn should be turn-2, got %s", s.CurrentTurn.TurnID)
	}
}

func TestVoiceSessionEndSessionCancelsCurrent(t *testing.T) {
	s := NewVoiceSession("sess-6", "conv-6", "char-6")
	s.BeginTurn("turn-1", "unfinished")
	s.EndSession()
	if s.CurrentTurn != nil {
		t.Fatal("current turn should be nil after end session")
	}
	if !s.EndedAt.IsZero() {
		if len(s.CompletedTurns) != 1 {
			t.Fatalf("expected 1 completed turn, got %d", len(s.CompletedTurns))
		}
	}
}

func TestVoiceSessionGetFinalTurnsOnlyReturnsCommitted(t *testing.T) {
	s := NewVoiceSession("sess-7", "conv-7", "char-7")
	s.BeginTurn("turn-1", "cancelled text")
	s.CancelTurn("turn-1")
	s.BeginTurn("turn-2", "final text")
	s.CommitTurn("turn-2")
	s.BeginTurn("turn-3", "cancel this too")
	s.CancelTurn("turn-3")

	finals := s.GetFinalTurns()
	if len(finals) != 1 {
		t.Fatalf("expected 1 final turn, got %d", len(finals))
	}
	if finals[0].TurnID != "turn-2" {
		t.Fatalf("expected turn-2, got %s", finals[0].TurnID)
	}
}

func TestVoiceSessionUpdateTurnText(t *testing.T) {
	s := NewVoiceSession("sess-8", "conv-8", "char-8")
	s.BeginTurn("turn-1", "hello")
	s.UpdateTurnText("hello world")
	if s.CurrentTurn.Text != "hello world" {
		t.Fatalf("expected text to be updated, got %s", s.CurrentTurn.Text)
	}
}
