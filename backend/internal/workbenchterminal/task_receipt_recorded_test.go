package workbenchterminal

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordedTaskReceiptsDoNotInventLiveness(t *testing.T) {
	journal, err := NewTaskReceiptJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	receipt := TaskReceipt{SessionID: strings.Repeat("a", 40), TaskID: "task-a", ConversationID: "conv-a", WorkspaceURI: "amitia://workspace/@task/", StartedAt: time.Now().UTC(), State: "running", LatestSequence: 1}
	if err := journal.Record("space-a:device-a:login-a:conv-a", receipt); err != nil {
		t.Fatal(err)
	}
	rows, err := journal.ListRecorded("space-a:device-a:login-a:conv-a", "conv-a", "task-a")
	if err != nil || len(rows) != 1 || rows[0].State != "running" {
		t.Fatalf("recorded history changed status: %+v %v", rows, err)
	}
	recovered, err := journal.List("space-a:device-a:login-a:conv-a", "conv-a", "task-a", nil)
	if err != nil || len(recovered) != 1 || recovered[0].State != "interrupted" {
		t.Fatalf("live view did not resolve offline: %+v %v", recovered, err)
	}
	other, err := journal.ListRecorded("space-a:device-a:other-login:conv-a", "conv-a", "task-a")
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-session read: %+v %v", other, err)
	}
}
