package workbenchterminal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func receiptFixture(t *testing.T) (*TaskReceiptJournal, TaskReceipt) {
	t.Helper()
	store, err := NewTaskReceiptJournal(filepath.Join(t.TempDir(), "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	return store, TaskReceipt{SessionID: strings.Repeat("a", 40), TaskID: "task-1", ConversationID: "conv-1", WorkspaceURI: "amitia://workspace/@repo/", Mode: "pty", StartedAt: time.Now().UTC(), State: "running", LatestSequence: 1}
}

func TestTaskReceiptPersistenceAndInterruptedRestart(t *testing.T) {
	store, r := receiptFixture(t)
	if err := store.Record("actor-1", r); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewTaskReceiptJournal(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	interrupted, err := reopened.List("actor-1", "conv-1", "task-1", nil)
	if err != nil || len(interrupted) != 1 || interrupted[0].State != "interrupted" || interrupted[0].EvidenceKind != "terminal-lifecycle-metadata-not-test-proof" {
		t.Fatalf("restart result %+v %v", interrupted, err)
	}
	running, err := reopened.List("actor-1", "conv-1", "task-1", map[string]bool{r.SessionID: true})
	if err != nil || len(running) != 1 || running[0].State != "running" {
		t.Fatalf("active result %+v %v", running, err)
	}
}

func TestTaskReceiptScopeAndNoRawContent(t *testing.T) {
	store, r := receiptFixture(t)
	if err := store.Record("actor-secret", r); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ owner, conv, task string }{{"other", "conv-1", "task-1"}, {"actor-secret", "conv-2", "task-1"}, {"actor-secret", "conv-1", "task-2"}} {
		result, err := store.List(item.owner, item.conv, item.task, nil)
		if err != nil || len(result) != 0 {
			t.Fatalf("leaked receipt %+v %v", result, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(store.dir, r.SessionID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "actor-secret") || strings.Contains(string(raw), "input") || strings.Contains(string(raw), "data\"") {
		t.Fatal("unexpected content in persisted record")
	}
	info, _ := os.Stat(filepath.Join(store.dir, r.SessionID+".json"))
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("permissive file mode %v", info.Mode())
	}
}

func TestTaskReceiptTerminalFinalityAndConcurrentUpdates(t *testing.T) {
	store, r := receiptFixture(t)
	if err := store.Record("owner", r); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			copy := r
			copy.LatestSequence = uint64(i + 2)
			_ = store.Record("owner", copy)
		}(i)
	}
	wg.Wait()
	code := 0
	r.State = "exited"
	r.ExitCode = &code
	r.LatestSequence = 100
	if err := store.Record("owner", r); err != nil {
		t.Fatal(err)
	}
	r.State = "running"
	if err := store.Record("owner", r); err != nil {
		t.Fatal(err)
	}
	result, err := store.List("owner", "conv-1", "task-1", nil)
	if err != nil || len(result) != 1 || result[0].State != "exited" || result[0].ExitCode == nil || *result[0].ExitCode != 0 {
		t.Fatalf("final state %+v %v", result, err)
	}
	other := r
	other.TaskID = "task-2"
	if err := store.Record("owner", other); !errors.Is(err, ErrTaskBinding) {
		t.Fatalf("identity replacement allowed: %v", err)
	}
}

func TestTaskReceiptRejectsMalformedAndSymlink(t *testing.T) {
	store, r := receiptFixture(t)
	r.SessionID = "../escape"
	if err := store.Record("owner", r); !errors.Is(err, ErrInvalid) {
		t.Fatalf("path accepted %v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(store.dir, link); err == nil {
		if _, err := NewTaskReceiptJournal(link); !errors.Is(err, ErrInvalid) {
			t.Fatalf("symlink directory accepted: %v", err)
		}
	}
	r.SessionID = strings.Repeat("a", 40)
	file := filepath.Join(store.dir, r.SessionID+".json")
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), file); err == nil {
		if err := store.Record("owner", r); !errors.Is(err, ErrInvalid) {
			t.Fatalf("symlink receipt accepted: %v", err)
		}
	}
}

func TestTaskReceiptPrunesOldAndCaps(t *testing.T) {
	store, r := receiptFixture(t)
	for i := 0; i < maxStoredTaskReceipts+10; i++ {
		r.SessionID = strings.Repeat("a", 34) + strings.ToLower(strings.TrimPrefix(strings.ToUpper(formatHex6(i)), "0X"))
		if err := store.Record("owner", r); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Prune(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.dir)
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	if err != nil || count > maxStoredTaskReceipts {
		t.Fatalf("not bounded: %d %v", len(entries), err)
	}
}
func formatHex6(n int) string {
	const chars = "0123456789abcdef"
	var out [6]byte
	for i := 5; i >= 0; i-- {
		out[i] = chars[n&15]
		n >>= 4
	}
	return string(out[:])
}
