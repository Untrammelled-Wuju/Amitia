package workbenchterminal

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskReceiptManagerCloseAndRevocation(t *testing.T) {
	for _, state := range []string{"closed", "revoked"} {
		t.Run(state, func(t *testing.T) {
			journal, err := NewTaskReceiptJournal(filepath.Join(t.TempDir(), "records"))
			if err != nil {
				t.Fatal(err)
			}
			manager, _, oldID := sessionFixture(t)
			manager.EnableTaskReceipts(journal)
			s := manager.sessions[oldID]
			delete(manager.sessions, oldID)
			s.id = strings.Repeat("b", 40)
			s.taskID = "task-z"
			s.conversationID = "conv-z"
			s.workspaceURI = "amitia://workspace/@a/"
			s.receiptJournal = journal
			manager.sessions[s.id] = s
			s.recordTaskReceipt("running")
			if state == "closed" {
				err = manager.CloseSession("first", s.id)
			} else {
				err = manager.RevokeSession("first", s.id)
			}
			if err != nil {
				t.Fatal(err)
			}
			results, err := manager.TaskReceipts("first", "conv-z", "task-z")
			if err != nil || len(results) != 1 || results[0].State != state {
				t.Fatalf("unexpected receipts %+v %v", results, err)
			}
			if _, err = manager.Read("first", s.id, 0); !errors.Is(err, ErrNotFound) {
				t.Fatal("session still active")
			}
		})
	}
}

func TestTaskReceiptManagerInterruptedAfterRestart(t *testing.T) {
	journal, err := NewTaskReceiptJournal(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	s := TaskReceipt{SessionID: strings.Repeat("c", 40), TaskID: "task-d", ConversationID: "conv-d", WorkspaceURI: "amitia://workspace/@a/", Mode: "pty", StartedAt: time.Now().UTC(), State: "running"}
	if err = journal.Record("principal", s); err != nil {
		t.Fatal(err)
	}
	newManager := NewManager()
	defer newManager.Close()
	newManager.EnableTaskReceipts(journal)
	receipts, err := newManager.TaskReceipts("principal", "conv-d", "task-d")
	if err != nil || len(receipts) != 1 || receipts[0].State != "interrupted" {
		t.Fatalf("restart %+v %v", receipts, err)
	}
}
